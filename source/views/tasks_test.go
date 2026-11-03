package views

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
)

func tasksFixture() []tasks.Snapshot {
	base := time.Date(2026, 11, 2, 14, 3, 22, 0, time.UTC)
	return []tasks.Snapshot{
		{
			ID:          "3",
			Kind:        tasks.KindUpdate,
			State:       tasks.StateRunning,
			ManagerID:   "brew",
			ManagerName: "Homebrew",
			Package:     "typescript",
			Version:     "14.1.1",
			NewVersion:  "15.0.0",
			StartedAt:   base,
			Command:     "brew upgrade typescript",
			ExecPath:    "/opt/homebrew/bin/brew",
			OutputSeq:   2,
		},
		{
			ID:          "2",
			Kind:        tasks.KindInstall,
			State:       tasks.StateQueued,
			ManagerID:   "npm",
			ManagerName: "NPM",
			Package:     "eslint",
			NewVersion:  "9.9.0",
			StartedAt:   base.Add(-time.Second),
		},
		{
			ID:          "1",
			Kind:        tasks.KindUninstall,
			State:       tasks.StateFailed,
			ManagerID:   "pip",
			ManagerName: "Pip",
			Package:     "requests",
			Version:     "2.31.0",
			StartedAt:   base.Add(-time.Minute),
			FinishedAt:  base,
			Message:     "pip failed",
			Command:     "pip uninstall requests",
			ExecPath:    "/usr/bin/pip3",
		},
	}
}

func tasksViewFixture() *tasksView {
	v := &tasksView{
		data:          fixture(),
		selectedIndex: -1,
		splitPercent:  64,
		outputFor:     "3",
		outputSeq:     2,
		outputReady:   true,
		output: tasks.Output{Lines: []tasks.Line{
			{Text: "\x1b[32m==>\x1b[0m Upgrading typescript"},
			{Text: "==> Pouring typescript"},
		}},
	}
	v.apply(tasksFixture())
	return v
}

func TestTasksViewListsOneTaskPerPackage(t *testing.T) {
	tt := render(t, tasksViewFixture().view, 1000, 560)

	for _, want := range []string{
		locales.TableOperation(),
		locales.TablePackage(),
		locales.TableVersion(),
		locales.TableManager(),
		locales.TableStatus(),
		locales.TasksKindUpdate(),
		locales.TasksKindInstall(),
		locales.TasksKindUninstall(),
		"typescript",
		"14.1.1",
		"15.0.0",
		"Homebrew",
		"NPM",
		"Pip",
		locales.TasksStatusRunning(),
		locales.TasksStatusQueued(),
		locales.TasksStatusFailed(),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
}

// A running task's status is live: the pinwheel keeps the frame loop going.
func TestRunningTaskStatusAnimates(t *testing.T) {
	v := &tasksView{data: fixture()}
	tt := render(t, func(c *ui.Context) {
		v.statusBadge(c, tasks.Snapshot{State: tasks.StateRunning})
	}, 180, 40)
	if !tt.FramePending() {
		t.Fatal("the running status requests no further frames")
	}
}

func TestTasksViewEmptyState(t *testing.T) {
	v := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
	tt := render(t, v.view, 1000, 560)

	if !tt.HasText(locales.TasksEmpty()) {
		t.Errorf("empty state missing: %v", tt.Texts())
	}
	if !tt.HasText(locales.TasksSelectTask()) {
		t.Errorf("the console panel should stay visible: %v", tt.Texts())
	}
}

// Clicking a task opens its console in the permanent panel.
func TestTaskClickOpensConsole(t *testing.T) {
	v := tasksViewFixture()
	tt := render(t, v.view, 1000, 560)

	if tt.HasText("==> Upgrading typescript") {
		t.Fatalf("console shown before a task was chosen: %v", tt.Texts())
	}
	if err := tt.Click("typescript"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()

	if v.selectedID != "3" {
		t.Fatalf("selected %q, want task 3", v.selectedID)
	}
	for _, want := range []string{
		"==> Upgrading typescript",
		"==> Pouring typescript",
		"$ brew upgrade typescript",
		"/opt/homebrew/bin/brew",
		locales.TasksCancel(),
	} {
		if !tt.HasText(want) {
			t.Errorf("console is missing %q: %v", want, tt.Texts())
		}
	}
}

// A live refresh keeps the old console on screen, reloads when output arrives
// during a fetch, and ignores a completion for a task that is no longer chosen.
func TestTaskOutputRefreshState(t *testing.T) {
	v := &tasksView{}
	request, fetch := v.beginOutput("3", 1)
	if !fetch {
		t.Fatal("the first console load did not start")
	}
	v.finishOutput("3", request, tasks.Output{Lines: []tasks.Line{{Text: "first"}}, Seq: 1})

	request, fetch = v.beginOutput("3", 2)
	if !fetch {
		t.Fatal("a newer sequence did not start a refresh")
	}
	if !v.outputReady || len(v.output.Lines) != 1 || v.output.Lines[0].Text != "first" {
		t.Fatalf("the displayed console was cleared while refreshing: %+v", v.output)
	}

	// Output arrives during the fetch: fold it into the next request.
	if _, fetch = v.beginOutput("3", 3); fetch {
		t.Fatal("a second fetch started while one was in flight")
	}
	if !v.finishOutput("3", request, tasks.Output{Lines: []tasks.Line{{Text: "first"}, {Text: "second"}}, Seq: 2}) {
		t.Fatal("newer output did not ask for another reload")
	}
	request, fetch = v.beginOutput("3", 3)
	if !fetch {
		t.Fatal("the deferred reload did not start")
	}
	v.finishOutput("3", request, tasks.Output{Lines: []tasks.Line{{Text: "first"}, {Text: "second"}, {Text: "third"}}, Seq: 3})

	// A completion for the previous task must not overwrite the new one.
	stale, _ := v.beginOutput("3", 4)
	v.beginOutput("4", 0)
	if v.finishOutput("3", stale, tasks.Output{Lines: []tasks.Line{{Text: "stale"}}, Seq: 4}) {
		t.Fatal("a stale completion asked for a reload")
	}
	if v.outputFor != "4" {
		t.Fatalf("selected output = %q, want task 4", v.outputFor)
	}
}

func TestTaskConsoleStates(t *testing.T) {
	nodes := []tasks.Snapshot{
		{
			ID: "1", Kind: tasks.KindInstall, State: tasks.StateQueued,
			ManagerName: "NPM", Package: "queued", StartedAt: time.Now(),
		},
		{
			ID: "2", Kind: tasks.KindInstall, State: tasks.StateSucceeded,
			ManagerName: "NPM", Package: "stored", StartedAt: time.Now(),
		},
		{
			ID: "3", Kind: tasks.KindInstall, State: tasks.StateRunning,
			ManagerName: "NPM", Package: "quiet", StartedAt: time.Now(),
		},
	}

	cases := map[string]string{
		"1": locales.TasksQueuedHint(),
		"3": locales.TasksNoOutput(),
	}
	for id, want := range cases {
		v := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
		v.apply(nodes)
		v.selectedID = id
		v.outputFor = id
		v.outputReady = true
		for i := range v.all {
			if v.all[i].ID == id {
				v.selectedIndex = i
			}
		}
		tt := render(t, v.view, 1000, 560)
		if !tt.HasText(want) {
			t.Errorf("task %s: missing %q in %v", id, want, tt.Texts())
		}
	}

	// A task from the store loads its persisted console on selection.
	v := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
	v.apply(nodes)
	v.selectedID, v.selectedIndex = "2", 1
	v.outputFor = "2"
	v.outputReady = true
	v.output = tasks.Output{Lines: []tasks.Line{{Text: "stored output"}}}
	tt := render(t, v.view, 1000, 560)
	if !tt.HasText("stored output") {
		t.Errorf("stored output missing: %v", tt.Texts())
	}
}

// A failed task offers a retry; a retry waits while its package runs.
func TestTaskRetry(t *testing.T) {
	v := tasksViewFixture()
	var got tasks.Snapshot
	v.onRetry = func(task tasks.Snapshot, _ actionOrigin) { got = task }
	v.selectedID, v.selectedIndex = "1", -1
	v.sync()
	for i := range v.rows {
		if v.rows[i].ID == "1" {
			v.selectedIndex = i
		}
	}

	tt := render(t, v.view, 1000, 560)
	if err := tt.Click(locales.TasksRetry()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if got.ID != "1" || got.Package != "requests" {
		t.Fatalf("retry got %+v", got)
	}

	busy := tasksViewFixture()
	busy.data.active = map[string]string{"pip:requests": "remove"}
	busy.selectedID = "1"
	busy.onRetry = func(task tasks.Snapshot, _ actionOrigin) { got = task }
	got = tasks.Snapshot{}
	tt = render(t, busy.view, 1000, 560)
	if err := tt.Click(locales.TasksRetry()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if got.ID != "" {
		t.Fatalf("retry fired while the package runs: %+v", got)
	}
}

// A running task can be cancelled from its row and from the console panel.
func TestTaskCancel(t *testing.T) {
	v := tasksViewFixture()
	var got tasks.Snapshot
	v.onCancel = func(task tasks.Snapshot) { got = task }
	v.selectedID = "3"
	v.sync()

	tt := render(t, v.view, 1000, 560)
	if err := tt.Click(locales.TasksCancel()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if got.ID != "3" {
		t.Fatalf("cancel got %+v", got)
	}
}

func TestTaskFilters(t *testing.T) {
	v := tasksViewFixture()
	v.query = "typescript"
	v.sync()
	if len(v.rows) != 1 || v.rows[0].ID != "3" {
		t.Fatalf("query rows = %+v", v.rows)
	}

	v.query = ""
	v.opFilter = map[tasks.Kind]bool{tasks.KindInstall: true}
	v.sync()
	if len(v.rows) != 1 || v.rows[0].ID != "2" {
		t.Fatalf("operation rows = %+v", v.rows)
	}

	v.opFilter = map[tasks.Kind]bool{
		tasks.KindInstall: true, tasks.KindUpdate: true, tasks.KindUninstall: true,
	}
	v.managerAll = false
	v.managerFilter = map[string]bool{"pip": true}
	v.sync()
	if len(v.rows) != 1 || v.rows[0].ID != "1" {
		t.Fatalf("manager rows = %+v", v.rows)
	}

	v.query = "nothing"
	v.sync()
	if len(v.rows) != 0 {
		t.Fatalf("combined filters kept %+v", v.rows)
	}
	tt := render(t, v.view, 1000, 560)
	if !tt.HasText(locales.TasksNoMatch()) {
		t.Errorf("filtered empty state missing: %v", tt.Texts())
	}
}

// A reload keeps the chosen task even when a new running task moves the rows.
func TestTasksReloadKeepsSelection(t *testing.T) {
	v := tasksViewFixture()
	v.selectedID = "1"
	v.sync()

	next := append([]tasks.Snapshot{{
		ID: "4", Kind: tasks.KindInstall, State: tasks.StateRunning,
		ManagerName: "Cargo", Package: "ripgrep", StartedAt: time.Now(),
	}}, tasksFixture()...)
	v.apply(next)
	v.sync()

	if v.selectedID != "1" {
		t.Fatalf("selection = %q, want task 1", v.selectedID)
	}
	if v.rows[v.selectedIndex].ID != "1" {
		t.Fatalf("selected row = %+v", v.rows[v.selectedIndex])
	}
}

func TestTasksClearSelectionWhenGone(t *testing.T) {
	v := tasksViewFixture()
	v.selectedID = "1"
	v.sync()

	v.apply(tasksFixture()[:1])
	v.sync()

	if v.selectedID != "" || v.selectedIndex != -1 {
		t.Fatalf("selection survived the task: %q at %d", v.selectedID, v.selectedIndex)
	}
}

// The version column shows an update as old → new.
func TestTaskVersionCell(t *testing.T) {
	v := tasksViewFixture()
	tt := render(t, v.view, 1000, 560)
	if !tt.HasText("14.1.1") || !tt.HasText("15.0.0") || !tt.HasText("→") {
		t.Fatalf("version cell missing the arrow pair: %v", tt.Texts())
	}
}

func TestCancelledTaskShowsCancelledAndRetry(t *testing.T) {
	nodes := []tasks.Snapshot{{
		ID: "9", Kind: tasks.KindUpdate, State: tasks.StateCancelled,
		ManagerID: "brew", ManagerName: "Homebrew", Package: "cancelled",
		Version: "1.0", NewVersion: "1.1", StartedAt: time.Now(),
	}}

	v := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
	v.apply(nodes)
	tt := render(t, v.view, 1000, 560)
	if !tt.HasText(locales.TasksStatusCancelled()) {
		t.Errorf("cancelled badge missing: %v", tt.Texts())
	}

	selected := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
	selected.apply(nodes)
	selected.selectedID = "9"
	tt = render(t, selected.view, 1000, 560)
	if !tt.HasText(locales.TasksRetry()) {
		t.Errorf("cancelled task offers no retry: %v", tt.Texts())
	}
}

func TestStoredOutputLoadingState(t *testing.T) {
	nodes := []tasks.Snapshot{{
		ID: "5", Kind: tasks.KindInstall, State: tasks.StateSucceeded,
		ManagerID: "npm", ManagerName: "NPM", Package: "stored",
		StartedAt: time.Now(),
	}}

	loading := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
	loading.apply(nodes)
	loading.selectedID = "5"
	tt := render(t, loading.view, 1000, 560)
	if !tt.HasText(locales.TasksOutputLoading()) {
		t.Errorf("loading state missing: %v", tt.Texts())
	}

	loaded := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
	loaded.apply(nodes)
	loaded.selectedID = "5"
	loaded.outputFor = "5"
	loaded.outputReady = true
	loaded.output = tasks.Output{Lines: []tasks.Line{{Text: "persisted line"}}}
	tt = render(t, loaded.view, 1000, 560)
	if !tt.HasText("persisted line") {
		t.Errorf("stored console missing: %v", tt.Texts())
	}
}

func TestTaskTableShowsStartAndFinishTimes(t *testing.T) {
	start := time.Now().Add(-time.Minute)
	finish := time.Now()
	v := &tasksView{data: fixture(), selectedIndex: -1, splitPercent: 64}
	v.apply([]tasks.Snapshot{{
		ID: "7", Kind: tasks.KindUpdate, State: tasks.StateSucceeded,
		ManagerID: "brew", ManagerName: "Homebrew", Package: "timed",
		Version: "1.0", NewVersion: "1.1", StartedAt: start, FinishedAt: finish,
	}})
	tt := render(t, v.view, 1280, 560)
	for _, want := range []string{
		locales.TableStarted(), locales.TableFinished(),
		clockLabel(start), clockLabel(finish),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
}

// The task page's operation and manager filters are multi-selects.
func TestTaskFilterMultiSelect(t *testing.T) {
	ops := tasksViewFixture()
	tt := render(t, ops.view, 1000, 560)
	if err := tt.Click(locales.TasksAllOperations()); err != nil {
		t.Fatalf("open operations: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.TasksKindUpdate() + " (1)"); err != nil {
		t.Fatalf("uncheck update: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.TasksKindUninstall() + " (1)"); err != nil {
		t.Fatalf("uncheck uninstall: %v", err)
	}
	tt.Frame()
	if len(ops.rows) != 1 || ops.rows[0].Kind != tasks.KindInstall {
		t.Fatalf("operation multi-select rows = %+v", ops.rows)
	}

	managers := tasksViewFixture()
	tt = render(t, managers.view, 1000, 560)
	if err := tt.Click(locales.PackagesAllManagers()); err != nil {
		t.Fatalf("open managers: %v", err)
	}
	tt.Frame()
	if err := tt.Click("NPM (1)"); err != nil {
		t.Fatalf("uncheck NPM: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Pip (1)"); err != nil {
		t.Fatalf("uncheck Pip: %v", err)
	}
	tt.Frame()
	if len(managers.rows) != 1 || managers.rows[0].ID != "3" {
		t.Fatalf("manager multi-select rows = %+v", managers.rows)
	}
}
