package views

import (
	"github.com/RUBisco0211/unipac/app"
	"github.com/RUBisco0211/unipac/backend/core/manager"
)

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
	busy       bool
	version    int

	toasts []toast
}

// toast is a message queued for the next frame, because MyGo adds toasts while
// building rather than from the job that produced the result.
type toast struct {
	title string
	tone  string
}

func newData(service *app.App) *data {
	d := &data{names: make(map[string]string)}
	d.managers = service.ListManagers()
	for _, m := range d.managers {
		d.names[m.ID] = m.Name
	}
	d.load(service)
	return d
}

func (d *data) load(service *app.App) {
	rows, err := service.GetCachedPackages()
	if err != nil {
		d.loadErr = err
		return
	}
	d.loadErr = nil
	d.rows = rows
	d.version++
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
