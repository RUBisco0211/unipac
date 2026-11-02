package main

import (
	"context"
	"sync"
)

// maxConcurrentActions bounds how many package commands run at once. It is a
// resource bound, not a package-manager rule: a large selection must not start
// hundreds of compiler or download processes. In practice the per-manager slot
// below is the tighter limit.
const maxConcurrentActions = 8

// actionLimiter admits at most maxConcurrentActions package actions at once,
// and never two for the same manager. Different packages do not make two
// commands independent: Homebrew serializes on a global lock, and npm and pip
// write the same global prefix, so concurrent actions for one manager fail
// intermittently rather than run faster.
type actionLimiter struct {
	slots chan struct{}
	mu    sync.Mutex
	// managers holds one slot per manager ID. Managers are few, so entries are
	// kept for the life of the process.
	managers map[string]chan struct{}
}

func newActionLimiter(limit int) *actionLimiter {
	return &actionLimiter{
		slots:    make(chan struct{}, limit),
		managers: make(map[string]chan struct{}),
	}
}

// acquire waits for the manager's own slot before the global slot. A batch for
// one manager therefore queues on that manager without consuming the global
// capacity that other managers need. The returned function releases both.
// Waiting stops when ctx is cancelled, which is what a quit does.
func (l *actionLimiter) acquire(ctx context.Context, managerID string) (func(), error) {
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

func (l *actionLimiter) managerSlot(managerID string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	slot, ok := l.managers[managerID]
	if !ok {
		slot = make(chan struct{}, 1)
		l.managers[managerID] = slot
	}
	return slot
}
