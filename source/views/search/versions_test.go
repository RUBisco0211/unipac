package search

import (
	"errors"
	"strconv"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/egoist/mygo/ui"
)

// versionedFixture is the shared inventory with NPM advertising the version
// lookup, which is the one manager that does.
func versionedFixture() *inventory.Data {
	d := fixture()
	d.Managers[1].Capabilities.Install = true
	d.Managers[1].Capabilities.ListVersions = true // NPM
	return d
}

// asked is what a page under test asked the shell to do.
type asked struct {
	// lookedUp is the package of each version lookup, and its sequence number.
	lookedUp []string
	seqs     []int
	// installed is the name of each plain install.
	installed []string
	// chosen is the version of each installed-from-the-picker action, keyed by
	// package name; "" is an install of the newest.
	chosen map[string]string
}

// versionedPage is a search page showing one npm result, with its lookups
// answered by the test instead of the shell.
func versionedPage(t *testing.T) (*View, *asked) {
	t.Helper()
	rec := &asked{chosen: map[string]string{}}
	s := &View{Data: versionedFixture()}
	s.OnInstall = func(pkg manager.Package, _ flight.Origin) {
		rec.installed = append(rec.installed, pkg.Name)
	}
	s.OnInstallVersion = func(pkg manager.Package, version string, _ flight.Origin) {
		rec.chosen[pkg.Name] = version
	}
	s.OnVersions = func(pkg manager.Package, seq int) {
		rec.lookedUp = append(rec.lookedUp, pkg.Name)
		rec.seqs = append(rec.seqs, seq)
	}
	s.BeginQuery()
	s.Apply(1, "vite", manager.SearchResult{Packages: []manager.Package{
		{Name: "vite", Manager: "npm", Version: "5.0.0"},
	}}, nil)
	return s, rec
}

// vite is the package the page found.
func vite() manager.Package { return manager.Package{Name: "vite", Manager: "npm"} }

// openPicker opens the version dialog from the row's install button.
func openPicker(t *testing.T, s *View, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.ActionsInstall()); err != nil {
		t.Fatalf("click install: %v", err)
	}
	tt.Frame()
}

// A manager that lists versions installs nothing until the user chose one: the
// install button opens the picker and asks for the versions.
func TestInstallOpensTheVersionPicker(t *testing.T) {
	s, rec := versionedPage(t)
	tt := render(t, s.Build, 900, 500)

	openPicker(t, s, tt)

	if !s.picker.open {
		t.Fatal("the picker did not open")
	}
	if !tt.HasText(locales.VersionsTitle()) || !tt.HasText(locales.VersionsFor("vite", "NPM")) {
		t.Fatalf("the dialog does not say what it is for: %v", tt.Texts())
	}
	if len(rec.lookedUp) != 1 || rec.lookedUp[0] != "vite" {
		t.Fatalf("lookups = %v, want one for vite", rec.lookedUp)
	}
	if len(rec.installed) != 0 || len(rec.chosen) != 0 {
		t.Fatalf("an install ran before a version was chosen: %+v", rec)
	}
	if !tt.HasText(locales.VersionsLoading()) {
		t.Errorf("the dialog does not say it is loading: %v", tt.Texts())
	}

	// The answer arrives off the UI thread and shows as the list.
	s.SetVersions(vite(), rec.seqs[0], []string{"1.0.0", "1.1.0", "1.2.0"}, nil)
	tt.Frame()
	for _, version := range []string{"1.0.0", "1.1.0", "1.2.0"} {
		if !tt.HasText(version) {
			t.Errorf("version %q is missing: %v", version, tt.Texts())
		}
	}
	if tt.HasText(locales.VersionsLoading()) {
		t.Errorf("the dialog still claims to be loading: %v", tt.Texts())
	}
	// The newest is the choice the dialog opens on, so confirming takes one
	// click and finding an older version is the exception.
	if s.picker.chosen != "1.2.0" {
		t.Errorf("chosen = %q, want the newest version", s.picker.chosen)
	}
}

// A manager without the capability keeps the plain install button it had: there
// is nothing to choose, and the reason is the manager.
func TestInstallSkipsThePickerWithoutTheCapability(t *testing.T) {
	d := fixture() // neither manager lists versions
	d.Managers[0].Capabilities.Install = true
	s := &View{Data: d}
	plain := 0
	s.OnInstall = func(manager.Package, flight.Origin) { plain++ }
	s.OnInstallVersion = func(manager.Package, string, flight.Origin) {
		t.Error("the picker ran for a manager without the capability")
	}
	s.OnVersions = func(manager.Package, int) { t.Error("a version lookup ran for a manager without the capability") }
	s.BeginQuery()
	s.Apply(1, "ripgrep", manager.SearchResult{Packages: []manager.Package{
		{Name: "ripgrep", Manager: "brew", Version: "14.1.1"},
	}}, nil)

	tt := render(t, s.Build, 900, 500)
	if err := tt.Click(locales.ActionsInstall()); err != nil {
		t.Fatalf("click install: %v", err)
	}
	tt.Frame()
	if plain != 1 {
		t.Fatalf("the plain install ran %d times, want one", plain)
	}
	if s.picker.open || tt.HasText(locales.VersionsTitle()) {
		t.Error("the picker opened for a manager that cannot list versions")
	}
}

// Choosing a version and confirming it installs that version, and nothing else.
func TestChoosingAVersionInstallsIt(t *testing.T) {
	s, rec := versionedPage(t)
	tt := render(t, s.Build, 900, 500)
	openPicker(t, s, tt)
	s.SetVersions(vite(), rec.seqs[0], []string{"1.0.0", "1.1.0"}, nil)
	tt.Frame()

	if _, ok := tt.Find(locales.VersionsConfirm()); !ok {
		t.Fatalf("the confirm is missing: %v", tt.Texts())
	}
	if err := tt.Click("1.1.0"); err != nil {
		t.Fatalf("choose a version: %v", err)
	}
	tt.Frame()
	if s.picker.chosen != "1.1.0" {
		t.Fatalf("chosen = %q, want 1.1.0", s.picker.chosen)
	}
	if err := tt.Click(locales.VersionsConfirm()); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	tt.Frame()

	if got := rec.chosen["vite"]; got != "1.1.0" {
		t.Fatalf("installed version = %q, want 1.1.0", got)
	}
	if len(rec.installed) != 0 {
		t.Errorf("the plain install also ran: %v", rec.installed)
	}
	if s.picker.open || tt.HasText(locales.VersionsTitle()) {
		t.Error("the dialog stayed open after confirming")
	}
}

// The list is newest first, whatever order the answer came in.
func TestVersionListIsNewestFirst(t *testing.T) {
	s, rec := versionedPage(t)
	tt := render(t, s.Build, 900, 500)
	openPicker(t, s, tt)
	// npm answers oldest first.
	s.SetVersions(vite(), rec.seqs[0], []string{"1.0.0", "1.1.0", "1.2.0"}, nil)
	tt.Frame()

	previous := float32(-1)
	for _, version := range []string{"1.2.0", "1.1.0", "1.0.0"} {
		rect, ok := tt.Find(version)
		if !ok {
			t.Fatalf("%q is missing: %v", version, tt.Texts())
		}
		if rect.Y < previous {
			t.Fatalf("%q sits above the version newer than it", version)
		}
		previous = rect.Y
	}
}

// Typing in the dialog filters what is shown, without asking the backend again:
// the whole answer is already in hand.
func TestVersionSearchFiltersWithoutFetchingAgain(t *testing.T) {
	s, rec := versionedPage(t)
	tt := render(t, s.Build, 900, 500)
	openPicker(t, s, tt)
	s.SetVersions(vite(), rec.seqs[0], []string{"1.0.0", "1.1.0", "1.2.0"}, nil)
	tt.Frame()

	tt.Type("1.2")
	tt.Frame()
	if !tt.HasText("1.2.0") {
		t.Fatalf("the matching version is missing: %v", tt.Texts())
	}
	for _, hidden := range []string{"1.0.0", "1.1.0"} {
		if tt.HasText(hidden) {
			t.Errorf("%q survived the filter: %v", hidden, tt.Texts())
		}
	}
	if len(rec.lookedUp) != 1 {
		t.Errorf("filtering fetched again: %v", rec.lookedUp)
	}

	// A query that matches nothing says so instead of showing an empty list.
	tt.Type("zzz")
	tt.Frame()
	if !tt.HasText(locales.VersionsEmpty()) {
		t.Errorf("no-match state missing: %v", tt.Texts())
	}
}

// A long answer is capped at what a picker can show, and says it was cut. The
// query is matched against the whole answer, so an old version is reachable.
func TestVersionListIsCappedAndSaysSo(t *testing.T) {
	s, rec := versionedPage(t)
	versions := make([]string, 0, maxVersions+20)
	for i := range maxVersions + 20 {
		versions = append(versions, "1.0."+strconv.Itoa(i)) // oldest first
	}
	tt := render(t, s.Build, 900, 500)
	openPicker(t, s, tt)
	s.SetVersions(vite(), rec.seqs[0], versions, nil)
	tt.Frame()

	shown, cut := s.shownVersions()
	if len(shown) != maxVersions || !cut {
		t.Fatalf("shown %d versions, cut %v, want %d and cut", len(shown), cut, maxVersions)
	}
	if shown[0] != "1.0."+strconv.Itoa(maxVersions+19) {
		t.Errorf("the newest version is %q, not at the top", shown[0])
	}
	if !tt.HasText(locales.VersionsTruncated(strconv.Itoa(maxVersions))) {
		t.Errorf("the cut is not mentioned: %v", tt.Texts())
	}

	// An old version the cap hid is still reachable by typing its number.
	s.picker.query = "1.0.0"
	shown, cut = s.shownVersions()
	if cut || len(shown) == 0 || shown[len(shown)-1] != "1.0.0" {
		t.Fatalf("a query did not reach the whole answer: %v", shown)
	}
}

// An answer to a lookup that has been superseded is dropped, so a slow failure
// cannot replace the newer answer.
func TestStaleVersionAnswerIsDropped(t *testing.T) {
	s := &View{Data: versionedFixture()}
	first, fetch := s.BeginVersions(vite())
	if !fetch {
		t.Fatal("the first lookup asked for nothing")
	}
	// It fails, so opening the picker again retries.
	s.SetVersions(vite(), first, nil, errors.New("npm failed"))
	second, fetch := s.BeginVersions(vite())
	if !fetch || second == first {
		t.Fatalf("the retry did not register a new lookup: %d", second)
	}

	s.SetVersions(vite(), first, []string{"1.0.0"}, nil)
	if state := s.versions[inventory.Key(vite())]; len(state.versions) != 0 {
		t.Fatalf("the superseded answer was installed: %v", state.versions)
	}
	if state := s.versions[inventory.Key(vite())]; !state.loading {
		t.Error("the retry stopped loading when an older answer arrived")
	}

	s.SetVersions(vite(), second, []string{"2.0.0"}, nil)
	if state := s.versions[inventory.Key(vite())]; state.loading || len(state.versions) != 1 || state.versions[0] != "2.0.0" {
		t.Fatalf("the current answer was not installed: %+v", state)
	}
}

// A list already in hand is not fetched again: a second open shows it at once,
// and only a failure is retried.
func TestFetchedVersionsAreReused(t *testing.T) {
	s := &View{Data: versionedFixture()}
	seq, _ := s.BeginVersions(vite())
	s.SetVersions(vite(), seq, []string{"1.0.0"}, nil)

	if _, fetch := s.BeginVersions(vite()); fetch {
		t.Error("a list in hand was fetched again")
	}
}

// A failed lookup says so and leaves the package installable: the dialog offers
// the newest version, which is what the row's button installed before.
func TestFailedLookupStillInstallsTheNewest(t *testing.T) {
	s, rec := versionedPage(t)
	tt := render(t, s.Build, 900, 500)
	openPicker(t, s, tt)
	s.SetVersions(vite(), rec.seqs[0], nil, errors.New("npm view failed"))
	tt.Frame()

	if !tt.HasText(locales.VersionsFailed()) {
		t.Fatalf("the failure is not shown: %v", tt.Texts())
	}
	if tt.HasText(locales.VersionsConfirm()) {
		t.Errorf("a choice is confirmed with nothing to choose: %v", tt.Texts())
	}
	if err := tt.Click(locales.VersionsInstallLatest()); err != nil {
		t.Fatalf("install latest: %v", err)
	}
	tt.Frame()
	if version, ok := rec.chosen["vite"]; !ok || version != "" {
		t.Fatalf("installed %q, want the newest version", version)
	}
}

// Opening the picker on another row starts with nothing typed and nothing
// chosen: the last row's choice is not this row's.
func TestPickerStartsCleanForAnotherRow(t *testing.T) {
	s := &View{Data: versionedFixture()}
	s.OnVersions = func(manager.Package, int) {}
	s.OnInstallVersion = func(manager.Package, string, flight.Origin) {}

	s.openVersions(vite())
	s.picker.query, s.picker.chosen = "9", "9.1.0"

	other := manager.Package{Name: "eslint", Manager: "npm"}
	s.openVersions(other)
	if s.picker.query != "" || s.picker.chosen != "" {
		t.Fatalf("the previous row's state survived: query %q, chosen %q", s.picker.query, s.picker.chosen)
	}
}

// newestFirst is the picker's order, so it is the one the tests pin down.
func TestNewestFirst(t *testing.T) {
	cases := []struct {
		name   string
		got    []string
		wanted []string
	}{
		{
			name:   "numeric parts compare as numbers",
			got:    []string{"1.9.0", "1.10.0", "1.2.0"},
			wanted: []string{"1.10.0", "1.9.0", "1.2.0"},
		},
		{
			name:   "a release is above its prereleases",
			got:    []string{"1.0.0-beta.2", "1.0.0", "1.0.0-beta.10", "1.0.0-alpha"},
			wanted: []string{"1.0.0", "1.0.0-beta.10", "1.0.0-beta.2", "1.0.0-alpha"},
		},
		{
			name:   "a longer version is above its prefix",
			got:    []string{"1.2", "1.2.1", "2"},
			wanted: []string{"2", "1.2.1", "1.2"},
		},
		{
			name: "a part built from words sorts below numbers",
			// 1.0.0a's last part is a word, so it sits under 1.0.0's 0, and
			// under the release that carries the same numbers.
			got:    []string{"1.0.0", "1.0.0-rc.1", "1.0.0.1", "1.0.0a"},
			wanted: []string{"1.0.0.1", "1.0.0", "1.0.0-rc.1", "1.0.0a"},
		},
		{
			name:   "build metadata carries no order",
			got:    []string{"1.0.0+build.2", "1.0.0+build.10"},
			wanted: []string{"1.0.0+build.2", "1.0.0+build.10"},
		},
	}
	for _, tc := range cases {
		got := newestFirst(tc.got)
		for i := range tc.wanted {
			if got[i] != tc.wanted[i] {
				t.Errorf("%s: newestFirst(%v) = %v, want %v", tc.name, tc.got, got, tc.wanted)
				break
			}
		}
	}
}

// A dialog that arrives with a list has the newest version chosen, because
// installing the latest is what the row's install button did before the picker
// existed; the user confirms once or chooses another.
func TestNewestVersionIsChosenByDefault(t *testing.T) {
	s, rec := versionedPage(t)
	tt := render(t, s.Build, 900, 500)
	openPicker(t, s, tt)
	if s.picker.chosen != "" {
		t.Fatalf("a version was chosen before the lookup answered: %q", s.picker.chosen)
	}
	s.SetVersions(vite(), rec.seqs[0], []string{"1.1.0", "1.0.0", "2.0.0"}, nil)
	tt.Frame()

	if s.picker.chosen != "2.0.0" {
		t.Fatalf("chosen = %q, want the newest 2.0.0", s.picker.chosen)
	}
	if err := tt.Click(locales.VersionsConfirm()); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	tt.Frame()
	if got := rec.chosen["vite"]; got != "2.0.0" {
		t.Fatalf("installed version = %q, want the newest", got)
	}
}

// A choice already made is not replaced when the list is looked at again.
func TestChosenVersionSurvivesAReopen(t *testing.T) {
	s, rec := versionedPage(t)
	tt := render(t, s.Build, 900, 500)
	openPicker(t, s, tt)
	s.SetVersions(vite(), rec.seqs[0], []string{"1.0.0", "2.0.0"}, nil)
	tt.Frame()

	if err := tt.Click("1.0.0"); err != nil {
		t.Fatalf("choose a version: %v", err)
	}
	tt.Frame()
	// Closing and opening the same row keeps the choice the user made.
	s.picker.open = false
	tt.Frame()
	s.openVersions(vite())
	tt.Frame()
	if s.picker.chosen != "1.0.0" {
		t.Fatalf("chosen = %q, want the version the user picked", s.picker.chosen)
	}
}
