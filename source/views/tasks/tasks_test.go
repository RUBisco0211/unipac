package tasks

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/dock"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/egoist/mygo/ui"
)

func render(t *testing.T, build func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	return testui.Render(build, w, h)
}

func viewFixture() *View {
	v := &View{
		Data:          testui.Data(),
		SelectedIndex: -1,
		Console:       dock.State{Height: 240},
		outputFor:     "3",
		outputSeq:     2,
		outputReady:   true,
		output: tasks.Output{Lines: []tasks.Line{
			{Text: "\x1b[32m==>\x1b[0m Upgrading typescript"},
			{Text: "==> Pouring typescript"},
		}},
	}
	v.Apply(testui.Tasks())
	return v
}

func TestTasksViewListsOneTaskPerPackage(t *testing.T) {
	tt := render(t, viewFixture().Build, 1000, 560)

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
	v := &View{Data: testui.Data()}
	tt := render(t, func(c *ui.Context) {
		v.statusBadge(c, tasks.Snapshot{State: tasks.StateRunning})
	}, 180, 40)
	if !tt.FramePending() {
		t.Fatal("the running status requests no further frames")
	}
}

func TestTasksViewEmptyState(t *testing.T) {
	v := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
	tt := render(t, v.Build, 1000, 560)

	if !tt.HasText(locales.TasksEmpty()) {
		t.Errorf("empty state missing: %v", tt.Texts())
	}
	if !tt.HasText(locales.TasksSelectTask()) {
		t.Errorf("the console panel should stay visible: %v", tt.Texts())
	}
}

// Clicking a task opens its console in the permanent panel.
func TestTaskClickOpensConsole(t *testing.T) {
	v := viewFixture()
	tt := render(t, v.Build, 1000, 560)

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
		// The command is a block to copy, named as it reads in a shell; the
		// executable behind it is not a line of its own.
		"brew upgrade typescript",
		locales.CommonCopyCommand(),
		locales.TasksCancel(),
	} {
		if !tt.HasText(want) {
			t.Errorf("console is missing %q: %v", want, tt.Texts())
		}
	}
	if tt.HasText("/opt/homebrew/bin/brew") {
		t.Errorf("the executable path is still shown: %v", tt.Texts())
	}
}

// A live refresh keeps the old console on screen, reloads when output arrives
// during a fetch, and ignores a completion for a task that is no longer chosen.
func TestTaskOutputRefreshState(t *testing.T) {
	v := &View{}
	request, fetch := v.BeginOutput("3", 1)
	if !fetch {
		t.Fatal("the first console load did not start")
	}
	v.FinishOutput("3", request, tasks.Output{Lines: []tasks.Line{{Text: "first"}}, Seq: 1})

	request, fetch = v.BeginOutput("3", 2)
	if !fetch {
		t.Fatal("a newer sequence did not start a refresh")
	}
	if !v.outputReady || len(v.output.Lines) != 1 || v.output.Lines[0].Text != "first" {
		t.Fatalf("the displayed console was cleared while refreshing: %+v", v.output)
	}

	// Output arrives during the fetch: fold it into the next request.
	if _, fetch = v.BeginOutput("3", 3); fetch {
		t.Fatal("a second fetch started while one was in flight")
	}
	if !v.FinishOutput("3", request, tasks.Output{Lines: []tasks.Line{{Text: "first"}, {Text: "second"}}, Seq: 2}) {
		t.Fatal("newer output did not ask for another reload")
	}
	request, fetch = v.BeginOutput("3", 3)
	if !fetch {
		t.Fatal("the deferred reload did not start")
	}
	v.FinishOutput("3", request, tasks.Output{Lines: []tasks.Line{{Text: "first"}, {Text: "second"}, {Text: "third"}}, Seq: 3})

	// A completion for the previous task must not overwrite the new one.
	stale, _ := v.BeginOutput("3", 4)
	v.BeginOutput("4", 0)
	if v.FinishOutput("3", stale, tasks.Output{Lines: []tasks.Line{{Text: "stale"}}, Seq: 4}) {
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
		v := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
		v.Apply(nodes)
		v.selectedID = id
		v.outputFor = id
		v.outputReady = true
		for i := range v.all {
			if v.all[i].ID == id {
				v.SelectedIndex = i
			}
		}
		tt := render(t, v.Build, 1000, 560)
		if !tt.HasText(want) {
			t.Errorf("task %s: missing %q in %v", id, want, tt.Texts())
		}
	}

	// A task from the store loads its persisted console on selection.
	v := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
	v.Apply(nodes)
	v.selectedID, v.SelectedIndex = "2", 1
	v.outputFor = "2"
	v.outputReady = true
	v.output = tasks.Output{Lines: []tasks.Line{{Text: "stored output"}}}
	tt := render(t, v.Build, 1000, 560)
	if !tt.HasText("stored output") {
		t.Errorf("stored output missing: %v", tt.Texts())
	}
}

// A failed task offers a retry; a retry waits while its package runs.
func TestTaskRetry(t *testing.T) {
	v := viewFixture()
	var got tasks.Snapshot
	v.OnRetry = func(task tasks.Snapshot, _ flight.Origin) { got = task }
	v.selectedID, v.SelectedIndex = "1", -1
	v.Sync()
	for i := range v.rows {
		if v.rows[i].ID == "1" {
			v.SelectedIndex = i
		}
	}

	tt := render(t, v.Build, 1000, 560)
	if err := tt.Click(locales.TasksRetry()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if got.ID != "1" || got.Package != "requests" {
		t.Fatalf("retry got %+v", got)
	}

	busy := viewFixture()
	busy.Data.Active = map[string]string{"pip:requests": "remove"}
	busy.selectedID = "1"
	busy.OnRetry = func(task tasks.Snapshot, _ flight.Origin) { got = task }
	got = tasks.Snapshot{}
	tt = render(t, busy.Build, 1000, 560)
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
	v := viewFixture()
	var got tasks.Snapshot
	v.OnCancel = func(task tasks.Snapshot) { got = task }
	v.selectedID = "3"
	v.Sync()

	tt := render(t, v.Build, 1000, 560)
	if err := tt.Click(locales.TasksCancel()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if got.ID != "3" {
		t.Fatalf("cancel got %+v", got)
	}
}

func TestTaskFilters(t *testing.T) {
	v := viewFixture()
	v.query = "typescript"
	v.Sync()
	if len(v.rows) != 1 || v.rows[0].ID != "3" {
		t.Fatalf("query rows = %+v", v.rows)
	}

	v.query = ""
	v.opFilter = map[tasks.Kind]bool{tasks.KindInstall: true}
	v.Sync()
	if len(v.rows) != 1 || v.rows[0].ID != "2" {
		t.Fatalf("operation rows = %+v", v.rows)
	}

	v.opFilter = map[tasks.Kind]bool{
		tasks.KindInstall: true, tasks.KindUpdate: true, tasks.KindUninstall: true,
	}
	v.managerAll = false
	v.managerFilter = map[string]bool{"pip": true}
	v.Sync()
	if len(v.rows) != 1 || v.rows[0].ID != "1" {
		t.Fatalf("manager rows = %+v", v.rows)
	}

	v.query = "nothing"
	v.Sync()
	if len(v.rows) != 0 {
		t.Fatalf("combined filters kept %+v", v.rows)
	}
	tt := render(t, v.Build, 1000, 560)
	if !tt.HasText(locales.TasksNoMatch()) {
		t.Errorf("filtered empty state missing: %v", tt.Texts())
	}
}

// A reload keeps the chosen task even when a new running task moves the rows.
func TestTasksReloadKeepsSelection(t *testing.T) {
	v := viewFixture()
	v.selectedID = "1"
	v.Sync()

	next := append([]tasks.Snapshot{{
		ID: "4", Kind: tasks.KindInstall, State: tasks.StateRunning,
		ManagerName: "Cargo", Package: "ripgrep", StartedAt: time.Now(),
	}}, testui.Tasks()...)
	v.Apply(next)
	v.Sync()

	if v.selectedID != "1" {
		t.Fatalf("selection = %q, want task 1", v.selectedID)
	}
	if v.rows[v.SelectedIndex].ID != "1" {
		t.Fatalf("selected row = %+v", v.rows[v.SelectedIndex])
	}
}

func TestTasksClearSelectionWhenGone(t *testing.T) {
	v := viewFixture()
	v.selectedID = "1"
	v.Sync()

	v.Apply(testui.Tasks()[:1])
	v.Sync()

	if v.selectedID != "" || v.SelectedIndex != -1 {
		t.Fatalf("selection survived the task: %q at %d", v.selectedID, v.SelectedIndex)
	}
}

// The version column shows an update as old → new.
func TestTaskVersionCell(t *testing.T) {
	v := viewFixture()
	tt := render(t, v.Build, 1000, 560)
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

	v := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
	v.Apply(nodes)
	tt := render(t, v.Build, 1000, 560)
	if !tt.HasText(locales.TasksStatusCancelled()) {
		t.Errorf("cancelled badge missing: %v", tt.Texts())
	}

	selected := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
	selected.Apply(nodes)
	selected.selectedID = "9"
	tt = render(t, selected.Build, 1000, 560)
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

	loading := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
	loading.Apply(nodes)
	loading.selectedID = "5"
	tt := render(t, loading.Build, 1000, 560)
	if !tt.HasText(locales.TasksOutputLoading()) {
		t.Errorf("loading state missing: %v", tt.Texts())
	}

	loaded := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
	loaded.Apply(nodes)
	loaded.selectedID = "5"
	loaded.outputFor = "5"
	loaded.outputReady = true
	loaded.output = tasks.Output{Lines: []tasks.Line{{Text: "persisted line"}}}
	tt = render(t, loaded.Build, 1000, 560)
	if !tt.HasText("persisted line") {
		t.Errorf("stored console missing: %v", tt.Texts())
	}
}

// The started column carries when a task began and how long it took, which is
// what a task list is read for; the finish time itself is a tooltip.
func TestTaskTableShowsWhenAndHowLong(t *testing.T) {
	start := time.Now().Add(-2 * time.Minute)
	finish := time.Now()
	v := &View{Data: testui.Data(), SelectedIndex: -1, Console: dock.State{Height: 240}}
	v.Apply([]tasks.Snapshot{{
		ID: "7", Kind: tasks.KindUpdate, State: tasks.StateSucceeded,
		ManagerID: "brew", ManagerName: "Homebrew", Package: "timed",
		Version: "1.0", NewVersion: "1.1", StartedAt: start, FinishedAt: finish,
	}})
	tt := render(t, v.Build, 1280, 560)
	for _, want := range []string{
		locales.TableStarted(), clockLabel(start),
		locales.DurationMinutes(2, 0),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText(locales.TableStarted() + " ") {
		t.Errorf("the finish time is a column of its own again: %v", tt.Texts())
	}
}

// The task page's operation and manager filters are multi-selects.
func TestTaskFilterMultiSelect(t *testing.T) {
	ops := viewFixture()
	tt := render(t, ops.Build, 1000, 560)
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

	managers := viewFixture()
	tt = render(t, managers.Build, 1000, 560)
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

// The status filter narrows the page to the tasks in the chosen states, and its
// counts say what each status would show under the other filters.
func TestTaskStatusFilter(t *testing.T) {
	v := viewFixture()
	v.ensureFilters()
	v.statusFilter = map[tasks.State]bool{tasks.StateFailed: true}
	v.Sync()

	if len(v.rows) != 1 || v.rows[0].ID != "1" {
		t.Fatalf("rows = %+v, want only the failed task", v.rows)
	}

	counts, total := v.statusCounts()
	if total != 3 {
		t.Errorf("total = %d, want all three tasks counted", total)
	}
	if counts[tasks.StateRunning] != 1 || counts[tasks.StateQueued] != 1 || counts[tasks.StateFailed] != 1 {
		t.Errorf("counts = %v, want one of each status", counts)
	}

	// The label names the one status chosen, and says "all" when every one is.
	if got := v.statusFilterLabel(); got != statusLabel(tasks.StateFailed) {
		t.Errorf("label = %q, want %q", got, statusLabel(tasks.StateFailed))
	}
	v.statusFilter = nil
	v.ensureFilters()
	if got := v.statusFilterLabel(); got != locales.TasksAllStatuses() {
		t.Errorf("label = %q, want every status", got)
	}
}

// Revealing a task drops the filters that could hide it, status included.
func TestRevealClearsTheStatusFilter(t *testing.T) {
	v := viewFixture()
	v.ensureFilters()
	v.statusFilter = map[tasks.State]bool{tasks.StateFailed: true}
	if !v.Reveal("3") {
		t.Fatal("task 3 was not found")
	}
	v.Sync()
	if !v.allStatusesSelected() {
		t.Errorf("status filter survived a reveal: %v", v.statusFilter)
	}
	if len(v.rows) != len(v.all) {
		t.Errorf("rows = %d of %d, want every task shown", len(v.rows), len(v.all))
	}
}

// The version cell is one line and clips, so it carries its value as a tooltip.
func TestTaskVersionHasATooltip(t *testing.T) {
	v := viewFixture()
	tt := render(t, v.Build, 1000, 560)

	const version = "2.31.0"
	target, ok := tt.Find(version)
	if !ok {
		t.Fatalf("%q is not on screen", version)
	}
	before := 0
	for _, text := range tt.Texts() {
		if text == version {
			before++
		}
	}
	tt.Move(target.X+target.W/2, target.Y+target.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	after := 0
	for _, text := range tt.Texts() {
		if text == version {
			after++
		}
	}
	if after != before+1 {
		t.Errorf("%q showed %d times before the hover and %d after; want one more for its tooltip", version, before, after)
	}
}

// The dock's own chevron folds the console's body away and opens it again; the
// dock slides, so a frame after the press is still moving. The header stays,
// naming the task whose console is folded.
func TestConsoleChevronFoldsTheBody(t *testing.T) {
	v := viewFixture()
	v.selectedID = "3"
	v.Sync()

	dockView := render(t, v.Build, 1000, 560)
	dockView.Frame()
	const line = "==> Upgrading typescript"
	if !dockView.HasText(line) {
		t.Fatalf("the console is not there to fold: %v", dockView.Texts())
	}

	if err := dockView.Click(locales.TasksConsoleHide()); err != nil {
		t.Fatalf("click: %v", err)
	}
	dockView.Frame()
	if !dockView.FramePending() {
		t.Error("folding the console started no animation")
	}
	for range 40 {
		dockView.Advance(30 * time.Millisecond)
	}
	dockView.Frame()
	if dockView.HasText(line) {
		t.Errorf("the console's body is still shown: %v", dockView.Texts())
	}
	if !dockView.HasText(taskTitle(v.all[0])) {
		t.Errorf("the folded dock dropped the task's name: %v", dockView.Texts())
	}

	// The chevron names the action that will happen next.
	if err := dockView.Click(locales.TasksConsoleShow()); err != nil {
		t.Fatalf("click to open: %v", err)
	}
	for range 40 {
		dockView.Advance(30 * time.Millisecond)
	}
	dockView.Frame()
	if !dockView.HasText(line) {
		t.Errorf("the console did not come back: %v", dockView.Texts())
	}
}

// Choosing a task opens the console, since its console is the reason to choose
// it, but leaves a dock the user folded folded: the header still names the task.
func TestChoosingATaskOpensTheConsole(t *testing.T) {
	v := viewFixture()
	v.Sync()
	v.SelectedIndex = 1
	v.choose()
	if v.Console.Closed {
		t.Error("choosing a task left the console folded")
	}

	v.ToggleConsole()
	if !v.Console.Closed {
		t.Fatal("the toggle did not fold the console")
	}
	v.Sync()
	v.SelectedIndex = 0
	v.choose()
	if !v.Console.Closed {
		t.Error("choosing a task undid the user's fold")
	}

	// Revealing one from a notification opens it: the user asked for the console
	// by name.
	if !v.Reveal("3") {
		t.Fatal("task 3 is missing")
	}
	if v.Console.Closed {
		t.Error("revealing a task left the console folded")
	}
}

func TestSelectAcrossLines(t *testing.T) {
	v := &View{
		Data:          nil,
		SelectedIndex: 0,
		selectedID:    "1",
		Console:       dock.State{Height: 260},
		outputFor:     "1",
		outputSeq:     1,
		outputReady:   true,
		output: tasks.Output{Lines: []tasks.Line{
			{Text: "first line of output"},
			{Text: "second line of output"},
			{Text: "third line of output"},
		}},
	}
	v.Apply([]tasks.Snapshot{{
		ID: "1", Kind: tasks.KindInstall, State: tasks.StateSucceeded,
		ManagerID: "npm", ManagerName: "NPM", Package: "demo", StartedAt: time.Now(),
	}})
	v.Sync()

	tt := render(t, v.Build, 800, 600)
	tt.Frame()

	a, ok := tt.Find("first line of output")
	if !ok {
		t.Fatalf("first line is missing: %v", tt.Texts())
	}
	b, ok := tt.Find("third line of output")
	if !ok {
		t.Fatalf("third line is missing: %v", tt.Texts())
	}
	// Drag from the start of the first line into the third. The line's box is the
	// log's whole width, but the floating copy button covers its top-right
	// corner, so the drag ends on the text rather than at the far edge.
	tt.Move(a.X+2, a.Y+a.H/2)
	tt.Press(a.X+2, a.Y+a.H/2)
	tt.Move(b.X+200, b.Y+b.H/2)
	tt.Release(b.X+200, b.Y+b.H/2)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Frame()

	got := tt.Clipboard()
	for _, want := range []string{"first line", "second line", "third line"} {
		if !strings.Contains(got, want) {
			t.Errorf("the copied selection is missing %q: %q", want, got)
		}
	}
}

// The dock copies the two things a console is copied for: the command the task
// ran, from the header, and the output, from the log's corner — without the
// terminal's escape sequences, which are noise in whatever it is pasted into.
func TestConsoleCopyActions(t *testing.T) {
	v := viewFixture()
	v.selectedID = "3"
	v.Sync()

	tt := render(t, v.Build, 1000, 560)
	tt.Frame()
	if err := tt.Click(locales.CommonCopyCommand()); err != nil {
		t.Fatalf("copy command: %v", err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got != "brew upgrade typescript" {
		t.Errorf("the command copied %q", got)
	}

	if err := tt.Click(locales.CommonCopyOutput()); err != nil {
		t.Fatalf("copy output: %v", err)
	}
	tt.Frame()
	got := tt.Clipboard()
	want := "==> Upgrading typescript\n==> Pouring typescript"
	if got != want {
		t.Errorf("the output copied %q, want %q", got, want)
	}
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("the copied output kept its escape sequences: %q", got)
	}
}

// The header's controls keep one gap between them whatever the task's state. A
// state with no cancel or retry used to build an empty slot there, which took
// the gaps on both its sides and left a wider space after the copy button than
// between the rest.
func TestHeaderControlsKeepOneGap(t *testing.T) {
	now := time.Now()
	cases := map[string]struct {
		state tasks.State
		next  string
	}{
		"running":   {tasks.StateRunning, locales.TasksCancel()},
		"failed":    {tasks.StateFailed, locales.TasksRetry()},
		"succeeded": {tasks.StateSucceeded, ""},
	}
	for name, tc := range cases {
		v := &View{
			Data:          testui.Data(),
			SelectedIndex: -1,
			selectedID:    "1",
			Console:       dock.State{Height: 200},
			outputFor:     "1",
			outputSeq:     1,
			outputReady:   true,
			output:        tasks.Output{Lines: []tasks.Line{{Text: "line"}}},
		}
		v.Apply([]tasks.Snapshot{{
			ID: "1", Kind: tasks.KindUpdate, State: tc.state,
			ManagerID: "brew", ManagerName: "Homebrew", Package: "typescript",
			StartedAt: now.Add(-time.Minute), FinishedAt: now,
			Message: "failed", Command: "brew upgrade typescript",
		}})
		tt := render(t, v.Build, 804, 620)
		tt.Frame()

		labels := []string{locales.CommonCopyCommand(), locales.TasksConsoleHide()}
		if tc.next != "" {
			labels = append(labels, tc.next)
		}
		var rects []ui.Rect
		for _, label := range labels {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%s: %q is missing", name, label)
			}
			rects = append(rects, r)
		}
		slices.SortFunc(rects, func(a, b ui.Rect) int { return int(a.X - b.X) })
		for i := 1; i < len(rects); i++ {
			if gap := rects[i].X - (rects[i-1].X + rects[i-1].W); gap != 8 {
				t.Errorf("%s: the gap before control %d is %.0f, want 8", name, i, gap)
			}
		}
	}
}

// The copy button floating over the log keeps the header's trailing edge, so the
// two columns of controls read as one.
func TestOutputCopyAlignsWithHeaderControls(t *testing.T) {
	v := viewFixture()
	v.selectedID = "3"
	v.Sync()
	tt := render(t, v.Build, 1000, 560)
	tt.Frame()

	header, ok := tt.Find(locales.TasksConsoleHide())
	if !ok {
		t.Fatal("the header's fold control is missing")
	}
	copied, ok := tt.Find(locales.CommonCopyOutput())
	if !ok {
		t.Fatalf("the output's copy button is missing: %v", tt.Texts())
	}
	if got, want := copied.X+copied.W, header.X+header.W; math.Abs(float64(got-want)) > 0.5 {
		t.Errorf("the copy button ends at %.1f, want the header's edge at %.1f", got, want)
	}
}
