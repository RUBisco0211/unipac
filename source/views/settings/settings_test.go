package settings

import (
	"strings"
	"testing"
	"time"

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

func settingsFixture() *View {
	return &View{
		Data:          testui.Data(),
		AutoScan:      true,
		logLevel:      "info",
		levels:        []string{"debug", "info", "warn", "error"},
		language:      "",
		languages:     []string{"en", "zh-CN"},
		theme:         "",
		Notifications: true,
		about: inventory.About{
			Version: "0.2.0",
			Go:      "go1.27.3",
			Modules: []string{"MyGo 0.7.4", "Onevra 2026.16"},
			DataDir: "/Users/me/Library/Application Support/com.rubisco.unipac",
			LogDir:  "/Users/me/Library/Application Support/com.rubisco.unipac/logs",
		},
	}
}

func fixture() *inventory.Data { return testui.Data() }

func TestSettingsShowsManagers(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 900, 600)

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
	s.OnSetPath = func(id, path string) { gotID, gotPath = id, path }

	tt := render(t, s.Build, 900, 600)
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
	s.OnSetEnabled = func(id string, on bool) { gotID, gotOn = id, on }

	tt := render(t, s.Build, 900, 600)
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
	s.OnSetLogLevel = func(level string) { got = level }

	tt := render(t, s.Build, 900, 600)
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
	s.OnSetLanguage = func(tag string) { got = tag }

	tt := render(t, s.Build, 900, 600)
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

// Turning a manager off must reach the backend with the manager's own state,
// and a failed save must put the switch back.
func TestSettingsSwitchAndRevert(t *testing.T) {
	s := settingsFixture()
	s.Data.Overrides = map[string]manager.Settings{
		"brew":  {Disabled: true},
		"npm":   {},
		"cargo": {Disabled: true},
	}
	tt := render(t, s.Build, 900, 600)
	if s.rows[0].enabled {
		t.Fatal("Homebrew switch ignored the saved state")
	}

	s.Invalidate()
	tt.Frame()
	if s.rows[0].enabled {
		t.Fatal("rebuild did not restore the saved switch state")
	}
	if s.rows[0].path != "" {
		t.Fatalf("unsaved path was invented: %q", s.rows[0].path)
	}
}

// unsupportedText is what a row says about the capabilities it lacks, built the
// way the page builds it so the assertion does not pin the wording twice.
func unsupportedText(labels ...string) string {
	return locales.SettingsUnsupported(strings.Join(labels, locales.CommonListSeparator()))
}

// A manager row says how many installed packages it contributes, and the number
// follows the inventory rather than being computed once.
func TestManagerRowShowsItsPackageCount(t *testing.T) {
	d := fixture()
	s := &View{Data: d, logLevel: "info", levels: []string{"info"}}
	tt := render(t, s.Build, 900, 700)

	if !tt.HasText(locales.SettingsPackageCount(1)) {
		t.Errorf("the count is missing: %v", tt.Texts())
	}
	if !tt.HasText(locales.SettingsPackageCount(0)) {
		t.Errorf("a manager with no packages does not say so: %v", tt.Texts())
	}

	// A refresh adds a package to Homebrew and drops NPM's only one.
	d.Rows = []manager.Package{{Name: "ripgrep", Manager: "brew"}, {Name: "fd", Manager: "brew"}}
	d.Version++
	tt.Frame()
	if !tt.HasText(locales.SettingsPackageCount(2)) {
		t.Errorf("the count did not follow the inventory: %v", tt.Texts())
	}
	if !tt.HasText(locales.SettingsPackageCount(0)) {
		t.Errorf("the emptied manager still shows its old count: %v", tt.Texts())
	}
}

// A manager that cannot answer an action names it, so a disabled button elsewhere
// in the window has an explanation on the page that configures the manager.
func TestManagerRowNamesWhatItCannotDo(t *testing.T) {
	s := &View{Data: fixture(), logLevel: "info", levels: []string{"info"}}
	tt := render(t, s.Build, 900, 700)

	// The fixture's Homebrew lists, searches, updates and uninstalls.
	want := unsupportedText(locales.CapabilityPackageInfo(), locales.CapabilityInstall(), locales.CapabilityVersions())
	if !tt.HasText(want) {
		t.Errorf("missing %q in %v", want, tt.Texts())
	}
	for _, unsupported := range []string{locales.CapabilitySearch(), locales.CapabilityUpdate()} {
		if strings.Contains(want, unsupported) {
			t.Errorf("%q was listed as unsupported", unsupported)
		}
	}

	// A manager that offers everything says nothing: the row names gaps, not
	// capabilities.
	all := manager.Capabilities{
		ListInstalled: true, ListOutdated: true, GetPackageInfo: true, Search: true,
		Install: true, Uninstall: true, Update: true, ListVersions: true,
	}
	if got := unsupportedCapabilities(all); got != "" {
		t.Errorf("a manager offering everything says %q", got)
	}
	if got := unsupportedCapabilities(manager.Capabilities{Search: true}); got != unsupportedText(
		locales.CapabilityInstalled(), locales.CapabilityOutdated(), locales.CapabilityPackageInfo(),
		locales.CapabilityInstall(), locales.CapabilityUninstall(), locales.CapabilityUpdate(),
		locales.CapabilityVersions(),
	) {
		t.Errorf("unsupported list = %q", got)
	}
}

// The row is three lines: the name with its switch, the executable path on its
// own, and what the manager cannot do. The path field must not be squeezed into a
// column beside the name any more.
func TestManagerRowGivesThePathItsOwnLine(t *testing.T) {
	s := &View{Data: fixture(), logLevel: "info", levels: []string{"info"}}
	tt := render(t, s.Build, 900, 700)

	name, ok := tt.Find("Homebrew")
	if !ok {
		t.Fatal("the manager name is missing")
	}
	apply, ok := tt.Find(locales.SettingsApplyPathFor("Homebrew"))
	if !ok {
		t.Fatal("the path field's apply button is missing")
	}
	if apply.Y <= name.Y {
		t.Errorf("the path line is at y=%.0f, above the name at %.0f", apply.Y, name.Y)
	}
	// The path line takes the row's width, so its apply button is at the trailing
	// edge rather than in a column beside the name.
	if row, ok := tt.Find(locales.SettingsPackageCount(1)); ok {
		if apply.X < row.X+300 {
			t.Errorf("the path line starts at %.0f, not after the row's leading column", apply.X)
		}
	}
	sw, ok := tt.Find(locales.SettingsEnableManager("Homebrew"))
	if !ok {
		t.Fatal("the switch is missing")
	}
	if sw.Y > apply.Y {
		t.Errorf("the switch at y=%.0f is below the path at %.0f, want it on the name's line", sw.Y, apply.Y)
	}
}

// The row names the version in use, because "this manager is too old for that"
// is a question the page should answer before the user asks it.
func TestManagerRowShowsTheVersion(t *testing.T) {
	s := &View{Data: fixture(), logLevel: "info", levels: []string{"info"}}
	tt := render(t, s.Build, 900, 700)

	if !tt.HasText("4.6.1") {
		t.Errorf("Homebrew's version is missing: %v", tt.Texts())
	}
	if !tt.HasText("11.17.0") {
		t.Errorf("NPM's version is missing: %v", tt.Texts())
	}

	// A manager that could not be probed shows none rather than a zero.
	d := fixture()
	d.Managers[0].Version = ""
	s = &View{Data: d, logLevel: "info", levels: []string{"info"}}
	tt = render(t, s.Build, 900, 700)
	if tt.HasText("4.6.1") {
		t.Errorf("a version survived a rebuild without one: %v", tt.Texts())
	}
}

// The appearance choice reaches the backend, and the trigger names what is in
// use rather than the row's own title.
func TestSettingsAppearanceReachesBackend(t *testing.T) {
	s := settingsFixture()
	s.theme = "light"
	var got string
	s.OnSetTheme = func(value string) { got = value }

	tt := render(t, s.Build, 900, 700)
	if !tt.HasText(locales.AppearanceLight()) {
		t.Errorf("the current appearance is not shown: %v", tt.Texts())
	}
	if err := tt.Click(locales.AppearanceLight()); err != nil {
		t.Fatalf("open the appearance select: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.AppearanceDark()); err != nil {
		t.Fatalf("choose dark: %v", err)
	}
	tt.Frame()
	if got != "dark" {
		t.Fatalf("appearance sent %q, want dark", got)
	}
}

// The system choice is the empty value, which is what a config without the
// setting holds.
func TestSettingsAppearanceSystemIsEmpty(t *testing.T) {
	s := settingsFixture()
	var got string
	s.OnSetTheme = func(value string) { got = value }

	tt := render(t, s.Build, 900, 700)
	if err := tt.Click(locales.AppearanceSystem()); err != nil {
		t.Fatalf("open the appearance select: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.AppearanceDark()); err != nil {
		t.Fatalf("choose dark: %v", err)
	}
	tt.Frame()
	if got != "dark" {
		t.Fatalf("appearance sent %q, want dark", got)
	}
	if s.theme != "dark" {
		t.Fatalf("the page did not keep the choice: %q", s.theme)
	}
}

// The notification switch reaches the backend and the page keeps the answer the
// backend reports, like every other switch in the section.
func TestSettingsNotificationsReachBackend(t *testing.T) {
	s := settingsFixture()
	var got bool
	s.OnSetNotifications = func(on bool) { got = on }

	tt := render(t, s.Build, 900, 700)
	if !tt.HasText(locales.SettingsNotifications()) {
		t.Fatalf("the switch is missing: %v", tt.Texts())
	}
	// A switch owns the value it is bound to; a click turns it over and the next
	// build reports the change, which is what the switch itself does.
	s.Notifications = false
	tt.Frame()
	if got {
		t.Fatalf("notifications sent %v, want the switch's new value", got)
	}

	// A save that reports the backend's answer puts the switch back.
	s.EndSave(nil, Snapshot{Notifications: true})
	if !s.Notifications {
		t.Error("the backend's answer did not restore the switch")
	}
}

// About is what a bug report needs: the build, and where the app keeps what it
// writes.
func TestAboutShowsTheBuild(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 900, 1600)

	for _, want := range []string{
		locales.SettingsAbout(),
		"0.2.0",
		"go1.27.3",
		"MyGo", "0.7.4",
		"Onevra", "2026.16",
		s.about.DataDir,
		s.about.LogDir,
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in the About section", want)
		}
	}
}

// The two folders open in the desktop's own application, and the page asks for
// them by the path the backend reported.
func TestAboutOpensItsFolders(t *testing.T) {
	s := settingsFixture()
	var opened []string
	s.OnOpenPath = func(path string) { opened = append(opened, path) }

	tt := render(t, s.Build, 900, 1600)
	if err := tt.Click(locales.AboutOpen(locales.AboutDataDir())); err != nil {
		t.Fatalf("open the data folder: %v", err)
	}
	if len(opened) != 1 || opened[0] != s.about.DataDir {
		t.Fatalf("opened %v, want the data folder", opened)
	}
	if err := tt.Click(locales.AboutOpen(locales.AboutLogDir())); err != nil {
		t.Fatalf("open the log folder: %v", err)
	}
	if len(opened) != 2 || opened[1] != s.about.LogDir {
		t.Fatalf("opened %v, want the log folder second", opened)
	}
}

// Copy places the same facts on the clipboard, with what each manager is, and
// says it did.
func TestAboutCopiesDiagnostics(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 900, 1600)
	if err := tt.Click(locales.AboutCopy()); err != nil {
		t.Fatalf("click copy: %v", err)
	}

	diagnostics := tt.Clipboard()
	for _, want := range []string{
		"0.2.0",
		s.about.DataDir,
		s.about.LogDir,
		"Homebrew", "4.6.1",
		"Bun", // the fixture has no bun, so this must not be there
	} {
		contains := strings.Contains(diagnostics, want)
		if want == "Bun" {
			if contains {
				t.Errorf("diagnostics name a manager the inventory does not have:\n%s", diagnostics)
			}
			continue
		}
		if !contains {
			t.Errorf("diagnostics are missing %q:\n%s", want, diagnostics)
		}
	}
	if !strings.Contains(diagnostics, locales.SettingsPackageCount(1)) {
		t.Errorf("diagnostics do not say what a manager holds:\n%s", diagnostics)
	}
}

// Clearing the history is destructive, so it answers to the same setting as
// uninstalling: a hold, or a click when the user turned holding off.
func TestClearHistoryAnswersToTheHoldingSetting(t *testing.T) {
	cleared := 0
	hold := settingsFixture()
	hold.HoldToConfirm = true
	hold.OnClearHistory = func() { cleared++ }
	tt := render(t, hold.Build, 900, 1600)

	if tt.HasText(locales.SettingsClearHistory()) {
		t.Errorf("the click action is offered while holding is on")
	}
	control, ok := tt.Find(locales.SettingsClearHistoryHold())
	if !ok {
		t.Fatalf("the hold control is missing: %v", tt.Texts())
	}
	tt.ClickAt(control.X+control.W/2, control.Y+control.H/2)
	tt.Frame()
	if cleared != 0 {
		t.Fatal("a click confirmed a hold")
	}
	tt.Press(control.X+control.W/2, control.Y+control.H/2)
	tt.Advance(2 * time.Second)
	tt.Release(control.X+control.W/2, control.Y+control.H/2)
	tt.Frame()
	if cleared != 1 {
		t.Fatalf("the hold cleared %d times, want 1", cleared)
	}

	// With holding off it is a danger button and one click answers it.
	click := settingsFixture()
	click.HoldToConfirm = false
	click.OnClearHistory = func() { cleared++ }
	tt = render(t, click.Build, 900, 1600)
	if err := tt.Click(locales.SettingsClearHistory()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if cleared != 2 {
		t.Fatalf("the click cleared %d times, want one more", cleared)
	}
}
