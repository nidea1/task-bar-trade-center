package inventory

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type FetchFunc func(context.Context, int) error
type RateLimitFunc func(error) bool

type RefreshQueue struct {
	mu             sync.Mutex
	fetch          FetchFunc
	rateLimited    RateLimitFunc
	baseDelay      time.Duration
	jitter         float64
	backoffs       []time.Duration
	status         RefreshStatus
	pending        []int
	seen           map[int]struct{}
	running        bool
	backoffStep    int
	backoffUntil   time.Time
	lastStartedAt  time.Time
	lastFinishedAt time.Time
	OnBackoff      func(itemID int, err error, queueRemaining int, backoffUntil time.Time)
}

func NewRefreshQueue(fetch FetchFunc, rateLimited RateLimitFunc) *RefreshQueue {
	return &RefreshQueue{
		fetch:       fetch,
		rateLimited: rateLimited,
		baseDelay:   3 * time.Second,
		jitter:      0.30,
		backoffs:    []time.Duration{15 * time.Minute, 30 * time.Minute, 60 * time.Minute},
		seen:        make(map[int]struct{}),
	}
}

func (queue *RefreshQueue) SetBaseDelay(delay time.Duration) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.baseDelay = delay
}

func (queue *RefreshQueue) Enqueue(ids []int) int {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	return queue.enqueueLocked(ids, false, time.Now())
}

func (queue *RefreshQueue) EnqueuePriority(ids []int) int {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	return queue.enqueueLocked(ids, true, time.Now())
}

func (queue *RefreshQueue) enqueueLocked(ids []int, priority bool, now time.Time) int {
	if !queue.backoffUntil.IsZero() && now.Before(queue.backoffUntil) {
		return 0
	}
	added := 0
	if priority {
		added = queue.enqueuePriorityLocked(ids)
	} else {
		for _, id := range ids {
			if id <= 0 {
				continue
			}
			if _, exists := queue.seen[id]; exists {
				continue
			}
			queue.seen[id] = struct{}{}
			queue.pending = append(queue.pending, id)
			added++
		}
	}
	queue.status.Queued = len(queue.pending)
	if !queue.running && len(queue.pending) > 0 {
		queue.running = true
		queue.status.Refreshing = true
		queue.status.Completed = 0
		queue.status.LastError = ""
		queue.lastStartedAt = now
		go queue.run()
	}
	return added
}

func (queue *RefreshQueue) enqueuePriorityLocked(ids []int) int {
	protected := 0
	if queue.running && len(queue.pending) > 0 {
		protected = 1
	}

	added := 0
	priorityIDs := make([]int, 0, len(ids))
	prioritySeen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := prioritySeen[id]; exists {
			continue
		}

		_, alreadySeen := queue.seen[id]
		removed := false
		for index := protected; index < len(queue.pending); index++ {
			if queue.pending[index] != id {
				continue
			}
			queue.pending = append(queue.pending[:index], queue.pending[index+1:]...)
			removed = true
			break
		}
		if alreadySeen && !removed {
			continue
		}
		if !alreadySeen {
			queue.seen[id] = struct{}{}
			added++
		}
		prioritySeen[id] = struct{}{}
		priorityIDs = append(priorityIDs, id)
	}

	if len(priorityIDs) > 0 {
		next := make([]int, 0, len(queue.pending)+len(priorityIDs))
		next = append(next, queue.pending[:protected]...)
		next = append(next, priorityIDs...)
		next = append(next, queue.pending[protected:]...)
		queue.pending = next
	}
	return added
}

func (queue *RefreshQueue) Status() RefreshStatus {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	status := queue.status
	if !queue.backoffUntil.IsZero() {
		status.BackoffUntil = queue.backoffUntil.Format(time.RFC3339)
	}
	if !queue.lastStartedAt.IsZero() {
		status.LastStartedAt = queue.lastStartedAt.Format(time.RFC3339)
	}
	if !queue.lastFinishedAt.IsZero() {
		status.LastFinishedAt = queue.lastFinishedAt.Format(time.RFC3339)
	}
	if status.Refreshing && status.Completed > 0 && status.Queued > 0 && !queue.lastStartedAt.IsZero() {
		remaining := estimatedRemainingDuration(time.Since(queue.lastStartedAt), status.Completed, status.Queued)
		if remaining > 0 {
			status.EstimatedRemainingSeconds = int((remaining + time.Second - 1) / time.Second)
		}
	}
	return status
}

func (queue *RefreshQueue) run() {
	for {
		id, ok := queue.next()
		if !ok {
			queue.finish("")
			return
		}
		if err := queue.fetch(context.Background(), id); err != nil {
			if queue.rateLimited != nil && queue.rateLimited(err) {
				queue.enterBackoff(id, err)
				return
			}
			queue.finish(err.Error())
			return
		}
		queue.markCompleted(id)
		time.Sleep(queue.delay())
	}
}

func (queue *RefreshQueue) next() (int, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.pending) == 0 {
		return 0, false
	}
	return queue.pending[0], true
}

func (queue *RefreshQueue) markCompleted(id int) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.pending) > 0 && queue.pending[0] == id {
		queue.pending = queue.pending[1:]
		if len(queue.pending) == 0 {
			queue.pending = nil
		}
	}
	delete(queue.seen, id)
	queue.status.Completed++
	queue.status.Queued = len(queue.pending)
	queue.status.LastError = ""
	queue.backoffStep = 0
}

func (queue *RefreshQueue) finish(lastError string) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.running = false
	queue.status.Refreshing = false
	queue.status.Queued = len(queue.pending)
	queue.status.LastError = lastError
	queue.lastFinishedAt = time.Now()
}

func (queue *RefreshQueue) enterBackoff(id int, err error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	delay := queue.backoffs[len(queue.backoffs)-1]
	if queue.backoffStep < len(queue.backoffs) {
		delay = queue.backoffs[queue.backoffStep]
		queue.backoffStep++
	}
	queue.running = false
	queue.status.Refreshing = false
	queue.backoffUntil = time.Now().Add(delay)
	queue.status.Queued = len(queue.pending)
	queue.status.LastError = err.Error()
	queue.lastFinishedAt = time.Now()

	if queue.OnBackoff != nil {
		backoffUntil := queue.backoffUntil
		queueRemaining := len(queue.pending)
		queue.mu.Unlock()
		queue.OnBackoff(id, err, queueRemaining, backoffUntil)
		queue.mu.Lock()
	}
}

func (queue *RefreshQueue) delay() time.Duration {
	if queue.jitter <= 0 {
		return queue.baseDelay
	}
	min := 1 - queue.jitter
	max := 1 + queue.jitter
	factor := min + rand.Float64()*(max-min)
	return time.Duration(float64(queue.baseDelay) * factor)
}

func estimatedRemainingDuration(elapsed time.Duration, completed int, queued int) time.Duration {
	if elapsed <= 0 || completed <= 0 || queued <= 0 {
		return 0
	}
	return time.Duration(float64(elapsed) / float64(completed) * float64(queued))
}

func (queue *RefreshQueue) BackoffUntil() time.Time {
	if queue == nil {
		return time.Time{}
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.backoffUntil
}

func (queue *RefreshQueue) TriggerBackoff(itemID int, err error) {
	if queue == nil {
		return
	}
	if queue.rateLimited != nil && queue.rateLimited(err) {
		queue.enterBackoff(itemID, err)
	}
}
