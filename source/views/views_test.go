package views

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func fixture() *data {
	return &data{
		names:   map[string]string{"brew": "Homebrew", "npm": "NPM"},
		version: 1,
		managers: []manager.Info{
			{ID: "brew", Name: "Homebrew", Enabled: true, Available: true, ExecPath: "/opt/homebrew/bin/brew",
				Capabilities: manager.Capabilities{ListInstalled: true, ListOutdated: true, Search: true, Update: true, Uninstall: true}},
			{ID: "npm", Name: "NPM", Enabled: true, Available: true,
				Capabilities: manager.Capabilities{ListInstalled: true, Update: true, Uninstall: true}},
			{ID: "cargo", Name: "Cargo", Enabled: false},
		},
		rows: []manager.Package{
			{Name: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true, Description: "Line-oriented search tool"},
			{Name: "typescript", Version: "5.9.3", LatestVersion: "6.0.0", Manager: "npm", Installed: true, Outdated: true},
		},
	}
}

func settingsFixture() *settingsView {
	return &settingsView{
		data:      fixture(),
		autoScan:  true,
		logLevel:  "info",
		levels:    []string{"debug", "info", "warn", "error"},
		language:  "",
		languages: []string{"en", "zh-CN"},
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

// A background refresh must keep the cached rows on screen: the window opens on
// them and only the thin bar says a scan is running.
func TestRefreshKeepsCachedRows(t *testing.T) {
	d := fixture()
	d.refreshing = true
	l := &listView{data: d}
	tt := render(t, l.view, 900, 500)

	if !tt.HasText("ripgrep") || !tt.HasText("typescript") {
		t.Errorf("cached rows hidden while refreshing: %v", tt.Texts())
	}
}

func TestOutdatedEmptyState(t *testing.T) {
	d := fixture()
	for i := range d.rows {
		d.rows[i].Outdated = false
	}
	l := &listView{data: d, outdatedOnly: true}
	tt := render(t, l.view, 900, 500)

	if !tt.HasText(locales.OutdatedEmpty()) {
		t.Errorf("outdated empty state missing: %v", tt.Texts())
	}
}

func TestUpdateActionRespectsCapability(t *testing.T) {
	d := fixture()
	d.managers[1].Capabilities.Update = false // npm
	d.rows = d.rows[1:]                       // typescript is outdated
	called := 0
	l := &listView{data: d, onUpdate: func(manager.Package, actionOrigin) { called++ }}

	tt := render(t, l.view, 1000, 500)
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
	d.managers = append(d.managers, manager.Info{
		ID: "cargo", Name: "Cargo", Enabled: true,
		Capabilities: manager.Capabilities{ListInstalled: true, Update: false},
	})
	d.names["cargo"] = "Cargo"
	d.rows = append(d.rows,
		manager.Package{Name: "stale-crate", Version: "1.0.0", Manager: "cargo", Outdated: true})

	var got []manager.Package
	l := &listView{data: d, allowRemove: true, onBulkUpdate: func(pkgs []manager.Package, _ actionOrigin) { got = pkgs }}
	l.sync()
	for _, pkg := range l.rows {
		l.choose(pkg, true)
	}

	tt := render(t, l.view, 1000, 500)
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
	d.managers[1].Capabilities.Update = false // npm
	d.rows = d.rows[1:]                       // typescript, outdated
	l := &listView{data: d}

	tt := render(t, l.view, 1000, 500)
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

// A request that arrives mid-scan must not be dropped: the follow-up scan is
// what stops an action from leaving pre-action rows on screen.
func TestRefreshQueuesOneFollowUp(t *testing.T) {
	d := &data{}

	if !d.beginRefresh() {
		t.Fatal("first refresh did not start")
	}
	if d.beginRefresh() {
		t.Fatal("second refresh started while one was running")
	}
	if !d.refreshPending {
		t.Fatal("the mid-scan request was dropped")
	}
	if !d.finishRefresh(nil, nil, nil) {
		t.Fatal("finishing did not report the queued scan")
	}
	if d.refreshPending {
		t.Fatal("the queued scan was not cleared")
	}
	if !d.beginRefresh() {
		t.Fatal("the queued scan did not start")
	}
}

func TestFirstScanSaysScanning(t *testing.T) {
	d := fixture()
	d.rows = nil
	d.refreshing = true
	l := &listView{data: d}
	tt := render(t, l.view, 900, 500)

	if !tt.HasText(locales.PackagesScanning()) {
		t.Fatalf("first scan shows no progress text: %v", tt.Texts())
	}
	if tt.HasText(locales.PackagesEmpty()) {
		t.Fatalf("first scan claims the inventory is empty: %v", tt.Texts())
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
	l := &listView{data: fixture()}
	l.sync()
	l.managers["npm"] = false
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
	outdated.managers = map[string]bool{"brew": true}
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

	l.managers["brew"] = false
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
	l := &listView{data: d, onUpdate: func(manager.Package, actionOrigin) { called++ }}

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
	l := &listView{data: d, onUpdate: func(pkg manager.Package, _ actionOrigin) { called++; got = pkg }}

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
	s := settingsFixture()
	tt := render(t, s.view, 900, 600)

	for _, want := range []string{
		"Homebrew", "NPM", "Cargo",
		locales.SettingsGeneral(), locales.SettingsManagers(),
		locales.SettingsStartupScan(), locales.SettingsLogLevel(),
		locales.SettingsLanguage(), locales.SettingsLanguageSystem(),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	// A manager the backend could not preflight says so; one that is simply
	// off does not need a second label next to its switch.
	if !tt.HasText(locales.SettingsNotFound()) {
		t.Errorf("missing the not-found state in %v", tt.Texts())
	}
	// Manager IDs belong to the backend, not the page.
	for _, text := range tt.Texts() {
		if text == "brew" || text == "cargo" {
			t.Errorf("settings page exposes a manager ID: %v", tt.Texts())
		}
	}
}

// A path typed into a manager's field is sent when its Apply is pressed, and a
// frame in between must not rebuild the row and lose the draft.
func TestSettingsApplySendsPath(t *testing.T) {
	s := settingsFixture()
	var gotID, gotPath string
	s.onSetPath = func(id, path string) { gotID, gotPath = id, path }

	tt := render(t, s.view, 900, 600)
	s.rows[0].path = "/usr/local/bin/brew"
	tt.Frame()
	if s.rows[0].path != "/usr/local/bin/brew" {
		t.Fatalf("a rebuild discarded the draft: %q", s.rows[0].path)
	}

	if err := tt.Click(locales.SettingsApply()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if gotID != "brew" || gotPath != "/usr/local/bin/brew" {
		t.Fatalf("apply sent %q %q, want brew /usr/local/bin/brew", gotID, gotPath)
	}
}

// The switch on a manager row carries that manager's ID to the backend.
func TestSettingsSwitchReachesBackend(t *testing.T) {
	s := settingsFixture()
	var gotID string
	var gotOn bool
	s.onSetEnabled = func(id string, on bool) { gotID, gotOn = id, on }

	tt := render(t, s.view, 900, 600)
	if err := tt.Click(locales.SettingsEnableManager("Homebrew")); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if gotID != "brew" || gotOn {
		t.Fatalf("switch sent %q %v, want brew false", gotID, gotOn)
	}
}

// Choosing a log level reaches the backend, and the select shows the value.
func TestSettingsLogLevelReachesBackend(t *testing.T) {
	s := settingsFixture()
	var got string
	s.onSetLogLevel = func(level string) { got = level }

	tt := render(t, s.view, 900, 600)
	// The trigger shows the current level; clicking it opens the popup.
	if err := tt.Click(locales.LogLevelInfo()); err != nil {
		t.Fatalf("open the level select: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.LogLevelDebug()); err != nil {
		t.Fatalf("choose a level: %v", err)
	}
	tt.Frame()
	if got != "debug" {
		t.Fatalf("log level sent %q, want debug", got)
	}
}

// Choosing a language reaches the backend, and the trigger shows the choice.
func TestSettingsLanguageReachesBackend(t *testing.T) {
	s := settingsFixture()
	var got string
	s.onSetLanguage = func(tag string) { got = tag }

	tt := render(t, s.view, 900, 600)
	if err := tt.Click(locales.SettingsLanguageSystem()); err != nil {
		t.Fatalf("open the language select: %v", err)
	}
	tt.Frame()
	if err := tt.Click("简体中文"); err != nil {
		t.Fatalf("choose a language: %v", err)
	}
	tt.Frame()
	if got != "zh-CN" {
		t.Fatalf("language sent %q, want zh-CN", got)
	}
	if !tt.HasText("简体中文") {
		t.Errorf("the trigger kept the old language: %v", tt.Texts())
	}
}

// A search can return after a manager was turned off. Its results must not
// reappear just because the page already rebuilt its rows while it was running.
func TestSearchResultsArrivingAfterManagerOffAreFiltered(t *testing.T) {
	d := fixture()
	s := &searchView{data: d, ran: true}
	d.setManagers([]manager.Info{
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

// Turning a manager off while its filter is active must not leave the page on
// a filter the manager select no longer offers.
func TestDisabledManagerFilterResets(t *testing.T) {
	d := fixture()
	l := &listView{data: d, chosen: map[string]struct{}{}}
	l.sync()
	l.managers["brew"] = false // select NPM only

	// The rescan that follows the setting drops the manager's rows.
	d.rows = d.rows[:1]
	d.setManagers([]manager.Info{
		{ID: "brew", Name: "Homebrew", Enabled: true, Available: true},
	}, map[string]manager.Settings{})
	l.sync()

	if l.managers["npm"] || !l.managers["brew"] {
		t.Fatalf("filter kept the turned-off manager: %+v", l.managers)
	}
	if len(l.rows) != 1 || l.rows[0].Manager != "brew" {
		t.Fatalf("rows after reset = %+v", l.rows)
	}
}

// Results from a manager turned off after the search ran must not linger with
// a working install button.
func TestSearchDropsResultsFromTurnedOffManagers(t *testing.T) {
	d := fixture()
	s := &searchView{
		data: d,
		ran:  true,
		results: []manager.Package{
			{Name: "typescript", Manager: "npm"},
			{Name: "ripgrep", Manager: "brew"},
		},
	}
	d.setManagers([]manager.Info{
		{ID: "brew", Name: "Homebrew", Enabled: true, Available: true},
		{ID: "npm", Name: "NPM", Enabled: false, Available: true},
	}, map[string]manager.Settings{})

	s.syncManagers()

	if len(s.results) != 1 || s.results[0].Manager != "brew" {
		t.Fatalf("results after the manager was turned off: %+v", s.results)
	}
}

// Turning a manager off must reach the backend with the manager's own state,
// and a failed save must put the switch back.
func TestSettingsSwitchAndRevert(t *testing.T) {
	s := settingsFixture()
	s.data.overrides = map[string]manager.Settings{
		"brew":  {Disabled: true},
		"npm":   {},
		"cargo": {Disabled: true},
	}
	tt := render(t, s.view, 900, 600)
	if s.rows[0].enabled {
		t.Fatal("Homebrew switch ignored the saved state")
	}

	s.invalidate()
	tt.Frame()
	if s.rows[0].enabled {
		t.Fatal("rebuild did not restore the saved switch state")
	}
	if s.rows[0].path != "" {
		t.Fatalf("unsaved path was invented: %q", s.rows[0].path)
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

// A search that is still running shows the same indeterminate bar as a package
// refresh, over the body that says what is happening.
func TestSearchViewShowsProgressWhileSearching(t *testing.T) {
	s := &searchView{data: fixture(), ran: true, searching: true, resultQuery: "rip"}
	tt := render(t, s.view, 900, 500)
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

// The search page's manager multi-select sends the selection to the backend and
// reruns the current query with it.
func TestSearchManagerFilterGoesToTheBackend(t *testing.T) {
	d := fixture()
	d.managers[1].Capabilities.Search = true // NPM
	s := &searchView{data: d, ran: true, resultQuery: "query"}
	var (
		calls int
		gotQ  string
		gotID []string
	)
	s.submit = func(query string, managerIDs []string) {
		calls++
		gotQ = query
		gotID = managerIDs
	}
	s.syncManagers()
	if ids := s.selectedManagerIDs(); ids != nil {
		t.Fatalf("all managers should send nil, got %v", ids)
	}

	tt := render(t, s.view, 1000, 500)
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

// Unchecking every manager is different from "all": the backend is asked for
// nothing, not for everything.
func TestSearchNoManagersIsAnEmptySelection(t *testing.T) {
	d := fixture()
	d.managers[1].Capabilities.Search = true
	s := &searchView{data: d}
	s.syncManagers()
	for id := range s.managerFilter {
		s.managerFilter[id] = false
	}
	ids := s.selectedManagerIDs()
	if ids == nil || len(ids) != 0 {
		t.Fatalf("empty selection = %v, want a non-nil empty slice", ids)
	}
}

// The installed and outdated pages' manager filter is a multi-select; checking
// and unchecking managers rebuilds the table from the cached rows.
func TestManagerFilterMultiSelect(t *testing.T) {
	l := &listView{data: fixture(), chosen: map[string]struct{}{}}
	tt := render(t, l.view, 1000, 500)

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
