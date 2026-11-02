package views

import (
	"fmt"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
)

// longList is a list long enough to scroll, with a type column that sorts into
// a different order than the name column.
func longList() *listView {
	d := fixture()
	d.rows = nil
	for i := range 200 {
		d.rows = append(d.rows, manager.Package{
			Name:      fmt.Sprintf("pkg-%03d", i),
			Manager:   "brew",
			Installed: true,
			IsGUI:     i%2 == 0,
		})
	}
	return &listView{data: d, chosen: map[string]struct{}{}, allowRemove: true}
}

// Sorting a column moves rows, so a table that kept its place by the item at
// its top would scroll to wherever that item went, which often means the end.
func TestSortKeepsScrollOffset(t *testing.T) {
	l := longList()
	tt := render(t, l.view, 900, 400)

	l.table.List.ScrollTo(100, ui.Start)
	tt.Frame()
	before, _ := l.table.List.Visible()

	if err := tt.Click(locales.TableType()); err != nil {
		t.Fatalf("click the type header: %v", err)
	}
	tt.Frame()
	after, _ := l.table.List.Visible()

	if after != before {
		t.Fatalf("sorting scrolled the table from row %d to %d", before, after)
	}
}

// Filtering to fewer rows than the offset must land on the last rows, as a list
// clamped to its content does, rather than stay past the end.
func TestFilterKeepsScrollWithinRange(t *testing.T) {
	l := longList()
	tt := render(t, l.view, 900, 400)

	l.table.List.ScrollTo(100, ui.Start)
	tt.Frame()

	l.query = "pkg-1"
	tt.Frame()
	first, last := l.table.List.Visible()

	if last != len(l.rows)-1 {
		t.Fatalf("visible %d..%d of %d rows, want the end of the list", first, last, len(l.rows))
	}
}
