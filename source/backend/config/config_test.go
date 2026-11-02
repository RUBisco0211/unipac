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
