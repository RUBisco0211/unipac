package views

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/backend/core/manager"
	"github.com/RUBisco0211/unipac/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func fixture() *data {
	return &data{
		names:   map[string]string{"brew": "Homebrew", "npm": "NPM"},
		version: 1,
		managers: []manager.Info{
			{ID: "brew", Name: "Homebrew", Enabled: true, ExecPath: "/opt/homebrew/bin/brew",
				Capabilities: manager.Capabilities{ListInstalled: true, ListOutdated: true, Search: true, Update: true, Uninstall: true}},
			{ID: "npm", Name: "NPM", Enabled: true,
				Capabilities: manager.Capabilities{ListInstalled: true, Update: true, Uninstall: true}},
			{ID: "cargo", Name: "Cargo", Enabled: false},
		},
		rows: []manager.Package{
			{Name: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true, Description: "Line-oriented search tool"},
			{Name: "typescript", Version: "5.9.3", LatestVersion: "6.0.0", Manager: "npm", Installed: true, Outdated: true},
		},
	}
}

func render(t *testing.T, build func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		build(c)
	}, w, h)
}

func TestListViewShowsRows(t *testing.T) {
	l := &listView{data: fixture()}
	tt := render(t, l.view, 900, 500)

	for _, want := range []string{
		"ripgrep", "14.1.1", "Line-oriented search tool",
		"Homebrew", "typescript", "6.0.0", locales.TypeCli(),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
}

func TestListViewEmptyState(t *testing.T) {
	d := fixture()
	d.rows = nil
	l := &listView{data: d}
	tt := render(t, l.view, 900, 500)
	if !tt.HasText(locales.PackagesEmpty()) {
		t.Errorf("empty state not shown: %v", tt.Texts())
	}
}

func TestEmptyStateIsCentredWithText(t *testing.T) {
	d := fixture()
	d.rows = nil
	l := &listView{data: d}
	tt := render(t, l.view, 900, 500)

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

func TestRefreshingShowsSkeleton(t *testing.T) {
	d := fixture()
	d.refreshing = true
	l := &listView{data: d}
	tt := render(t, l.view, 900, 500)

	if tt.HasText("ripgrep") {
		t.Errorf("rows shown while refreshing: %v", tt.Texts())
	}
	if tt.HasText(locales.PackagesEmpty()) {
		t.Errorf("empty state shown while refreshing: %v", tt.Texts())
	}
}

func TestOutdatedViewFiltersRows(t *testing.T) {
	l := &listView{data: fixture(), outdatedOnly: true}
	tt := render(t, l.view, 900, 500)

	if !tt.HasText("typescript") {
		t.Errorf("outdated row missing: %v", tt.Texts())
	}
	if tt.HasText("ripgrep") {
		t.Errorf("up-to-date row should be filtered out: %v", tt.Texts())
	}
}

func TestListViewManagerFilter(t *testing.T) {
	l := &listView{data: fixture(), manager: "brew"}
	l.sync()
	if len(l.rows) != 1 || l.rows[0].Name != "ripgrep" {
		t.Fatalf("manager filter kept %d rows: %+v", len(l.rows), l.rows)
	}
}

func TestListViewQueryFilter(t *testing.T) {
	l := &listView{data: fixture(), query: "line-oriented"}
	l.sync()
	if len(l.rows) != 1 || l.rows[0].Name != "ripgrep" {
		t.Fatalf("query filter kept %d rows: %+v", len(l.rows), l.rows)
	}

	l.query = "nothing matches this"
	l.sync()
	if len(l.rows) != 0 {
		t.Fatalf("query filter kept %d rows", len(l.rows))
	}
}

func TestManagerCountsFollowThePage(t *testing.T) {
	d := fixture()

	installed := &listView{data: d}
	counts, total := installed.managerCounts()
	if counts["brew"] != 1 || counts["npm"] != 1 || total != 2 {
		t.Fatalf("installed counts = %v, total %d", counts, total)
	}

	// The outdated page counts only what it can show.
	outdated := &listView{data: d, outdatedOnly: true}
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
	outdated.manager = "brew"
	if _, total = outdated.managerCounts(); total != 1 {
		t.Fatalf("manager filter changed the counts: total %d", total)
	}

	d.rows = append(d.rows, manager.Package{Name: "git", Manager: "brew", Outdated: true})
	d.version++
	if counts, total = outdated.managerCounts(); counts["brew"] != 1 || total != 2 {
		t.Fatalf("counts after a new row = %v, total %d", counts, total)
	}
}

func TestManagerSelectListsManagersWithCounts(t *testing.T) {
	l := &listView{data: fixture(), chosen: map[string]struct{}{}}
	tt := render(t, l.view, 1000, 500)

	exact := func(want string) bool {
		for _, text := range tt.Texts() {
			if text == want {
				return true
			}
		}
		return false
	}
	if exact("2") || exact("1") {
		t.Fatalf("counts shown before opening: %v", tt.Texts())
	}
	if err := tt.Click(locales.PackagesAllManagers()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	// Two packages overall, one in each manager.
	if !exact("2") {
		t.Errorf("total count missing: %v", tt.Texts())
	}
	if !exact("1") {
		t.Errorf("per-manager count missing: %v", tt.Texts())
	}
}

func TestListViewSortsByName(t *testing.T) {
	l := &listView{data: fixture()}
	l.sortOrder = ui.SortOrder{Column: "name"}
	l.sync()
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
	l := &listView{data: fixture()}
	l.sync()
	l.choose(l.rows[0], true)
	l.chosen["brew:gone"] = struct{}{}

	selected := l.selectedPackages()
	if len(selected) != 1 || selected[0].Name != "ripgrep" {
		t.Fatalf("selectedPackages = %+v", selected)
	}

	l.manager = "npm"
	l.sync()
	if len(l.chosen) != 0 {
		t.Fatalf("selection kept %d hidden rows", len(l.chosen))
	}
}

func TestBulkBarAppearsWithSelection(t *testing.T) {
	d := fixture()
	plain := &listView{data: d, allowRemove: true}
	withoutSelection := render(t, plain.view, 1000, 500)
	if withoutSelection.HasText(locales.PackagesBulkUpdate()) {
		t.Errorf("bulk actions shown without a selection")
	}

	selected := &listView{data: d, allowRemove: true}
	selected.sync()
	selected.choose(selected.rows[0], true)
	withSelection := render(t, selected.view, 1000, 500)
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
	l := &listView{data: fixture(), outdatedOnly: true, allowRemove: false}
	l.sync()
	l.choose(l.rows[0], true)
	tt := render(t, l.view, 1000, 500)

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
	d.rows = d.rows[:1] // ripgrep is up to date
	called := 0
	l := &listView{data: d, onUpdate: func(manager.Package) { called++ }}

	tt := render(t, l.view, 1000, 500)
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
	d.rows = d.rows[1:] // typescript is outdated
	var got manager.Package
	called := 0
	l := &listView{data: d, onUpdate: func(pkg manager.Package) { called++; got = pkg }}

	tt := render(t, l.view, 1000, 500)
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

func TestSettingsShowsManagers(t *testing.T) {
	u := &UI{data: fixture()}
	tt := render(t, u.settings, 900, 500)

	for _, want := range []string{"Homebrew", "/opt/homebrew/bin/brew", "Cargo", locales.SettingsDisabled()} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
}

func TestSidebarSwitchesPage(t *testing.T) {
	u := &UI{}
	tt := render(t, u.left, 240, 400)

	if u.page != PagePackages {
		t.Fatalf("initial page = %v, want packages", u.page)
	}
	if err := tt.Click(locales.NavOutdated()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if u.page != PageOutdated {
		t.Fatalf("page after click = %v, want outdated", u.page)
	}
}

// TestLoadingBarPaints checks the indeterminate bar actually draws something:
// the sweep is time-based, so the clock is advanced into the middle of a pass.
func TestLoadingBarPaints(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		ui.Column(c).Fill().Children(func() { loadingBar(c).Width(240) })
	}, 240, 12)
	tt.Advance(600 * time.Millisecond)

	img := tt.Image()
	b := img.Bounds()
	colors := map[uint32]struct{}{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			colors[uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(bl>>8)] = struct{}{}
		}
	}
	if len(colors) < 2 {
		t.Fatalf("loading bar painted one colour only: %v", colors)
	}
}

func TestSearchViewHintBeforeSearch(t *testing.T) {
	s := &searchView{data: fixture()}
	tt := render(t, s.view, 900, 500)
	if !tt.HasText(locales.SearchHint()) {
		t.Errorf("hint not shown: %v", tt.Texts())
	}
}

func TestSearchViewRendersResults(t *testing.T) {
	s := &searchView{
		data:    fixture(),
		ran:     true,
		query:   "rip",
		results: []manager.Package{{Name: "ripgrep", Version: "14.1.1", LatestVersion: "15.0.0", Manager: "brew"}},
	}
	tt := render(t, s.view, 900, 500)
	for _, want := range []string{"ripgrep", "15.0.0", "Homebrew"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	// A search result is not installed, so its installed version is not shown.
	if tt.HasText("14.1.1") {
		t.Errorf("search results show an installed version: %v", tt.Texts())
	}
}
