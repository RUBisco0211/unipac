package tasks

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/jmoiron/sqlx"
)

func target(name string) Target {
	return Target{ManagerID: "brew", ManagerName: "Homebrew", Name: name, Version: "1.0"}
}

func TestTaskRecordsCommandOutputAndOutcome(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this machine")
	}
	m := NewManager()
	task := m.Begin(context.Background(), KindUpdate, target("git"))
	task.Start()
	if _, err := util.Run(task.Context(), sh, []string{"-c", "echo upgrading"}, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	task.Finish(false, "brew failed")

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("listed %d tasks, want 1", len(list))
	}
	got := list[0]
	if got.ID == "" || got.Kind != KindUpdate || got.State != StateFailed {
		t.Fatalf("task = %+v, want a failed update with an ID", got)
	}
	if got.ManagerID != "brew" || got.ManagerName != "Homebrew" || got.Package != "git" {
		t.Fatalf("target = %+v", got)
	}
	if !strings.Contains(got.Command, "echo upgrading") {
		t.Errorf("command = %q, want the command line", got.Command)
	}
	if got.ExecPath != sh {
		t.Errorf("exec path = %q, want %q", got.ExecPath, sh)
	}
	if output := m.Output(got.ID); len(output.Lines) != 1 || output.Lines[0].Text != "upgrading" {
		t.Fatalf("output = %+v", output)
	}
	if got.Message != "brew failed" || got.FinishedAt.IsZero() {
		t.Fatalf("outcome = %q at %v", got.Message, got.FinishedAt)
	}
}

func TestListOrdersRunningThenQueuedThenFinished(t *testing.T) {
	m := NewManager()
	queued := m.Begin(context.Background(), KindInstall, target("queued"))
	running := m.Begin(context.Background(), KindUpdate, target("running"))
	running.Start()
	finished := m.Begin(context.Background(), KindUninstall, target("finished"))
	finished.Start()
	finished.Finish(true, "")

	list := m.List()
	if len(list) != 3 {
		t.Fatalf("listed %d tasks", len(list))
	}
	order := []string{list[0].ID, list[1].ID, list[2].ID}
	want := []string{idString(running), idString(queued), idString(finished)}
	if !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func idString(task *Task) string {
	return strconv.FormatInt(task.id, 10)
}

func TestStopAllFailsUnfinishedTasks(t *testing.T) {
	m := NewManager()
	queued := m.Begin(context.Background(), KindInstall, target("queued"))
	running := m.Begin(context.Background(), KindInstall, target("running"))
	running.Start()
	done := m.Begin(context.Background(), KindInstall, target("done"))
	done.Start()
	done.Finish(true, "")

	m.StopAll(InterruptedMessage)

	if queued.Context().Err() == nil {
		t.Error("a stopped task kept a live context")
	}
	states := map[string]State{}
	for _, task := range m.List() {
		states[task.Package] = task.State
	}
	if states["queued"] != StateCancelled || states["running"] != StateCancelled {
		t.Fatalf("unfinished states = %v, want cancelled", states)
	}
	if states["done"] != StateSucceeded {
		t.Fatalf("a finished task changed: %v", states)
	}
	if m.PendingCount() != 0 {
		t.Fatalf("pending count after stop = %d", m.PendingCount())
	}
}

// The command, target, state, times and console output survive a restart; a
// task left running by the old process is cancelled.
func TestTasksPersistAcrossRestart(t *testing.T) {
	oldRoot := util.AppRoot
	util.AppRoot = t.TempDir()
	defer func() { util.AppRoot = oldRoot }()
	if Default != nil {
		Default.Close()
	}

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	finished := Default.Begin(ctx, KindInstall, Target{
		ManagerID: "brew", ManagerName: "Homebrew", Name: "persisted", Version: "2.0",
	})
	finished.Start()
	finished.report(util.Event{
		Kind:    util.EventCommand,
		Command: "brew install --cask persisted",
		Path:    "/opt/homebrew/bin/brew",
	})
	finished.report(util.Event{Kind: util.EventOutput, Text: "==> Downloading"})
	finished.report(util.Event{Kind: util.EventOutput, Text: "installed"})
	finished.Finish(true, "Installed")
	// Simulate a crash while another task is still running: do not finish it,
	// and close without StopAll.
	left := Default.Begin(ctx, KindUpdate, target("left-running"))
	left.Start()
	Default.Close()

	if err := Init(ctx); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	defer Default.Close()

	list := Default.List()
	if len(list) != 2 {
		t.Fatalf("loaded %d tasks, want 2", len(list))
	}
	byName := map[string]Snapshot{}
	for _, task := range list {
		byName[task.Package] = task
	}
	done := byName["persisted"]
	if done.State != StateSucceeded || done.StartedAt.IsZero() || done.FinishedAt.IsZero() {
		t.Fatalf("finished task after reload = %+v", done)
	}
	if done.Version != "2.0" {
		t.Errorf("target after reload = %+v", done)
	}
	if done.Command != "brew install --cask persisted" || done.ExecPath != "/opt/homebrew/bin/brew" {
		t.Errorf("command after reload = %q at %q", done.Command, done.ExecPath)
	}
	if done.Message != "Installed" {
		t.Errorf("message after reload = %q", done.Message)
	}
	if done.NewVersion != "" {
		t.Errorf("install target new version = %q", done.NewVersion)
	}
	output := Default.Output(done.ID)
	if len(output.Lines) != 2 || output.Lines[0].Text != "==> Downloading" {
		t.Fatalf("persisted output = %+v", output)
	}
	interrupted := byName["left-running"]
	if interrupted.State != StateCancelled || interrupted.Message != InterruptedMessage {
		t.Fatalf("unfinished task after reload = %+v, want cancelled", interrupted)
	}
}

// Long output keeps its latest lines.
func TestOutputIsBounded(t *testing.T) {
	m := NewManager()
	task := m.Begin(context.Background(), KindInstall, target("x"))
	task.Start()
	for i := 0; i < maxOutput+5; i++ {
		task.report(util.Event{Kind: util.EventOutput, Text: "line " + strconv.Itoa(i)})
	}
	got := m.List()[0]
	output := m.Output(got.ID)
	if len(output.Lines) != maxOutput {
		t.Fatalf("kept %d lines, want %d", len(output.Lines), maxOutput)
	}
	if first := output.Lines[0].Text; first != "line 5" {
		t.Fatalf("oldest kept line = %q, want line 5", first)
	}
	if last := output.Lines[len(output.Lines)-1].Text; last != "line "+strconv.Itoa(maxOutput+4) {
		t.Fatalf("newest kept line = %q", last)
	}
	if !got.Truncated {
		t.Fatal("truncation was not reported")
	}
}

func TestCancelStopsAndKeepsTheTask(t *testing.T) {
	m := NewManager()
	running := m.Begin(context.Background(), KindUpdate, Target{
		ManagerID: "brew", ManagerName: "Homebrew", Name: "git", Version: "2.0", NewVersion: "2.1",
	})
	running.Start()
	if err := m.Cancel(idString(running)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if running.Context().Err() == nil {
		t.Error("a cancelled task kept a live context")
	}
	got := m.List()[0]
	if got.State != StateCancelled || got.Message != CancelledMessage {
		t.Fatalf("cancelled task = %+v", got)
	}
	if got.Version != "2.0" || got.NewVersion != "2.1" {
		t.Fatalf("versions after cancel = %q -> %q", got.Version, got.NewVersion)
	}
	if m.PendingCount() != 0 {
		t.Fatalf("pending count = %d", m.PendingCount())
	}
	if err := m.Cancel(idString(running)); err == nil {
		t.Fatal("cancelling a finished task succeeded")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	m := NewManager()
	m.Close()
	m.Close()
}

func TestCancelReturnsSentinels(t *testing.T) {
	m := NewManager()
	if err := m.Cancel("not-an-id"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("invalid id error = %v, want ErrTaskNotFound", err)
	}
	task := m.Begin(context.Background(), KindInstall, target("x"))
	if err := m.Cancel(idString(task)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := m.Cancel(idString(task)); !errors.Is(err, ErrTaskNotRunning) {
		t.Fatalf("finished task error = %v, want ErrTaskNotRunning", err)
	}
}

func TestCancelQueuedTaskPreventsStart(t *testing.T) {
	m := NewManager()
	queued := m.Begin(context.Background(), KindInstall, target("queued"))
	if err := m.Cancel(idString(queued)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if queued.Start() {
		t.Fatal("a cancelled task started")
	}
	if got := m.List()[0]; got.State != StateCancelled {
		t.Fatalf("state = %v", got.State)
	}
}

// The task table existed before version pairs and stored output; startup must
// add the new columns rather than fail on an older cache.
func TestMigratesOldTaskTable(t *testing.T) {
	oldRoot := util.AppRoot
	util.AppRoot = t.TempDir()
	defer func() { util.AppRoot = oldRoot }()
	if Default != nil {
		Default.Close()
	}

	path, err := util.SafeJoin(util.AppRoot, "cache/tasks.db")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlx.Open(sqliteDriver, sqliteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE package_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kind TEXT NOT NULL, state TEXT NOT NULL,
		manager_id TEXT NOT NULL, manager_name TEXT NOT NULL,
		package TEXT NOT NULL, version TEXT NOT NULL DEFAULT '',
		is_gui INTEGER NOT NULL DEFAULT 0,
		command TEXT NOT NULL DEFAULT '', exec_path TEXT NOT NULL DEFAULT '',
		message TEXT NOT NULL DEFAULT '',
		started_at INTEGER NOT NULL, finished_at INTEGER NOT NULL DEFAULT 0)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO package_tasks
		(kind, state, manager_id, manager_name, package, version, started_at)
		VALUES ('install', 'succeeded', 'brew', 'Homebrew', 'old', '1.0', 1)`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := Init(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	defer Default.Close()
	list := Default.List()
	if len(list) != 1 || list[0].Package != "old" {
		t.Fatalf("loaded %+v", list)
	}
	if list[0].NewVersion != "" || list[0].Truncated {
		t.Fatalf("defaults after migration = %+v", list[0])
	}
}

// Clearing the history drops the tasks that have an outcome, in memory and in the
// store, and leaves what is still running with its record.
func TestClearHistoryKeepsRunningTasks(t *testing.T) {
	oldRoot := util.AppRoot
	util.AppRoot = t.TempDir()
	defer func() { util.AppRoot = oldRoot }()
	if Default != nil {
		Default.Close()
	}

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	finished := Default.Begin(ctx, KindInstall, target("finished"))
	finished.Start()
	finished.Finish(true, "Installed")
	running := Default.Begin(ctx, KindUpdate, target("running"))
	running.Start()

	if err := Default.ClearHistory(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	list := Default.List()
	if len(list) != 1 || list[0].Package != "running" {
		t.Fatalf("tasks after clearing = %+v, want the running one", list)
	}

	// And the store agrees: a reload shows the same one.
	Default.Close()
	if err := Init(ctx); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	defer Default.Close()
	reloaded := Default.List()
	if len(reloaded) != 1 || reloaded[0].Package != "running" {
		t.Fatalf("tasks after reloading = %+v, want the running one", reloaded)
	}
}

// A task that finishes while the clear is running must end up consistent: kept
// everywhere or gone everywhere. finishState records the outcome in memory
// before its store write is enqueued, so the narrow window between the two let
// a cleared task come back from the store on the next launch. The interleaving
// is timing-dependent, so the test hammers it: many rounds, each finishing a
// task around a clear and then reading the store.
func TestClearHistoryAgainstAFinishingTask(t *testing.T) {
	oldRoot := util.AppRoot
	util.AppRoot = t.TempDir()
	defer func() { util.AppRoot = oldRoot }()
	if Default != nil {
		Default.Close()
	}
	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer Default.Close()

	for round := range 200 {
		finishing := Default.Begin(ctx, KindInstall, target("hammer"))
		finishing.Start()
		done := make(chan struct{})
		go func() {
			defer close(done)
			finishing.Finish(true, "Installed")
		}()
		// The clear races the finish; either order must agree.
		err := Default.ClearHistory()
		<-done

		list := Default.List()
		inMemory := 0
		for _, task := range list {
			if task.Package == "hammer" && task.State != StateQueued && task.State != StateRunning {
				inMemory++
			}
		}
		if err != nil {
			t.Fatalf("round %d: clear: %v", round, err)
		}

		// Read the store the way a restart would.
		Default.Close()
		if err := Init(ctx); err != nil {
			t.Fatalf("round %d: re-init: %v", round, err)
		}
		reloaded := Default.List()
		inStore := 0
		for _, task := range reloaded {
			if task.Package == "hammer" {
				inStore++
			}
		}
		if inMemory != inStore {
			t.Fatalf("round %d: %d finished hammer in memory, %d in the store", round, inMemory, inStore)
		}
	}
}

// The clear drops a task whose outcome write has not landed yet: its row still
// reads queued in the store, and its later write must update nothing. This is
// the interleaving the snapshot exists for — finishState records the outcome in
// memory first, so a task finishing during a clear is dropped by ID from a row
// that still says queued.
func TestClearFinishedDropsAQueuedRowByID(t *testing.T) {
	oldRoot := util.AppRoot
	util.AppRoot = t.TempDir()
	defer func() { util.AppRoot = oldRoot }()
	if Default != nil {
		Default.Close()
	}

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer Default.Close()

	task := Default.Begin(ctx, KindInstall, target("in-flight"))
	task.Start()
	// The store row exists and reads queued; the outcome write has not run.
	id := taskID(task)

	submitted, err := Default.submitWait(func(s *store) error {
		// Simulate the snapshot's decision: the task is being dropped even
		// though its store row still reads queued.
		if err := s.clearFinished([]int64{id}); err != nil {
			return err
		}
		// The outcome write lands afterwards, as finishState's would.
		return s.finish(id, StateSucceeded, "Installed", time.Now())
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if !submitted {
		t.Fatal("no store to clear")
	}

	// The row is gone: the update matched nothing.
	var count int
	store, release := Default.acquireStore()
	defer release()
	if store != nil {
		if err := store.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM package_tasks WHERE id = ?`, id); err != nil {
			t.Fatalf("count: %v", err)
		}
	}
	if count != 0 {
		t.Fatalf("%d rows survive a clear that dropped them by ID", count)
	}
}

// taskID reads a task's store ID for the tests above.
func taskID(t *Task) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.id
}
