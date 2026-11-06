// Package tasks records the package actions UniPac runs. One task is one
// operation on one package, and its command, state, times and console output
// survive a restart so the page can show what happened and retry a failure.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Kind is what a task does.
type Kind string

const (
	KindInstall   Kind = "install"
	KindUpdate    Kind = "update"
	KindUninstall Kind = "uninstall"
)

// State is where a task is in its life.
type State string

const (
	// StateQueued is a task waiting for a free action slot.
	StateQueued State = "queued"
	// StateRunning is a task whose command is running.
	StateRunning State = "running"
	// StateSucceeded is a task that finished without an error.
	StateSucceeded State = "succeeded"
	// StateFailed is a task whose command failed.
	StateFailed State = "failed"
	// StateCancelled is a task the user stopped, or one UniPac interrupted
	// when it quit before the task finished.
	StateCancelled State = "cancelled"
)

const (
	// InterruptedMessage is what a task records when UniPac closed before it
	// finished.
	InterruptedMessage = "Interrupted before completion"
	// CancelledMessage is what a task records when the user stops it.
	CancelledMessage = "Cancelled by user"
)

var (
	// ErrTaskNotFound reports an ID with no task behind it.
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskNotRunning reports a task that already finished or was cancelled.
	ErrTaskNotRunning = errors.New("task is not running")
)

// Target is the package a task operates on. It is kept with the task so a
// failed operation can be retried, even after a restart.
type Target struct {
	ManagerID   string
	ManagerName string
	Name        string
	// Version is the version before the operation: the installed version for
	// an update or uninstall, empty for an install.
	Version string
	// NewVersion is the version the operation installs or upgrades to.
	NewVersion string
}

// Line is one line of a task's console output.
type Line struct {
	Text string
}

// Output is a task's console, loaded on demand. A live task answers from
// memory; a task loaded from the store returns its persisted lines here.
type Output struct {
	Lines     []Line
	Truncated bool
	// Seq is the sequence of the last line in a live task's console. A stored
	// console reports 0; it has no further updates.
	Seq int
}

// Snapshot is a task as the interface shows it. It copies the task's state, so
// the page can read it on the UI thread while the action keeps writing.
type Snapshot struct {
	ID          string
	Kind        Kind
	State       State
	ManagerID   string
	ManagerName string
	Package     string
	Version     string
	NewVersion  string
	Command     string
	ExecPath    string
	StartedAt   time.Time
	FinishedAt  time.Time
	Message     string
	Truncated   bool
	// OutputSeq is the number of console lines the task has produced. The page
	// uses it to reload the selected console only when it changed.
	OutputSeq int
}

const (
	// maxOutput bounds the lines kept per task, in memory and in the store.
	// A package manager that prints more than this is pathological; the flag
	// tells the page the console is cut.
	maxOutput = 20000
	// maxCommands bounds the command lines kept for one task, in case an
	// adapter runs a command per package internally.
	maxCommands = 20
	// storedTasks is how many tasks the database keeps; older finished rows
	// are pruned at startup.
	storedTasks = 500
	// notifyEvery coalesces output bursts into one interface update.
	notifyEvery = 80 * time.Millisecond
)

// Manager keeps this session's tasks and their store. It is safe for
// concurrent use: actions append output from their own goroutine, the interface
// lists snapshots.
type Manager struct {
	mu     sync.Mutex
	ctx    context.Context
	store  *store
	next   int64
	tasks  []*Task
	notify func()
	// pending says a notification is already scheduled, so a burst of output
	// results in one repaint.
	pending bool
	// writes tracks store operations admitted before Close, so Close can wait
	// for the last console lines instead of closing the database under them.
	writes sync.WaitGroup
	// closing and closed make store admission and repeated Close calls safe.
	closing   bool
	closed    bool
	closeDone chan struct{}
	// queue serializes store writes off the command's goroutine, so a console
	// line does not wait for its own database transaction. queueMu serializes
	// senders with the close sentinel without holding mu on a full queue.
	queue      chan func(*store)
	queueMu    sync.Mutex
	writerDone chan struct{}
}

// Default is the app's task manager. UniPac runs one process, so the tasks are
// process-wide.
var Default = NewManager()

// NewManager builds an empty, memory-only task manager. Init replaces it with
// one backed by the store.
func NewManager() *Manager {
	return &Manager{ctx: context.Background(), closeDone: make(chan struct{})}
}

// Init opens the task store, loads the history, marks tasks left unfinished by
// a previous process cancelled, and installs the manager as Default.
func Init(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s, err := openStore(ctx)
	if err != nil {
		return err
	}
	if err := s.migrate(); err != nil {
		s.close()
		return err
	}
	// A process that starts is the only one that can be running tasks, so any
	// queued or running row is left over from a quit or a crash.
	if err := s.markInterrupted(InterruptedMessage, time.Now()); err != nil {
		s.close()
		return err
	}
	if err := s.prune(storedTasks); err != nil {
		slog.ErrorContext(ctx, "tasks: pruning history", "error", err)
	}
	rows, err := s.load(storedTasks)
	if err != nil {
		s.close()
		return err
	}

	m := NewManager()
	m.ctx = ctx
	m.store = s
	m.queue = make(chan func(*store), 256)
	m.writerDone = make(chan struct{})
	go m.writeLoop()
	m.tasks = make([]*Task, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		task := taskFromRow(rows[i], m)
		m.tasks = append(m.tasks, task)
		if task.id > m.next {
			m.next = task.id
		}
	}
	Default = m
	slog.InfoContext(ctx, "tasks: history loaded", "count", len(m.tasks))
	return nil
}

// Close releases the task store after the writes already admitted have
// finished. It is safe to call more than once.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		done := m.closeDone
		m.mu.Unlock()
		<-done
		return
	}
	m.closed = true
	m.closing = true
	s := m.store
	done := m.closeDone
	queue := m.queue
	writerDone := m.writerDone
	m.mu.Unlock()

	if queue != nil {
		m.queueMu.Lock()
		queue <- nil
		m.queueMu.Unlock()
	}
	if writerDone != nil {
		<-writerDone
	}
	m.writes.Wait()
	if s != nil {
		s.close()
	}
	m.mu.Lock()
	m.store = nil
	m.mu.Unlock()
	close(done)
}

// acquireStore admits one store operation. Once Close begins, no new
// operation is admitted; the returned release must run when the operation
// finishes. A nil store means memory-only mode or a closed manager.
func (m *Manager) acquireStore() (*store, func()) {
	m.mu.Lock()
	if m.store == nil || m.closing {
		m.mu.Unlock()
		return nil, func() {}
	}
	m.writes.Add(1)
	s := m.store
	m.mu.Unlock()
	return s, m.writes.Done
}

// submit queues a store write for the background writer. It holds the manager
// lock only until the queue accepts the work, so a full queue is the only
// backpressure the command's output goroutine sees.
func (m *Manager) submit(fn func(*store)) {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	m.mu.Lock()
	if m.store == nil || m.closing {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	m.queue <- fn
}

// submitWait queues a store write and waits for it, unlike submit. It reports
// whether there was a store to write to at all, and holds the queue while it
// waits so nothing can slip in between.
func (m *Manager) submitWait(fn func(*store) error) (bool, error) {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	m.mu.Lock()
	if m.store == nil || m.closing {
		m.mu.Unlock()
		return false, nil
	}
	m.mu.Unlock()

	done := make(chan error, 1)
	m.queue <- func(s *store) { done <- fn(s) }
	return true, <-done
}

func (m *Manager) writeLoop() {
	defer close(m.writerDone)
	for fn := range m.queue {
		if fn == nil {
			return
		}
		fn(m.store)
	}
}

// SetListener registers fn, called after tasks change. It runs off the UI
// thread, at most once per notifyEvery, so the interface posts one repaint for
// a burst of console output.
func (m *Manager) SetListener(fn func()) {
	m.mu.Lock()
	m.notify = fn
	m.mu.Unlock()
}

// Begin creates a queued task for one package and returns it. The task's
// Context carries the command reporter; hand it to the action.
func (m *Manager) Begin(ctx context.Context, kind Kind, target Target) *Task {
	store, done := m.acquireStore()
	if ctx == nil {
		ctx = m.ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	task := &Task{
		kind:       kind,
		target:     target,
		state:      StateQueued,
		started:    time.Now(),
		ctx:        ctx,
		cancel:     cancel,
		manager:    m,
		outputKept: true,
	}

	var id int64
	if store != nil {
		var err error
		id, err = store.insert(taskRow{
			Kind:        kind,
			State:       StateQueued,
			ManagerID:   target.ManagerID,
			ManagerName: target.ManagerName,
			Package:     target.Name,
			Version:     target.Version,
			NewVersion:  target.NewVersion,
			StartedAt:   task.started.UnixNano(),
		})
		if err != nil {
			slog.Error("tasks: saving a new task", "error", err)
		}
		done()
	}

	m.mu.Lock()
	if id > 0 {
		if id > m.next {
			m.next = id
		}
	} else {
		// A memory-only or failed store gets a negative ID, which a later row
		// from SQLite can never reuse.
		m.next++
		id = -m.next
	}
	task.id = id
	m.tasks = append(m.tasks, task)
	m.mu.Unlock()

	m.changed()
	return task
}

// ClearHistory drops the tasks that have an outcome, in memory and in the store,
// and leaves the queued and running ones alone. The listener runs, so a page
// showing the history reloads it.
//
// The queue's lock is held from the snapshot to the store clear. finishState
// records the outcome in memory before its store write is enqueued, so a task
// that finishes while the clear runs looks finished in memory but still queued
// in the store; deciding from the store's own state would keep its row and let
// its write land after the clear, which brings it back on the next launch. The
// snapshot is taken under the lock instead, and the store drops exactly those
// tasks by ID — a write that lands afterwards updates nothing.
func (m *Manager) ClearHistory() error {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()

	m.mu.Lock()
	drop := make([]int64, 0, len(m.tasks))
	kept := m.tasks[:0]
	for _, task := range m.tasks {
		if task.pending() {
			kept = append(kept, task)
		} else {
			drop = append(drop, task.id)
		}
	}
	m.tasks = kept
	m.mu.Unlock()

	if m.hasStore() {
		errc := make(chan error, 1)
		m.queue <- func(s *store) { errc <- s.clearFinished(drop) }
		if err := <-errc; err != nil {
			return err
		}
	}
	m.changed()
	return nil
}

// hasStore reports whether the manager writes to a store. The queue's lock
// must be held; it answers under it.
func (m *Manager) hasStore() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store != nil && !m.closing
}

// List returns every task, running ones first, then queued ones, then finished
// ones newest first.
func (m *Manager) List() []Snapshot {
	m.mu.Lock()
	list := slices.Clone(m.tasks)
	m.mu.Unlock()

	out := make([]Snapshot, len(list))
	for i, task := range list {
		out[i] = task.snapshot()
	}
	slices.SortStableFunc(out, func(a, b Snapshot) int {
		ra, rb := stateRank(a.State), stateRank(b.State)
		if ra != rb {
			return ra - rb
		}
		if ra <= 1 {
			// Running and queued tasks keep the order they were started in.
			return a.StartedAt.Compare(b.StartedAt)
		}
		if !a.FinishedAt.Equal(b.FinishedAt) {
			return b.FinishedAt.Compare(a.FinishedAt)
		}
		return b.StartedAt.Compare(a.StartedAt)
	})
	return out
}

// Output returns a task's console lines. A task this process ran answers from
// memory; a task loaded from the store reads its persisted lines.
func (m *Manager) Output(id string) Output {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Output{}
	}

	m.mu.Lock()
	var task *Task
	for _, candidate := range m.tasks {
		if candidate.id == n {
			task = candidate
			break
		}
	}
	m.mu.Unlock()
	if task == nil {
		return Output{}
	}

	task.mu.Lock()
	kept := task.outputKept
	lines := task.orderedOutput()
	truncated := task.truncated
	seq := task.nextSeq
	task.mu.Unlock()
	if kept {
		return Output{Lines: lines, Truncated: truncated, Seq: seq}
	}
	store, done := m.acquireStore()
	if store == nil {
		return Output{Truncated: truncated}
	}
	defer done()
	loaded, err := store.loadOutput(n)
	if err != nil {
		slog.Error("tasks: loading console output", "id", id, "error", err)
		return Output{Truncated: truncated}
	}
	return Output{Lines: loaded, Truncated: truncated}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func stateRank(state State) int {
	switch state {
	case StateRunning:
		return 0
	case StateQueued:
		return 1
	default:
		return 2
	}
}

// PendingCount reports how many tasks are queued or running. The window asks
// before it closes.
func (m *Manager) PendingCount() int {
	m.mu.Lock()
	list := slices.Clone(m.tasks)
	m.mu.Unlock()
	n := 0
	for _, task := range list {
		if task.pending() {
			n++
		}
	}
	return n
}

// StopAll marks every unfinished task cancelled, cancelling the running ones.
// Queued tasks have not started, so they are cancelled too: the operation did
// not complete.
func (m *Manager) StopAll(message string) {
	m.mu.Lock()
	list := slices.Clone(m.tasks)
	m.mu.Unlock()
	for _, task := range list {
		task.stop(message)
	}
	m.changed()
}

// Cancel stops one task: its context is cancelled, which kills the child
// process, and its state becomes StateCancelled.
func (m *Manager) Cancel(id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}
	m.mu.Lock()
	var task *Task
	for _, candidate := range m.tasks {
		if candidate.id == n {
			task = candidate
			break
		}
	}
	m.mu.Unlock()
	if task == nil {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}
	return task.cancelByUser()
}

// changed schedules one listener call. While one is pending, further changes
// ride along with it.
func (m *Manager) changed() {
	m.mu.Lock()
	if m.notify == nil || m.pending {
		m.mu.Unlock()
		return
	}
	m.pending = true
	m.mu.Unlock()

	time.AfterFunc(notifyEvery, func() {
		m.mu.Lock()
		m.pending = false
		fn := m.notify
		m.mu.Unlock()
		if fn != nil {
			fn()
		}
	})
}

func (m *Manager) storeState(id int64, state State) {
	m.submit(func(s *store) {
		if err := s.markRunning(id); err != nil {
			slog.Error("tasks: saving task state", "id", id, "state", state, "error", err)
		}
	})
}

func (m *Manager) storeCommand(id int64, command, execPath string) {
	m.submit(func(s *store) {
		if err := s.setCommand(id, command, execPath); err != nil {
			slog.Error("tasks: saving task command", "id", id, "error", err)
		}
	})
}

func (m *Manager) storeFinish(id int64, state State, message string, at time.Time) {
	m.submit(func(s *store) {
		if err := s.finish(id, state, message, at); err != nil {
			slog.Error("tasks: saving task outcome", "id", id, "error", err)
		}
	})
}

func (m *Manager) storeOutput(id int64, seq int, text string) {
	m.submit(func(s *store) {
		if err := s.appendOutput(id, seq, text); err != nil {
			slog.Error("tasks: saving console output", "id", id, "seq", seq, "error", err)
		}
	})
}

func (m *Manager) storeTruncated(id int64) {
	m.submit(func(s *store) {
		if err := s.setTruncated(id); err != nil {
			slog.Error("tasks: saving console truncation", "id", id, "error", err)
		}
	})
}

// Task is one package operation.
type Task struct {
	mu         sync.Mutex
	id         int64
	kind       Kind
	target     Target
	state      State
	started    time.Time
	finished   time.Time
	message    string
	commands   []string
	execPath   string
	output     []Line
	outputAt   int
	nextSeq    int
	truncated  bool
	outputKept bool
	ctx        context.Context
	cancel     context.CancelFunc
	manager    *Manager
}

// Context returns a context whose commands report their invocation and output
// to this task.
func (t *Task) Context() context.Context {
	t.mu.Lock()
	ctx := t.ctx
	t.mu.Unlock()
	return util.WithReporter(ctx, t.report)
}

// Start moves a queued task to running, now that a slot is free. It reports
// false when the task was cancelled first.
func (t *Task) Start() bool {
	t.mu.Lock()
	if t.state != StateQueued {
		t.mu.Unlock()
		return false
	}
	t.state = StateRunning
	t.mu.Unlock()
	t.manager.storeState(t.id, StateRunning)
	t.manager.changed()
	return true
}

// Cancelled reports whether the user (or a quit) stopped this task.
func (t *Task) Cancelled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == StateCancelled
}

// Finish records the outcome. ok false marks the task failed; message is the
// adapter's result text or the error. The first outcome wins.
func (t *Task) Finish(ok bool, message string) {
	t.complete(ok, message)
}

// stop marks an unfinished task cancelled because the app is closing.
func (t *Task) stop(message string) {
	t.mu.Lock()
	unfinished := t.state == StateQueued || t.state == StateRunning
	t.mu.Unlock()
	if !unfinished {
		return
	}
	t.finishState(StateCancelled, message)
}

// cancelByUser stops a running or queued task at the user's request.
func (t *Task) cancelByUser() error {
	t.mu.Lock()
	unfinished := t.state == StateQueued || t.state == StateRunning
	t.mu.Unlock()
	if !unfinished {
		return ErrTaskNotRunning
	}
	t.finishState(StateCancelled, CancelledMessage)
	return nil
}

func (t *Task) complete(ok bool, message string) {
	state := StateSucceeded
	if !ok {
		state = StateFailed
	}
	t.finishState(state, message)
}

func (t *Task) finishState(state State, message string) {
	t.mu.Lock()
	if t.state == StateSucceeded || t.state == StateFailed || t.state == StateCancelled {
		t.mu.Unlock()
		return
	}
	t.state = state
	t.message = message
	t.finished = time.Now()
	cancel := t.cancel
	t.cancel = nil
	at := t.finished
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	t.manager.storeFinish(t.id, state, message, at)
	t.manager.changed()
}

// report receives the command and output events the action's commands emit.
func (t *Task) report(ev util.Event) {
	switch ev.Kind {
	case util.EventCommand:
		t.mu.Lock()
		if len(t.commands) < maxCommands {
			t.commands = append(t.commands, ev.Command)
		}
		if t.execPath == "" {
			t.execPath = ev.Path
		}
		command, execPath := strings.Join(t.commands, "\n"), t.execPath
		t.mu.Unlock()
		t.manager.storeCommand(t.id, command, execPath)
	case util.EventOutput:
		t.mu.Lock()
		var truncate bool
		line := Line{Text: ev.Text}
		if len(t.output) < maxOutput {
			t.output = append(t.output, line)
		} else {
			t.output[t.outputAt] = line
			t.outputAt = (t.outputAt + 1) % maxOutput
			if !t.truncated {
				t.truncated = true
				truncate = true
			}
		}
		seq := t.nextSeq
		t.nextSeq++
		t.mu.Unlock()
		if truncate {
			t.manager.storeTruncated(t.id)
		}
		t.manager.storeOutput(t.id, seq, ev.Text)
		t.manager.changed()
	}
}

// orderedOutput returns the console in line order. output is a ring once it is
// full, so appending never copies the whole buffer.
func (t *Task) orderedOutput() []Line {
	if len(t.output) < maxOutput || t.outputAt == 0 {
		return slices.Clone(t.output)
	}
	lines := make([]Line, 0, len(t.output))
	lines = append(lines, t.output[t.outputAt:]...)
	lines = append(lines, t.output[:t.outputAt]...)
	return lines
}

func (t *Task) pending() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == StateQueued || t.state == StateRunning
}

func (t *Task) snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	snapshot := Snapshot{
		ID:          strconv.FormatInt(t.id, 10),
		Kind:        t.kind,
		State:       t.state,
		ManagerID:   t.target.ManagerID,
		ManagerName: t.target.ManagerName,
		Package:     t.target.Name,
		Version:     t.target.Version,
		NewVersion:  t.target.NewVersion,
		Command:     strings.Join(t.commands, "\n"),
		ExecPath:    t.execPath,
		StartedAt:   t.started,
		FinishedAt:  t.finished,
		Message:     t.message,
		Truncated:   t.truncated,
		OutputSeq:   t.nextSeq,
	}
	return snapshot
}
