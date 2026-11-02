// Package views holds UniPac's native interface: the window shell, one builder
// per page, and the retained state each page shows.
//
// Builders run on the UI thread for every frame. They must not perform I/O,
// start work, or keep elements across passes; application state is loaded into
// the structs here before the window opens and passed by pointer.
package views

import (
	"log/slog"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
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
	settings  *settingsView
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
	u.settings = newSettings(service, u.data)
	u.settings.onSetEnabled = u.SetManagerEnabled
	u.settings.onSetPath = u.SetManagerExecPath
	u.settings.onReload = u.ReloadManagers
	u.settings.onSetAutoScan = u.SetAutoScanOnLaunch
	u.settings.onSetLogLevel = u.SetLogLevel
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
	u.shell.Window().OnClosed(func() { mygo.App.Quit() })
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
func (u *UI) Search(query string) {
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
		results, err := u.svc.SearchPackages(query)
		win.Update(func() {
			if seq != u.search.seq {
				return
			}
			u.search.apply(query, results, err)
		})
	}()
}

// Install, Update and Remove run one package action at a time and refresh the
// inventory when it finishes.
func (u *UI) Install(pkg manager.Package) {
	u.runAction("install:"+packageKey(pkg), func() (manager.ActionResult, error) {
		return u.svc.InstallPackage(pkg.Manager, pkg.Name, manager.ActionOptions{})
	}, locales.ToastInstalled())
}

func (u *UI) Update(pkg manager.Package) {
	u.runAction("update:"+packageKey(pkg), func() (manager.ActionResult, error) {
		return u.svc.UpdatePackage(pkg.Manager, pkg.Name, manager.ActionOptions{})
	}, locales.ToastUpdated())
}

func (u *UI) Remove(pkg manager.Package) {
	u.runAction("remove:"+packageKey(pkg), func() (manager.ActionResult, error) {
		return u.svc.UninstallPackage(pkg, manager.ActionOptions{})
	}, locales.ToastRemoved())
}

func (u *UI) BulkUpdate(pkgs []manager.Package) {
	u.runAction("bulk-update", func() (manager.ActionResult, error) {
		return u.svc.BatchUpdatePackages(pkgs, manager.ActionOptions{})
	}, locales.ToastUpdated())
}

func (u *UI) BulkRemove(pkgs []manager.Package) {
	u.runAction("bulk-remove", func() (manager.ActionResult, error) {
		return u.svc.BatchUninstallPackages(pkgs, manager.ActionOptions{})
	}, locales.ToastRemoved())
}

// runAction runs one action off the UI thread. Its outcome becomes a toast,
// queued for the next frame because MyGo adds toasts while building.
func (u *UI) runAction(key string, fn func() (manager.ActionResult, error), done string) {
	if u.shell == nil || u.data.busy {
		return
	}
	u.data.busy = true
	u.data.busyKey = key
	win := u.shell.Window()
	go func() {
		result, err := fn()
		win.Update(func() {
			u.data.busy = false
			u.data.busyKey = ""
			switch {
			case err != nil:
				u.data.queueToast(err.Error(), components.ToastError)
			case !result.Success:
				u.data.queueToast(result.Message, components.ToastError)
			default:
				u.data.queueToast(done, components.ToastSuccess)
			}
			u.Refresh()
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
	case PageSettings:
		return locales.NavSettings()
	default:
		return locales.NavPackages()
	}
}

// windowTitle is the title bar's content area: the history arrows and the page
// title, both on the leading edge. It stays visible when the sidebar is
// collapsed, which the sidebar header does not.
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
				u.nav(c, PagePackages, locales.NavPackages(), lucide.Package())
				u.nav(c, PageOutdated, locales.NavOutdated(), lucide.CircleFadingArrowUp())
				u.nav(c, PageSearch, locales.NavSearch(), lucide.Search())
			})
			components.SidebarSection(c, locales.SidebarApp(), func() {
				u.nav(c, PageSettings, locales.NavSettings(), lucide.Settings())
			})
		})
	})
}

func (u *UI) nav(c *ui.Context, page Page, label string, icon *ui.SVG) {
	if components.SidebarItem(c, label, icon, u.page == page).Clicked() {
		u.navigate(page)
	}
}

func (u *UI) content(c *ui.Context) {
	for _, t := range u.data.drainToasts() {
		c.AddToast(ui.Toast{Title: t.title, Type: t.tone})
	}
	ui.Column(c).Fill().Padding(sidebar.HeaderHeight(c)+16, 0, 0, 0).Children(func() {
		switch u.page {
		case PagePackages:
			u.installed.view(c)
		case PageOutdated:
			u.outdated.view(c)
		case PageSearch:
			u.search.view(c)
		case PageSettings:
			u.settings.view(c)
		}
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
		win.Update(func() {
			s.busy = false
			s.detecting = false
			// These come from the backend either way, so a failed save puts the
			// startup switch and the log level back with the manager rows.
			s.autoScan = autoScan
			s.logLevel = level
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
