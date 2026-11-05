// Package command arbitrates package-manager commands. It bounds how many run
// at once, keeps one manager's tasks from overlapping, and keeps a cache scan
// from overlapping either.
package command

import (
	"context"
	"slices"
	"sync"
)

// MaxConcurrentTasks bounds how many package commands run at once. It is a
// resource bound, not a package-manager rule: a large selection must not start
// hundreds of compiler or download processes. In practice the per-manager slot
// below is the tighter limit.
const MaxConcurrentTasks = 8

// Limiter admits at most MaxConcurrentTasks tasks at once, and never two for the
// same manager. Different packages do not make two commands independent:
// Homebrew serializes on a global lock, and npm and pip write the same global
// prefix, so two tasks for one manager fail intermittently rather than finish
// sooner.
type Limiter struct {
	slots chan struct{}
	mu    sync.Mutex
	// managers holds one slot per manager ID. Managers are few, so entries are
	// kept for the life of the process.
	managers map[string]chan struct{}
}

// NewLimiter returns a limiter with the given global slot count.
func NewLimiter(limit int) *Limiter {
	return &Limiter{
		slots:    make(chan struct{}, limit),
		managers: make(map[string]chan struct{}),
	}
}

// Acquire waits for the manager's own slot before the global slot. A batch for
// one manager therefore queues on that manager without consuming the global
// capacity that other managers need. The returned function releases both.
// Waiting stops when ctx is cancelled, which is what a quit does.
func (l *Limiter) Acquire(ctx context.Context, managerID string) (func(), error) {
	manager := l.managerSlot(managerID)
	select {
	case manager <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case l.slots <- struct{}{}:
	case <-ctx.Done():
		<-manager
		return nil, ctx.Err()
	}

	return func() {
		<-manager
		<-l.slots
	}, nil
}

// AcquireAll holds every manager slot for a scan, in ID order so two
// scans cannot deadlock. An action waits until the scan finishes, and the scan
// waits for actions already running, so a package-manager command and a cache
// scan never overlap.
func (l *Limiter) AcquireAll(ctx context.Context, managerIDs []string) (func(), error) {
	ids := slices.Clone(managerIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	slots := make([]chan struct{}, 0, len(ids))
	for _, id := range ids {
		slot := l.managerSlot(id)
		select {
		case slot <- struct{}{}:
			slots = append(slots, slot)
		case <-ctx.Done():
			for i := len(slots) - 1; i >= 0; i-- {
				<-slots[i]
			}
			return nil, ctx.Err()
		}
	}
	return func() {
		for i := len(slots) - 1; i >= 0; i-- {
			<-slots[i]
		}
	}, nil
}

func (l *Limiter) managerSlot(managerID string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	slot, ok := l.managers[managerID]
	if !ok {
		slot = make(chan struct{}, 1)
		l.managers[managerID] = slot
	}
	return slot
}
