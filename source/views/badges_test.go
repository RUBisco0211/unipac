package views

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
)

func uiWithRunningTasks() *UI {
	u := &UI{
		data:    fixture(),
		history: []Page{PagePackages},
	}
	u.tasks = &tasksView{selectedIndex: -1}
	list := tasksFixture()
	// Two running tasks, one queued and one failed: the badges count running
	// tasks only.
	list[1].State = tasks.StateRunning
	list = append(list, tasks.Snapshot{
		ID: "4", Kind: tasks.KindInstall, State: tasks.StateRunning,
		ManagerName: "Cargo", Package: "ripgrep", StartedAt: time.Now(), OutputKept: true,
	})
	u.tasks.apply(list)
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
	u.flyToTasks(actionOrigin{X: 100, Y: 300})
	if !u.jobFlight.active {
		t.Fatal("the flight did not start")
	}

	tt := ui.NewTester(func(c *ui.Context) {
		root := ui.Column(c).Fill()
		root.Children(func() { u.flightOverlay(c, root) })
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
	if u.jobFlight.active {
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

	if !exactText(tt, "3") || !exactText(tt, "1") {
		t.Fatalf("initial badges missing: %v", tt.Texts())
	}

	// The outdated package became current.
	for i := range u.data.rows {
		u.data.rows[i].Outdated = false
	}
	tt.Frame()
	if exactText(tt, "1") {
		t.Errorf("outdated badge survived a refresh: %v", tt.Texts())
	}
	if !exactText(tt, "3") {
		t.Errorf("running task badge disappeared: %v", tt.Texts())
	}

	// All but one task finished.
	u.tasks.apply(tasksFixture())
	tt.Frame()
	if exactText(tt, "3") {
		t.Errorf("running task badge did not decrease: %v", tt.Texts())
	}
	if !exactText(tt, "1") {
		t.Errorf("running task badge missing: %v", tt.Texts())
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
