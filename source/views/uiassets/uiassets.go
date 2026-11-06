// Package uiassets holds the views' own embedded assets: files compiled into
// the binary, read by name, and never listed by hand anywhere in the interface.
// Only the views and the packages they share import it.
//
// The marks are the brand images of the package managers, one file per manager
// named by its ID. They are the brands' own colored artwork, so they are drawn
// as images rather than as icons: a mark is named by the manager that shows it,
// not by the brand that drew it — pip shows Python's mark and cargo shows
// Cargo's crate. See marks/LICENSE for where each file came from and under what
// terms.
package uiassets

import (
	"embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed marks/*
var marks embed.FS

// markCache decodes each file once; every lookup returns the same shared image.
var markCache sync.Map

// ManagerLogo returns the brand mark of the manager with the given ID — the
// file marks/<id>.svg or marks/<id>.png — and nil when there is none. A manager
// with no file here shows without a mark, and the interface stays free of
// manager names.
func ManagerLogo(id string) ui.ImageSource {
	if cached, ok := markCache.Load(id); ok {
		logo, _ := cached.(ui.ImageSource)
		return logo
	}
	var logo ui.ImageSource
	data, err := marks.ReadFile("marks/" + id + ".svg")
	if err != nil {
		data, err = marks.ReadFile("marks/" + id + ".png")
	}
	if err != nil {
		// No file is a normal state; the negative answer is remembered so the
		// lookup stays cheap on every frame that draws a row.
		markCache.Store(id, nil)
		return nil
	}
	// A file that does not decode is a build mistake, not a runtime state: the
	// mark never changes while the app runs.
	logo, err = ui.DecodeImage(data)
	if err != nil {
		panic("uiassets: manager mark " + id + " does not decode: " + err.Error())
	}
	markCache.Store(id, logo)
	return logo
}
