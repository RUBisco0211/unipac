package search

import (
	"strconv"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func render(t *testing.T, build func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	return testui.Render(build, w, h)
}

func fixture() *inventory.Data { return testui.Data() }

// A search can return after a manager was turned off. Its results must not
// reappear just because the page already rebuilt its rows while it was running.
func TestSearchResultsArrivingAfterManagerOffAreFiltered(t *testing.T) {
	d := fixture()
	s := &View{Data: d, ran: true}
	d.SetManagers([]manager.Info{
		{ID: "brew", Name: "Homebrew", Enabled: true, Available: true},
		{ID: "npm", Name: "NPM", Enabled: false, Available: true},
	}, map[string]manager.Settings{})
	s.syncManagers() // the page rebuilt while the search was in flight

	s.apply("typescript", manager.SearchResult{Packages: []manager.Package{
		{Name: "typescript", Manager: "npm"},
		{Name: "ripgrep", Manager: "brew"},
	}}, nil)

	if len(s.results) != 1 || s.results[0].Manager != "brew" {
		t.Fatalf("results from the turned-off manager survived: %+v", s.results)
	}
}

// Results from a manager turned off after the search ran must not linger with
// a working install button.
func TestSearchDropsResultsFromTurnedOffManagers(t *testing.T) {
	d := fixture()
	s := &View{
		Data: d,
		ran:  true,
		results: []manager.Package{
			{Name: "typescript", Manager: "npm"},
			{Name: "ripgrep", Manager: "brew"},
		},
	}
	d.SetManagers([]manager.Info{
		{ID: "brew", Name: "Homebrew", Enabled: true, Available: true},
		{ID: "npm", Name: "NPM", Enabled: false, Available: true},
	}, map[string]manager.Settings{})

	s.syncManagers()

	if len(s.results) != 1 || s.results[0].Manager != "brew" {
		t.Fatalf("results after the manager was turned off: %+v", s.results)
	}
}

// A search that is still running shows the same indeterminate bar as a package
// refresh, over the body that says what is happening.
func TestSearchViewShowsProgressWhileSearching(t *testing.T) {
	s := &View{Data: fixture(), ran: true, searching: true, resultQuery: "rip"}
	tt := render(t, s.Build, 900, 500)
	if !tt.HasText(locales.SearchSearching()) {
		t.Fatalf("searching state not shown: %v", tt.Texts())
	}
	if _, ok := tt.Find(locales.SearchProgress()); !ok {
		t.Fatalf("search progress bar missing: %v", tt.Texts())
	}

	s.searching = false
	tt.Frame()
	if _, ok := tt.Find(locales.SearchProgress()); ok {
		t.Fatalf("search progress bar stayed after the search: %v", tt.Texts())
	}
}

func TestSearchViewHintBeforeSearch(t *testing.T) {
	s := &View{Data: fixture()}
	tt := render(t, s.Build, 900, 500)
	if !tt.HasText(locales.SearchHint()) {
		t.Errorf("hint not shown: %v", tt.Texts())
	}
}

func TestSearchViewRendersResults(t *testing.T) {
	s := &View{
		Data:    fixture(),
		ran:     true,
		query:   "rip",
		results: []manager.Package{{Name: "ripgrep", Version: "14.1.1", LatestVersion: "15.0.0", Manager: "brew"}},
	}
	tt := render(t, s.Build, 900, 500)
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

// The search page's manager multi-select sends the selection to the backend and
// reruns the current query with it.
func TestSearchManagerFilterGoesToTheBackend(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Search = true // NPM
	s := &View{Data: d, ran: true, resultQuery: "query", activeQuery: "query"}
	var (
		calls int
		gotQ  string
		gotID []string
	)
	s.Submit = func(query string, managerIDs []string) {
		calls++
		gotQ = query
		gotID = managerIDs
	}
	s.syncManagers()
	if ids := s.selectedManagerIDs(); ids != nil {
		t.Fatalf("all managers should send nil, got %v", ids)
	}

	tt := render(t, s.Build, 1000, 500)
	if err := tt.Click(locales.PackagesAllManagers()); err != nil {
		t.Fatalf("open the filter: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Homebrew"); err != nil {
		t.Fatalf("uncheck Homebrew: %v", err)
	}
	tt.Frame()

	if calls != 1 || gotQ != "query" {
		t.Fatalf("search rerun = %d for %q", calls, gotQ)
	}
	if len(gotID) != 1 || gotID[0] != "npm" {
		t.Fatalf("manager IDs sent = %v, want npm only", gotID)
	}
}

// A manager change while the first search is still in flight reruns that
// query, even though no results have arrived yet.
func TestSearchRerunUsesInFlightQuery(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Search = true
	s := &View{Data: d, activeQuery: "bar"}
	var got string
	s.Submit = func(query string, _ []string) { got = query }
	s.syncManagers()
	s.rerunSearch()
	if got != "bar" {
		t.Fatalf("reran %q, want the in-flight query", got)
	}
}

// A manager list refresh from an unrelated settings save keeps the user's
// search-manager choices.
func TestSearchManagerFilterSurvivesListRefresh(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Search = true
	s := &View{Data: d}
	s.syncManagers()
	s.managerFilter["npm"] = false
	s.managerAll = s.allManagersSelected()

	d.ManagersVersion++
	s.syncManagers()
	if s.managerFilter["npm"] {
		t.Fatal("the narrowed search filter was reset")
	}
	if s.managerFilter["brew"] != true {
		t.Fatal("a still-valid selection was dropped")
	}
}

// A search finishing after a manager change must not tell the view that the new
// list was already reconciled.
func TestSearchApplyLeavesManagerReconciliation(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Search = true
	s := &View{Data: d, activeQuery: "query"}
	s.syncManagers()

	d.Managers[2].Enabled = true
	d.Managers[2].Capabilities.Search = true
	d.ManagersVersion++
	s.apply("query", manager.SearchResult{}, nil)
	s.syncManagers()

	if !s.managerFilter["cargo"] {
		t.Fatal("the manager list was not reconciled after the search finished")
	}
}

// A newly searchable manager joins an "all" selection but not a narrowed one.
func TestSearchManagerAllIncludesNewManager(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Search = true
	s := &View{Data: d}
	s.syncManagers()

	d.Managers[2].Enabled = true
	d.Managers[2].Capabilities.Search = true
	d.ManagersVersion++
	s.syncManagers()
	if !s.managerFilter["cargo"] {
		t.Fatal("the new searchable manager did not join the all selection")
	}

	s.managerFilter["cargo"] = false
	s.managerAll = false
	d.ManagersVersion++
	s.syncManagers()
	if s.managerFilter["cargo"] {
		t.Fatal("a narrowed selection gained a manager")
	}
}

// Unchecking every manager is different from "all": the backend is asked for
// nothing, not for everything.
func TestSearchNoManagersIsAnEmptySelection(t *testing.T) {
	d := fixture()
	d.Managers[1].Capabilities.Search = true
	s := &View{Data: d}
	s.syncManagers()
	for id := range s.managerFilter {
		s.managerFilter[id] = false
	}
	ids := s.selectedManagerIDs()
	if ids == nil || len(ids) != 0 {
		t.Fatalf("empty selection = %v, want a non-nil empty slice", ids)
	}
}

// A search a manager filled to its limit says so, rather than looking like the
// whole answer.
func TestSearchSaysWhenTheListWasCut(t *testing.T) {
	s := &View{
		Data: fixture(), ran: true, resultQuery: "a", truncated: true,
		results: []manager.Package{{Name: "ripgrep", Manager: "brew"}},
	}
	tt := render(t, s.Build, 900, 500)
	note := locales.SearchTruncated(strconv.Itoa(manager.MaxSearchResults))
	if !tt.HasText(note) {
		t.Errorf("the cut is not mentioned: %v", tt.Texts())
	}

	s.truncated = false
	tt.Frame()
	if tt.HasText(note) {
		t.Errorf("the cut is mentioned after a full answer: %v", tt.Texts())
	}
}

// A result the inventory already holds says so; the next one does not.
func TestSearchMarksInstalledResults(t *testing.T) {
	s := &View{
		Data: fixture(), ran: true, resultQuery: "type",
		results: []manager.Package{
			{Name: "vite", Manager: "npm"},
			{Name: "typescript", Manager: "npm"},
		},
	}
	tt := render(t, s.Build, 900, 500)

	badge, ok := tt.Find(locales.SearchInstalled())
	if !ok {
		t.Fatalf("the installed result is not marked: %v", tt.Texts())
	}
	row, ok := tt.Find("typescript")
	if !ok {
		t.Fatal("the installed row is missing")
	}
	if badge.Y < row.Y-2 || badge.Y > row.Y+row.H {
		t.Errorf("the badge at %v is not on the installed row %v", badge, row)
	}
	seen := 0
	for _, text := range tt.Texts() {
		if text == locales.SearchInstalled() {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("%d results are marked installed, want 1", seen)
	}
}

// The search table ends where the package tables do, and its install button is
// built once, in the actions column rather than in the trailing pad.
func TestSearchTableInsetsAndOneActionButton(t *testing.T) {
	const width = 900
	s := &View{Data: fixture()}
	s.BeginQuery()
	s.Apply(1, "vite", manager.SearchResult{Packages: []manager.Package{
		{Name: "vite", Manager: "npm", Version: "5.0.0"},
	}}, nil)
	tt := render(t, s.Build, width, 500)

	seen := 0
	for _, text := range tt.Texts() {
		if text == locales.ActionsInstall() {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("%d install buttons, want one", seen)
	}
	button, ok := tt.Find(locales.ActionsInstall())
	if !ok {
		t.Fatal("the install button is missing")
	}
	if right := button.X + button.W; right > width-12 {
		t.Errorf("the install button ends at %.0f, inside the trailing pad of a %d window: %+v", right, width, button)
	}
}

// A result the inventory holds at another version offers an update instead of an
// install, and says so beside its name; the same version is simply installed.
func TestSearchRowOffersAnUpdateForANewerVersion(t *testing.T) {
	s := &View{Data: fixture()}
	s.BeginQuery()
	s.Apply(1, "typescript", manager.SearchResult{Packages: []manager.Package{
		{Name: "typescript", Manager: "npm", Version: "6.0.0"}, // the inventory has 5.9.3
	}}, nil)
	tt := render(t, s.Build, 900, 500)

	if !tt.HasText(locales.SearchUpdateAvailable()) {
		t.Errorf("the newer version is not marked: %v", tt.Texts())
	}
	if tt.HasText(locales.SearchInstalled()) {
		t.Errorf("the row claims to be installed and current: %v", tt.Texts())
	}
	if !tt.HasText(locales.ActionsUpdate()) {
		t.Errorf("the row does not offer an update: %v", tt.Texts())
	}
	if tt.HasText(locales.ActionsInstall()) {
		t.Errorf("the row still offers an install: %v", tt.Texts())
	}
}

// The stored version is the current one: installed, and the button stays an
// install, which is how a package is reinstalled.
func TestSearchRowMarksTheInstalledVersion(t *testing.T) {
	s := &View{Data: fixture()}
	s.BeginQuery()
	s.Apply(1, "typescript", manager.SearchResult{Packages: []manager.Package{
		{Name: "typescript", Manager: "npm", Version: "5.9.3"},
	}}, nil)
	tt := render(t, s.Build, 900, 500)

	if !tt.HasText(locales.SearchInstalled()) {
		t.Errorf("the installed version is not marked: %v", tt.Texts())
	}
	if tt.HasText(locales.SearchUpdateAvailable()) {
		t.Errorf("a current package is marked updatable: %v", tt.Texts())
	}
	if !tt.HasText(locales.ActionsInstall()) {
		t.Errorf("the install button is missing: %v", tt.Texts())
	}
}

// The update button calls the page's update, not its install.
func TestSearchUpdateCallsOnUpdate(t *testing.T) {
	s := &View{Data: fixture()}
	var installed, updated []string
	s.OnInstall = func(pkg manager.Package, _ flight.Origin) { installed = append(installed, pkg.Name) }
	s.OnUpdate = func(pkg manager.Package, _ flight.Origin) { updated = append(updated, pkg.Name) }
	s.BeginQuery()
	s.Apply(1, "typescript", manager.SearchResult{Packages: []manager.Package{
		{Name: "typescript", Manager: "npm", Version: "6.0.0"},
	}}, nil)

	tt := render(t, s.Build, 900, 500)
	if err := tt.Click(locales.ActionsUpdate()); err != nil {
		t.Fatalf("click: %v", err)
	}
	if len(updated) != 1 || updated[0] != "typescript" {
		t.Errorf("updated = %v, want typescript", updated)
	}
	if len(installed) != 0 {
		t.Errorf("install was called for an update: %v", installed)
	}
}

// The header's first cell and the row's share the table's inset: Onevra lays both
// out with the same padding, so an inset on a cell alone leaves the header behind.
func TestSearchHeaderStartsWhereItsRowsDo(t *testing.T) {
	s := &View{Data: fixture()}
	s.BeginQuery()
	s.Apply(1, "typescript", manager.SearchResult{Packages: []manager.Package{
		{Name: "typescript", Manager: "npm", Version: "6.0.0"},
	}}, nil)
	tt := render(t, s.Build, 900, 500)

	header, ok := tt.Find(locales.TablePackage())
	if !ok {
		t.Fatal("the package header is missing")
	}
	if want := theme.Palette(false).Space(1.5) + theme.Palette(false).Space(2.5); header.X != want {
		t.Errorf("the header starts at %.0f, want the table's inset and cell padding, %.0f", header.X, want)
	}
	row, ok := tt.Find("typescript")
	if !ok {
		t.Fatal("the result row is missing")
	}
	// The row starts with a 16-wide icon and an 8-wide gap before its name.
	if want := header.X + 16 + 8; row.X != want {
		t.Errorf("the name starts at %.0f, want %.0f under the header", row.X, want)
	}
}

// A working row's button turns, and holds still when the desktop asks for less
// motion.
func TestSearchWorkingButtonTurns(t *testing.T) {
	s := &View{Data: fixture()}
	s.BeginQuery()
	s.Apply(1, "typescript", manager.SearchResult{Packages: []manager.Package{
		{Name: "typescript", Manager: "npm", Version: "6.0.0"},
	}}, nil)
	s.Data.BeginTask("update", []manager.Package{{Manager: "npm", Name: "typescript"}})

	tt := render(t, s.Build, 900, 500)
	if !tt.HasText(locales.ActionsUpdating()) {
		t.Fatalf("the row does not say it is updating: %v", tt.Texts())
	}
	if !tt.FramePending() {
		t.Fatal("the working button asks for no further frames")
	}
}
