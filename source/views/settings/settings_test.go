package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/tasks"
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

func settingsFixture() *View {
	return &View{
		Data:      fixture(),
		AutoScan:  true,
		logLevel:  "info",
		levels:    []string{"debug", "info", "warn", "error"},
		language:  "",
		languages: []string{"en", "zh-CN"},
		theme:     "",
		accent:    "",
		accents:   []string{"", "blue", "purple", "pink", "green", "graphite"},
		notify: Notify{
			Enabled:   true,
			Install:   Outcome{Success: true, Failure: true},
			Update:    Outcome{Success: true, Failure: true},
			Uninstall: Outcome{Success: true, Failure: true},
			Updates:   true,
		},
		notifyKinds: []tasks.Kind{tasks.KindInstall, tasks.KindUpdate, tasks.KindUninstall},
		about: inventory.About{
			Version:    "0.2.0",
			Go:         "go1.27.3",
			Modules:    []string{"MyGo 0.7.4", "Onevra 2026.16"},
			DataDir:    "/Users/me/Library/Application Support/com.rubisco.unipac",
			Repository: "https://github.com/RUBisco0211/unipac",
		},
	}
}

// choose opens one category from the sidebar. The surface shows one category at
// a time, so a test about a setting has to get there first.
func choose(t *testing.T, tt *ui.Tester, title string) {
	t.Helper()
	if err := tt.Click(title); err != nil {
		t.Fatalf("open %q: %v", title, err)
	}
	tt.Frame()
}

// The page is one surface: the app's settings under their own section, and a
// category per manager under the next. The general category is the one showing
// when the page opens, because it holds what a user changes most.
func TestSettingsShowsItsCategories(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)

	for _, want := range []string{
		locales.NavSettings(), locales.NavManagers(),
		locales.SettingsGeneral(), locales.SettingsTasks(), locales.SettingsAbout(),
		// The managers are categories here, not a page of their own.
		"Homebrew", "NPM", "Cargo",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	for _, want := range []string{
		locales.SettingsAppearance(), locales.SettingsAccent(), locales.SettingsLanguage(),
		locales.SettingsStartupScan(), locales.SettingsNotifications(), locales.SettingsLogLevel(),
	} {
		if !tt.HasText(want) {
			t.Errorf("the general category is missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText(locales.SettingsManagerStatus()) {
		t.Errorf("a manager's rows are showing instead of the general ones: %v", tt.Texts())
	}
}

// Choosing a category shows that page and no other: the surface builds one
// category at a time, which is what keeps a manager's path field off screen
// while the user is elsewhere.
func TestSettingsCategoryReplacesTheContent(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)

	choose(t, tt, locales.SettingsTasks())
	if !tt.HasText(locales.SettingsHistory()) {
		t.Fatalf("the tasks category did not show: %v", tt.Texts())
	}
	if tt.HasText(locales.SettingsAppearance()) {
		t.Errorf("the general rows stayed: %v", tt.Texts())
	}

	choose(t, tt, locales.SettingsGeneral())
	if !tt.HasText(locales.SettingsAppearance()) {
		t.Errorf("going back to general did not restore its rows: %v", tt.Texts())
	}
}

// The search field reaches every category at once, which is the reason a manager
// fact and an app setting share one surface: "executable" finds the three
// managers' path fields without knowing which page they are on.
func TestSearchReachesTheManagers(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)

	s.search.Query = "executable"
	tt.Frame()
	if !tt.HasText(locales.SettingsManagerExecutable()) {
		t.Fatalf("the search did not reach the manager rows: %v", tt.Texts())
	}
	for _, want := range []string{"Homebrew", "NPM", "Cargo"} {
		if !tt.HasText(want) {
			t.Errorf("the result is missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText(locales.SettingsAppearance()) {
		t.Errorf("a row that does not match is showing: %v", tt.Texts())
	}

	// Choosing a category is how the search is left.
	choose(t, tt, locales.SettingsGeneral())
	if s.search.Query != "" {
		t.Errorf("the query survived a category choice: %q", s.search.Query)
	}
	if !tt.HasText(locales.SettingsAppearance()) {
		t.Errorf("the chosen category is not showing: %v", tt.Texts())
	}
}

// Choosing a log level reaches the backend, and the select shows the value.
func TestSettingsLogLevelReachesBackend(t *testing.T) {
	s := settingsFixture()
	var got string
	s.OnSetLogLevel = func(level string) { got = level }

	tt := render(t, s.Build, 1200, 900)
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

	tt := render(t, s.Build, 1200, 900)
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

// The appearance is the system's, or one the user pinned, and the select carries
// the choice to the backend.
func TestSettingsAppearanceReachesBackend(t *testing.T) {
	s := settingsFixture()
	s.theme = "light"
	var got string
	s.OnSetTheme = func(value string) { got = value }

	tt := render(t, s.Build, 1200, 900)
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

	tt := render(t, s.Build, 1200, 900)
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

// The theme color is a choice from the palette the backend offers, and it
// reaches the backend like every other choice in the category.
func TestSettingsAccentReachesBackend(t *testing.T) {
	s := settingsFixture()
	var got string
	s.OnSetAccent = func(value string) { got = value }

	tt := render(t, s.Build, 1200, 900)
	if !tt.HasText(locales.SettingsAccent()) {
		t.Fatalf("the accent row is missing: %v", tt.Texts())
	}
	// The trigger shows the saved color; clicking it opens the palette.
	if err := tt.Click(locales.AccentDefault()); err != nil {
		t.Fatalf("open the accent choice: %v", err)
	}
	tt.Frame()
	for _, want := range []string{locales.AccentBlue(), locales.AccentPurple(), locales.AccentGraphite()} {
		if !tt.HasText(want) {
			t.Errorf("the palette is missing %q: %v", want, tt.Texts())
		}
	}
	if err := tt.Click(locales.AccentBlue()); err != nil {
		t.Fatalf("choose a color: %v", err)
	}
	tt.Frame()
	if got != "blue" {
		t.Fatalf("accent sent %q, want blue", got)
	}
	// The page keeps the choice, which is what both windows read as they draw.
	if s.Accent() != "blue" {
		t.Errorf("the page did not keep the color: %q", s.Accent())
	}
}

// Notifications is a page of its own: the master switch first, then which
// finished operations are announced by outcome, then the updates switch.
func TestSettingsNotificationsArePerOperation(t *testing.T) {
	s := settingsFixture()
	var gotKind tasks.Kind
	var gotSucceeded, gotOn bool
	s.OnSetNotify = func(kind tasks.Kind, succeeded, on bool) {
		gotKind, gotSucceeded, gotOn = kind, succeeded, on
	}

	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, locales.SettingsNotifications())

	for _, want := range []string{
		locales.SettingsNotifyMaster(),
		locales.SettingsNotifyOperations(),
		locales.TasksKindInstall(), locales.TasksKindUpdate(), locales.TasksKindUninstall(),
		locales.SettingsNotifySuccess(), locales.SettingsNotifyFailure(),
		locales.SettingsNotifyUpdates(), locales.SettingsNotifyUpdatesOn(),
	} {
		if !tt.HasText(want) {
			t.Errorf("the notifications page is missing %q: %v", want, tt.Texts())
		}
	}

	// One operation's failure switch sends that operation and outcome, and no
	// other switch moves with it.
	failureLabel := locales.SettingsNotifyOutcome(locales.TasksKindUpdate(), locales.SettingsNotifyFailure())
	if err := tt.Click(failureLabel); err != nil {
		t.Fatalf("click %q: %v", failureLabel, err)
	}
	tt.Frame()
	if gotKind != tasks.KindUpdate || gotSucceeded || gotOn {
		t.Fatalf("sent %q succeeded=%v on=%v, want update failure off", gotKind, gotSucceeded, gotOn)
	}
	if !s.Notifies(tasks.KindInstall, false) || !s.Notifies(tasks.KindUpdate, true) {
		t.Error("one switch changed another")
	}

	// A save that reports the backend's answer puts the boxes back.
	s.EndSave(nil, Snapshot{Notify: Notify{
		Enabled:   true,
		Install:   Outcome{Success: true, Failure: true},
		Update:    Outcome{Success: true, Failure: true},
		Uninstall: Outcome{Success: true, Failure: false},
		Updates:   true,
	}})
	if !s.Notifies(tasks.KindUpdate, false) {
		t.Error("the backend's answer did not restore the update switch")
	}
	if s.Notifies(tasks.KindUninstall, false) {
		t.Error("the backend's answer did not turn the uninstall switch off")
	}
}

// The master switch is the first row, and off means nothing is announced: the
// other switches keep their answers but are not read, and cannot be changed.
func TestNotificationsMasterSwitch(t *testing.T) {
	s := settingsFixture()
	var got bool
	s.OnSetNotifyEnabled = func(on bool) { got = on }

	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, locales.SettingsNotifications())
	master, ok := tt.Find(locales.SettingsNotifyMaster())
	if !ok {
		t.Fatalf("the master switch is missing: %v", tt.Texts())
	}
	operations, _ := tt.Find(locales.SettingsNotifyOperations())
	if master.Y >= operations.Y {
		t.Errorf("the master switch at y=%.0f is not above the operations at y=%.0f", master.Y, operations.Y)
	}
	if err := tt.Click(locales.SettingsNotifyMasterToggle()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if got {
		t.Fatal("the master switch sent on, want off")
	}

	// Off silences everything the page offers, whatever the other switches say.
	s.notify.Enabled = false
	if s.Notifies(tasks.KindInstall, true) || s.NotifiesUpdates() {
		t.Error("the master switch off still announces")
	}
}

// About is what a bug report needs: the build, and where the app keeps what it
// writes.
func TestAboutShowsTheBuild(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, locales.SettingsAbout())

	for _, want := range []string{
		"0.2.0",
		"go1.27.3",
		"MyGo", "0.7.4",
		"Onevra", "2026.16",
		s.about.DataDir,
		s.about.Repository,
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in the About category: %v", want, tt.Texts())
		}
	}
}

// The two places open in the desktop's own applications, and the page asks for
// them by the path and the address the backend reported.
func TestAboutOpensItsPlaces(t *testing.T) {
	s := settingsFixture()
	var opened []string
	var openedURLs []string
	s.OnOpenPath = func(path string) { opened = append(opened, path) }
	s.OnOpenURL = func(url string) { openedURLs = append(openedURLs, url) }

	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, locales.SettingsAbout())
	if err := tt.Click(locales.AboutOpen(locales.AboutDataDir())); err != nil {
		t.Fatalf("open the data folder: %v", err)
	}
	if len(opened) != 1 || opened[0] != s.about.DataDir {
		t.Fatalf("opened %v, want the data folder", opened)
	}
	tt.Frame()
	if err := tt.Click(locales.AboutRepositoryOpen()); err != nil {
		t.Fatalf("open the repository: %v", err)
	}
	tt.Frame()
	if len(openedURLs) != 1 || openedURLs[0] != s.about.Repository {
		t.Fatalf("opened %v, want the repository", openedURLs)
	}
}

// Copy places the same facts on the clipboard, with what each manager is, and
// says it did.
func TestAboutCopiesDiagnostics(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, locales.SettingsAbout())
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

// Clearing the history asks once in its own dialog, and only the dialog's
// confirm reaches the backend: the button opens the question, nothing else.
func TestClearHistoryAsksInADialog(t *testing.T) {
	cleared := 0
	s := settingsFixture()
	s.OnClearHistory = func() { cleared++ }

	tt := render(t, s.Build, 1200, 900)
	choose(t, tt, locales.SettingsTasks())
	if err := tt.Click(locales.SettingsClearHistory()); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if cleared != 0 {
		t.Fatal("the button cleared without asking")
	}
	if !tt.HasText(locales.SettingsClearHistoryMessage()) {
		t.Fatalf("the question is not shown: %v", tt.Texts())
	}

	// Keeping the history closes the dialog and clears nothing.
	if err := tt.Click(locales.CommonCancel()); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if tt.HasText(locales.SettingsClearHistoryMessage()) {
		t.Error("the dialog stayed open after keeping the history")
	}
	if err := tt.Click(locales.SettingsClearHistory()); err != nil {
		t.Fatalf("click again: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.SettingsClearHistoryConfirm()); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	tt.Frame()
	if cleared != 1 {
		t.Fatalf("cleared %d times, want 1", cleared)
	}
	if tt.HasText(locales.SettingsClearHistoryMessage()) {
		t.Error("the dialog stayed open after confirming")
	}
}

// A failed save says so above the surface, whichever setting it was about: the
// app's settings and the managers' overrides are one config file.
func TestFailedSaveSaysSo(t *testing.T) {
	s := settingsFixture()
	tt := render(t, s.Build, 1200, 900)

	s.BeginSave()
	tt.Frame()
	s.EndSave(errors.New("permission denied"), Snapshot{})
	tt.Frame()
	if !tt.HasText(locales.SettingsSaveError("permission denied")) {
		t.Errorf("the failure is not shown: %v", tt.Texts())
	}
}

// Revert puts the controls back on what the backend last reported: a write that
// answered installs its answer through here, and a refused path edit must not
// stay on screen as if it had been saved. It says nothing about the error line —
// that is the write's outcome, which EndSave owns.
func TestRevertRestoresTheSavedState(t *testing.T) {
	s := settingsFixture()
	s.notify.Enabled = false
	s.theme = "dark"

	s.Revert(Snapshot{AutoScan: true, Notify: Notify{Enabled: true}, Theme: ""})
	if !s.notify.Enabled || s.theme != "" {
		t.Fatalf("controls = notifications %v theme %q, want the snapshot's", s.notify.Enabled, s.theme)
	}
}
