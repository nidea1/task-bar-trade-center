package inventory

import (
	"context"
	"errors"
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
	queue := NewRefreshQueue(func(_ context.Context, _ int) error {
		return rateErr
	}, func(err error) bool { return err == rateErr })
	queue.baseDelay = 0

	queue.Enqueue([]int{10})
	waitForQueue(t, queue)
	status := queue.Status()
	if status.BackoffUntil == "" || status.LastError == "" {
		t.Fatalf("status = %+v, want backoff and error", status)
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
