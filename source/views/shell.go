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
	u.Refresh()
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
			u.settings(c)
		}
	})
}

func (u *UI) settings(c *ui.Context) {
	components.ScrollView(c, components.ScrollViewOptions{}).
		Fill().
		Children(func() {
			ui.Column(c).FillWidth().Gap(10).Padding(0, 16, 16, 16).Children(func() {
				components.Text(c, locales.SettingsManagers()).
					FontSize(12).Bold().TextColor(c.Theme().TextMuted)
				for _, m := range u.data.managers {
					managerCard(c, m)
				}
			})
		})
}

func managerCard(c *ui.Context, m manager.Info) {
	t := c.Theme()
	ui.Column(c).FillWidth().Shrink(0).Gap(10).
		Padding(16).Radius(t.Radius).
		Background(t.Surface.Alpha(1)).
		Border(1, t.Border).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				components.Text(c, m.Name).FontSize(15).Bold()
				if m.Enabled {
					components.Badge(c, locales.SettingsEnabled(), components.BadgeOptions{Tone: components.BadgeSuccess})
				} else {
					components.Badge(c, locales.SettingsDisabled(), components.BadgeOptions{Tone: components.BadgeDanger})
				}
				ui.Spacer(c)
				components.Text(c, m.ID).TextColor(t.TextMuted)
			})

			if m.ExecPath != "" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					components.Text(c, locales.SettingsExecutable()).TextColor(t.TextMuted)
					components.Text(c, m.ExecPath).SingleLine().TextColor(t.TextMuted)
				})
			}

			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Text(c, locales.SettingsCapabilities()).TextColor(t.TextMuted)
				names := capabilityList(m.Capabilities)
				if len(names) == 0 {
					components.Text(c, locales.SettingsNoCapabilities()).TextColor(t.TextMuted)
					return
				}
				for _, name := range names {
					components.Badge(c, name)
				}
			})
		})
}

// capabilityList names what an adapter claims, so the page explains why an
// action is disabled elsewhere.
func capabilityList(caps manager.Capabilities) []string {
	var names []string
	add := func(on bool, name string) {
		if on {
			names = append(names, name)
		}
	}
	add(caps.ListInstalled, locales.CapabilityList())
	add(caps.ListOutdated, locales.CapabilityOutdated())
	add(caps.GetPackageInfo, locales.CapabilityInfo())
	add(caps.Search, locales.CapabilitySearch())
	add(caps.Install, locales.CapabilityInstall())
	add(caps.Uninstall, locales.CapabilityUninstall())
	add(caps.Update, locales.CapabilityUpdate())
	add(caps.ListVersions, locales.CapabilityVersions())
	return names
}
