package managers

import (
	"errors"
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

func managersPage() *View { return &View{Data: fixture()} }

// The page has one tab per manager, and the first one is showing.
func TestManagersPageHasATabPerManager(t *testing.T) {
	tt := render(t, managersPage().Build, 900, 700)

	for _, want := range []string{"Homebrew", "NPM", "Cargo", locales.SettingsManagerStatus()} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	if !tt.HasText(locales.SettingsEnableManager("Homebrew")) {
		t.Fatalf("the first manager's panel is not the default: %v", tt.Texts())
	}
	// Manager IDs belong to the backend, not the page.
	for _, text := range tt.Texts() {
		if text == "brew" || text == "cargo" {
			t.Errorf("the page exposes a manager ID: %v", tt.Texts())
		}
	}
}

// The tabs choose which manager's panel shows, and the choice follows the manager
// rather than the position, so a rescan keeps it.
func TestManagerTabsChooseTheManager(t *testing.T) {
	m := managersPage()
	tt := render(t, m.Build, 900, 700)

	if tt.HasText(locales.SettingsEnableManager("NPM")) {
		t.Fatalf("a second panel is showing: %v", tt.Texts())
	}
	if err := tt.Click("NPM"); err != nil {
		t.Fatalf("open the NPM tab: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.SettingsEnableManager("NPM")) {
		t.Fatalf("the NPM tab did not show its panel: %v", tt.Texts())
	}
	if tt.HasText(locales.SettingsEnableManager("Homebrew")) {
		t.Errorf("the previous panel is still showing: %v", tt.Texts())
	}
	if m.tabID != "npm" {
		t.Errorf("the page kept tab %q, want npm", m.tabID)
	}

	// A rescan that adds a manager keeps the tab on the one being looked at.
	m.Data.Managers = append(m.Data.Managers, manager.Info{ID: "uv", Name: "UV Tool", Enabled: true})
	m.Data.ManagersVersion++
	tt.Frame()
	if m.tabID != "npm" || !tt.HasText(locales.SettingsEnableManager("NPM")) {
		t.Fatalf("a rescan moved the tab to %q", m.tabID)
	}
}

// The panel is a list of answers: a label per fact, and the value across the rest
// of the row, in a fixed order.
func TestManagerPanelListsItsFactsInOrder(t *testing.T) {
	tt := render(t, managersPage().Build, 900, 700)

	labels := []string{
		locales.SettingsManagerStatus(),
		locales.SettingsManagerEnabled(),
		locales.SettingsManagerVersion(),
		locales.SettingsManagerExecutable(),
		locales.SettingsManagerPackages(),
		locales.SettingsCapabilities(),
	}
	previous := float32(-1)
	for _, label := range labels {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the %q row is missing: %v", label, tt.Texts())
		}
		if rect.Y <= previous {
			t.Errorf("the %q row at y=%.0f is not below the one before it at %.0f", label, rect.Y, previous)
		}
		previous = rect.Y
	}

	// The path field and its apply button live in the executable row.
	apply, ok := tt.Find(locales.SettingsApplyPathFor("Homebrew"))
	if !ok {
		t.Fatal("the path field's apply button is missing")
	}
	executable, _ := tt.Find(locales.SettingsManagerExecutable())
	packages, _ := tt.Find(locales.SettingsManagerPackages())
	if apply.Y < executable.Y-20 || apply.Y > packages.Y {
		t.Errorf("the path at y=%.0f is not in the executable row, between %.0f and %.0f",
			apply.Y, executable.Y, packages.Y)
	}
}

// The panel names the version in use, because "this manager is too old for that"
// is a question the page should answer before the user asks it.
func TestManagerPanelShowsTheVersion(t *testing.T) {
	tt := render(t, managersPage().Build, 900, 700)

	if !tt.HasText("4.6.1") {
		t.Errorf("Homebrew's version is missing: %v", tt.Texts())
	}
	if err := tt.Click("NPM"); err != nil {
		t.Fatalf("open the NPM tab: %v", err)
	}
	tt.Frame()
	if !tt.HasText("11.17.0") {
		t.Errorf("NPM's version is missing: %v", tt.Texts())
	}

	// A manager that could not be probed shows a dash rather than a zero.
	d := fixture()
	d.Managers[0].Version = ""
	tt = render(t, (&View{Data: d}).Build, 900, 700)
	if tt.HasText("4.6.1") {
		t.Errorf("a version survived a rebuild without one: %v", tt.Texts())
	}
	if !tt.HasText("-") {
		t.Errorf("a missing version shows nothing at all: %v", tt.Texts())
	}
}

// A manager that could not preflight says so in its own tab, and is not called
// available.
func TestManagerPanelSaysWhenTheExecutableIsMissing(t *testing.T) {
	tt := render(t, managersPage().Build, 900, 700)

	if tt.HasText(locales.SettingsNotFound()) {
		t.Fatalf("the default tab claims a manager is missing: %v", tt.Texts())
	}
	if err := tt.Click("Cargo"); err != nil {
		t.Fatalf("open the Cargo tab: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.SettingsNotFound()) {
		t.Errorf("missing the not-found state in Cargo's tab: %v", tt.Texts())
	}
	if tt.HasText(locales.SettingsManagerAvailable()) {
		t.Errorf("a manager that cannot run is called available: %v", tt.Texts())
	}
}

// A manager says how many installed packages it contributes, and the number
// follows the inventory rather than being computed once.
func TestManagerPanelShowsItsPackageCount(t *testing.T) {
	d := fixture()
	tt := render(t, (&View{Data: d}).Build, 900, 700)

	if !tt.HasText(locales.SettingsManagerPackages()) {
		t.Errorf("the packages row is missing: %v", tt.Texts())
	}
	if !tt.HasText(locales.SettingsPackageCount(1)) {
		t.Errorf("the count is missing: %v", tt.Texts())
	}

	// A manager with nothing to its name says so, in its own tab.
	if err := tt.Click("Cargo"); err != nil {
		t.Fatalf("open the Cargo tab: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.SettingsPackageCount(0)) {
		t.Errorf("a manager with no packages does not say so: %v", tt.Texts())
	}

	// A refresh moves the number with the inventory.
	if err := tt.Click("Homebrew"); err != nil {
		t.Fatalf("open the Homebrew tab: %v", err)
	}
	d.Rows = []manager.Package{{Name: "ripgrep", Manager: "brew"}, {Name: "fd", Manager: "brew"}}
	d.Version++
	tt.Frame()
	if !tt.HasText(locales.SettingsPackageCount(2)) {
		t.Errorf("the count did not follow the inventory: %v", tt.Texts())
	}
}

// Every capability is named with a badge, so a gap is visible: the badge's tone
// says which is which, and the list is the adapter guide's order.
func TestManagerPanelBadgesEveryCapability(t *testing.T) {
	tt := render(t, managersPage().Build, 900, 700)

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

// The switch on a manager's panel carries that manager's ID to the backend.
func TestManagerSwitchReachesBackend(t *testing.T) {
	m := managersPage()
	var gotID string
	var gotOn bool
	m.OnSetEnabled = func(id string, on bool) { gotID, gotOn = id, on }

	tt := render(t, m.Build, 900, 700)
	if err := tt.Click(locales.SettingsEnableManager("Homebrew")); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if gotID != "brew" || gotOn {
		t.Fatalf("switch sent %q %v, want brew false", gotID, gotOn)
	}
}

// A path typed into a manager's field is sent when its Apply is pressed, and a
// frame in between must not rebuild the row and lose the draft.
func TestApplySendsPath(t *testing.T) {
	m := managersPage()
	var gotID, gotPath string
	m.OnSetPath = func(id, path string) { gotID, gotPath = id, path }

	tt := render(t, m.Build, 900, 700)
	m.rows[0].path = "/usr/local/bin/brew"
	tt.Frame()
	if m.rows[0].path != "/usr/local/bin/brew" {
		t.Fatalf("a rebuild discarded the draft: %q", m.rows[0].path)
	}

	if err := tt.Click(locales.SettingsApply()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if gotID != "brew" || gotPath != "/usr/local/bin/brew" {
		t.Fatalf("apply sent %q %q, want brew /usr/local/bin/brew", gotID, gotPath)
	}
}

// The saved settings are what the panel shows, and a rebuild puts them back:
// nothing the backend did not report stays on screen.
func TestPanelShowsTheSavedState(t *testing.T) {
	m := managersPage()
	m.Data.Overrides = map[string]manager.Settings{
		"brew":  {Disabled: true},
		"npm":   {},
		"cargo": {Disabled: true},
	}
	tt := render(t, m.Build, 900, 700)
	if m.rows[0].enabled {
		t.Fatal("Homebrew's switch ignored the saved state")
	}

	m.Invalidate()
	tt.Frame()
	if m.rows[0].enabled {
		t.Fatal("the rebuild did not restore the saved state")
	}
	if m.rows[0].path != "" {
		t.Fatalf("an unsaved path was invented: %q", m.rows[0].path)
	}
}

// A failed save says so on the page, and puts the controls back.
func TestFailedSaveSaysSo(t *testing.T) {
	m := managersPage()
	tt := render(t, m.Build, 900, 700)

	m.BeginSave()
	tt.Frame()
	m.EndSave(errors.New("permission denied"))
	tt.Frame()
	if !tt.HasText(locales.SettingsSaveError("permission denied")) {
		t.Errorf("the failure is not shown: %v", tt.Texts())
	}
	if m.busy {
		t.Error("the page is still busy after the save failed")
	}
}
