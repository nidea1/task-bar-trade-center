package inventory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRefreshQueueProcessesEachIDOnce(t *testing.T) {
	var fetched []int
	queue := NewRefreshQueue(func(_ context.Context, id int) error {
		fetched = append(fetched, id)
		return nil
	}, nil)
	queue.baseDelay = 0

	added := queue.Enqueue([]int{1, 2, 2, 3})
	if added != 3 {
		t.Fatalf("added = %d, want 3", added)
	}
	waitForQueue(t, queue)
	if len(fetched) != 3 {
		t.Fatalf("fetched = %+v, want 3 unique IDs", fetched)
	}
}

func TestRefreshQueueBacksOffOnRateLimit(t *testing.T) {
	rateErr := errors.New("status 429 too many requests")
	var attempts int
	recovered := make(chan struct{})
	queue := NewRefreshQueue(func(_ context.Context, _ int) error {
		attempts++
		if attempts == 1 {
			return rateErr
		}
		close(recovered)
		return nil
	}, func(err error) bool { return err == rateErr })
	queue.baseDelay = 0
	queue.backoffs = []time.Duration{100 * time.Millisecond}

	queue.Enqueue([]int{10})
	waitForQueue(t, queue)
	status := queue.Status()
	if status.BackoffUntil == "" || status.LastError == "" {
		t.Fatalf("status = %+v, want backoff and error", status)
	}
	select {
	case <-recovered:
	case <-time.After(2 * time.Second):
		t.Fatal("queue did not automatically resume after backoff")
	}
	waitForQueue(t, queue)
}

type timedRateLimit struct{ until time.Time }

func (err timedRateLimit) Error() string      { return "429" }
func (err timedRateLimit) RetryAt() time.Time { return err.until }

func TestRefreshQueueRetainsNewItemsDuringSharedCooldown(t *testing.T) {
	backoff := make(chan time.Time, 1)
	fetched := make(chan int, 3)
	var attempts int
	queue := NewRefreshQueue(func(_ context.Context, id int) error {
		attempts++
		if attempts == 1 {
			return fmt.Errorf("listing: %w", timedRateLimit{time.Now().Add(100 * time.Millisecond)})
		}
		fetched <- id
		return nil
	}, func(err error) bool { var limit timedRateLimit; return errors.As(err, &limit) })
	queue.baseDelay = 0
	queue.backoffs = []time.Duration{time.Hour}
	queue.OnBackoff = func(_ int, _ error, _ int, until time.Time) { backoff <- until }
	queue.Enqueue([]int{1})
	deadline := <-backoff
	if added := queue.Enqueue([]int{2}); added != 1 {
		t.Fatalf("added=%d during cooldown, want 1", added)
	}
	queue.TriggerBackoff(2, timedRateLimit{deadline})
	if !queue.BackoffUntil().Equal(deadline) {
		t.Fatal("duplicate cooldown extended the deadline")
	}
	for _, want := range []int{1, 2} {
		select {
		case got := <-fetched:
			if got != want {
				t.Fatalf("fetched=%d want=%d", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("queue failed to resume item %d using shared deadline", want)
		}
	}
	waitForQueue(t, queue)
	if queue.Status().BackoffUntil != "" {
		t.Fatal("expired backoff was not cleared")
	}
}

func TestRefreshQueueReleasesPendingStorageWhenEmpty(t *testing.T) {
	queue := NewRefreshQueue(func(_ context.Context, _ int) error {
		return nil
	}, nil)
	queue.baseDelay = 0

	ids := make([]int, 1000)
	for i := range ids {
		ids[i] = i + 1
	}
	queue.Enqueue(ids)
	waitForQueue(t, queue)

	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.pending != nil {
		t.Fatalf("pending = len %d cap %d, want nil after queue drains", len(queue.pending), cap(queue.pending))
	}
	if len(queue.seen) != 0 {
		t.Fatalf("seen = %d entries, want empty", len(queue.seen))
	}
}

func TestRefreshQueueStatusEstimatesRemainingSeconds(t *testing.T) {
	queue := NewRefreshQueue(func(_ context.Context, _ int) error {
		return nil
	}, nil)
	queue.status = RefreshStatus{Refreshing: true, Completed: 2, Queued: 4}
	queue.lastStartedAt = time.Now().Add(-10 * time.Second)

	status := queue.Status()
	if status.EstimatedRemainingSeconds < 19 || status.EstimatedRemainingSeconds > 22 {
		t.Fatalf("estimated remaining seconds = %d, want about 20", status.EstimatedRemainingSeconds)
	}
}

func TestRefreshQueueEnqueueResetsCompletedForNewRun(t *testing.T) {
	block := make(chan struct{})
	queue := NewRefreshQueue(func(_ context.Context, _ int) error {
		<-block
		return nil
	}, nil)
	queue.status.Completed = 8
	queue.baseDelay = 0

	queue.Enqueue([]int{1})
	status := queue.Status()
	close(block)
	waitForQueue(t, queue)

	if status.Completed != 0 {
		t.Fatalf("completed at new run start = %d, want 0", status.Completed)
	}
}

func TestRefreshQueuePriorityMovesPendingAfterActive(t *testing.T) {
	started := make(chan int, 4)
	releaseFirst := make(chan struct{})
	var mu sync.Mutex
	var fetched []int

	queue := NewRefreshQueue(func(_ context.Context, id int) error {
		started <- id
		if id == 1 {
			<-releaseFirst
		}
		mu.Lock()
		fetched = append(fetched, id)
		mu.Unlock()
		return nil
	}, nil)
	queue.baseDelay = 0

	if added := queue.Enqueue([]int{1, 2, 3}); added != 3 {
		t.Fatalf("initial added = %d, want 3", added)
	}
	if got := <-started; got != 1 {
		t.Fatalf("first started = %d, want 1", got)
	}
	if added := queue.EnqueuePriority([]int{3, 4}); added != 1 {
		t.Fatalf("priority added = %d, want 1 new id", added)
	}
	close(releaseFirst)
	waitForQueue(t, queue)

	mu.Lock()
	defer mu.Unlock()
	want := []int{1, 3, 4, 2}
	if len(fetched) != len(want) {
		t.Fatalf("fetched = %+v, want %+v", fetched, want)
	}
	for i := range want {
		if fetched[i] != want[i] {
			t.Fatalf("fetched = %+v, want %+v", fetched, want)
		}
	}
}

func waitForQueue(t *testing.T, queue *RefreshQueue) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !queue.Status().Refreshing {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("queue did not finish: %+v", queue.Status())
}
