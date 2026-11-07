package manager

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// testAdapter exists only to carry an Info through Preflight.
type testAdapter struct{ info *Info }

func (a *testAdapter) Info() *Info { return a.info }

func (a *testAdapter) Preflight() error { return nil }

func (a *testAdapter) Setup() error { return nil }

func (a *testAdapter) Detail(Package) (Detail, error) { return Detail{}, nil }

func (a *testAdapter) ListInstalled() ([]Package, error) { return nil, nil }

func (a *testAdapter) ListOutdated() ([]Package, error) { return nil, nil }

func (a *testAdapter) Search(string) ([]Package, error) { return nil, nil }

func (a *testAdapter) Install([]Package, ActionOptions) (ActionResult, error) {
	return SuccessResult(""), nil
}

func (a *testAdapter) Uninstall([]Package, ActionOptions) (ActionResult, error) {
	return SuccessResult(""), nil
}

func (a *testAdapter) Update([]Package, ActionOptions) (ActionResult, error) {
	return SuccessResult(""), nil
}

func (a *testAdapter) ListVersions(Package) ([]string, error) { return nil, nil }

// A configured executable is the user's choice. Preflight must report it as
// unusable rather than quietly running a different binary found on PATH.
func TestPreflightKeepsAConfiguredPathThatCannotRun(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this machine")
	}
	missing := filepath.Join(t.TempDir(), "missing")
	adp := &testAdapter{info: &Info{ID: "fake", ExecName: filepath.Base(sh), ExecPath: missing}}

	if err := Preflight(adp, context.Background()); err == nil {
		t.Fatal("preflight accepted a configured path that cannot run")
	}
	if adp.Info().ExecPath != missing {
		t.Fatalf("preflight replaced the configured path with %q", adp.Info().ExecPath)
	}
}

// Without a configured path the executable is found on PATH.
func TestPreflightDetectsWithoutAConfiguredPath(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this machine")
	}
	adp := &testAdapter{info: &Info{ID: "fake", ExecName: filepath.Base(sh)}}

	if err := Preflight(adp, context.Background()); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if adp.Info().ExecPath != sh {
		t.Fatalf("exec path = %q, want %q", adp.Info().ExecPath, sh)
	}
}

// The version a manager reports is read where an unavailable one was already
// being ruled out, so the row that shows it costs no command of its own.
func TestPreflightRecordsTheVersion(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake-tool")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'fake-tool 1.4.2 (abc)'\n"), 0o755); err != nil {
		t.Fatalf("write the fake tool: %v", err)
	}
	adp := &testAdapter{info: &Info{ID: "fake", ExecName: "fake-tool", ExecPath: script}}

	if err := Preflight(adp, context.Background()); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if adp.Info().Version != "1.4.2" {
		t.Fatalf("version = %q, want 1.4.2", adp.Info().Version)
	}
}

// A manager that could not be probed has no version, and the row says nothing
// rather than inventing one.
func TestPreflightLeavesTheVersionEmptyWhenItCannotProbe(t *testing.T) {
	adp := &testAdapter{info: &Info{ID: "fake", ExecName: "fake-tool", ExecPath: filepath.Join(t.TempDir(), "missing")}}

	if err := Preflight(adp, context.Background()); err == nil {
		t.Fatal("preflight accepted a missing executable")
	}
	if adp.Info().Version != "" {
		t.Fatalf("version = %q, want none", adp.Info().Version)
	}
}
