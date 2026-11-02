package manager

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

// testAdapter exists only to carry an Info through Preflight.
type testAdapter struct{ info *Info }

func (a *testAdapter) Info() *Info { return a.info }

func (a *testAdapter) Preflight() error { return nil }

func (a *testAdapter) Setup() error { return nil }

func (a *testAdapter) GetPackageInfo(Package) (string, error) { return "", nil }

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
