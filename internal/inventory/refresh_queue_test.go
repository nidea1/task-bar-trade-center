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
