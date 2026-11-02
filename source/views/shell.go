// Package views holds UniPac's native interface: the window shell, one builder
// per page, and the retained state each page shows.
//
// Builders run on the UI thread for every frame. They must not perform I/O,
// start work, or keep elements across passes; application state is loaded into
// the structs here before the window opens and passed by pointer.
package views

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
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

// UI is the retained state of the main window.
type UI struct {
	svc   Service
	shell *sidebar.Shell

	page    Page
	history []Page
	cursor  int

	data      *data
	installed *listView
	outdated  *listView
	search    *searchView
	tasks     *tasksView
	settings  *settingsView

	// quitPrompting keeps one quit dialog open at a time. quitAccepted is set
	// once the user chose to leave anyway, so the close and quit handlers stop
	// standing in the way. Both are touched from the quit goroutine.
	quitPrompting atomic.Bool
	quitAccepted  atomic.Bool

	// jobTarget is the title bar's tasks button rectangle, laid out each frame;
	// jobFlight carries a package icon from an action's control to it.
	jobTarget ui.Rect
	jobFlight flight

	// knownTasks is the last state seen for each task, so the listener can tell
	// a task that just finished from one whose history was loaded.
	knownTasks map[string]tasks.State
}

// New builds the window state. Everything the pages need is loaded here, so no
// builder has to touch the backend.
func New(service Service) *UI {
	u := &UI{
		svc:     service,
		data:    newData(service),
		history: []Page{PagePackages},
	}
	u.installed = &listView{
		data:         u.data,
		chosen:       map[string]struct{}{},
		allowRemove:  true,
		onRefresh:    u.Refresh,
		onUpdate:     u.Update,
		onRemove:     u.Remove,
		onBulkUpdate: u.BulkUpdate,
		onBulkRemove: u.BulkRemove,
	}
	u.outdated = &listView{
		data:         u.data,
		chosen:       map[string]struct{}{},
		outdatedOnly: true,
		onRefresh:    u.Refresh,
		onUpdate:     u.Update,
		onBulkUpdate: u.BulkUpdate,
	}
	u.search = &searchView{
		data:      u.data,
		submit:    u.Search,
		onInstall: u.Install,
	}
	u.tasks = &tasksView{
		data:          u.data,
		selectedIndex: -1,
		splitPercent:  64,
		onRetry:       u.RetryTask,
		onCancel:      u.CancelTask,
		onSelect:      u.loadTaskOutput,
	}
	u.tasks.apply(service.ListTasks())
	u.knownTasks = taskStates(u.tasks.all)
	u.settings = newSettings(service, u.data)
	u.settings.onSetEnabled = u.SetManagerEnabled
	u.settings.onSetPath = u.SetManagerExecPath
	u.settings.onReload = u.ReloadManagers
	u.settings.onSetAutoScan = u.SetAutoScanOnLaunch
	u.settings.onSetLogLevel = u.SetLogLevel
	u.settings.onSetLanguage = u.SetLanguage
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
			u.tasks.apply(list)
			u.announceTaskResults(previous, list)
			u.knownTasks = taskStates(list)
			// A refresh started while another task for the same manager was still
			// running could race its package manager. Wait for the last task to
			// finish, then scan once.
			if unfinishedStates(previous) && !u.hasUnfinishedTasks() && !u.quitAccepted.Load() {
				u.Refresh()
			}
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
	if u.settings.autoScan {
		u.Refresh()
	}
}

// Refresh rescans installed packages off the UI thread and publishes the rows
// through Window.Update. A request that arrives mid-scan is remembered and runs
// once more when it finishes, so an action that lands during a scan is not left
// showing pre-action rows; only one extra scan is queued, never a stack.
func (u *UI) Refresh() {
	if u.shell == nil {
		return
	}
	if !u.data.beginRefresh() {
		return
	}
	slog.Debug("views: refresh started")
	win := u.shell.Window()
	go func() {
		refreshErr := u.svc.UpdateCache()
		rows, loadErr := u.svc.GetCachedPackages()
		win.Update(func() {
			if u.data.finishRefresh(rows, refreshErr, loadErr) {
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
	u.search.seq++
	seq := u.search.seq
	u.search.searching = true
	u.search.err = nil
	win := u.shell.Window()
	go func() {
		results, err := u.svc.SearchPackages(query, managerIDs)
		win.Update(func() {
			if seq != u.search.seq {
				return
			}
			u.search.apply(query, results, err)
		})
	}()
}

// Install, Update and Remove run one package operation and refresh the
// inventory when it finishes. Operations on different packages run
// independently; the backend serializes one manager at a time.
func (u *UI) Install(pkg manager.Package, at actionOrigin) {
	u.runAction("install", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
		return u.svc.InstallPackage(pkg, manager.ActionOptions{})
	}, locales.ToastInstalled())
}

func (u *UI) Update(pkg manager.Package, at actionOrigin) {
	u.runAction("update", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
		return u.svc.UpdatePackage(pkg, manager.ActionOptions{})
	}, locales.ToastUpdated())
}

func (u *UI) Remove(pkg manager.Package, at actionOrigin) {
	u.runAction("remove", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
		return u.svc.UninstallPackage(pkg, manager.ActionOptions{})
	}, locales.ToastRemoved())
}

func (u *UI) BulkUpdate(pkgs []manager.Package, at actionOrigin) {
	u.runAction("update", pkgs, at, func() (manager.ActionResult, error) {
		return u.svc.BatchUpdatePackages(pkgs, manager.ActionOptions{})
	}, locales.ToastUpdated())
}

func (u *UI) BulkRemove(pkgs []manager.Package, at actionOrigin) {
	u.runAction("remove", pkgs, at, func() (manager.ActionResult, error) {
		return u.svc.BatchUninstallPackages(pkgs, manager.ActionOptions{})
	}, locales.ToastRemoved())
}

// runAction runs an operation off the UI thread and marks its packages until it
// finishes, so their rows wait while other packages stay usable. The outcome
// becomes a toast, queued for the next frame because MyGo adds toasts while
// building.
func (u *UI) runAction(op string, pkgs []manager.Package, at actionOrigin, fn func() (manager.ActionResult, error), done string) {
	if u.shell == nil || !u.data.beginAction(op, pkgs) {
		return
	}
	u.flyToTasks(at)
	win := u.shell.Window()
	go func() {
		result, err := fn()
		win.Update(func() {
			u.data.finishAction(pkgs)
			switch {
			case err != nil:
				u.data.queueToast(err.Error(), components.ToastError)
			case result.Message == tasks.CancelledMessage:
				// The cancel action already told the user.
			case !result.Success:
				u.data.queueToast(result.Message, components.ToastError)
			default:
				u.data.queueToast(done, components.ToastSuccess)
			}
			// Do not scan while another task may still be running for the same
			// manager; the task listener refreshes when the last one finishes.
			if !u.hasUnfinishedTasks() {
				u.Refresh()
			}
		})
	}()
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
		u.tasks.apply(u.svc.ListTasks())
	}
}

// CancelTask stops one running or queued task and tells the user. The task
// listener moves the row to cancelled; the action's goroutine returns.
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
				u.data.queueToast(message, components.ToastError)
				return
			}
			u.data.queueToast(locales.ToastCancelled(), components.ToastInfo)
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
		u.tasks.output = tasks.Output{}
		u.tasks.outputFor = ""
		u.tasks.outputSeq = 0
		u.tasks.outputLoading = false
		return
	}
	task, ok := u.tasks.taskByID(id)
	if !ok {
		return
	}
	if u.tasks.outputFor == id && u.tasks.outputLoading {
		return
	}
	if u.tasks.outputFor == id && u.tasks.outputSeq == task.OutputSeq {
		return
	}
	u.tasks.output = tasks.Output{}
	u.tasks.outputFor = id
	u.tasks.outputSeq = task.OutputSeq
	u.tasks.outputLoading = true
	win := u.shell.Window()
	go func() {
		output := u.svc.TaskOutput(id)
		win.Update(func() {
			if u.tasks.outputFor != id {
				return
			}
			u.tasks.output = output
			u.tasks.outputLoading = false
		})
	}()
}

// openTask brings a task from a notification click into view.
func (u *UI) openTask(id string) {
	if u.svc == nil {
		return
	}
	u.tasks.apply(u.svc.ListTasks())
	if !u.tasks.reveal(id) {
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
	operation := kindLabel(task.Kind)
	title := locales.NotificationTaskSucceeded(operation)
	body := task.Package
	if task.ManagerName != "" {
		body += " · " + task.ManagerName
	}
	if task.State == tasks.StateFailed {
		title = locales.NotificationTaskFailed(operation)
		if message := taskMessage(task); message != "" {
			body += " — " + truncateMessage(message, 140)
		}
	}
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

func unfinishedStates(states map[string]tasks.State) bool {
	for _, state := range states {
		if state == tasks.StateQueued || state == tasks.StateRunning {
			return true
		}
	}
	return false
}

func (u *UI) hasUnfinishedTasks() bool {
	return u.svc != nil && u.svc.UnfinishedTasks() > 0
}

func truncateMessage(message string, limit int) string {
	runes := []rune(message)
	if len(runes) <= limit {
		return message
	}
	return string(runes[:limit]) + "…"
}

// RetryTask runs a failed task's operation again. The new attempt is a new
// task: the failed one keeps its outcome and console.
func (u *UI) RetryTask(task tasks.Snapshot, at actionOrigin) {
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
		IsGUI:         task.IsGUI,
	}
	switch task.Kind {
	case tasks.KindInstall:
		u.runAction("install", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
			return u.svc.InstallPackage(pkg, opt)
		}, locales.ToastInstalled())
	case tasks.KindUpdate:
		u.runAction("update", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
			return u.svc.UpdatePackage(pkg, opt)
		}, locales.ToastUpdated())
	case tasks.KindUninstall:
		u.runAction("remove", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
			return u.svc.UninstallPackage(pkg, opt)
		}, locales.ToastRemoved())
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
// tasks run. Its rectangle is the destination of the flight from an action.
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
	for _, pkg := range u.data.rows {
		if pkg.Outdated && u.data.enabled(pkg.Manager) {
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
	return u.tasks.runningCount()
}

func (u *UI) content(c *ui.Context) {
	for _, t := range u.data.drainToasts() {
		c.AddToast(ui.Toast{Title: t.title, Type: t.tone})
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
			switch u.page {
			case PagePackages:
				u.installed.view(c)
			case PageOutdated:
				u.outdated.view(c)
			case PageSearch:
				u.search.view(c)
			case PageTasks:
				u.tasks.view(c)
			case PageSettings:
				u.settings.view(c)
			}
		})
		u.flightOverlay(c, root)
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
	if s.busy {
		// A second change would race the config write. Put the control back
		// where the saved state says it is instead of silently dropping it.
		s.invalidate()
		return
	}
	s.busy = true
	s.err = ""
	win := u.shell.Window()
	go func() {
		err := fn()
		managers := u.svc.ListManagers()
		overrides := u.svc.ManagerOverrides()
		autoScan := u.svc.AutoScanOnLaunch()
		level := u.svc.LogLevel()
		language := u.svc.Language()
		win.Update(func() {
			s.busy = false
			s.detecting = false
			// These come from the backend either way, so a failed save puts the
			// startup switch and the log level back with the manager rows.
			s.autoScan = autoScan
			s.logLevel = level
			s.language = language
			if err != nil {
				s.err = err.Error()
				s.invalidate()
				return
			}
			u.data.setManagers(managers, overrides)
			if rescan {
				u.Refresh()
			}
		})
	}()
}
