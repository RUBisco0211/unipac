package settings

import (
	"errors"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
)

// The managers section has one category per manager the registry reported, and
// the first of them is not showing when the page opens: the app's own settings
// come first.
func TestManagersComeFromTheRegistry(t *testing.T) {
	tt := render(t, settingsFixture().Build, 1200, 900)

	for _, want := range []string{"Homebrew", "NPM", "Cargo"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	// Manager IDs belong to the backend, not the page.
	for _, text := range tt.Texts() {
		if text == "brew" || text == "cargo" {
			t.Errorf("the page exposes a manager ID: %v", tt.Texts())
		}
	}
	if tt.HasText(locales.SettingsManagerStatus()) {
		t.Error("a manager's page is showing before one was chosen")
	}
}

// A manager's page has two areas: what can be changed about it, then what the
// page reports about it, each under its own heading. The editable one comes
// first, because it is what a user opened the page to do.
func TestManagerPageListsItsTwoAreas(t *testing.T) {
	tt := render(t, settingsFixture().Build, 1200, 900)
	choose(t, tt, "Homebrew")

	if !tt.HasText(locales.SettingsEnableManager("Homebrew")) {
		t.Fatalf("the chosen manager's page is not showing: %v", tt.Texts())
	}
	if !tt.HasText(locales.SettingsManagersConfigure()) || !tt.HasText(locales.SettingsManagersInfo()) {
		t.Fatalf("the page does not name its two areas: %v", tt.Texts())
	}
	ordered := make([]ui.Rect, 0, 8)
	for _, label := range []string{
		locales.SettingsManagersConfigure(),
		locales.SettingsManagerEnabled(),
		locales.SettingsManagerExecutable(),
		locales.SettingsManagersInfo(),
		locales.SettingsManagerStatus(),
		locales.SettingsManagerVersion(),
		locales.SettingsManagerPackages(),
		locales.SettingsManagerWebsite(),
		locales.SettingsCapabilities(),
	} {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the %q row is missing: %v", label, tt.Texts())
		}
		ordered = append(ordered, rect)
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i].Y <= ordered[i-1].Y {
			t.Fatalf("row %d at y=%.0f is not below row %d at y=%.0f", i, ordered[i].Y, i-1, ordered[i-1].Y)
		}
	}

	// The path field and its apply button live in the executable row, inside the
	// editable area.
	apply, ok := tt.Find(locales.SettingsApply())
	if !ok {
		t.Fatal("the path field's apply button is missing")
	}
	executable, _ := tt.Find(locales.SettingsManagerExecutable())
	details, _ := tt.Find(locales.SettingsManagersInfo())
	if apply.Y < executable.Y-20 || apply.Y > details.Y {
		t.Errorf("the apply button at y=%.0f is not in the executable row, between %.0f and %.0f",
			apply.Y, executable.Y, details.Y)
	}
}

// Each manager says where its own documentation is, and the row opens it in the
// browser: the address is the adapter's, not the page's.
func TestManagerPageShowsItsWebsite(t *testing.T) {
	s := settingsFixture()
	var opened []string
	s.OnOpenURL = func(url string) { opened = append(opened, url) }

	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, "Homebrew")
	if !tt.HasText("https://brew.sh") {
		t.Fatalf("the website is missing: %v", tt.Texts())
	}
	if err := tt.Click(locales.SettingsManagerWebsiteOpen()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if len(opened) != 1 || opened[0] != "https://brew.sh" {
		t.Fatalf("opened %v, want the manager's website", opened)
	}

	// Another manager's page carries its own address.
	choose(t, tt, "NPM")
	if !tt.HasText("https://www.npmjs.com") {
		t.Errorf("NPM's website is missing: %v", tt.Texts())
	}
}

// The page names the version in use, because "this manager is too old for that"
// is a question it should answer before the user asks it.
func TestManagerPageShowsTheVersion(t *testing.T) {
	tt := render(t, settingsFixture().Build, 1200, 900)
	choose(t, tt, "Homebrew")
	if !tt.HasText("4.6.1") {
		t.Errorf("Homebrew's version is missing: %v", tt.Texts())
	}

	choose(t, tt, "NPM")
	if !tt.HasText("11.17.0") {
		t.Errorf("NPM's version is missing: %v", tt.Texts())
	}

	// A manager that could not be probed shows a dash rather than a zero.
	d := fixture()
	d.Managers[0].Version = ""
	tt = render(t, (&View{Data: d}).Build, 1200, 900)
	choose(t, tt, "Homebrew")
	if tt.HasText("4.6.1") {
		t.Errorf("a version survived a rebuild without one: %v", tt.Texts())
	}
	if !tt.HasText("-") {
		t.Errorf("a missing version shows nothing at all: %v", tt.Texts())
	}
}

// A manager that could not preflight says so on its own page, and is not called
// available.
func TestManagerPageSaysWhenTheExecutableIsMissing(t *testing.T) {
	tt := render(t, settingsFixture().Build, 1200, 900)
	choose(t, tt, "Homebrew")
	if tt.HasText(locales.SettingsNotFound()) {
		t.Fatalf("an available manager claims its executable is missing: %v", tt.Texts())
	}

	choose(t, tt, "Cargo")
	if !tt.HasText(locales.SettingsNotFound()) {
		t.Errorf("missing the not-found state on Cargo's page: %v", tt.Texts())
	}
	if tt.HasText(locales.SettingsManagerAvailable()) {
		t.Errorf("a manager that cannot run is called available: %v", tt.Texts())
	}
}

// A manager says how many installed packages it contributes, and the number
// follows the inventory rather than being computed once.
func TestManagerPageShowsItsPackageCount(t *testing.T) {
	d := fixture()
	tt := render(t, (&View{Data: d}).Build, 1200, 900)
	choose(t, tt, "Homebrew")
	if !tt.HasText(locales.SettingsPackageCount(1)) {
		t.Errorf("the count is missing: %v", tt.Texts())
	}

	// A manager with nothing to its name says so, on its own page.
	choose(t, tt, "Cargo")
	if !tt.HasText(locales.SettingsPackageCount(0)) {
		t.Errorf("a manager with no packages does not say so: %v", tt.Texts())
	}

	// A refresh moves the number with the inventory.
	choose(t, tt, "Homebrew")
	d.Rows = []manager.Package{{Name: "ripgrep", Manager: "brew"}, {Name: "fd", Manager: "brew"}}
	d.Version++
	tt.Frame()
	if !tt.HasText(locales.SettingsPackageCount(2)) {
		t.Errorf("the count did not follow the inventory: %v", tt.Texts())
	}
}

// Every capability is named with a badge, so a gap is visible: the badge's tone
// says which is which, and the list is the adapter guide's order.
func TestManagerPageBadgesEveryCapability(t *testing.T) {
	tt := render(t, settingsFixture().Build, 1200, 900)
	choose(t, tt, "Homebrew")

	if !tt.HasText(locales.SettingsCapabilities()) {
		t.Errorf("the capabilities row is missing: %v", tt.Texts())
	}
	for _, name := range []string{
		locales.CapabilityInstalled(), locales.CapabilityOutdated(), locales.CapabilityPackageInfo(),
		locales.CapabilitySearch(), locales.CapabilityInstall(), locales.CapabilityUninstall(),
		locales.CapabilityUpdate(), locales.CapabilityVersions(),
	} {
		if !tt.HasText(name) {
			t.Errorf("capability %q is missing: %v", name, tt.Texts())
		}
	}
}

// The switch on a manager's page carries that manager's ID to the backend.
func TestManagerSwitchReachesBackend(t *testing.T) {
	s := settingsFixture()
	var gotID string
	var gotOn bool
	s.OnSetManagerEnabled = func(id string, on bool) { gotID, gotOn = id, on }

	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, "Homebrew")
	if err := tt.Click(locales.SettingsEnableManager("Homebrew")); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if gotID != "brew" || gotOn {
		t.Fatalf("switch sent %q %v, want brew false", gotID, gotOn)
	}
}

// The chosen category follows the manager rather than its position, so a rescan
// that adds one keeps the page the user was on.
func TestChosenManagerSurvivesARescan(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, "NPM")

	s.Data.Managers = append(s.Data.Managers, manager.Info{ID: "uv", Name: "UV Tool", Enabled: true})
	s.Data.ManagersVersion++
	tt.Frame()
	if !tt.HasText(locales.SettingsEnableManager("NPM")) {
		t.Fatalf("a rescan moved the page away from NPM: %v", tt.Texts())
	}
	if !tt.HasText("UV Tool") {
		t.Errorf("the registry's new manager has no category: %v", tt.Texts())
	}
}

// A path typed into a manager's field is sent when its Apply is pressed, and a
// frame in between must not rebuild the row and lose the draft.
func TestApplySendsPath(t *testing.T) {
	s := settingsFixture()
	var gotID, gotPath string
	s.OnSetManagerPath = func(id, path string) { gotID, gotPath = id, path }

	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, "Homebrew")
	s.managers.rows[0].path = "/usr/local/bin/brew"
	tt.Frame()
	if s.managers.rows[0].path != "/usr/local/bin/brew" {
		t.Fatalf("a rebuild discarded the draft: %q", s.managers.rows[0].path)
	}

	if err := tt.Click(locales.SettingsApply()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if gotID != "brew" || gotPath != "/usr/local/bin/brew" {
		t.Fatalf("apply sent %q %q, want brew /usr/local/bin/brew", gotID, gotPath)
	}
}

// The saved settings are what the page shows, and a rebuild puts them back:
// nothing the backend did not report stays on screen.
func TestManagerPageShowsTheSavedState(t *testing.T) {
	s := settingsFixture()
	s.Data.Overrides = map[string]manager.Settings{
		"brew":  {Disabled: true},
		"npm":   {},
		"cargo": {Disabled: true},
	}
	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, "Homebrew")
	if s.managers.rows[0].enabled {
		t.Fatal("Homebrew's switch ignored the saved state")
	}

	s.managers.invalidate()
	tt.Frame()
	if s.managers.rows[0].enabled {
		t.Fatal("the rebuild did not restore the saved state")
	}
	if s.managers.rows[0].path != "" {
		t.Fatalf("an unsaved path was invented: %q", s.managers.rows[0].path)
	}
}

// Detect again sits beside the managers section's heading: it is about the
// managers as a whole, not about the app's own settings.
func TestDetectAgainReachesBackend(t *testing.T) {
	s := settingsFixture()
	reloaded := 0
	s.OnReloadManagers = func() { reloaded++ }

	tt := render(t, s.Build, 1200, 900)
	if err := tt.Click(locales.SettingsRedetect()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if reloaded != 1 {
		t.Fatalf("reloaded %d times, want 1", reloaded)
	}
	if !tt.HasText(locales.SettingsDetecting()) {
		t.Errorf("the action does not say it is working: %v", tt.Texts())
	}

	// The save's outcome ends it, so the action reads as idle again.
	s.EndSave(nil, Snapshot{})
	tt.Frame()
	if !tt.HasText(locales.SettingsRedetect()) {
		t.Errorf("the action did not go back to idle: %v", tt.Texts())
	}
}

// A failed save leaves the draft where it was and says so: the typing is the
// user's, and the error line is what says it was not saved.
func TestFailedManagerSaveSaysSo(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, "Homebrew")

	s.BeginSave()
	s.managers.rows[0].path = "/tmp/draft"
	tt.Frame()
	s.EndSave(errors.New("permission denied"), Snapshot{})
	tt.Frame()
	if s.busy {
		t.Error("the page is still busy after the save failed")
	}
	if !tt.HasText(locales.SettingsSaveError("permission denied")) {
		t.Errorf("the failure is not shown: %v", tt.Texts())
	}
	if s.managers.rows[0].path != "/tmp/draft" {
		t.Errorf("the draft was thrown away: %q", s.managers.rows[0].path)
	}
}

// A dropped change keeps the page busy and the draft on screen: nothing failed,
// the write that beat it is still running, and a rebuild never eats typing.
func TestDroppedManagerChangeKeepsTheDraft(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, "Homebrew")

	s.BeginSave()
	s.managers.rows[0].path = "/tmp/draft"
	s.Revert(Snapshot{})
	tt.Frame()
	if !s.busy {
		t.Fatal("the revert ended the write that is still running")
	}
	if s.managers.rows[0].path != "/tmp/draft" {
		t.Fatalf("draft = %q, want the rebuild to have carried it over", s.managers.rows[0].path)
	}
	if s.err != "" {
		t.Errorf("a dropped change reported an error: %q", s.err)
	}
}
