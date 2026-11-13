package views

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/list"
	"github.com/RUBisco0211/unipac/source/views/panel"
	"github.com/RUBisco0211/unipac/source/views/settings"
	taskspage "github.com/RUBisco0211/unipac/source/views/tasks"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

func render(t *testing.T, build func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	return testui.Render(build, w, h)
}

func uiWithRunningTasks() *UI {
	u := &UI{
		data:    testui.Data(),
		history: []Page{PagePackages},
	}
	u.tasks = &taskspage.View{SelectedIndex: -1}
	list := testui.Tasks()
	// Two running tasks, one queued and one failed: the badges count running
	// tasks only.
	list[1].State = tasks.StateRunning
	list = append(list, tasks.Snapshot{
		ID: "4", Kind: tasks.KindInstall, State: tasks.StateRunning,
		ManagerName: "Cargo", Package: "ripgrep", StartedAt: time.Now(),
	})
	u.tasks.Apply(list)
	return u
}

func exactText(tt *ui.Tester, want string) bool {
	for _, text := range tt.Texts() {
		if text == want {
			return true
		}
	}
	return false
}

func TestSidebarBadgesCountOutdatedAndRunning(t *testing.T) {
	u := uiWithRunningTasks()
	tt := render(t, u.left, 260, 520)

	// One outdated package in the fixture (typescript), and three running
	// tasks (the fixture's plus the one made running and the one added).
	if !exactText(tt, "1") {
		t.Errorf("outdated badge missing: %v", tt.Texts())
	}
	if !exactText(tt, "3") {
		t.Errorf("running task badge missing: %v", tt.Texts())
	}
}

// The count sits after the label, on the item's trailing edge, not on the
// icon it belongs to.
func TestSidebarBadgesTrailTheLabel(t *testing.T) {
	u := uiWithRunningTasks()
	tt := render(t, u.left, 260, 520)

	item, ok := tt.Find(locales.NavTasks())
	if !ok {
		t.Fatalf("tasks item missing: %v", tt.Texts())
	}
	badge, ok := tt.Find("3")
	if !ok {
		t.Fatalf("tasks badge missing: %v", tt.Texts())
	}
	// The text of the badge sits in the trailing end of the item, not on the
	// leading icon.
	if badge.X+badge.W/2 < item.X+item.W*0.75 {
		t.Fatalf("badge %+v is not on the trailing edge of the item %+v", badge, item)
	}
	if right := badge.X + badge.W; right < item.X+item.W*0.9 {
		t.Fatalf("badge %+v is not right-aligned in the item %+v", badge, item)
	}
}

func TestTasksButtonShowsBadgeAndNavigates(t *testing.T) {
	u := uiWithRunningTasks()
	tt := render(t, u.windowTitle, 700, 48)
	// The button's rectangle is only known after its first layout.
	tt.Frame()

	if !exactText(tt, "3") {
		t.Errorf("title bar badge missing: %v", tt.Texts())
	}
	if u.jobTarget.W <= 0 {
		t.Fatal("the tasks button rectangle was not recorded")
	}
	if err := tt.Click(locales.NavTasks()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if u.page != PageTasks {
		t.Fatalf("page = %v, want tasks", u.page)
	}
}

func TestFlightMovesToTasksButton(t *testing.T) {
	u := &UI{jobTarget: ui.Rect{X: 700, Y: 20, W: 28, H: 28}}
	u.jobFlight.ToTasks(flight.Origin{X: 100, Y: 300}, u.jobTarget)
	if !u.jobFlight.Active() {
		t.Fatal("the flight did not start")
	}

	tt := ui.NewTester(func(c *ui.Context) {
		root := ui.Column(c).Fill()
		root.Children(func() { u.jobFlight.Draw(c, root) })
	}, 800, 560)
	tt.Frame()

	start, ok := tt.Find(locales.TasksFlight())
	if !ok {
		t.Fatalf("flight icon missing: %v", tt.Texts())
	}
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	mid, ok := tt.Find(locales.TasksFlight())
	if !ok {
		t.Fatalf("flight icon missing halfway: %v", tt.Texts())
	}
	if mid.X <= start.X || mid.Y >= start.Y {
		t.Fatalf("flight did not travel toward the tasks button: start %+v, mid %+v", start, mid)
	}

	// The flight is a 300 ms signal; it must be gone soon after, not parked on
	// the button.
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if u.jobFlight.Active() {
		t.Fatal("the flight still runs after its short duration")
	}
	if tt.HasText(locales.TasksFlight()) {
		t.Errorf("the flight icon stayed after arriving: %v", tt.Texts())
	}
}

// The badges are derived state: a refresh or a task change shows on the next
// frame without reopening the window.
func TestSidebarBadgesFollowState(t *testing.T) {
	u := uiWithRunningTasks()
	tt := render(t, u.left, 260, 520)

	if !exactText(tt, "1") || !exactText(tt, "3") {
		t.Fatalf("initial badges missing: %v", tt.Texts())
	}

	// The outdated package became current; the running badge stays where it was.
	for i := range u.data.Rows {
		u.data.Rows[i].Outdated = false
	}
	tt.Frame()
	if exactText(tt, "1") {
		t.Errorf("outdated badge survived a refresh: %v", tt.Texts())
	}
	if !exactText(tt, "3") {
		t.Errorf("running task badge disappeared: %v", tt.Texts())
	}

	// All but one task finished, and the badge counts what is left.
	u.tasks.Apply(testui.Tasks())
	tt.Frame()
	if !exactText(tt, "1") {
		t.Errorf("running task badge did not decrease: %v", tt.Texts())
	}
}

// Notifications fire for a task that just finished, not for history loaded
// from the store and not for cancellations.
func TestFinishedTasksOnlyTransitions(t *testing.T) {
	previous := map[string]tasks.State{
		"old":       tasks.StateSucceeded,
		"running":   tasks.StateRunning,
		"queued":    tasks.StateQueued,
		"cancelled": tasks.StateCancelled,
	}
	list := []tasks.Snapshot{
		{ID: "old", State: tasks.StateSucceeded},
		{ID: "running", State: tasks.StateFailed},
		{ID: "queued", State: tasks.StateSucceeded},
		{ID: "cancelled", State: tasks.StateCancelled},
		{ID: "new", State: tasks.StateSucceeded},
		{ID: "live", State: tasks.StateRunning},
	}
	got := finishedTasks(previous, list)
	if len(got) != 3 {
		t.Fatalf("finished = %+v, want the transitioned and new tasks", got)
	}
	ids := map[string]bool{}
	for _, task := range got {
		ids[task.ID] = true
	}
	for _, id := range []string{"running", "queued", "new"} {
		if !ids[id] {
			t.Errorf("missing %q in %+v", id, got)
		}
	}
}

func TestSidebarSwitchesPage(t *testing.T) {
	u := &UI{}
	tt := render(t, u.left, 240, 400)

	if u.page != PagePackages {
		t.Fatalf("initial page = %v, want packages", u.page)
	}
	if err := tt.Click(locales.NavOutdated()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if u.page != PageOutdated {
		t.Fatalf("page after click = %v, want outdated", u.page)
	}
}

// uiWithPages is a window whose content area can switch between the two list
// pages, which is enough to exercise the page transition.
func uiWithPages() *UI {
	u := &UI{data: testui.Data(), history: []Page{PagePackages}, page: PagePackages}
	u.installed = &list.View{Data: u.data, AllowRemove: true}
	u.outdated = &list.View{Data: u.data, OutdatedOnly: true}
	return u
}

// ink counts the pixels of r that differ from the window's background, which is
// how a test sees what a page drew: labels do not cover the copy of a page that
// is on its way out.
func ink(tt *ui.Tester, r ui.Rect) int {
	corner := tt.Image().At(1, 1)
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if tt.Image().At(x, y) != corner {
				n++
			}
		}
	}
	return n
}

func TestPageSwitchFadesTheArrivingPage(t *testing.T) {
	u := uiWithPages()
	tt := testui.Render(u.content, 900, 500)
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("the page area animates while nothing changes")
	}
	gone, ok := tt.Find("ripgrep")
	if !ok {
		t.Fatalf("the installed page is missing: %v", tt.Texts())
	}
	if ink(tt, gone) == 0 {
		t.Fatal("the installed page draws nothing where its row is")
	}

	// The outdated page drops the current package. It takes the content area
	// while it fades in from just below; the page it replaces is gone at once,
	// so nothing of it is left to draw at a wrong width.
	u.page = PageOutdated
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("switching pages started no transition")
	}
	if !tt.HasText("typescript") {
		t.Errorf("the arriving page is missing: %v", tt.Texts())
	}
	if left := ink(tt, gone); left != 0 {
		t.Errorf("the page that left still draws %d pixels", left)
	}
	start, ok := tt.Find("typescript")
	if !ok {
		t.Fatal("the arriving row was not laid out")
	}

	// Halfway the transition still runs, and the page sits where the entry put
	// it rather than at its final place.
	tt.Advance(100 * time.Millisecond)
	if !tt.FramePending() {
		t.Fatal("the transition ended before its halfway point")
	}
	mid, ok := tt.Find("typescript")
	if !ok {
		t.Fatal("the arriving row vanished midway")
	}

	tt.Advance(200 * time.Millisecond)
	if tt.FramePending() {
		t.Fatal("the transition never ended")
	}
	end, ok := tt.Find("typescript")
	if !ok {
		t.Fatal("the arriving row vanished")
	}
	if mid.Y > start.Y || end.Y > mid.Y {
		t.Errorf("the page did not rise into place: start %v, halfway %v, end %v", start.Y, mid.Y, end.Y)
	}
	if rise := start.Y - end.Y; rise < 6 {
		t.Errorf("the page started %v DIPs below its place, want the 8 of the entry", rise)
	}
}

// A page box must not interpolate its layout: the table finds its width on the
// frame after it is built, and growing through that width takes the whole
// duration, which shows as the page flashing at a narrower width.
func TestPageTransitionSnapsLayout(t *testing.T) {
	settled := false
	build := func(c *ui.Context) {
		width := float32(120)
		if settled {
			width = 480
		}
		ui.Column(c).Fill().Children(func() {
			ui.Box(c).Key(pageKey(PagePackages)).Width(width).Height(80).
				Label("page").Transition(pageTransition)
		})
	}
	tt := testui.Render(build, 600, 200)
	tt.Frame()

	settled = true
	tt.Frame()
	found, ok := tt.Find("page")
	if !ok {
		t.Fatal("the page box is not on screen")
	}
	if found.W < 470 || found.W > 490 {
		t.Fatalf("the page box is %.0f wide on the frame it settled, want 480", found.W)
	}
	tt.Advance(250 * time.Millisecond)
	if after, _ := tt.Find("page"); after.W != found.W {
		t.Fatalf("the page box kept resizing: %.0f then %.0f", found.W, after.W)
	}
}

// A failed action says what failed and on what, not the command's output: the
// output is the task's console, and the toast points at the task.
func TestFailureToastNamesTheOperationAndItsTask(t *testing.T) {
	got := failureToast("update", []manager.Package{{Manager: "npm", Name: "bun"}}, "28")
	if want := locales.ToastUpdateFailed("bun"); got.Title != want {
		t.Errorf("title = %q, want %q", got.Title, want)
	}
	if got.Tone != components.ToastError {
		t.Errorf("tone = %q, want an error toast", got.Tone)
	}
	if got.TaskID != "28" {
		t.Errorf("task = %q, want the task the action created", got.TaskID)
	}
}

// A batch counts its packages instead of naming one of them.
func TestFailureToastCountsABatch(t *testing.T) {
	pkgs := []manager.Package{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	got := failureToast("remove", pkgs, "")
	if want := locales.ToastUninstallFailed(locales.ToastSeveralPackages("3")); got.Title != want {
		t.Errorf("title = %q, want %q", got.Title, want)
	}
	if got.TaskID != "" {
		t.Errorf("task = %q, want none", got.TaskID)
	}
}

// The task an action created is the newest one its packages have; another
// manager's package of the same name is not it.
func TestNewestTaskFindsTheTaskJustRun(t *testing.T) {
	list := []tasks.Snapshot{
		{ID: "9", ManagerID: "brew", Package: "bun"},
		{ID: "27", ManagerID: "npm", Package: "bun"},
		{ID: "28", ManagerID: "npm", Package: "bun"},
	}
	if got := newestTask(list, []manager.Package{{Manager: "npm", Name: "bun"}}); got != "28" {
		t.Fatalf("task = %q, want 28", got)
	}
	if got := newestTask(list, []manager.Package{{Manager: "cargo", Name: "bun"}}); got != "" {
		t.Fatalf("task = %q, want none", got)
	}
}

// stubService answers the two calls opening a task makes; the embedded
// interface stands in for the rest.
type stubService struct {
	inventory.Service
	list []tasks.Snapshot
}

func (s stubService) ListTasks() []tasks.Snapshot    { return s.list }
func (s stubService) TaskOutput(string) tasks.Output { return tasks.Output{} }

// The toast about a task carries the action that opens it, and the action lands
// on the tasks page with that task selected.
func TestToastActionOpensItsTask(t *testing.T) {
	data := testui.Data()
	u := &UI{
		data:  data,
		tasks: &taskspage.View{Data: data, SelectedIndex: -1},
		svc: stubService{list: []tasks.Snapshot{{
			ID: "28", Kind: tasks.KindUpdate, State: tasks.StateFailed,
			ManagerID: "npm", ManagerName: "NPM", Package: "bun",
		}}},
	}
	toast := u.toastFor(inventory.Toast{Title: "bun failed", Tone: components.ToastError, TaskID: "28"})
	if toast.Action != locales.ToastShowTask() || toast.OnAction == nil {
		t.Fatalf("toast = %+v, want the action that shows the task", toast)
	}

	toast.OnAction()
	if u.page != PageTasks {
		t.Errorf("page = %v, want tasks", u.page)
	}
	// The page places its selection in the frame it builds, as the window does.
	u.tasks.Sync()
	if task, ok := u.tasks.Selected(); !ok || task.ID != "28" {
		t.Errorf("selected = %+v, want task 28", task)
	}
}

// A toast with no task shows no action.
func TestToastWithoutTaskHasNoAction(t *testing.T) {
	u := &UI{data: testui.Data()}
	if toast := u.toastFor(inventory.Toast{Title: "Updated"}); toast.Action != "" || toast.OnAction != nil {
		t.Fatalf("toast = %+v, want no action", toast)
	}
}

// A notification says what the toast says, and where the task was, never the
// command's output: the console is the task's, and clicking opens it.
func TestNotificationTextMatchesTheToast(t *testing.T) {
	failed := tasks.Snapshot{
		ID: "28", Kind: tasks.KindUpdate, State: tasks.StateFailed,
		ManagerName: "NPM", Package: "bun", Message: "npm warn ERESOLVE\nnpm error EUNSUPPORTEDPROTOCOL",
	}
	title, body := notificationText(failed)
	if want := locales.ToastUpdateFailed("bun"); title != want {
		t.Errorf("title = %q, want the toast's %q", title, want)
	}
	if body != "bun · NPM" {
		t.Errorf("body = %q, want the package and its manager only", body)
	}

	succeeded := tasks.Snapshot{ID: "29", Kind: tasks.KindInstall, State: tasks.StateSucceeded, Package: "mole"}
	if title, body := notificationText(succeeded); title != locales.ToastInstalled() || body != "mole" {
		t.Errorf("notification = %q / %q, want %q / mole", title, body, locales.ToastInstalled())
	}
}

// The success toast and the notification word an outcome the same way, from the
// task kind the operation records.
func TestOutcomeLabelsFollowTheTaskKind(t *testing.T) {
	for op, want := range map[string]tasks.Kind{
		"install": tasks.KindInstall,
		"update":  tasks.KindUpdate,
		"remove":  tasks.KindUninstall,
	} {
		if got := taskKind(op); got != want {
			t.Errorf("taskKind(%q) = %q, want %q", op, got, want)
		}
	}
	if got := successLabel(tasks.KindInstall); got != locales.ToastInstalled() {
		t.Errorf("install success = %q", got)
	}
	if got := failureLabel(tasks.KindUninstall, "mole"); got != locales.ToastUninstallFailed("mole") {
		t.Errorf("uninstall failure = %q", got)
	}
}

// A page can put buttons in the title bar, beside its title: the shell builds
// whatever the page showing offers, and only for that page.
func TestPageTitleActions(t *testing.T) {
	u := uiWithPages()
	u.tasks = &taskspage.View{Data: u.data, SelectedIndex: -1, Console: panel.State{Width: 380}}
	u.installed.TitleActions = func(c *ui.Context) {
		components.Text(c, "page-action")
	}

	tt := render(t, u.windowTitle, 700, 48)
	tt.Frame()
	if !tt.HasText("page-action") {
		t.Errorf("the page's title action is missing: %v", tt.Texts())
	}

	// Another page has none, so the slot builds nothing for it.
	u.page = PageTasks
	tt.Frame()
	if tt.HasText("page-action") {
		t.Errorf("the previous page's action is still in the title bar: %v", tt.Texts())
	}
}

// Without one, the title bar is the history arrows, the title and the tasks
// button, as before.
func TestTitleBarWithoutPageActions(t *testing.T) {
	u := uiWithPages()
	tt := render(t, u.windowTitle, 700, 48)
	tt.Frame()
	if !tt.HasText(locales.NavPackages()) || !tt.HasText(locales.NavTasks()) {
		t.Errorf("the title bar lost its parts: %v", tt.Texts())
	}
}

// quitService stands in for the app: it reports the pending tasks the dialog is
// about and remembers being asked to stop them.
type quitService struct {
	inventory.Service
	pending int
	stopped bool
}

func (s *quitService) UnfinishedTasks() int           { return s.pending }
func (s *quitService) StopTasks()                     { s.stopped = true }
func (s *quitService) ListTasks() []tasks.Snapshot    { return nil }
func (s *quitService) TaskOutput(string) tasks.Output { return tasks.Output{} }

// uiWithQuitPrompt is a window showing the quit dialog for two tasks.
func uiWithQuitPrompt(t *testing.T, hold bool) (*UI, *quitService, *ui.Tester) {
	t.Helper()
	svc := &quitService{pending: 2}
	u := uiWithPages()
	u.svc = svc
	u.tasks = &taskspage.View{Data: u.data, SelectedIndex: -1, Console: panel.State{Width: 380}}
	u.settings = &settings.View{HoldToConfirm: hold}
	quit := 0
	u.quitApp = func() { quit++ }
	u.quitPending, u.quitOpen = 2, true

	tt := render(t, u.content, 900, 520)
	tt.Frame()
	return u, svc, tt
}

// The window asks in its own dialog, and keeping running closes it with the
// tasks untouched.
func TestQuitDialogKeepsRunning(t *testing.T) {
	u, svc, tt := uiWithQuitPrompt(t, true)
	if !tt.HasText(locales.TasksQuitTitle()) {
		t.Fatalf("the question is missing: %v", tt.Texts())
	}
	if err := tt.Click(locales.TasksQuitCancel()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if u.quitOpen {
		t.Error("the dialog stayed open")
	}
	if svc.stopped || u.quitAccepted.Load() {
		t.Error("keeping running stopped the tasks")
	}
}

// With holding on, the confirm is a hold: pressing it briefly does nothing.
func TestQuitConfirmNeedsAHold(t *testing.T) {
	u, svc, tt := uiWithQuitPrompt(t, true)
	confirm, ok := tt.Find(locales.TasksQuitConfirm())
	if !ok {
		t.Fatalf("the confirm control is missing: %v", tt.Texts())
	}
	tt.ClickAt(confirm.X+confirm.W/2, confirm.Y+confirm.H/2)
	tt.Frame()
	if svc.stopped || u.quitAccepted.Load() {
		t.Error("a click confirmed a hold")
	}

	// Held long enough, it confirms: the tasks are stopped and the app leaves.
	tt.Press(confirm.X+confirm.W/2, confirm.Y+confirm.H/2)
	tt.Advance(2 * time.Second)
	tt.Release(confirm.X+confirm.W/2, confirm.Y+confirm.H/2)
	tt.Frame()
	if !svc.stopped {
		t.Error("the hold did not stop the tasks")
	}
	if !u.quitAccepted.Load() {
		t.Error("the hold did not accept the quit")
	}
}

// With holding off, the confirm is a button and one click answers it.
func TestQuitConfirmClicksWhenHoldingIsOff(t *testing.T) {
	u, svc, tt := uiWithQuitPrompt(t, false)
	if err := tt.Click(locales.TasksQuitConfirm()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if !svc.stopped || !u.quitAccepted.Load() {
		t.Error("the click did not confirm the quit")
	}
}

// Messages stack at the bottom right, away from the title bar and the sidebar's
// controls.
func TestToastsSitBottomRight(t *testing.T) {
	if toastOptions.Placement != components.ToastBottomEnd {
		t.Fatalf("toasts are placed %v, want the bottom end", toastOptions.Placement)
	}
	tt := render(t, func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { components.Toasts(c, toastOptions) })
		c.Toast("a-message")
	}, 900, 500)
	tt.Frame()

	toast, ok := tt.Find("a-message")
	if !ok {
		t.Fatalf("the toast is missing: %v", tt.Texts())
	}
	if toast.X+toast.W/2 < 450 {
		t.Errorf("the toast is at %.0f, on the leading half of a 900 wide window", toast.X)
	}
	if toast.Y+toast.H/2 < 250 {
		t.Errorf("the toast is at y=%.0f, on the top half of a 500 high window", toast.Y)
	}
}

// wiredService answers the calls building the window makes.
type wiredService struct {
	inventory.Service
}

func (wiredService) ListManagers() []manager.Info                  { return nil }
func (wiredService) ManagerOverrides() map[string]manager.Settings { return nil }
func (wiredService) GetCachedPackages() ([]manager.Package, error) { return nil, nil }
func (wiredService) ListTasks() []tasks.Snapshot                   { return nil }
func (wiredService) AutoScanOnLaunch() bool                        { return false }
func (wiredService) HoldToConfirm() bool                           { return true }
func (wiredService) LogLevel() string                              { return "" }
func (wiredService) LogLevels() []string                           { return nil }
func (wiredService) Language() string                              { return "" }
func (wiredService) Languages() []string                           { return nil }
func (wiredService) Theme() string                                 { return "" }
func (wiredService) Accent() string                                { return "" }
func (wiredService) Accents() []string                             { return nil }
func (wiredService) Proxy() string                                 { return "" }
func (wiredService) SetProxy(string) error                         { return nil }
func (wiredService) SystemProxy() string                           { return "" }
func (wiredService) NotificationsEnabled() bool                    { return true }
func (wiredService) Notifications(tasks.Kind, bool) bool           { return true }
func (wiredService) NotificationKinds() []tasks.Kind {
	return []tasks.Kind{tasks.KindInstall, tasks.KindUpdate, tasks.KindUninstall}
}
func (wiredService) NotifyUpdates() bool    { return true }
func (wiredService) About() inventory.About { return inventory.About{} }

// The updater's own state: a fake that cannot update keeps the About rows to the
// ones a build without updates shows.
func (wiredService) UpdatesEnabled() bool        { return true }
func (wiredService) AutoCheckUpdates() bool      { return true }
func (wiredService) AutoDownloadUpdates() bool   { return false }
func (wiredService) SetAutoCheckUpdates(bool)    {}
func (wiredService) SetAutoDownloadUpdates(bool) {}
func (wiredService) CheckForUpdates()            {}

// The search page's version picker is the shell's to answer. A page that lost
// the wiring would quietly install the newest version instead of asking, which
// looks like a working button.
func TestSearchPageAsksTheShellForVersions(t *testing.T) {
	u := New(wiredService{})
	if u.search.OnVersions == nil {
		t.Error("the search page cannot ask for a package's versions")
	}
	if u.search.OnInstallVersion == nil {
		t.Error("the search page cannot install a chosen version")
	}
}

// The notification switch is what decides whether a finished task is announced,
// and a window whose settings page has not been built still announces.
func TestNotificationsFollowEachSwitch(t *testing.T) {
	u := &UI{data: testui.Data()}
	if !u.notificationsOn(tasks.KindInstall, true) {
		t.Error("a window without a settings page does not announce")
	}
	// Update failures turned off in Settings, everything else left on.
	u.settings = settings.New(quietService{quiet: tasks.KindUpdate}, testui.Data())
	if u.notificationsOn(tasks.KindUpdate, false) {
		t.Error("the switch did not stop that operation's failures")
	}
	if !u.notificationsOn(tasks.KindUpdate, true) {
		t.Error("the switch stopped the same operation's successes")
	}
	if !u.notificationsOn(tasks.KindInstall, false) || !u.notificationsOn(tasks.KindUninstall, true) {
		t.Error("one switched-off outcome silenced another")
	}
}

// quietService reports every operation as announced but one outcome of one.
type quietService struct {
	wiredService
	quiet tasks.Kind
}

func (s quietService) Notifications(kind tasks.Kind, succeeded bool) bool {
	return kind != s.quiet || succeeded
}

// Clearing the history is the backend's job, and the page reloads from what it
// left behind. A window without one does nothing rather than panicking.
func TestClearTaskHistoryWithoutAWindowIsInert(t *testing.T) {
	u := &UI{data: testui.Data(), tasks: &taskspage.View{Data: testui.Data(), SelectedIndex: -1}}
	u.ClearTaskHistory()
	u.OpenPath("")
	if len(u.tasks.All()) != 0 {
		t.Errorf("a window with no service changed the tasks")
	}
}

// Settings are a window of their own, not a page: the sidebar entry opens it and
// the page the window behind is showing does not change.
func TestSidebarSettingsOpensTheWindow(t *testing.T) {
	u := uiWithPages()
	opened := 0
	u.openSettings = func() { opened++ }
	tt := render(t, u.left, 260, 520)

	if _, ok := tt.Find(locales.NavSettings()); !ok {
		t.Fatalf("the settings entry is missing: %v", tt.Texts())
	}
	if err := tt.Click(locales.NavSettings()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if opened != 1 {
		t.Fatalf("opened %d times, want 1", opened)
	}
	if u.page != PagePackages {
		t.Errorf("the settings entry moved the window to page %v", u.page)
	}
}

// Cmd+, opens it too, which is where a user looks for settings after the first
// time. It works with nothing focused and from a nested content tree, which is
// what the shell builds: a shortcut scoped to the content column would stay
// silent until something in it took the focus.
func TestSettingsShortcutOpensTheWindow(t *testing.T) {
	u := uiWithPages()
	u.tasks = &taskspage.View{SelectedIndex: -1}
	opened := 0
	u.openSettings = func() { opened++ }
	tt := render(t, func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { u.content(c) })
	}, 900, 520)

	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	if opened != 1 {
		t.Fatalf("the shortcut opened the window %d times, want 1", opened)
	}
}

// Writes that arrive while one is running queue behind it and run in the order
// they were made: nothing is dropped, and no control is disabled while a write
// runs — disabling the page on every change is what made it flash.
func TestQueuedSettingsWritesRunInTheOrderTheyWereMade(t *testing.T) {
	svc := &recordingService{calls: make(chan string, 8)}
	u := New(svc)
	if u.shell != nil {
		t.Fatal("New built a shell outside Run")
	}
	if u.saving {
		t.Fatal("no write is running yet")
	}

	// The first write is in flight; two more arrive while it runs.
	u.saving = true
	u.SetNotifications(tasks.KindInstall, true, true)
	u.SetNotifications(tasks.KindUpdate, true, true)
	select {
	case got := <-svc.calls:
		t.Fatalf("a queued write ran early: %q", got)
	default:
	}
	if len(u.pendingSaves) != 2 {
		t.Fatalf("%d writes queued, want 2", len(u.pendingSaves))
	}

	// The in-flight write lands; the queue runs one at a time, in order.
	u.saving = false
	u.runPendingSaves()
	for _, want := range []string{"install", "update"} {
		select {
		case got := <-svc.calls:
			if got != want {
				t.Fatalf("queued writes ran as %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("the queued write %q never ran", want)
		}
	}
	select {
	case got := <-svc.calls:
		t.Fatalf("an extra write ran: %q", got)
	default:
	}
	if u.saving {
		t.Error("a write is still running after the queue drained")
	}
}

// recordingService records the writes it is asked for, one signal per call, so
// the test can read them in order.
type recordingService struct {
	wiredService
	calls chan string
}

func (s *recordingService) SetNotifications(kind tasks.Kind, succeeded, on bool) error {
	s.calls <- string(kind)
	return nil
}
