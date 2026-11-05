package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeDirs creates the directories a test wants to exist.
func makeDirs(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Only the tool directories that exist are added, after the ones already there,
// without duplicates.
func TestExtendPath(t *testing.T) {
	home := t.TempDir()
	tools := t.TempDir()
	makeDirs(t, tools, "brew/bin", "cargo/bin")

	homeTool := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(homeTool, 0o755); err != nil {
		t.Fatal(err)
	}

	got := extendPath("/usr/bin:/bin", home,
		filepath.Join(tools, "brew", "bin"),
		filepath.Join(tools, "missing", "bin"),
		"/usr/bin",
		homeTool,
		filepath.Join(tools, "cargo", "bin"),
	)
	want := strings.Join([]string{
		"/usr/bin", "/bin",
		filepath.Join(tools, "brew", "bin"),
		homeTool,
		filepath.Join(tools, "cargo", "bin"),
	}, string(os.PathListSeparator))
	if got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

// A tool in an added directory is found by name afterwards, which is what makes
// a launch from outside a shell work.
func TestEnsureToolPathFindsATool(t *testing.T) {
	home := t.TempDir()
	tools := t.TempDir()
	binDir := filepath.Join(tools, "brew", "bin")
	makeDirs(t, tools, "brew/bin")

	bin := filepath.Join(binDir, "fake-manager")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")
	saved := ToolDirs
	ToolDirs = []string{binDir}
	t.Cleanup(func() { ToolDirs = saved })

	if _, err := GetExecPath("fake-manager"); err == nil {
		t.Fatal("the tool was found before the path was extended")
	}
	EnsureToolPath()
	if path, err := GetExecPath("fake-manager"); err != nil || path != bin {
		t.Fatalf("GetExecPath = %q, %v; want %q", path, err, bin)
	}
}

// A PATH that already holds the directories is left alone.
func TestExtendPathLeavesAnExtendedPath(t *testing.T) {
	home := t.TempDir()
	tools := t.TempDir()
	makeDirs(t, tools, "brew/bin")

	bin := filepath.Join(tools, "brew", "bin")
	path := strings.Join([]string{"/usr/bin", bin}, string(os.PathListSeparator))
	if got := extendPath(path, home, bin); got != path {
		t.Errorf("path = %q, want it unchanged %q", got, path)
	}
}
