package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
