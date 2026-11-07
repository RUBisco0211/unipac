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
	"github.com/RUBisco0211/unipac/source/views/panel"
	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/RUBisco0211/unipac/source/views/search"
	"github.com/RUBisco0211/unipac/source/views/settings"
	taskspage "github.com/RUBisco0211/unipac/source/views/tasks"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/plain"
	"github.com/oneo-me/onevra/source/shells/sidebar"
)

// Page identifies the page shown in the content area.
type Page int

const (
	PagePackages Page = iota
	PageOutdated
	PageSearch
	PageTasks
)

// toastOptions places the window's messages at the bottom right, out of the way
// of the title bar and the sidebar's own controls.
var toastOptions = components.ToastOptions{Placement: components.ToastBottomEnd}

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
	// settingsShell is the settings window, nil until it is first asked for and
	// again once it is closed. Settings are a config file changed now and then,
	// not a page of the package lists.
	settingsShell *plain.Shell
	// openSettings shows that window. It is a field rather than a method call so
	// a test can watch the sidebar item and the shortcut without a native window
	// behind them; Run installs it.
	openSettings func()

	// saving says a settings write is running, so a second change waits rather
	// than racing the config file. lastSnapshot is what the backend last
	// reported, which is what a dropped change's controls go back to.
	saving       bool
	lastSnapshot settings.Snapshot

	// quitOpen shows the window's own quit dialog, and quitPending says how many
	// tasks it is about. quitAccepted is set once the user chose to leave
	// anyway, so the close and quit handlers stop standing in the way.
	quitOpen     bool
	quitPending  int
	quitAccepted atomic.Bool
	// quitApp leaves the app. It is a field so a test can answer the dialog
	// without an event loop behind it.
	quitApp func()

	// jobTarget is the title bar's tasks button rectangle, laid out each frame;
	// jobFlight carries a package icon from a task's control to it.
	jobTarget ui.Rect
	jobFlight flight.Flight

	// knownTasks is the last state seen for each task, so the listener can tell
	// a task that just finished from one whose history was loaded.
	knownTasks map[string]tasks.State

	// announcedUpdates is how many updates the last system notification named, so
	// a scan that finds the same number again stays quiet.
	announcedUpdates int
	// notifyUpdates tells the user that this many packages have a newer version.
	// It is a field so a test can answer it without the system's notifications;
	// Run installs the real one.
	notifyUpdates func(count int)
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
		Data:             u.data,
		Submit:           u.Search,
		OnInstall:        u.Install,
		OnUpdate:         u.Update,
		OnVersions:       u.Versions,
		OnInstallVersion: u.InstallVersion,
	}
	u.tasks = &taskspage.View{
		Data:          u.data,
		SelectedIndex: -1,
		Console:       panel.State{Width: 380},
		OnRetry:       u.RetryTask,
		OnCancel:      u.CancelTask,
		OnSelect:      u.loadTaskOutput,
	}
	u.tasks.TitleActions = u.tasks.TitleAction
	u.tasks.Apply(service.ListTasks())
	u.knownTasks = taskStates(u.tasks.All())
	u.settings = settings.New(service, u.data)
	u.settings.OnSetAutoScan = u.SetAutoScanOnLaunch
	u.settings.OnSetLogLevel = u.SetLogLevel
	u.settings.OnSetLanguage = u.SetLanguage
	u.settings.OnSetTheme = u.SetTheme
	u.settings.OnSetNotifyEnabled = u.SetNotificationsEnabled
	u.settings.OnSetNotify = u.SetNotifications
	u.settings.OnSetNotifyUpdates = u.SetNotifyUpdates
	u.settings.OnSetAccent = u.SetAccent
	u.settings.OnSetManagerEnabled = u.SetManagerEnabled
	u.settings.OnSetManagerPath = u.SetManagerExecPath
	u.settings.OnReloadManagers = u.ReloadManagers
	u.settings.OnOpenPath = u.OpenPath
	u.settings.OnOpenURL = u.OpenURL
	u.settings.OnClearHistory = u.ClearTaskHistory
	holds := func() bool { return u.settings.HoldToConfirm }
	u.installed.HoldsToConfirm = holds
	u.outdated.HoldsToConfirm = holds
	u.settings.OnSetHold = u.SetHoldToConfirm
	// The controls bind to what the backend last reported, so a change dropped
	// because another write is running goes back to this.
	u.lastSnapshot = settings.Snapshot{
		AutoScan:      service.AutoScanOnLaunch(),
		LogLevel:      service.LogLevel(),
		Language:      service.Language(),
		Theme:         service.Theme(),
		Accent:        service.Accent(),
		HoldToConfirm: service.HoldToConfirm(),
		Notify:        settings.NotifyFrom(service),
	}
	return u
}

// Run creates the main window. Call it on the UI thread from WhenReady.
func (u *UI) Run() {
	u.shell = sidebar.NewShell(sidebar.ShellOptions{
		Title:        "UniPac",
		Theme:        u.theme,
		StateKey:     "main",
		MinWidth:     1024,
		MinHeight:    620,
		LeftMinWidth: 220,
		LeftMaxWidth: 320,
		Left:         u.left,
		LeftHeader:   u.leftHeader,
		ContentTitle: u.windowTitle,
		Content:      u.content,
		Toasts:       toastOptions,
	})
	win := u.shell.Window()
	u.openSettings = u.showSettings
	win.OnClose(func(e *mygo.CloseEvent) {
		if u.quitAccepted.Load() {
			return
		}
		pending := u.svc.UnfinishedTasks()
		slog.Info("views: window close requested", "pending", pending)
		if pending > 0 {
			e.PreventDefault()
			u.askToQuit(win, pending)
		}
	})
	// Cmd+Q and the app menu quit without asking the window first.
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if u.quitAccepted.Load() {
			return
		}
		pending := u.svc.UnfinishedTasks()
		slog.Info("views: quit requested", "pending", pending)
		if pending > 0 {
			e.PreventDefault()
			u.askToQuit(win, pending)
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
	u.notifyUpdates = u.systemNotifyUpdates
	mygo.App.OnNotificationClick(func(id string) {
		if id == "unipac-updates" {
			win.Update(func() { u.openOutdated() })
			return
		}
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
			// The settings window counts the same rows per manager, so a
			// refresh moves its numbers as well as the lists'.
			u.invalidateSettings()
			u.announceUpdatesAvailable()
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
		result, err := u.svc.SearchPackages(query, managerIDs)
		win.Update(func() {
			u.search.Apply(seq, query, result, err)
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

// InstallVersion installs the version chosen in the search page's dialog; an
// empty version asks for the newest, the way a plain install does.
func (u *UI) InstallVersion(pkg manager.Package, version string, at flight.Origin) {
	u.runTask("install", []manager.Package{pkg}, at, func() (manager.ActionResult, error) {
		return u.svc.InstallPackage(pkg, manager.ActionOptions{Version: version})
	})
}

// Versions looks up the versions a package can be installed at, off the UI
// thread, and hands the answer back to the search page with the sequence number
// it asked under. A failure is logged: the page shows a line, not the tool's
// output.
func (u *UI) Versions(pkg manager.Package, seq int) {
	if u.shell == nil || u.svc == nil {
		return
	}
	win := u.shell.Window()
	go func() {
		versions, err := u.svc.ListPackageVersions(pkg.Manager, pkg.Name)
		if err != nil {
			slog.Warn("views: listing versions failed", "manager", pkg.Manager, "package", pkg.Name, "error", err)
		}
		win.Update(func() {
			u.search.SetVersions(pkg, seq, versions, err)
		})
	}()
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

// openOutdated brings the Outdated page up, which is what the update
// notification is about.
func (u *UI) openOutdated() {
	if u.shell == nil {
		return
	}
	u.navigate(PageOutdated)
	win := u.shell.Window()
	win.Show()
	win.Focus()
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
// since the last listener update, unless Settings turned its operation off.
// Cancelled tasks are quiet: the user stopped them, or the app is quitting.
func (u *UI) announceTaskResults(previous map[string]tasks.State, list []tasks.Snapshot) {
	if u.quitAccepted.Load() {
		return
	}
	for _, task := range finishedTasks(previous, list) {
		if !u.notificationsOn(task.Kind, task.State == tasks.StateSucceeded) {
			continue
		}
		u.notifyTask(task)
	}
}

// notificationsOn reports whether a finished task of this kind and outcome should
// be announced. The setting follows the switches in Settings, and only system
// notifications answer to it: the window's toasts are not part of it.
func (u *UI) notificationsOn(kind tasks.Kind, succeeded bool) bool {
	return u.settings == nil || u.settings.Notifies(kind, succeeded)
}

// announceUpdatesAvailable says once, with a system notification, that a scan
// found packages with a newer version. It is not a toast: the window says what it
// just did, and this is for when nobody is looking at it. Announcing every scan
// would be noise, so it speaks when the number changes and stays quiet until it
// changes again.
func (u *UI) announceUpdatesAvailable() {
	if u.quitAccepted.Load() || u.settings == nil || !u.settings.NotifiesUpdates() {
		return
	}
	count := u.outdatedCount()
	if count == 0 {
		u.announcedUpdates = 0
		return
	}
	if count == u.announcedUpdates {
		return
	}
	u.announcedUpdates = count
	if u.notifyUpdates != nil {
		u.notifyUpdates(count)
	}
}

// systemNotifyUpdates tells the user that count packages have a newer version.
// Clicking it opens the Outdated page, which is the list it is about.
func (u *UI) systemNotifyUpdates(count int) {
	notification := mygo.NewNotification(mygo.NotificationOptions{
		ID:    "unipac-updates",
		Title: locales.NotificationUpdatesTitle(count),
		Body:  locales.NotificationUpdatesBody(),
		Group: "unipac.updates",
	})
	go func() {
		if !mygo.NotificationsSupported() {
			return
		}
		if err := notification.Show(); err != nil && !errors.Is(err, mygo.ErrNotificationsDenied) {
			slog.Debug("views: update notification failed", "error", err)
		}
	}()
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
		return locales.ToastUninstalled()
	default:
		return locales.ToastUpdated()
	}
}

func failureLabel(kind tasks.Kind, name string) string {
	switch kind {
	case tasks.KindInstall:
		return locales.ToastInstallFailed(name)
	case tasks.KindUninstall:
		return locales.ToastUninstallFailed(name)
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

// askToQuit opens the window's quit dialog for the tasks still running. The
// dialog is the app's own, not a native one, so it can ask for a hold: a quit
// that stops work is destructive, and the interface already answers uninstalling
// that way. The event that asked is prevented before this, so the window stays.
func (u *UI) askToQuit(win *mygo.Window, pending int) {
	if u.quitOpen {
		return
	}
	u.quitPending = pending
	u.quitOpen = true
	win.Invalidate()
}

// quitDialog draws the question, and answers it: keep running, or quit and let
// the tasks be marked cancelled.
func (u *UI) quitDialog(c *ui.Context) {
	t := c.Theme()
	components.Dialog(c, &u.quitOpen, components.DialogOptions{Label: locales.TasksQuitTitle()}, func(panel ui.Element) {
		pending := u.quitPending
		panel.Width(420).Padding(20).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, locales.TasksQuitTitle()).FontSize(16).Bold()
				components.Text(c, locales.TasksQuitMessage(pending)).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					if components.Button(c, locales.TasksQuitCancel(), components.ButtonOutline).Clicked() {
						u.quitOpen = false
						u.quitPending = 0
					}
					u.quitConfirm(c)
				})
			})
		})
	})
}

// quitConfirm is the confirm control: a hold, or a click when the user turned
// holding off.
func (u *UI) quitConfirm(c *ui.Context) {
	if u.settings == nil || !u.settings.HoldToConfirm {
		confirm := components.Button(c, locales.TasksQuitConfirm(), components.ButtonOutline)
		confirm.TextColor(c.Theme().Danger)
		if confirm.Clicked() {
			u.confirmQuit()
		}
		return
	}
	var held bool
	hold := components.HoldToConfirm(c, &held, locales.TasksQuitConfirm(),
		components.HoldToConfirmOptions{Tone: components.HoldDanger})
	if hold.Changed() {
		u.confirmQuit()
	}
}

// confirmQuit leaves: the tasks are stopped, which cancels the ones running, and
// the accepted flag lets the window close without asking again.
func (u *UI) confirmQuit() {
	u.quitOpen = false
	u.quitAccepted.Store(true)
	u.svc.StopTasks()
	if u.quitApp != nil {
		u.quitApp()
		return
	}
	mygo.App.Quit()
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
	default:
		return locales.NavPackages()
	}
}

// windowTitle is the title bar's content area: the history arrows and the page
// title on the leading edge, and the tasks button on the trailing edge. It
// stays visible when the sidebar is collapsed, which the sidebar header does
// not.
// titleActions is the buttons the page showing puts in the title bar, beside its
// title; nil for a page with none. It is what the header's trailing slot
// builds, and the shell keeps the history arrows and the tasks button to itself.
func (u *UI) titleActions() func(*ui.Context) {
	// A window builds its pages before it builds the header, but a test may
	// build the header alone.
	switch u.page {
	case PagePackages:
		if u.installed != nil {
			return u.installed.TitleActions
		}
	case PageOutdated:
		if u.outdated != nil {
			return u.outdated.TitleActions
		}
	case PageSearch:
		if u.search != nil {
			return u.search.TitleActions
		}
	case PageTasks:
		if u.tasks != nil {
			return u.tasks.TitleActions
		}
	}
	return nil
}

func (u *UI) windowTitle(c *ui.Context) {
	t := c.Theme()
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
		if actions := u.titleActions(); actions != nil {
			// The page's buttons sit at the trailing end, beside the tasks
			// button, so the bar reads as one group of actions.
			ui.Box(c).Shrink(0).Padding(0, t.Space(2), 0, t.Space(1)).Children(func() {
				actions(c)
			})
		}
		u.tasksButton(c)
	})
}

// tasksButton is the title bar's shortcut to the tasks page, with a badge while
// tasks run. Its rectangle is the destination of the flight from a task.
func (u *UI) tasksButton(c *ui.Context) {
	button := parts.TitleButton(c, lucide.ListTodo(), locales.NavTasks())
	if button.Clicked() {
		u.navigate(PageTasks)
	}
	u.jobTarget = button.Bounds()
	countBadge(c, u.runningTasks())
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
				counted := func(n int) func(*ui.Context, float32) {
					return func(c *ui.Context, size float32) { trailingCount(c, n) }
				}
				u.nav(c, PagePackages, locales.NavPackages(), lucide.Package(), nil)
				u.nav(c, PageOutdated, locales.NavOutdated(), lucide.CircleFadingArrowUp(), counted(u.outdatedCount()))
				u.nav(c, PageSearch, locales.NavSearch(), lucide.Search(), nil)
				u.nav(c, PageTasks, locales.NavTasks(), lucide.ListTodo(), counted(u.runningTasks()))
			})
			components.SidebarSection(c, locales.SidebarApp(), func() {
				// Settings is a window of its own, so this item opens it and never
				// shows as the current page.
				item := components.SidebarItem(c, locales.NavSettings(), lucide.Settings(), false)
				if item.Clicked() && u.openSettings != nil {
					u.openSettings()
				}
			})
		})
	})
}

// nav builds one sidebar item: an icon, a label, and an optional trailing slot
// after the label, which is how the counts sit on the item's right edge.
func (u *UI) nav(c *ui.Context, page Page, label string, icon *ui.SVG, trailing func(c *ui.Context, size float32)) {
	item := components.SidebarItemWithSlots(c, label, u.page == page, func(c *ui.Context, size float32) {
		components.Icon(c, icon).Size(size, size).TextColor(c.Theme().Accent)
	}, trailing)
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
	// Settings are one keystroke away, which is where a user looks for them after
	// the first time. Shortcut asks the window rather than the focused control:
	// OnShortcut would be scoped to this content column and stay silent while the
	// sidebar or nothing at all has the focus. ui.Cmd is Control where there is no
	// Cmd.
	if c.Shortcut(ui.Cmd, ui.KeyComma) && u.openSettings != nil {
		u.openSettings()
	}
	for _, t := range u.data.DrainToasts() {
		c.AddToast(u.toastFor(t))
	}
	u.quitDialog(c)
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

// SetHoldToConfirm saves whether a destructive confirmation holds or clicks.
func (u *UI) SetHoldToConfirm(on bool) {
	u.saveSettings(false, func() error {
		return u.svc.SetHoldToConfirm(on)
	})
}

// OpenPath opens a file or folder in the desktop's own application, which is how
// the About section shows the data the app keeps.
func (u *UI) OpenPath(path string) {
	if path == "" {
		return
	}
	go func() {
		if err := mygo.Shell.OpenPath(path); err != nil {
			slog.Warn("views: could not open a path", "path", path, "error", err)
			if u.shell == nil {
				return
			}
			u.shell.Window().Update(func() {
				u.data.QueueToast(inventory.Toast{Title: err.Error(), Tone: components.ToastError})
			})
		}
	}()
}

// OpenURL opens a web address in the browser, which is how the About section
// links the project's repository.
func (u *UI) OpenURL(url string) {
	if url == "" {
		return
	}
	go func() {
		if err := mygo.Shell.OpenExternal(url); err != nil {
			slog.Warn("views: could not open a URL", "url", url, "error", err)
			if u.shell == nil {
				return
			}
			u.shell.Window().Update(func() {
				u.data.QueueToast(inventory.Toast{Title: err.Error(), Tone: components.ToastError})
			})
		}
	}()
}

// ClearTaskHistory drops the tasks that have an outcome, off the UI thread, and
// reloads the page so the rows leave with them.
func (u *UI) ClearTaskHistory() {
	if u.shell == nil || u.svc == nil {
		return
	}
	win := u.shell.Window()
	go func() {
		err := u.svc.ClearTaskHistory()
		list := u.svc.ListTasks()
		win.Update(func() {
			if err != nil {
				u.data.QueueToast(inventory.Toast{Title: err.Error(), Tone: components.ToastError})
				return
			}
			u.tasks.Apply(list)
			u.data.QueueToast(inventory.Toast{Title: locales.ToastHistoryCleared(), Tone: components.ToastInfo})
		})
	}()
}

// SetNotifications saves whether a finished operation of this kind and outcome is
// announced.
func (u *UI) SetNotifications(kind tasks.Kind, succeeded, on bool) {
	u.saveSettings(false, func() error {
		return u.svc.SetNotifications(kind, succeeded, on)
	})
}

// SetNotificationsEnabled saves the master notification switch.
func (u *UI) SetNotificationsEnabled(on bool) {
	u.saveSettings(false, func() error {
		return u.svc.SetNotificationsEnabled(on)
	})
}

// SetNotifyUpdates saves whether a scan that finds updates says so.
func (u *UI) SetNotifyUpdates(on bool) {
	u.saveSettings(false, func() error {
		return u.svc.SetNotifyUpdates(on)
	})
}

// SetTheme saves the appearance; the backend applies it to the window and to
// the next launch.
func (u *UI) SetTheme(value string) {
	u.saveSettings(false, func() error {
		return u.svc.SetTheme(value)
	})
}

// SetAccent saves the theme color. Nothing is applied: both windows read the
// saved value as they draw, so the next frame of each is already in it.
func (u *UI) SetAccent(value string) {
	u.saveSettings(false, func() error {
		return u.svc.SetAccent(value)
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
	if u.settings == nil {
		return
	}
	if u.saving || u.shell == nil || u.settingsShell == nil {
		// A second change while one write runs would race the config file, so
		// it is dropped and its controls go back to what the backend last
		// reported, without an error: nothing failed, and the write that is
		// running keeps the page busy. No window means nothing is on screen to
		// change, and a change arriving then is dropped the same way.
		u.settings.Revert(u.lastSnapshot)
		return
	}
	u.saving = true
	s := u.settings
	s.BeginSave()
	// The page is published in its own window; the lists it changes are in the
	// other one, which is asked for a frame after the data moved.
	settingsWin := u.settingsShell.Window()
	mainWin := u.shell.Window()
	go func() {
		err := fn()
		managers := u.svc.ListManagers()
		overrides := u.svc.ManagerOverrides()
		saved := settings.Snapshot{
			AutoScan:      u.svc.AutoScanOnLaunch(),
			LogLevel:      u.svc.LogLevel(),
			Language:      u.svc.Language(),
			Theme:         u.svc.Theme(),
			Accent:        u.svc.Accent(),
			HoldToConfirm: u.svc.HoldToConfirm(),
			Notify:        settings.NotifyFrom(u.svc),
		}
		settingsWin.Update(func() {
			u.saving = false
			if err == nil {
				u.lastSnapshot = saved
			}
			s.EndSave(err, saved)
			if err != nil {
				return
			}
			u.data.SetManagers(managers, overrides)
			mainWin.Invalidate()
			if rescan {
				u.Refresh()
			}
		})
	}()
}
