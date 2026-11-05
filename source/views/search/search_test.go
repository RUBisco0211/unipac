package search

import (
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/egoist/mygo/ui"
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

	s.apply("typescript", []manager.Package{
		{Name: "typescript", Manager: "npm"},
		{Name: "ripgrep", Manager: "brew"},
	}, nil)

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
	s.apply("query", nil, nil)
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
