package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shell's PATH comes first, and what the process already had follows
// without duplicates: a directory the process was given still resolves.
func TestMergePaths(t *testing.T) {
	sep := string(os.PathListSeparator)
	got := mergePaths(
		strings.Join([]string{"/opt/homebrew/bin", "/usr/bin", "/bin"}, sep),
		strings.Join([]string{"/usr/sbin", "/usr/bin"}, sep),
	)
	want := strings.Join([]string{"/opt/homebrew/bin", "/usr/bin", "/bin", "/usr/sbin"}, sep)
	if got != want {
		t.Errorf("merged = %q, want %q", got, want)
	}
}

// A shell prints its banners before the PATH; the last line is the answer.
func TestLastLineSkipsWhatTheShellPrints(t *testing.T) {
	out := "Last login: Fri Nov  7 10:00:00\nempty PATH, empty mind\n/opt/homebrew/bin:/usr/bin:/bin\n"
	if got := lastLine(out); got != "/opt/homebrew/bin:/usr/bin:/bin" {
		t.Errorf("lastLine = %q", got)
	}
	if got := lastLine("\n\n"); got != "" {
		t.Errorf("lastLine of nothing = %q", got)
	}
}

// The shell is asked, and its PATH is what the process ends up with.
func TestEnsureToolPathAsksTheShell(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fakeshell")
	script := "#!/bin/sh\necho 'a banner from an rc file'\necho '/tmp/one:/tmp/two'\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", fake)
	t.Setenv("PATH", "/usr/bin:/bin")

	path := EnsureToolPath()
	if path != "/tmp/one:/tmp/two"+string(os.PathListSeparator)+"/usr/bin:/bin" {
		t.Errorf("path = %q", path)
	}
	if os.Getenv("PATH") != path {
		t.Errorf("PATH = %q, want what EnsureToolPath returned", os.Getenv("PATH"))
	}
}

// A shell that cannot be asked leaves PATH alone, so the app still starts.
func TestEnsureToolPathKeepsPathWhenTheShellFails(t *testing.T) {
	t.Setenv("SHELL", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("PATH", "/usr/bin:/bin")

	if path := EnsureToolPath(); path != "/usr/bin:/bin" {
		t.Errorf("path = %q, want the one it had", path)
	}
}
