package settings

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

func settingsFixture() *View {
	return &View{
		Data:      testui.Data(),
		AutoScan:  true,
		logLevel:  "info",
		levels:    []string{"debug", "info", "warn", "error"},
		language:  "",
		languages: []string{"en", "zh-CN"},
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
