package list

import (
	"fmt"
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/egoist/mygo/ui"
)

func render(t *testing.T, build func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	return testui.Render(build, w, h)
}

func fixture() *inventory.Data { return testui.Data() }

func TestListViewShowsRows(t *testing.T) {
	l := &View{Data: fixture()}
	tt := render(t, l.Build, 900, 500)

	for _, want := range []string{
		"ripgrep", "14.1.1", "Line-oriented search tool",
		"Homebrew", "typescript", "6.0.0",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
}

func TestListViewEmptyState(t *testing.T) {
	d := fixture()
	d.Rows = nil
	l := &View{Data: d}
	tt := render(t, l.Build, 900, 500)
	if !tt.HasText(locales.PackagesEmpty()) {
		t.Errorf("empty state not shown: %v", tt.Texts())
	}
}

func TestEmptyStateIsCentredWithText(t *testing.T) {
	d := fixture()
	d.Rows = nil
	l := &View{Data: d}
	tt := render(t, l.Build, 900, 500)

	if !tt.HasText(locales.PackagesEmpty()) {
		t.Fatalf("empty state text missing: %v", tt.Texts())
	}
	// The empty state is centred, so the text is not at the left edge.
	rect, ok := tt.Find(locales.PackagesEmpty())
	if !ok {
		t.Fatal("empty state text not found")
	}
	if rect.X < 300 {
		t.Errorf("empty state text starts at %v, want it centred", rect.X)
	}
}

// A background refresh must keep the cached rows on screen: the window opens on
// them and only the thin bar says a scan is running.
func TestRefreshKeepsCachedRows(t *testing.T) {
	d := fixture()
	d.Refreshing = true
	l := &View{Data: d}
	tt := render(t, l.Build, 900, 500)

	if !tt.HasText("ripgrep") || !tt.HasText("typescript") {
		t.Errorf("cached rows hidden while refreshing: %v", tt.Texts())
	}
}

func TestOutdatedEmptyState(t *testing.T) {
	d := fixture()
	for i := range d.Rows {
		d.Rows[i].Outdated = false
	}
	l := &View{Data: d, OutdatedOnly: true}
	tt := render(t, l.Build, 900, 500)

	if !tt.HasText(locales.OutdatedEmpty()) {
		t.Errorf("outdated empty state missing: %v", tt.Texts())
	}
}

func TestUpdateActionRespectsCapability(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Update = false // npm
	d.Rows = d.Rows[1:]                       // typescript is outdated
	called := 0
	l := &View{Data: d, OnUpdate: func(manager.Package, flight.Origin) { called++ }}

	tt := render(t, l.Build, 1000, 500)
	if err := tt.Click(locales.ActionsUpdate()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if called != 0 {
		t.Fatalf("update fired for a manager without the capability")
	}
}

func TestBulkUpdateSendsOnlyUpdatablePackages(t *testing.T) {
	d := fixture()
	// A manager that cannot update, holding an outdated package: the old code
	// sent it along and the batch failed after earlier groups had run.
	d.Managers = append(d.Managers, manager.Info{
		ID: "cargo", Name: "Cargo", Enabled: true,
		Capabilities: manager.Capabilities{ListInstalled: true, Update: false},
	})
	d.Names["cargo"] = "Cargo"
	d.Rows = append(d.Rows,
		manager.Package{Name: "stale-crate", Version: "1.0.0", Manager: "cargo", Outdated: true})

	var got []manager.Package
	l := &View{Data: d, AllowRemove: true, OnBulkUpdate: func(pkgs []manager.Package, _ flight.Origin) { got = pkgs }}
	l.Sync()
	for _, pkg := range l.rows {
		l.choose(pkg, true)
	}

	tt := render(t, l.Build, 1000, 500)
	if err := tt.Click(locales.PackagesBulkUpdate()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if len(got) != 1 || got[0].Name != "typescript" {
		t.Fatalf("bulk update sent %+v, want only the updatable package", got)
	}
}

func TestDisabledActionExplainsItself(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Update = false // npm
	d.Rows = d.Rows[1:]                       // typescript, outdated
	l := &View{Data: d}

	tt := render(t, l.Build, 1000, 500)
	rect, ok := tt.Find(locales.ActionsUpdate())
	if !ok {
		t.Fatalf("update control not found: %v", tt.Texts())
	}
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Advance(2 * time.Second)

	for _, text := range tt.Texts() {
		if text == locales.ActionUnsupported() {
			return
		}
	}
	t.Fatalf("disabled action gave no reason: %v", tt.Texts())
}

func TestFirstScanSaysScanning(t *testing.T) {
	d := fixture()
	d.Rows = nil
	d.Refreshing = true
	l := &View{Data: d}
	tt := render(t, l.Build, 900, 500)

	if !tt.HasText(locales.PackagesScanning()) {
		t.Fatalf("first scan shows no progress text: %v", tt.Texts())
	}
	if tt.HasText(locales.PackagesEmpty()) {
		t.Fatalf("first scan claims the inventory is empty: %v", tt.Texts())
	}
}

func TestOutdatedViewFiltersRows(t *testing.T) {
	l := &View{Data: fixture(), OutdatedOnly: true}
	tt := render(t, l.Build, 900, 500)

	if !tt.HasText("typescript") {
		t.Errorf("outdated row missing: %v", tt.Texts())
	}
	if tt.HasText("ripgrep") {
		t.Errorf("up-to-date row should be filtered out: %v", tt.Texts())
	}
}

func TestListViewManagerFilter(t *testing.T) {
	l := &View{Data: fixture()}
	l.Sync()
	l.managers["npm"] = false
	l.Sync()
	if len(l.rows) != 1 || l.rows[0].Name != "ripgrep" {
		t.Fatalf("manager filter kept %d rows: %+v", len(l.rows), l.rows)
	}
}

func TestListViewQueryFilter(t *testing.T) {
	l := &View{Data: fixture(), query: "line-oriented"}
	l.Sync()
	if len(l.rows) != 1 || l.rows[0].Name != "ripgrep" {
		t.Fatalf("query filter kept %d rows: %+v", len(l.rows), l.rows)
	}

	l.query = "nothing matches this"
	l.Sync()
	if len(l.rows) != 0 {
		t.Fatalf("query filter kept %d rows", len(l.rows))
	}
}

func TestManagerCountsFollowThePage(t *testing.T) {
	d := fixture()

	installed := &View{Data: d}
	counts, total := installed.managerCounts()
	if counts["brew"] != 1 || counts["npm"] != 1 || total != 2 {
		t.Fatalf("installed counts = %v, total %d", counts, total)
	}

	// The outdated page counts only what it can show.
	outdated := &View{Data: d, OutdatedOnly: true}
	counts, total = outdated.managerCounts()
	if counts["brew"] != 0 || counts["npm"] != 1 || total != 1 {
		t.Fatalf("outdated counts = %v, total %d", counts, total)
	}

	// The query narrows them too, and the manager filter does not.
	outdated.query = "ripgrep"
	if _, total = outdated.managerCounts(); total != 0 {
		t.Fatalf("outdated total after query = %d, want 0", total)
	}
	outdated.query = ""
	outdated.managers = map[string]bool{"brew": true}
	if _, total = outdated.managerCounts(); total != 1 {
		t.Fatalf("manager filter changed the counts: total %d", total)
	}

	d.Rows = append(d.Rows, manager.Package{Name: "git", Manager: "brew", Outdated: true})
	d.Version++
	if counts, total = outdated.managerCounts(); counts["brew"] != 1 || total != 2 {
		t.Fatalf("counts after a new row = %v, total %d", counts, total)
	}
}

func TestManagerSelectListsManagersWithCounts(t *testing.T) {
	l := &View{Data: fixture(), chosen: map[string]struct{}{}}
	tt := render(t, l.Build, 1000, 500)

	exact := func(want string) bool {
		for _, text := range tt.Texts() {
			if text == want {
				return true
			}
		}
		return false
	}
	if exact("Homebrew (1)") || exact("NPM (1)") {
		t.Fatalf("counts shown before opening: %v", tt.Texts())
	}
	if err := tt.Click(locales.PackagesAllManagers()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	// Two packages overall, one in each manager.
	if !exact(locales.PackagesAllManagers() + " (2)") {
		t.Errorf("total count missing: %v", tt.Texts())
	}
	if !exact("Homebrew (1)") || !exact("NPM (1)") {
		t.Errorf("per-manager counts missing: %v", tt.Texts())
	}
}

func TestListViewSortsByName(t *testing.T) {
	l := &View{Data: fixture()}
	l.sortOrder = ui.SortOrder{Column: "name"}
	l.Sync()
	if l.rows[0].Name != "ripgrep" || l.rows[1].Name != "typescript" {
		t.Fatalf("ascending order wrong: %q, %q", l.rows[0].Name, l.rows[1].Name)
	}

	l.sortOrder = ui.SortOrder{Column: "name", Descending: true}
	l.applySort()
	if l.rows[0].Name != "typescript" || l.rows[1].Name != "ripgrep" {
		t.Fatalf("descending order wrong: %q, %q", l.rows[0].Name, l.rows[1].Name)
	}
}

func TestSelectionTracksVisibleRowsOnly(t *testing.T) {
	l := &View{Data: fixture()}
	l.Sync()
	l.choose(l.rows[0], true)
	l.chosen["brew:gone"] = struct{}{}

	selected := l.selectedPackages()
	if len(selected) != 1 || selected[0].Name != "ripgrep" {
		t.Fatalf("selectedPackages = %+v", selected)
	}

	l.managers["brew"] = false
	l.Sync()
	if len(l.chosen) != 0 {
		t.Fatalf("selection kept %d hidden rows", len(l.chosen))
	}
}

func TestBulkBarAppearsWithSelection(t *testing.T) {
	d := fixture()
	plain := &View{Data: d, AllowRemove: true}
	withoutSelection := render(t, plain.Build, 1000, 500)
	if withoutSelection.HasText(locales.PackagesBulkUpdate()) {
		t.Errorf("bulk actions shown without a selection")
	}

	selected := &View{Data: d, AllowRemove: true}
	selected.Sync()
	selected.choose(selected.rows[0], true)
	withSelection := render(t, selected.Build, 1000, 500)
	if !withSelection.HasText(locales.PackagesBulkUpdate()) {
		t.Errorf("bulk update missing: %v", withSelection.Texts())
	}
	if !withSelection.HasText(locales.PackagesBulkRemove()) {
		t.Errorf("bulk remove missing: %v", withSelection.Texts())
	}
	if !withSelection.HasText(locales.CommonSelectionCount(1, 2)) {
		t.Errorf("selected chip missing: %v", withSelection.Texts())
	}
}

func TestOutdatedPageHidesUninstall(t *testing.T) {
	l := &View{Data: fixture(), OutdatedOnly: true, AllowRemove: false}
	l.Sync()
	l.choose(l.rows[0], true)
	tt := render(t, l.Build, 1000, 500)

	if tt.HasText(locales.ActionsRemove()) {
		t.Errorf("row uninstall exposed on the outdated page: %v", tt.Texts())
	}
	if tt.HasText(locales.PackagesBulkRemove()) {
		t.Errorf("bulk uninstall exposed on the outdated page: %v", tt.Texts())
	}
	if !tt.HasText(locales.PackagesBulkUpdate()) {
		t.Errorf("bulk update missing: %v", tt.Texts())
	}
}

func TestUpdateActionRespectsOutdatedFlag(t *testing.T) {
	d := fixture()
	d.Rows = d.Rows[:1] // ripgrep is up to date
	called := 0
	l := &View{Data: d, OnUpdate: func(manager.Package, flight.Origin) { called++ }}

	tt := render(t, l.Build, 1000, 500)
	if err := tt.Click(locales.ActionsUpdate()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if called != 0 {
		t.Fatalf("update fired for an up-to-date package")
	}
}

func TestUpdateActionFires(t *testing.T) {
	d := fixture()
	d.Rows = d.Rows[1:] // typescript is outdated
	var got manager.Package
	called := 0
	l := &View{Data: d, OnUpdate: func(pkg manager.Package, _ flight.Origin) { called++; got = pkg }}

	tt := render(t, l.Build, 1000, 500)
	if err := tt.Click(locales.ActionsUpdate()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if called != 1 {
		t.Fatalf("update fired %d times, want 1", called)
	}
	if got.Name != "typescript" || got.Manager != "npm" {
		t.Fatalf("wrong package: %+v", got)
	}
}

// Turning a manager off while its filter is active must not leave the page on
// a filter the manager select no longer offers.
func TestDisabledManagerFilterResets(t *testing.T) {
	d := fixture()
	l := &View{Data: d, chosen: map[string]struct{}{}}
	l.Sync()
	l.managers["brew"] = false // select NPM only

	// The rescan that follows the setting drops the manager's rows.
	d.Rows = d.Rows[:1]
	d.SetManagers([]manager.Info{
		{ID: "brew", Name: "Homebrew", Enabled: true, Available: true},
	}, map[string]manager.Settings{})
	l.Sync()

	if l.managers["npm"] || !l.managers["brew"] {
		t.Fatalf("filter kept the turned-off manager: %+v", l.managers)
	}
	if len(l.rows) != 1 || l.rows[0].Manager != "brew" {
		t.Fatalf("rows after reset = %+v", l.rows)
	}
}

// The installed and outdated pages' manager filter is a multi-select; checking
// and unchecking managers rebuilds the table from the cached rows.
// A manager enabled while everything was selected joins the selection; its
// packages must not disappear behind the filter.
func TestEnablingManagerKeepsAllSelected(t *testing.T) {
	d := fixture()
	l := &View{Data: d, chosen: map[string]struct{}{}}
	tt := render(t, l.Build, 1000, 500)

	d.Managers[2].Enabled = true
	d.ManagersVersion++
	d.Rows = append(d.Rows, manager.Package{Name: "serde", Version: "1.0.0", Manager: "cargo", Installed: true})
	d.Version++
	tt.Frame()

	if !l.managers["cargo"] {
		t.Fatal("the newly enabled manager was not selected")
	}
	found := false
	for _, row := range l.rows {
		if row.Name == "serde" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the new manager's package is hidden: %+v", l.rows)
	}
}

func TestManagerFilterMultiSelect(t *testing.T) {
	l := &View{Data: fixture(), chosen: map[string]struct{}{}}
	tt := render(t, l.Build, 1000, 500)

	if err := tt.Click(locales.PackagesAllManagers()); err != nil {
		t.Fatalf("open the filter: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Homebrew (1)"); err != nil {
		t.Fatalf("uncheck Homebrew: %v", err)
	}
	tt.Frame()

	if l.managers["brew"] {
		t.Fatal("Homebrew stayed selected")
	}
	if len(l.rows) != 1 || l.rows[0].Manager != "npm" {
		t.Fatalf("rows after unchecking = %+v", l.rows)
	}
}

// longList is a list long enough to scroll, with a manager column that sorts into
// a different order than the name column.
func longList() *View {
	d := fixture()
	d.Rows = nil
	for i := range 200 {
		d.Rows = append(d.Rows, manager.Package{
			Name:      fmt.Sprintf("pkg-%03d", i),
			Manager:   []string{"brew", "npm"}[i%2],
			Installed: true,
		})
	}
	return &View{Data: d, chosen: map[string]struct{}{}, AllowRemove: true}
}

// Sorting a column moves rows, so a table that kept its place by the item at
// its top would scroll to wherever that item went, which often means the end.
func TestSortKeepsScrollOffset(t *testing.T) {
	l := longList()
	tt := render(t, l.Build, 900, 400)

	l.table.List.ScrollTo(100, ui.Start)
	tt.Frame()
	before, _ := l.table.List.Visible()

	if err := tt.Click(locales.TableManager()); err != nil {
		t.Fatalf("click the manager header: %v", err)
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
	tt := render(t, l.Build, 900, 400)

	l.table.List.ScrollTo(100, ui.Start)
	tt.Frame()

	l.query = "pkg-1"
	tt.Frame()
	first, last := l.table.List.Visible()

	if last != len(l.rows)-1 {
		t.Fatalf("visible %d..%d of %d rows, want the end of the list", first, last, len(l.rows))
	}
}

// The row buttons belong to the actions column, and the trailing pad column
// holds nothing: with the buttons drawn in the pad they are squeezed to a
// sliver at the window edge, which is what a stale column index does.
func TestRowActionsStayInTheActionsColumn(t *testing.T) {
	const width = 900
	l := &View{Data: fixture(), AllowRemove: true}
	tt := render(t, l.Build, width, 500)

	if _, ok := tt.Find(locales.TableActions()); !ok {
		t.Fatal("the actions header is missing")
	}
	for _, want := range []string{locales.ActionsUpdate(), locales.ActionsRemove()} {
		button, ok := tt.Find(want)
		if !ok {
			t.Fatalf("%q is missing", want)
		}
		if button.W == 0 || button.H == 0 {
			t.Errorf("%q is clipped to %+v", want, button)
		}
		// 12 DIPs of pad follow the table, so nothing may reach its right edge.
		if right := button.X + button.W; right > width-12 {
			t.Errorf("%q ends at %.0f, inside the trailing pad of a %d window: %+v", want, right, width, button)
		}
	}
}
