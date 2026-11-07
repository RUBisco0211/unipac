package util

import (
	"os"
	"path/filepath"
	"testing"
)

// useTempConfigDir points os.UserConfigDir at a fresh directory for one test.
func useTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

// The data directory moved from "unipac" to the bundle identifier: what was
// there is carried over, so a version that already ran keeps its settings,
// cache and task history.
func TestMigrateAppRootMovesTheOldDirectory(t *testing.T) {
	home := useTempConfigDir(t)
	oldDir := filepath.Join(home, "Library", "Application Support", "unipac")
	if err := os.MkdirAll(filepath.Join(oldDir, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "unipac.json"), []byte(`{"language":"zh-CN"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := MigrateAppRoot("unipac")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !moved {
		t.Fatal("nothing was moved")
	}
	newDir := filepath.Join(home, "Library", "Application Support", AppDirName)
	data, err := os.ReadFile(filepath.Join(newDir, "unipac.json"))
	if err != nil {
		t.Fatalf("the config did not come with it: %v", err)
	}
	if string(data) != `{"language":"zh-CN"}` {
		t.Errorf("config = %s", data)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("the old directory is still there: %v", err)
	}
}

// A directory already in place wins: a migration never overwrites what the
// running version wrote.
func TestMigrateAppRootLeavesAMovedInDirectory(t *testing.T) {
	home := useTempConfigDir(t)
	base := filepath.Join(home, "Library", "Application Support")
	for _, name := range []string{"unipac", AppDirName} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, AppDirName, "unipac.json"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := MigrateAppRoot("unipac")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if moved {
		t.Error("a directory already there was replaced")
	}
	data, _ := os.ReadFile(filepath.Join(base, AppDirName, "unipac.json"))
	if string(data) != "new" {
		t.Errorf("the file in place changed: %s", data)
	}
	if _, err := os.Stat(filepath.Join(base, "unipac")); err != nil {
		t.Errorf("the old directory was removed: %v", err)
	}
}

// A first run has nothing to move and must not fail.
func TestMigrateAppRootOnAFirstRun(t *testing.T) {
	useTempConfigDir(t)
	moved, err := MigrateAppRoot("unipac")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if moved {
		t.Error("a first run moved something")
	}
}

// A folder under the home directory is shown with ~ in its place, and one
// elsewhere is shown as it is.
func TestShortPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory to compare with")
	}
	for _, tc := range []struct{ in, want string }{
		{home, "~"},
		{filepath.Join(home, "Library", "Application Support", "app"), "~/Library/Application Support/app"},
		{"/opt/homebrew/opt/ripgrep", "/opt/homebrew/opt/ripgrep"},
		{home + "x/y", home + "x/y"}, // not under home, however alike the prefix
		{"", ""},
	} {
		if got := ShortPath(tc.in); got != tc.want {
			t.Errorf("ShortPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
