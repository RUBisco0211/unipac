// Package uiassets holds the views' own embedded assets: files compiled into
// the binary, read by name, and never listed by hand anywhere in the
// interface. Only the views and the packages they share import it.
//
// The marks are the brand images of the package managers, one file per manager
// named by its ID. They are from Simple Icons (https://simpleicons.org), which
// publishes them under CC0; a mark is named by the manager that shows it, not
// by the brand that drew it — pip shows Python's mark and cargo shows Rust's,
// which is what cargo builds with. See marks/LICENSE.
package uiassets

import (
	"embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed marks/*.svg
var marks embed.FS

// markCache parses each file once; every lookup returns the same shared SVG.
var markCache sync.Map

// ManagerMark returns the brand mark of the manager with the given ID — the
// file marks/<id>.svg — and nil when there is none. A manager without a file
// here shows without a mark, and the interface stays free of manager names.
func ManagerMark(id string) *ui.SVG {
	if cached, ok := markCache.Load(id); ok {
		if svg, ok := cached.(*ui.SVG); ok {
			return svg
		}
		return nil
	}
	var svg *ui.SVG
	data, err := marks.ReadFile("marks/" + id + ".svg")
	if err != nil {
		// No file is a normal state; the negative answer is remembered so the
		// lookup stays cheap on every frame that draws a row.
		markCache.Store(id, (*ui.SVG)(nil))
	} else {
		// A file that does not parse is a build mistake, not a runtime state:
		// the mark never changes while the app runs.
		svg = ui.MustParseSVG(data)
		markCache.Store(id, svg)
	}
	return svg
}
