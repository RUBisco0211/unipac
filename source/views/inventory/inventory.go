// Package inventory holds the package inventory and the state of the jobs that
// fill and change it. Every page reads the same Data; only this package writes
// it.
package inventory

import (
	"slices"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
)

// Service is the part of the application service the views use. It is declared
// here because the app lives in package main, which cannot be imported.
type Service interface {
	ListManagers() []manager.Info
	GetCachedPackages() ([]manager.Package, error)
	UpdateCache() error
	SearchPackages(keyword string, managerIDs []string) (manager.SearchResult, error)
	// About is what the settings page says about the build and where the app
	// keeps its files.
	About() About
	// ClearTaskHistory drops the tasks that have an outcome, in memory and in
	// the store, and leaves the queued and running ones.
	ClearTaskHistory() error
	// ListPackageVersions is every version a manager published for a package,
	// oldest first, for the managers that advertise ListVersions.
	ListPackageVersions(managerID string, name string) ([]string, error)
	// PackageDetail is what a package's own page says about it.
	PackageDetail(managerID, name, version string) (manager.Detail, error)
	InstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
	UpdatePackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
	UninstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
	BatchUpdatePackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
	BatchUninstallPackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
	ManagerOverrides() map[string]manager.Settings
	SetManagerEnabled(id string, enabled bool) error
	SetManagerExecPath(id string, path string) error
	ReloadManagers() error
	Language() string
	Languages() []string
	Theme() string
	SetTheme(value string) error
	// Proxy is the saved proxy address, "" to follow the environment;
	// SystemProxy is what the desktop is configured with, empty when it has none.
	Proxy() string
	SetProxy(value string) error
	SystemProxy() string
	// Accent is the saved theme color, one of Accents; "" keeps the toolkit's.
	Accent() string
	Accents() []string
	SetAccent(value string) error
	// NotificationsEnabled is the master switch; Notifications reports one
	// operation's outcome under it, and NotifyUpdates the "an update is
	// available" switch. NotificationKinds is the operations the switches cover.
	NotificationsEnabled() bool
	SetNotificationsEnabled(on bool) error
	Notifications(kind tasks.Kind, succeeded bool) bool
	NotificationKinds() []tasks.Kind
	SetNotifications(kind tasks.Kind, succeeded, on bool) error
	NotifyUpdates() bool
	SetNotifyUpdates(on bool) error
	SetLanguage(tag string) error
	AutoScanOnLaunch() bool
	SetAutoScanOnLaunch(on bool) error
	HoldToConfirm() bool
	SetHoldToConfirm(on bool) error
	LogLevel() string
	LogLevels() []string
	SetLogLevel(level string) error
	ListTasks() []tasks.Snapshot
	TaskOutput(id string) tasks.Output
	SetTaskListener(fn func())
	UnfinishedTasks() int
	CancelTask(id string) error
	StopTasks()
}

// About is what the build is and where it keeps its files, for the page that
// answers "which version is this, and where did it put things".
type About struct {
	// Version is the app's own version, as the manifest names it.
	Version string
	// Go is the Go the binary was built with.
	Go string
	// Modules are the toolkit versions it bundles, as "MyGo 0.7.4".
	Modules []string
	// DataDir holds the cache, the config and the task store.
	DataDir string
	// Repository is the project's home on the web, where the source and the
	// releases live.
	Repository string
}

// Data is the package inventory shared by the pages, plus the state of the
// refresh and task jobs that fill and change it. It changes only on the UI
// thread; Version lets a page rebuild its filtered rows only when something
// actually changed.
//
// The fields are exported for reading: every page reads the same inventory.
// Writing goes through the methods, so a page cannot skip bumping a version or
// leave a package marked busy.
type Data struct {
	Rows       []manager.Package
	Managers   []manager.Info
	Overrides  map[string]manager.Settings
	Names      map[string]string
	LoadErr    error
	RefreshErr error
	Refreshing bool
	// refreshPending asks for another scan once the running one finishes, so an
	// task that lands mid-scan is not left showing pre-task data.
	refreshPending bool
	// active maps a package key to the operation running for it. Different
	// packages run independently; the backend limiter serializes one manager.
	Active  map[string]string
	Version int
	// installedVersions caches what the inventory holds, by package key, and the
	// inventory version it was built from.
	installedVersions map[string]string
	installedVersion  int

	// managersVersion changes when the manager list itself changes, so the
	// settings page rebuilds its rows only then and keeps the path being typed.
	ManagersVersion int

	Toasts []Toast
}

// Mark is what a page says about a package beside its name: nothing, that the
// inventory holds it, or that a newer version is available.
type Mark int

const (
	MarkNone Mark = iota
	MarkInstalled
	MarkUpdate
)

// Toast is a message queued for the next frame, because MyGo adds toasts while
// building rather than from the job that produced the result. TaskID, when set,
// is the task the message is about, so the toast can offer to show it.
type Toast struct {
	Title  string
	Tone   string
	TaskID string
}

// New loads the inventory the app starts with.
func New(service Service) *Data {
	d := &Data{Names: make(map[string]string)}
	d.SetManagers(service.ListManagers(), service.ManagerOverrides())
	d.load(service)
	return d
}

// setManagers installs the manager list and the user's saved settings after the
// settings page changed them.
func (d *Data) SetManagers(managers []manager.Info, overrides map[string]manager.Settings) {
	changed := !slices.Equal(d.Managers, managers)
	d.Managers = managers
	d.Overrides = overrides
	d.Names = make(map[string]string, len(managers))
	for _, m := range managers {
		d.Names[m.ID] = m.Name
	}
	if changed {
		d.ManagersVersion++
	}
	d.Version++
}

func (d *Data) load(service Service) {
	rows, err := service.GetCachedPackages()
	if err != nil {
		d.LoadErr = err
		return
	}
	d.LoadErr = nil
	d.Rows = rows
	d.Version++
}

// beginRefresh reports whether a scan should start. A request that arrives
// while one runs is remembered instead, so the caller runs one more scan when
// the current one finishes rather than dropping the request.
func (d *Data) BeginRefresh() bool {
	if d.Refreshing {
		d.refreshPending = true
		return false
	}
	d.Refreshing = true
	d.RefreshErr = nil
	return true
}

// finishRefresh installs a scan's outcome and reports whether another scan was
// asked for while it ran.
func (d *Data) FinishRefresh(rows []manager.Package, refreshErr, loadErr error) bool {
	d.applyRefresh(rows, refreshErr, loadErr)
	if !d.refreshPending {
		return false
	}
	d.refreshPending = false
	return true
}

// applyRefresh installs the outcome of a background scan. It runs on the UI
// thread, from Window.Update.
func (d *Data) applyRefresh(rows []manager.Package, refreshErr, loadErr error) {
	d.Refreshing = false
	d.RefreshErr = refreshErr
	if loadErr != nil {
		d.LoadErr = loadErr
		return
	}
	d.LoadErr = nil
	d.Rows = rows
	d.Version++
}

// InstalledVersion is the version the inventory holds for a package, and false
// when it holds none. A search result is not installed by definition, so the
// search page asks this instead.
func (d *Data) InstalledVersion(managerID, name string) (string, bool) {
	if d.installedVersion != d.Version {
		d.installedVersions = make(map[string]string, len(d.Rows))
		for _, pkg := range d.Rows {
			d.installedVersions[Key(pkg)] = pkg.Version
		}
		d.installedVersion = d.Version
	}
	version, ok := d.installedVersions[managerID+":"+name]
	return version, ok
}

// SearchMark says what a search result is relative to the inventory, which is
// what tells an installed package from one that has a newer version.
func (d *Data) SearchMark(pkg manager.Package) Mark {
	have, installed := d.InstalledVersion(pkg.Manager, pkg.Name)
	switch {
	case !installed:
		return MarkNone
	case pkg.Version == "" || have == "" || have == pkg.Version:
		// Without both versions there is nothing to compare, and the same
		// version is simply installed.
		return MarkInstalled
	default:
		return MarkUpdate
	}
}

// Key identifies a package across managers.
func Key(pkg manager.Package) string {
	return pkg.Manager + ":" + pkg.Name
}

// Matches reports whether a package matches query, which is compared
// case-insensitively against the name, the full name and the description.
func Matches(pkg manager.Package, query string) bool {
	if query == "" {
		return true
	}
	query = strings.ToLower(strings.TrimSpace(query))
	return strings.Contains(strings.ToLower(pkg.Name), query) ||
		strings.Contains(strings.ToLower(pkg.Fullname), query) ||
		strings.Contains(strings.ToLower(pkg.Description), query)
}

// BeginTask marks pkgs as running op, unless one of them is already running
// something. It reports whether the task started.
func (d *Data) BeginTask(op string, pkgs []manager.Package) bool {
	for _, pkg := range pkgs {
		if _, running := d.Active[Key(pkg)]; running {
			return false
		}
	}
	if d.Active == nil {
		d.Active = make(map[string]string, len(pkgs))
	}
	for _, pkg := range pkgs {
		d.Active[Key(pkg)] = op
	}
	return true
}

// EndTask clears what BeginTask marked.
func (d *Data) EndTask(pkgs []manager.Package) {
	for _, pkg := range pkgs {
		delete(d.Active, Key(pkg))
	}
}

// Running names the operation running for a package, "" when none is.
func (d *Data) Running(pkg manager.Package) string {
	return d.Active[Key(pkg)]
}

// AnyRunning reports whether any of pkgs is being operated on.
func (d *Data) AnyRunning(pkgs []manager.Package) bool {
	for _, pkg := range pkgs {
		if _, running := d.Active[Key(pkg)]; running {
			return true
		}
	}
	return false
}

func (d *Data) QueueToast(t Toast) {
	d.Toasts = append(d.Toasts, t)
}

// DrainToasts hands the queued messages to the builder and clears the queue.
func (d *Data) DrainToasts() []Toast {
	if len(d.Toasts) == 0 {
		return nil
	}
	out := d.Toasts
	d.Toasts = nil
	return out
}

// Name resolves an opaque manager ID to its backend-provided label.
func (d *Data) Name(id string) string {
	if name, ok := d.Names[id]; ok {
		return name
	}
	return id
}

// Capabilities reports what the manager behind a package claims. An unknown or
// turned-off manager claims nothing, so its actions stay disabled even before
// the next scan drops its rows.
func (d *Data) Capabilities(id string) manager.Capabilities {
	for _, m := range d.Managers {
		if m.ID == id {
			if !m.Enabled {
				return manager.Capabilities{}
			}
			return m.Capabilities
		}
	}
	return manager.Capabilities{}
}

// Enabled reports whether UniPac currently uses a manager.
func (d *Data) Enabled(id string) bool {
	for _, m := range d.Managers {
		if m.ID == id {
			return m.Enabled
		}
	}
	return false
}
