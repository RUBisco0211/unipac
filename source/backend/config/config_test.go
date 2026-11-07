package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// useTempRoot points the config at a fresh directory for one test.
func useTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	util.AppRoot = dir
	if err := Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return dir
}

func TestDefaultConfigScansOnLaunch(t *testing.T) {
	useTempRoot(t)
	if !Instance.Startup.ScansOnLaunch() {
		t.Fatal("a fresh config does not scan on launch")
	}
	if len(Instance.Managers) != 0 {
		t.Fatalf("a fresh config has manager settings: %v", Instance.Managers)
	}
	if Instance.Language != "" {
		t.Fatalf("a fresh config chose a language: %q", Instance.Language)
	}
}

// A config written by the previous version holds a snapshot of every manager
// under "adapters". Its exec_path was auto-detected, so it must not come back
// as a user setting that pins the executable.
func TestLegacyManagerSnapshotIsIgnored(t *testing.T) {
	dir := t.TempDir()
	util.AppRoot = dir
	legacy := `{
		"cache": {"path": "cache/package.db"},
		"log": {"level": "debug"},
		"adapters": {"brew": {"id": "brew", "exec_path": "/opt/homebrew/bin/brew", "enabled": true}}
	}`
	if err := os.WriteFile(filepath.Join(dir, "unipac.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if Instance.Log.Level != "debug" {
		t.Fatalf("log level = %q, want the saved debug", Instance.Log.Level)
	}
	if len(Instance.Managers) != 0 {
		t.Fatalf("legacy snapshot became manager settings: %v", Instance.Managers)
	}
	if !Instance.Startup.ScansOnLaunch() {
		t.Fatal("a config without the startup setting should scan on launch")
	}
}

func TestManagerSettingsPersist(t *testing.T) {
	dir := useTempRoot(t)

	if err := SetManagerEnabled("brew", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := SetManagerExecPath("npm", "  /usr/local/bin/npm "); err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := SetAutoScan(false); err != nil {
		t.Fatalf("auto scan: %v", err)
	}
	if err := SetLogLevel("warn"); err != nil {
		t.Fatalf("log level: %v", err)
	}
	if err := SetLanguage("zh-CN"); err != nil {
		t.Fatalf("language: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := loaded.Managers["brew"]; !got.Disabled {
		t.Fatalf("brew = %+v, want disabled", got)
	}
	if got := loaded.Managers["npm"].ExecPath; got != "/usr/local/bin/npm" {
		t.Fatalf("npm path = %q, want it trimmed and saved", got)
	}
	if loaded.Startup.ScansOnLaunch() {
		t.Fatal("scan on launch was not saved as off")
	}
	if loaded.Log.Level != "warn" {
		t.Fatalf("log level = %q, want warn", loaded.Log.Level)
	}
	if loaded.Language != "zh-CN" {
		t.Fatalf("language = %q, want zh-CN", loaded.Language)
	}

	// The file itself must not keep the manager snapshot from the old version.
	raw, err := os.ReadFile(filepath.Join(dir, "unipac.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); strings.Contains(got, `"adapters"`) {
		t.Fatalf("config still writes the legacy adapters block: %s", got)
	}
}

// A failed write must leave the running app on the values it started with, so
// the next start and the current session agree.
func TestFailedSaveKeepsTheRunningSettings(t *testing.T) {
	dir := useTempRoot(t)
	if err := SetManagerEnabled("brew", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	before := Instance.Managers["brew"]

	// A missing directory makes the write fail.
	filePath = filepath.Join(dir, "missing", "unipac.json")
	t.Cleanup(func() { filePath = filepath.Join(dir, "unipac.json") })

	if err := SetManagerExecPath("brew", "/opt/x"); err == nil {
		t.Fatal("expected the save to fail")
	}
	if got := Instance.Managers["brew"]; got != before {
		t.Fatalf("failed save changed the running settings: %+v", got)
	}
}

// A config file that does not parse is kept beside the new one: it is a file
// people edit by hand, and replacing it without a word would throw away every
// setting over a stray comma.
func TestUnparseableConfigIsKept(t *testing.T) {
	dir := t.TempDir()
	util.AppRoot = dir
	broken := `{
		"log": {"level": "debug"},
		"managers": {"brew": {"disabled": true},}
	}`
	path := filepath.Join(dir, "unipac.json")
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if Instance.Log.Level != "info" {
		t.Errorf("log level = %q, want the default", Instance.Log.Level)
	}

	kept, err := filepath.Glob(filepath.Join(dir, "unipac.json.bad-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 {
		t.Fatalf("kept files = %v, want the unreadable one", kept)
	}
	data, err := os.ReadFile(kept[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != broken {
		t.Errorf("the kept file changed: %s", data)
	}

	// The app goes on with a working config it can save into.
	if err := SetLogLevel("warn"); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Log.Level != "warn" {
		t.Errorf("log level = %q, want the saved warn", loaded.Log.Level)
	}
}

// A missing file is not an error worth keeping anything for: it is a first run.
func TestMissingConfigLeavesNoCopy(t *testing.T) {
	dir := t.TempDir()
	util.AppRoot = dir
	if err := Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	kept, err := filepath.Glob(filepath.Join(dir, "unipac.json.bad-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 0 {
		t.Errorf("a first run kept %v", kept)
	}
	if _, err := os.Stat(filepath.Join(dir, "unipac.json")); err != nil {
		t.Errorf("no config was written: %v", err)
	}
}

// Holding to confirm is on unless a config says otherwise, so a file written
// before the setting existed keeps the safer answer.
func TestHoldToConfirmDefaultsOn(t *testing.T) {
	useTempRoot(t)
	if !Instance.HoldsToConfirm() {
		t.Fatal("a fresh config does not hold to confirm")
	}
	if err := SetHoldToConfirm(false); err != nil {
		t.Fatalf("set: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.HoldsToConfirm() {
		t.Error("the saved off was lost")
	}
}

// The appearance and the notification switches persist like the rest of the
// settings, and an older config without them keeps announcing and follows the
// system.
func TestThemeAndNotificationsPersist(t *testing.T) {
	useTempRoot(t)

	if Instance.Notifies(tasks.KindUpdate, true) {
		if err := SetNotify(tasks.KindUpdate, true, false); err != nil {
			t.Fatalf("notifications: %v", err)
		}
	}
	if err := SetTheme(ThemeDark); err != nil {
		t.Fatalf("theme: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Notifies(tasks.KindUpdate, true) {
		t.Error("an operation turned off did not survive the round-trip")
	}
	// One switch is one operation and one outcome: the rest keep announcing.
	if !loaded.Notifies(tasks.KindInstall, true) || !loaded.Notifies(tasks.KindUpdate, false) ||
		!loaded.Notifies(tasks.KindUninstall, false) {
		t.Error("turning one switch off silenced another")
	}
	if loaded.Theme != ThemeDark {
		t.Fatalf("theme = %q, want dark", loaded.Theme)
	}

	// Following the system is the empty value, which is a real choice and
	// persists as one.
	if err := SetTheme(""); err != nil {
		t.Fatalf("system theme: %v", err)
	}
	loaded, err = Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Theme != "" {
		t.Fatalf("theme = %q, want the empty system value", loaded.Theme)
	}
}

// A config written with the single switch it replaced keeps it: whatever it said
// applies to every operation until one of the switches is saved, which is when
// the file converges on the per-operation ones.
func TestOneNotificationSwitchMigratesToAllOperations(t *testing.T) {
	dir := useTempRoot(t)

	old := `{"notifications": false}`
	if err := os.WriteFile(filepath.Join(dir, "unipac.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	// Init is the path the app takes: it is what installs the loaded file as the
	// running config, which SetNotify then writes.
	if err := Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, kind := range NotifyKinds() {
		for _, succeeded := range []bool{true, false} {
			if Instance.Notifies(kind, succeeded) {
				t.Errorf("the old switch was off, but %s succeeded=%v announces", kind, succeeded)
			}
		}
	}
	if Instance.NotifiesUpdates() {
		t.Error("the old switch was off, but the updates switch announces")
	}

	// Saving one switch rewrites the file in the shape it is read in now, and
	// keeps every other answer.
	if err := SetNotify(tasks.KindInstall, true, true); err != nil {
		t.Fatalf("set: %v", err)
	}
	if Instance.Notifications != nil {
		t.Error("the single switch survived a per-operation write")
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !loaded.Notifies(tasks.KindInstall, true) {
		t.Error("the saved switch did not survive")
	}
	if loaded.Notifies(tasks.KindInstall, false) || loaded.Notifies(tasks.KindUpdate, true) {
		t.Error("a switch that was not saved took the new one's value")
	}

	// The master switch and the updates switch are saved the same way.
	if err := SetNotifyEnabled(false); err != nil {
		t.Fatalf("master: %v", err)
	}
	if err := SetNotifyUpdates(false); err != nil {
		t.Fatalf("updates: %v", err)
	}
	loaded, err = Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.NotifiesEnabled() || loaded.NotifiesUpdates() {
		t.Error("the master or the updates switch did not survive")
	}
	// The master switch says nothing is announced, and the operation's own answer
	// is kept rather than forgotten: turning it back on restores it.
	if loaded.Notify.Install.Success == nil || !*loaded.Notify.Install.Success {
		t.Error("the master switch forgot the operation's own answer")
	}

	// A kind this build does not know is rejected rather than saved.
	if err := SetNotify("frobnicate", true, true); err == nil {
		t.Error("an unknown operation was saved")
	}
}

// The theme color is saved and survives a reload; a color the build does not
// know is rejected, so a hand edit cannot leave one the interface cannot draw.
func TestSetAccentRoundTrip(t *testing.T) {
	useTempRoot(t)

	if err := SetAccent(AccentBlue); err != nil {
		t.Fatalf("accent: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Accent != AccentBlue {
		t.Fatalf("accent = %q, want blue", loaded.Accent)
	}

	if err := SetAccent(AccentDefault); err != nil {
		t.Fatalf("default accent: %v", err)
	}
	if Instance.Accent != "" {
		t.Fatalf("accent = %q, want the empty default", Instance.Accent)
	}

	if err := SetAccent("chartreuse"); err == nil {
		t.Fatal("an unknown accent was saved")
	}
	if Instance.Accent != "" {
		t.Fatalf("the rejected color replaced the running one: %q", Instance.Accent)
	}
	// Every offered color is one the build knows, and the default comes first.
	accents := Accents()
	if len(accents) == 0 || accents[0] != AccentDefault {
		t.Fatalf("accents = %v, want the default first", accents)
	}
	for _, value := range accents {
		if !AccentChosen(value) {
			t.Errorf("%q is offered but not known", value)
		}
	}
}

// An appearance the app does not know is rejected rather than saved, so a
// hand edit or a stale client cannot write a value the window cannot show.
func TestSetThemeRejectsAnUnknownValue(t *testing.T) {
	useTempRoot(t)

	if err := SetTheme("blue"); err == nil {
		t.Fatal("an unknown appearance was saved")
	}
	if Instance.Theme == "blue" {
		t.Fatal("the rejected value replaced the running one")
	}
	if !ThemeChosen("") || !ThemeChosen(ThemeLight) || !ThemeChosen(ThemeDark) {
		t.Error("the known values were rejected")
	}
	if ThemeChosen("blue") {
		t.Error("an unknown value passed as known")
	}
}

// A config written before the two settings existed keeps the defaults: the
// notifications announce, and the appearance follows the system.
func TestThemeAndNotificationsDefaultOnOldConfigs(t *testing.T) {
	dir := useTempRoot(t)

	old := `{"language": "en"}`
	if err := os.WriteFile(filepath.Join(dir, "unipac.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, kind := range NotifyKinds() {
		for _, succeeded := range []bool{true, false} {
			if !loaded.Notifies(kind, succeeded) {
				t.Errorf("an older config lost the %s default", kind)
			}
		}
	}
	if !loaded.NotifiesUpdates() {
		t.Error("an older config lost the updates default")
	}
	if loaded.Theme != "" {
		t.Errorf("theme = %q, want the empty system default", loaded.Theme)
	}
}
