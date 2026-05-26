package scheduler

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrQueueClosed   = errors.New("queue closed")
	ErrQueueDisabled = errors.New("queue disabled")
	ErrQueueOverflow = errors.New("queue overflow")
	ErrStaleWork     = errors.New("stale work")
)

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

type Queue struct {
	mu     sync.Mutex
	items  []Item
	cap    int
	closed bool
	notify chan struct{}
}

func New(capacity int) *Queue {
	if capacity <= 0 {
		capacity = 1
	}
	return &Queue{cap: capacity, notify: make(chan struct{}, capacity)}
}

func (q *Queue) Push(item Item) (evicted *Item, accepted bool, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, false, ErrQueueClosed
	}
	if item.DeadlineAt.IsZero() || time.Now().UTC().After(item.DeadlineAt) {
		return nil, false, ErrStaleWork
	}
	if q.cap == 0 {
		return nil, false, ErrQueueDisabled
	}
	prevLen := len(q.items)
	if len(q.items) >= q.cap {
		worst := q.items[len(q.items)-1]
		if !betterThan(item, worst) {
			return nil, false, ErrQueueOverflow
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
	if len(q.items) > prevLen {
		q.notify <- struct{}{}
	}
	return evicted, accepted, nil
}

func (q *Queue) Pop(ctx context.Context) (Item, bool, error) {
	for {
		select {
		case <-ctx.Done():
			return Item{}, false, ctx.Err()
		case _, ok := <-q.notify:
			if !ok {
				return Item{}, false, nil
			}
		}
		q.mu.Lock()
		if len(q.items) > 0 {
			item := q.items[0]
			q.items[0] = Item{}
			q.items = q.items[1:]
			q.mu.Unlock()
			return item, true, nil
		}
		q.mu.Unlock()
	}
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *Queue) Cap() int { return q.cap }

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

func (q *Queue) Snapshot() []Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Item, len(q.items))
	copy(out, q.items)
	return out
}

func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	close(q.notify)
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
