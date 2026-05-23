package scheduler

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// Item is one bounded unit of scheduled work.
type Item struct {
	ID                string
	Priority          float64
	DeadlineAt        time.Time
	EnqueuedAt        time.Time
	ExpectedValue     float64
	ExpectedCost      float64
	ExpectedServiceMS int64
	Reason            string
}

// Queue is a bounded priority scheduler with admission eviction.
type Queue struct {
	mu     sync.Mutex
	items  []Item
	cap    int
	closed bool
	notify chan struct{}
}

// New creates a bounded priority queue.
func New(capacity int) *Queue {
	if capacity <= 0 {
		capacity = 1
	}
	q := &Queue{cap: capacity, notify: make(chan struct{}, 1)}
	return q
}

// Push inserts one item, evicting the lowest priority item if the queue is full and the new item is better.
func (q *Queue) Push(item Item) (evicted *Item, accepted bool, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, false, errors.New("queue closed")
	}
	if item.DeadlineAt.IsZero() || time.Now().UTC().After(item.DeadlineAt) {
		return nil, false, errors.New("stale work")
	}
	if q.cap == 0 {
		return nil, false, errors.New("queue disabled")
	}
	if len(q.items) >= q.cap {
		worst := q.items[len(q.items)-1]
		if !betterThan(item, worst) {
			return nil, false, errors.New("queue overflow")
		}
		ev := worst
		q.items = q.items[:len(q.items)-1]
		evicted = &ev
	}
	idx := sort.Search(len(q.items), func(i int) bool {
		return !betterThan(q.items[i], item)
	})
	q.items = append(q.items, Item{})
	copy(q.items[idx+1:], q.items[idx:])
	q.items[idx] = item
	accepted = true
	q.signal()
	return evicted, accepted, nil
}

// Pop returns the highest priority item, blocking until one exists or the queue closes.
func (q *Queue) Pop(ctx context.Context) (Item, bool, error) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			item := q.items[0]
			q.items[0] = Item{}
			q.items = q.items[1:]
			q.mu.Unlock()
			return item, true, nil
		}
		if q.closed {
			q.mu.Unlock()
			return Item{}, false, nil
		}
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return Item{}, false, ctx.Err()
		case <-q.notify:
		}
	}
}

// Len reports the number of queued items.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Cap reports the queue capacity.
func (q *Queue) Cap() int { return q.cap }

// OldestAge reports the age of the oldest queued item.
func (q *Queue) OldestAge(now time.Time) time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return 0
	}
	oldest := q.items[len(q.items)-1].EnqueuedAt
	if oldest.IsZero() {
		return 0
	}
	return now.UTC().Sub(oldest)
}

// Snapshot returns a copy of queued items.
func (q *Queue) Snapshot() []Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Item, len(q.items))
	copy(out, q.items)
	return out
}

// Close wakes blocked poppers and rejects new items.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.signal()
}

func betterThan(a, b Item) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	if !a.DeadlineAt.Equal(b.DeadlineAt) {
		if a.DeadlineAt.IsZero() {
			return false
		}
		if b.DeadlineAt.IsZero() {
			return true
		}
		return a.DeadlineAt.Before(b.DeadlineAt)
	}
	if !a.EnqueuedAt.Equal(b.EnqueuedAt) {
		return a.EnqueuedAt.Before(b.EnqueuedAt)
	}
	return a.ID < b.ID
}

func (q *Queue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}
