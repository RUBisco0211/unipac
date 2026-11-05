// Package views holds UniPac's native interface: the window shell that composes
// the pages, the background jobs they start, and the quit and notification
// handling around them. The pages themselves are separate packages; see
// views/inventory, views/parts, views/list, views/search, views/settings and
// views/tasks.
//
// Builders run on the UI thread for every frame. They must not perform I/O,
// start work, or keep elements across passes; application state is loaded into
// the page structs before the window opens and passed by pointer.
package views

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/list"
	"github.com/RUBisco0211/unipac/source/views/search"
	"github.com/RUBisco0211/unipac/source/views/settings"
	taskspage "github.com/RUBisco0211/unipac/source/views/tasks"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"
	"github.com/oneo-me/onevra/source/theme"
)

// Page identifies the page shown in the content area.
type Page int

const (
	PagePackages Page = iota
	PageOutdated
	PageSearch
	PageTasks
	PageSettings
)

// pageTransition is how the content area changes page: the arriving page fades
// in from just below, and the page that leaves is gone at once.
//
// There is no Exit on purpose. MyGo's exit draws a copy of the element tree, not
// the frame it showed: the copy is laid out again as an absolute element, and a
// virtualized table does not keep its width in it, so the old page flashed with
// a narrower table on the way out. MyGo's own router refuses the same thing for
// the same reason, and fades only the page coming in.
//
// Colors is set alone so that Position and Size stay out: a page whose table
// finds its width a frame after the switch must take that width at once, not
// grow into it for the whole duration.
var pageTransition = ui.ElementTransition{
	Duration: 200 * time.Millisecond,
	Colors:   true,
	Enter:    &ui.Motion{Opacity: 0, Y: 8},
}

// pageKey is a page's identity in the element tree. It changes with the page, so
// a switch plays one page leaving and another arriving instead of reusing one
// element for both.
func pageKey(p Page) string {
	return "page:" + strconv.Itoa(int(p))
}

// UI is the retained state of the main window.
type UI struct {
	svc   inventory.Service
	shell *sidebar.Shell

	page    Page
	history []Page
	cursor  int

	data      *inventory.Data
	installed *list.View
	outdated  *list.View
	search    *search.View
	tasks     *taskspage.View
	settings  *settings.View

	// quitPrompting keeps one quit dialog open at a time. quitAccepted is set
	// once the user chose to leave anyway, so the close and quit handlers stop
	// standing in the way. Both are touched from the quit goroutine.
	quitPrompting atomic.Bool
	quitAccepted  atomic.Bool

	// jobTarget is the title bar's tasks button rectangle, laid out each frame;
	// jobFlight carries a package icon from a task's control to it.
	jobTarget ui.Rect
	jobFlight flight.Flight

	// knownTasks is the last state seen for each task, so the listener can tell
	// a task that just finished from one whose history was loaded.
	knownTasks map[string]tasks.State
}

// New builds the window state. Everything the pages need is loaded here, so no
// builder has to touch the backend.
func New(service inventory.Service) *UI {
	u := &UI{
		svc:     service,
		data:    inventory.New(service),
		history: []Page{PagePackages},
	}
	u.installed = &list.View{
		Data:         u.data,
		AllowRemove:  true,
		OnRefresh:    u.Refresh,
		OnUpdate:     u.Update,
		OnRemove:     u.Remove,
		OnBulkUpdate: u.BulkUpdate,
		OnBulkRemove: u.BulkRemove,
	}
	u.outdated = &list.View{
		Data:         u.data,
		OutdatedOnly: true,
		OnRefresh:    u.Refresh,
		OnUpdate:     u.Update,
		OnBulkUpdate: u.BulkUpdate,
	}
	u.search = &search.View{
		Data:      u.data,
		Submit:    u.Search,
		OnInstall: u.Install,
	}
	u.tasks = &taskspage.View{
		Data:          u.data,
		SelectedIndex: -1,
		SplitPercent:  64,
		OnRetry:       u.RetryTask,
		OnCancel:      u.CancelTask,
		OnSelect:      u.loadTaskOutput,
	}
	u.tasks.Apply(service.ListTasks())
	u.knownTasks = taskStates(u.tasks.All())
	u.settings = settings.New(service, u.data)
	u.settings.OnSetEnabled = u.SetManagerEnabled
	u.settings.OnSetPath = u.SetManagerExecPath
	u.settings.OnReload = u.ReloadManagers
	u.settings.OnSetAutoScan = u.SetAutoScanOnLaunch
	u.settings.OnSetLogLevel = u.SetLogLevel
	u.settings.OnSetLanguage = u.SetLanguage
	return u
}

// Run creates the main window. Call it on the UI thread from WhenReady.
func (u *UI) Run() {
	u.shell = sidebar.NewShell(sidebar.ShellOptions{
		Title:        "UniPac",
		Theme:        theme.Onevra,
		StateKey:     "main",
		MinWidth:     1024,
		MinHeight:    620,
		LeftMinWidth: 220,
		LeftMaxWidth: 320,
		Left:         u.left,
		LeftHeader:   u.leftHeader,
		ContentTitle: u.windowTitle,
		Content:      u.content,
	})
	win := u.shell.Window()
	win.OnClose(func(e *mygo.CloseEvent) {
		if u.quitAccepted.Load() {
			return
		}
		if pending := u.svc.UnfinishedTasks(); pending > 0 {
			e.PreventDefault()
			u.askToQuit(pending)
		}
	})
	// Cmd+Q and the app menu quit without asking the window first.
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if u.quitAccepted.Load() {
			return
		}
		if pending := u.svc.UnfinishedTasks(); pending > 0 {
			e.PreventDefault()
			u.askToQuit(pending)
		}
	})
	win.OnClosed(func() { mygo.App.Quit() })
	// Task output arrives off the UI thread; the listener posts one repaint for
	// each burst. It captures the window here, because u.shell is only written
	// on the UI thread.
	u.svc.SetTaskListener(func() {
		list := u.svc.ListTasks()
		win.Update(func() {
			previous := u.knownTasks
			u.tasks.Apply(list)
			u.announceTaskResults(previous, list)
			u.knownTasks = taskStates(list)
		})
	})
	mygo.App.OnNotificationClick(func(id string) {
		taskID, ok := strings.CutPrefix(id, "unipac-task-")
		if !ok {
			return
		}
		win.Update(func() { u.openTask(taskID) })
	})
	slog.Debug("views: main window created")
	if u.settings.AutoScan {
		u.Refresh()
	}
}

// Refresh rescans installed packages off the UI thread and publishes the rows
// through Window.Update. A request that arrives mid-scan is remembered and runs
// once more when it finishes, so a task that lands during a scan is not left
// showing pre-task rows; only one extra scan is queued, never a stack.
func (u *UI) Refresh() {
	if u.shell == nil {
		return
	}
	if !u.data.BeginRefresh() {
		return
	}
	slog.Debug("views: refresh started")
	win := u.shell.Window()
	go func() {
		refreshErr := u.svc.UpdateCache()
		rows, loadErr := u.svc.GetCachedPackages()
		win.Update(func() {
			if u.data.FinishRefresh(rows, refreshErr, loadErr) {
				u.Refresh()
			}
		})
	}()
}

// Search runs a keyword search off the UI thread and publishes the results.
// managerIDs narrows it in the backend; nil asks every enabled manager.
func (u *UI) Search(query string, managerIDs []string) {
	if u.shell == nil || query == "" {
		return
	}
	// A newer query wins: the sequence number drops a slow earlier result.
	seq := u.search.BeginQuery()
	win := u.shell.Window()
	go func() {
		results, err := u.svc.SearchPackages(query, managerIDs)
		win.Update(func() {
			u.search.Apply(seq, query, results, err)
		})
	}()
}

// Install, Update and Remove run one package operation and refresh the
// inventory when it finishes. Operations on different packages run
// independently; the backend serializes one manager at a time.
func (u *UI) Install(pkg manager.Package, at flight.Origin) {
	u.runTask("install", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
		return u.svc.InstallPackage(pkg, manager.ActionOptions{})
	})
}

func (u *UI) Update(pkg manager.Package, at flight.Origin) {
	u.runTask("update", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
		return u.svc.UpdatePackage(pkg, manager.ActionOptions{})
	})
}

func (u *UI) Remove(pkg manager.Package, at flight.Origin) {
	u.runTask("remove", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
		return u.svc.UninstallPackage(pkg, manager.ActionOptions{})
	})
}

func (u *UI) BulkUpdate(pkgs []manager.Package, at flight.Origin) {
	u.runTask("update", pkgs, at, func() (manager.ActionResult, error) {
		return u.svc.BatchUpdatePackages(pkgs, manager.ActionOptions{})
	})
}

func (u *UI) BulkRemove(pkgs []manager.Package, at flight.Origin) {
	u.runTask("remove", pkgs, at, func() (manager.ActionResult, error) {
		return u.svc.BatchUninstallPackages(pkgs, manager.ActionOptions{})
	})
}

// runTask runs one package operation off the UI thread and marks its packages as
// busy until it finishes, so their rows wait while other packages stay usable.
// The outcome becomes a toast, queued for the next frame because MyGo adds
// toasts while building.
func (u *UI) runTask(op string, pkgs []manager.Package, at flight.Origin, fn func() (manager.ActionResult, error)) {
	if u.shell == nil || !u.data.BeginTask(op, pkgs) {
		return
	}
	u.jobFlight.ToTasks(at, u.jobTarget)
	win := u.shell.Window()
	go func() {
		result, err := fn()
		// The task exists as soon as the action returns, so the toast can point
		// at it and the console it kept. Asked off the UI thread: ListTasks
		// copies its snapshots under its own lock.
		failed := err != nil || !result.Success
		var taskID string
		if failed && u.svc != nil {
			taskID = newestTask(u.svc.ListTasks(), pkgs)
		}
		win.Update(func() {
			u.data.EndTask(pkgs)
			switch {
			case result.Message == tasks.CancelledMessage && err == nil:
				// The cancel action already told the user.
			case failed:
				u.data.QueueToast(failureToast(op, pkgs, taskID))
			default:
				u.data.QueueToast(inventory.Toast{Title: successLabel(taskKind(op)), Tone: components.ToastSuccess})
			}
			if !u.quitAccepted.Load() {
				u.Refresh()
			}
		})
	}()
}

// toastFor turns a queued message into the toast that shows it. One about a task
// offers to open it: the message is the outcome, and the console behind it is
// the task's, not something to repeat in a toast.
func (u *UI) toastFor(t inventory.Toast) ui.Toast {
	toast := ui.Toast{Title: t.Title, Type: t.Tone}
	if t.TaskID != "" {
		id := t.TaskID
		toast.Action = locales.ToastShowTask()
		toast.OnAction = func() { u.openTask(id) }
	}
	return toast
}

// failureToast is what a failed action says: which operation failed on what, and
// the way to its console. The command's output is the task's, not the toast's;
// it is often dozens of lines whose first ones are warnings.
func failureToast(op string, pkgs []manager.Package, taskID string) inventory.Toast {
	subject := ""
	switch len(pkgs) {
	case 1:
		subject = pkgs[0].Name
	default:
		subject = locales.ToastSeveralPackages(strconv.Itoa(len(pkgs)))
	}
	return inventory.Toast{
		Title:  failureLabel(taskKind(op), subject),
		Tone:   components.ToastError,
		TaskID: taskID,
	}
}

// newestTask is the task an action just created, found among the tasks of the
// packages it ran on: the action has returned, so its task is the newest one
// those packages have.
func newestTask(list []tasks.Snapshot, pkgs []manager.Package) string {
	type target struct{ manager, pkg string }
	want := make(map[target]struct{}, len(pkgs))
	for _, pkg := range pkgs {
		want[target{pkg.Manager, pkg.Name}] = struct{}{}
	}
	best, bestID := "", -1
	for _, task := range list {
		if _, ok := want[target{task.ManagerID, task.Package}]; !ok {
			continue
		}
		id, err := strconv.Atoi(task.ID)
		if err != nil {
			if best == "" {
				best = task.ID
			}
			continue
		}
		if id > bestID {
			best, bestID = task.ID, id
		}
	}
	return best
}

// navigate moves to a page and records it, so back and forward can replay the
// visits the way the old title bar's arrows did.
func (u *UI) navigate(page Page) {
	if page == u.page {
		return
	}
	if u.cursor < len(u.history)-1 {
		u.history = u.history[:u.cursor+1]
	}
	u.history = append(u.history, page)
	u.cursor = len(u.history) - 1
	u.page = page
	if page == PageTasks && u.svc != nil {
		// The listener updates the page from any page; reloading on entry as
		// well makes the page independent of one call having landed.
		u.tasks.Apply(u.svc.ListTasks())
	}
}

// CancelTask stops one running or queued task and tells the user. The task
// listener moves the row to cancelled; the task's goroutine returns.
func (u *UI) CancelTask(task tasks.Snapshot) {
	if u.svc == nil || u.shell == nil {
		return
	}
	win := u.shell.Window()
	go func() {
		err := u.svc.CancelTask(task.ID)
		win.Update(func() {
			if err != nil {
				message := err.Error()
				switch {
				case errors.Is(err, tasks.ErrTaskNotFound):
					message = locales.TasksCancelMissing()
				case errors.Is(err, tasks.ErrTaskNotRunning):
					message = locales.TasksCancelFinished()
				}
				u.data.QueueToast(inventory.Toast{Title: message, Tone: components.ToastError})
				return
			}
			u.data.QueueToast(inventory.Toast{Title: locales.ToastCancelled(), Tone: components.ToastInfo})
		})
	}()
}

// loadTaskOutput fetches the selected task's console. Live output is loaded
// the same way as history, so a task listener update copies one selected
// console rather than every running task's full output on every frame.
func (u *UI) loadTaskOutput(id string) {
	if u.svc == nil || u.shell == nil {
		return
	}
	if id == "" {
		u.tasks.ResetOutput()
		return
	}
	task, ok := u.tasks.TaskByID(id)
	if !ok {
		return
	}
	request, fetch := u.tasks.BeginOutput(id, task.OutputSeq)
	if !fetch {
		return
	}
	win := u.shell.Window()
	go func() {
		output := u.svc.TaskOutput(id)
		win.Update(func() {
			if u.tasks.FinishOutput(id, request, output) {
				u.loadTaskOutput(id)
			}
		})
	}()
}

// openTask brings a task from a notification click into view.
func (u *UI) openTask(id string) {
	if u.svc == nil {
		return
	}
	u.tasks.Apply(u.svc.ListTasks())
	if !u.tasks.Reveal(id) {
		return
	}
	u.navigate(PageTasks)
	u.loadTaskOutput(id)
}

// announceTaskResults shows a system notification for each task that finished
// since the last listener update. Cancelled tasks are quiet: the user stopped
// them, or the app is quitting.
func (u *UI) announceTaskResults(previous map[string]tasks.State, list []tasks.Snapshot) {
	if u.quitAccepted.Load() {
		return
	}
	for _, task := range finishedTasks(previous, list) {
		u.notifyTask(task)
	}
}

// finishedTasks are the tasks that succeeded or failed since the previous
// snapshot. History loaded at startup is in previous, so it stays quiet; a task
// that came and went between two listener updates is new and does notify.
func finishedTasks(previous map[string]tasks.State, list []tasks.Snapshot) []tasks.Snapshot {
	var out []tasks.Snapshot
	for _, task := range list {
		if task.State != tasks.StateSucceeded && task.State != tasks.StateFailed {
			continue
		}
		state, seen := previous[task.ID]
		if seen && state != tasks.StateQueued && state != tasks.StateRunning {
			continue
		}
		out = append(out, task)
	}
	return out
}

func (u *UI) notifyTask(task tasks.Snapshot) {
	title, body := notificationText(task)
	notification := mygo.NewNotification(mygo.NotificationOptions{
		ID:    "unipac-task-" + task.ID,
		Title: title,
		Body:  body,
		Group: "unipac.tasks",
	})
	go func() {
		if !mygo.NotificationsSupported() {
			return
		}
		if err := notification.Show(); err != nil && !errors.Is(err, mygo.ErrNotificationsDenied) {
			slog.Debug("views: task notification failed", "error", err)
		}
	}()
}

func taskStates(list []tasks.Snapshot) map[string]tasks.State {
	states := make(map[string]tasks.State, len(list))
	for _, task := range list {
		states[task.ID] = task.State
	}
	return states
}

// notificationText says what the toast says, then what the task was about. The
// command's output stays in the task: a notification shows a line, not a
// console, and clicking it opens that task.
func notificationText(task tasks.Snapshot) (title, body string) {
	title = successLabel(task.Kind)
	if task.State == tasks.StateFailed {
		title = failureLabel(task.Kind, task.Package)
	}
	body = task.Package
	if task.ManagerName != "" {
		body += " · " + task.ManagerName
	}
	return title, body
}

// taskKind is the task an action records. The op strings are what the pages use
// to mark a row busy.
func taskKind(op string) tasks.Kind {
	switch op {
	case "install":
		return tasks.KindInstall
	case "remove":
		return tasks.KindUninstall
	default:
		return tasks.KindUpdate
	}
}

// successLabel and failureLabel word an outcome once, for the toast and the
// notification, so the two never disagree.
func successLabel(kind tasks.Kind) string {
	switch kind {
	case tasks.KindInstall:
		return locales.ToastInstalled()
	case tasks.KindUninstall:
		return locales.ToastRemoved()
	default:
		return locales.ToastUpdated()
	}
}

func failureLabel(kind tasks.Kind, name string) string {
	switch kind {
	case tasks.KindInstall:
		return locales.ToastInstallFailed(name)
	case tasks.KindUninstall:
		return locales.ToastRemoveFailed(name)
	default:
		return locales.ToastUpdateFailed(name)
	}
}

// RetryTask runs a failed task's operation again. The new attempt is a new
// task: the failed one keeps its outcome and console.
func (u *UI) RetryTask(task tasks.Snapshot, at flight.Origin) {
	if u.svc == nil {
		return
	}
	version := task.NewVersion
	if version == "" {
		version = task.Version
	}
	opt := manager.ActionOptions{Version: version}
	pkg := manager.Package{
		Manager:       task.ManagerID,
		Name:          task.Package,
		Version:       task.Version,
		LatestVersion: task.NewVersion,
	}
	switch task.Kind {
	case tasks.KindInstall:
		u.runTask("install", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
			return u.svc.InstallPackage(pkg, opt)
		})
	case tasks.KindUpdate:
		u.runTask("update", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
			return u.svc.UpdatePackage(pkg, opt)
		})
	case tasks.KindUninstall:
		u.runTask("remove", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
			return u.svc.UninstallPackage(pkg, opt)
		})
	}
}

// askToQuit asks before leaving with unfinished tasks. It uses a native dialog
// so the question is visible even when the window is minimized, and runs off
// the UI thread because the dialog blocks until it is answered.
func (u *UI) askToQuit(pending int) {
	if u.shell == nil || !u.quitPrompting.CompareAndSwap(false, true) {
		return
	}
	win := u.shell.Window()
	go func() {
		result, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:        win,
			Type:          mygo.MessageWarning,
			Message:       locales.TasksQuitTitle(),
			Detail:        locales.TasksQuitMessage(pending),
			Buttons:       []string{locales.CommonCancel(), locales.TasksQuitConfirm()},
			DefaultButton: 0,
			CancelButton:  0,
			// "Quit anyway" leaves unfinished work behind.
			DestructiveButtons: []int{1},
		})
		u.quitPrompting.Store(false)
		if err != nil || result.Button != 1 {
			return
		}
		u.quitAccepted.Store(true)
		u.svc.StopTasks()
		win.Update(func() { mygo.App.Quit() })
	}()
}

func (u *UI) back() {
	if u.cursor > 0 {
		u.cursor--
		u.page = u.history[u.cursor]
	}
}

func (u *UI) forward() {
	if u.cursor < len(u.history)-1 {
		u.cursor++
		u.page = u.history[u.cursor]
	}
}

// pageTitle is the title bar's label: the page the window is showing.
func (u *UI) pageTitle() string {
	switch u.page {
	case PageOutdated:
		return locales.NavOutdated()
	case PageSearch:
		return locales.NavSearch()
	case PageTasks:
		return locales.NavTasks()
	case PageSettings:
		return locales.NavSettings()
	default:
		return locales.NavPackages()
	}
}

// windowTitle is the title bar's content area: the history arrows and the page
// title on the leading edge, and the tasks button on the trailing edge. It
// stays visible when the sidebar is collapsed, which the sidebar header does
// not.
func (u *UI) windowTitle(c *ui.Context) {
	ui.Row(c).FillWidth().FillHeight().AlignItems(ui.Center).Gap(2).Children(func() {
		back := components.IconButton(c, lucide.ChevronLeft(), locales.NavBack(), components.IconButtonPlain)
		back.Disabled(u.cursor <= 0)
		if back.Clicked() {
			u.back()
		}
		forward := components.IconButton(c, lucide.ChevronRight(), locales.NavForward(), components.IconButtonPlain)
		forward.Disabled(u.cursor >= len(u.history)-1)
		if forward.Clicked() {
			u.forward()
		}
		components.Text(c, u.pageTitle()).FontSize(13).Bold()
		ui.Spacer(c)
		u.tasksButton(c)
	})
}

// tasksButton is the title bar's shortcut to the tasks page, with a badge while
// tasks run. Its rectangle is the destination of the flight from a task.
func (u *UI) tasksButton(c *ui.Context) {
	t := c.Theme()
	box := ui.Box(c).Shrink(0).Width(t.Space(8)).Height(t.Space(7)).Center()
	box.Children(func() {
		button := components.IconButton(c, lucide.ListTodo(), locales.NavTasks(), components.IconButtonPlain)
		if button.Clicked() {
			u.navigate(PageTasks)
		}
		u.jobTarget = button.Bounds()
		countBadge(c, u.runningTasks())
	})
}

// leftHeader is the trailing part of the sidebar header, kept empty so the
// shell's toggle stands alone. The history arrows live in the title bar, which
// stays visible when the sidebar is collapsed.
func (u *UI) leftHeader(c *ui.Context) {}

func (u *UI) left(c *ui.Context) {
	// The sidebar paints no background of its own: the window's sidebar material
	// shows through it, which is the only thing that can blur the desktop. A
	// painted blur would sample the app's own transparent layer instead of what
	// is behind the window.
	// The inset lives on a wrapper: components.Sidebar has its own padding, and
	// applying one to it would replace that instead of adding to it.
	ui.Column(c).Fill().Padding(sidebar.HeaderHeight(c), 0, 6, 0).Children(func() {
		components.Sidebar(c).Children(func() {
			components.SidebarSection(c, locales.SidebarPackages(), func() {
				u.nav(c, PagePackages, locales.NavPackages(), lucide.Package(), 0)
				u.nav(c, PageOutdated, locales.NavOutdated(), lucide.CircleFadingArrowUp(), u.outdatedCount())
				u.nav(c, PageSearch, locales.NavSearch(), lucide.Search(), 0)
				u.nav(c, PageTasks, locales.NavTasks(), lucide.ListTodo(), u.runningTasks())
			})
			components.SidebarSection(c, locales.SidebarApp(), func() {
				u.nav(c, PageSettings, locales.NavSettings(), lucide.Settings(), 0)
			})
		})
	})
}

func (u *UI) nav(c *ui.Context, page Page, label string, icon *ui.SVG, count int) {
	// The count trails the label, so it sits on the item's right edge and the
	// label takes the room in between. The leading slot is only the icon.
	item := components.SidebarItemWithSlots(c, label, u.page == page, func(c *ui.Context, size float32) {
		components.Icon(c, icon).Size(size, size).TextColor(c.Theme().Accent)
	}, func(c *ui.Context, _ float32) {
		trailingCount(c, count)
	})
	if item.Clicked() {
		u.navigate(page)
	}
}

// trailingCount is a sidebar item's count badge, right-aligned after its
// label. It draws nothing for zero.
func trailingCount(c *ui.Context, count int) {
	if count <= 0 {
		return
	}
	t := c.Theme()
	label := strconv.Itoa(count)
	if count > 99 {
		label = "99+"
	}
	ui.Box(c).Shrink(0).Height(t.Space(3)).MinWidth(t.Space(3)).
		Padding(0, t.Space(0.75), 0, t.Space(0.75)).
		Radius(t.Space(1.5)).Background(t.Accent).Center().AlignSelf(ui.Center).
		Children(func() {
			components.Text(c, label).FontSize(8).Bold().TextColor(t.AccentText)
		})
}

// countBadge draws a small count on the top-right of the element it is built
// in, such as the title bar's tasks button. It draws nothing for zero.
func countBadge(c *ui.Context, count int) {
	if count <= 0 {
		return
	}
	t := c.Theme()
	label := strconv.Itoa(count)
	if count > 99 {
		label = "99+"
	}
	ui.Box(c).Absolute().Top(-1).Right(-1).PassThrough().
		Height(t.Space(2.5)).MinWidth(t.Space(2.5)).
		Padding(0, t.Space(0.5), 0, t.Space(0.5)).
		Radius(t.Space(1.25)).Background(t.Accent).Center().Children(func() {
		components.Text(c, label).FontSize(7).Bold().TextColor(t.AccentText)
	})
}

// outdatedCount is what the Outdated item's badge shows: packages with a newer
// version from a manager UniPac still uses.
func (u *UI) outdatedCount() int {
	if u.data == nil {
		return 0
	}
	n := 0
	for _, pkg := range u.data.Rows {
		if pkg.Outdated && u.data.Enabled(pkg.Manager) {
			n++
		}
	}
	return n
}

// runningTasks is what the Tasks item's and the title bar button's badges show.
func (u *UI) runningTasks() int {
	if u.tasks == nil {
		return 0
	}
	return u.tasks.RunningCount()
}

func (u *UI) content(c *ui.Context) {
	for _, t := range u.data.DrainToasts() {
		c.AddToast(u.toastFor(t))
	}
	// The tasks page keeps its own top inset: its console surface runs to the
	// window's top edge, like the sidebar's.
	top := sidebar.HeaderHeight(c) + 16
	if u.page == PageTasks {
		top = 0
	}
	root := ui.Column(c).Fill()
	root.Children(func() {
		ui.Column(c).Fill().Padding(top, 0, 0, 0).Children(func() {
			// The key follows the page, so MyGo reads a switch as this page
			// leaving and the next arriving.
			ui.Box(c).Key(pageKey(u.page)).Fill().Transition(pageTransition).Children(func() {
				switch u.page {
				case PagePackages:
					u.installed.Build(c)
				case PageOutdated:
					u.outdated.Build(c)
				case PageSearch:
					u.search.Build(c)
				case PageTasks:
					u.tasks.Build(c)
				case PageSettings:
					u.settings.Build(c)
				}
			})
		})
		u.jobFlight.Draw(c, root)
	})
}

// SetManagerEnabled turns one manager on or off. The switch has already moved,
// so the save runs off the UI thread and only the outcome is reported back.
func (u *UI) SetManagerEnabled(id string, enabled bool) {
	u.saveSettings(true, func() error {
		return u.svc.SetManagerEnabled(id, enabled)
	})
}

// SetManagerExecPath saves a manager's executable path. The preflight that
// follows runs in the job, never in the builder.
func (u *UI) SetManagerExecPath(id, path string) {
	u.saveSettings(true, func() error {
		return u.svc.SetManagerExecPath(id, path)
	})
}

// ReloadManagers preflights every manager again, for a tool installed while the
// app was open.
func (u *UI) ReloadManagers() {
	u.saveSettings(true, func() error {
		return u.svc.ReloadManagers()
	})
}

func (u *UI) SetAutoScanOnLaunch(on bool) {
	u.saveSettings(false, func() error {
		return u.svc.SetAutoScanOnLaunch(on)
	})
}

func (u *UI) SetLogLevel(level string) {
	u.saveSettings(false, func() error {
		return u.svc.SetLogLevel(level)
	})
}

func (u *UI) SetLanguage(tag string) {
	u.saveSettings(false, func() error {
		return u.svc.SetLanguage(tag)
	})
}

// saveSettings runs one settings write off the UI thread and publishes what the
// backend now reports. rescan asks for a new package scan, which is what makes
// a turned-off manager's rows leave the lists.
func (u *UI) saveSettings(rescan bool, fn func() error) {
	if u.shell == nil || u.settings == nil {
		return
	}
	s := u.settings
	if !s.BeginSave() {
		return
	}
	win := u.shell.Window()
	go func() {
		err := fn()
		managers := u.svc.ListManagers()
		overrides := u.svc.ManagerOverrides()
		saved := settings.Snapshot{
			AutoScan: u.svc.AutoScanOnLaunch(),
			LogLevel: u.svc.LogLevel(),
			Language: u.svc.Language(),
		}
		win.Update(func() {
			s.EndSave(err, saved)
			if err != nil {
				return
			}
			u.data.SetManagers(managers, overrides)
			if rescan {
				u.Refresh()
			}
		})
	}()
}
