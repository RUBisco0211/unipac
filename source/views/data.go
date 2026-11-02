package views

import (
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

// Service is the part of the application service the views use. It is declared
// here because the app lives in package main, which cannot be imported.
type Service interface {
	ListManagers() []manager.Info
	GetCachedPackages() ([]manager.Package, error)
	UpdateCache() error
	SearchPackages(keyword string) ([]manager.Package, error)
	InstallPackage(managerID, name string, opt manager.ActionOptions) (manager.ActionResult, error)
	UpdatePackage(managerID, name string, opt manager.ActionOptions) (manager.ActionResult, error)
	UninstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
	BatchUpdatePackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
	BatchUninstallPackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)
}

// data is the package inventory shared by the pages, plus the state of the
// refresh and action jobs that fill and change it. It changes only on the UI
// thread; version lets the list views rebuild their filtered rows only when
// something actually changed.
type data struct {
	rows       []manager.Package
	managers   []manager.Info
	names      map[string]string
	loadErr    error
	refreshErr error
	refreshing bool
	// refreshPending asks for another scan once the running one finishes, so an
	// action that lands mid-scan is not left showing pre-action data.
	refreshPending bool
	busy           bool
	// busyKey names the running action, such as "update:brew:git", so only the
	// control that started it says it is working.
	busyKey string
	version int

	toasts []toast
}

// toast is a message queued for the next frame, because MyGo adds toasts while
// building rather than from the job that produced the result.
type toast struct {
	title string
	tone  string
}

func newData(service Service) *data {
	d := &data{names: make(map[string]string)}
	d.managers = service.ListManagers()
	for _, m := range d.managers {
		d.names[m.ID] = m.Name
	}
	d.load(service)
	return d
}

func (d *data) load(service Service) {
	rows, err := service.GetCachedPackages()
	if err != nil {
		d.loadErr = err
		return
	}
	d.loadErr = nil
	d.rows = rows
	d.version++
}

// beginRefresh reports whether a scan should start. A request that arrives
// while one runs is remembered instead, so the caller runs one more scan when
// the current one finishes rather than dropping the request.
func (d *data) beginRefresh() bool {
	if d.refreshing {
		d.refreshPending = true
		return false
	}
	d.refreshing = true
	d.refreshErr = nil
	return true
}

// finishRefresh installs a scan's outcome and reports whether another scan was
// asked for while it ran.
func (d *data) finishRefresh(rows []manager.Package, refreshErr, loadErr error) bool {
	d.applyRefresh(rows, refreshErr, loadErr)
	if !d.refreshPending {
		return false
	}
	d.refreshPending = false
	return true
}

// applyRefresh installs the outcome of a background scan. It runs on the UI
// thread, from Window.Update.
func (d *data) applyRefresh(rows []manager.Package, refreshErr, loadErr error) {
	d.refreshing = false
	d.refreshErr = refreshErr
	if loadErr != nil {
		d.loadErr = loadErr
		return
	}
	d.loadErr = nil
	d.rows = rows
	d.version++
}

func (d *data) queueToast(title, tone string) {
	d.toasts = append(d.toasts, toast{title: title, tone: tone})
}

// drainToasts hands the queued messages to the builder and clears the queue.
func (d *data) drainToasts() []toast {
	if len(d.toasts) == 0 {
		return nil
	}
	out := d.toasts
	d.toasts = nil
	return out
}

// managerName resolves an opaque manager ID to its backend-provided label.
func (d *data) managerName(id string) string {
	if name, ok := d.names[id]; ok {
		return name
	}
	return id
}

// capabilities reports what the manager behind a package claims. An unknown ID
// claims nothing, so its actions stay disabled.
func (d *data) capabilities(id string) manager.Capabilities {
	for _, m := range d.managers {
		if m.ID == id {
			return m.Capabilities
		}
	}
	return manager.Capabilities{}
}
