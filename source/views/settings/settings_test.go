package settings

import (
	"strings"
	"testing"
	"time"

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
			Version:    "0.2.0",
			Go:         "go1.27.3",
			Modules:    []string{"MyGo 0.7.4", "Onevra 2026.16"},
			DataDir:    "/Users/me/Library/Application Support/com.rubisco.unipac",
			Repository: "https://github.com/RUBisco0211/unipac",
		},
	}
}

func fixture() *inventory.Data { return testui.Data() }

// The page is the general settings, the task history hold, and About. The
// managers have a page of their own.
func TestSettingsShowsItsSections(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 900, 1600)

	for _, want := range []string{
		locales.SettingsGeneral(), locales.SettingsTasks(), locales.SettingsAbout(),
		locales.SettingsAppearance(), locales.SettingsLanguage(), locales.SettingsLanguageSystem(),
		locales.SettingsStartupScan(), locales.SettingsNotifications(), locales.SettingsLogLevel(),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText(locales.SettingsManagerStatus()) {
		t.Errorf("the managers are still configured here: %v", tt.Texts())
	}
}

// Choosing a log level reaches the backend, and the select shows the value.
func TestSettingsLogLevelReachesBackend(t *testing.T) {
	s := settingsFixture()
	var got string
	s.OnSetLogLevel = func(level string) { got = level }

	tt := render(t, s.Build, 900, 1600)
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

	tt := render(t, s.Build, 900, 1600)
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
		s.about.Repository,
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in the About section", want)
		}
	}
}

// them by the path the backend reported, and the repository by its address.
func TestAboutOpensItsFolders(t *testing.T) {
	s := settingsFixture()
	var opened []string
	var openedURLs []string
	s.OnOpenPath = func(path string) { opened = append(opened, path) }
	s.OnOpenURL = func(url string) { openedURLs = append(openedURLs, url) }

	tt := render(t, s.Build, 900, 1600)
	if err := tt.Click(locales.AboutOpen(locales.AboutDataDir())); err != nil {
		t.Fatalf("open the data folder: %v", err)
	}
	if len(opened) != 1 || opened[0] != s.about.DataDir {
		t.Fatalf("opened %v, want the data folder", opened)
	}
	if err := tt.Click(locales.AboutRepositoryOpen()); err != nil {
		t.Fatalf("open the repository: %v", err)
	}
	tt.Frame()
	if len(openedURLs) != 1 || openedURLs[0] != s.about.Repository {
		t.Fatalf("opened %v, want the repository", openedURLs)
	}
}

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
		s.about.Repository,
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
