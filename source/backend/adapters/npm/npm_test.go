package npm

import (
	"slices"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

// An update installs the version the scan found, not whatever latest is by the
// time it runs, so the row the user acted on and the task agree.
func TestUpdateSpecsNameTheVersion(t *testing.T) {
	pkgs := []manager.Package{
		{Name: "@quasar/cli", Manager: "npm", Installed: true, Version: "5.0.4", LatestVersion: "5.0.9"},
		{Name: "bun", Manager: "npm", Installed: true, Version: "1.4.2", LatestVersion: "1.4.4"},
	}
	got := updateSpecs(pkgs, manager.ActionOptions{})
	want := []string{"@quasar/cli@5.0.9", "bun@1.4.4"}
	if !slices.Equal(got, want) {
		t.Fatalf("specs = %v, want %v", got, want)
	}
}

// A caller that chose a version keeps it; the scan's answer does not override.
func TestUpdateSpecsPreferTheRequestedVersion(t *testing.T) {
	pkgs := []manager.Package{{Name: "bun", Manager: "npm", Installed: true, LatestVersion: "1.4.4"}}
	got := updateSpecs(pkgs, manager.ActionOptions{Version: "1.2.0"})
	if want := []string{"bun@1.2.0"}; !slices.Equal(got, want) {
		t.Fatalf("specs = %v, want %v", got, want)
	}
}

// Without a version from either source the update asks for latest, which is
// what npm resolves on its own, rather than a bare name.
func TestUpdateSpecsFallBackToLatest(t *testing.T) {
	pkgs := []manager.Package{{Name: "bun", Manager: "npm", Installed: true}}
	got := updateSpecs(pkgs, manager.ActionOptions{})
	if want := []string{"bun@latest"}; !slices.Equal(got, want) {
		t.Fatalf("specs = %v, want %v", got, want)
	}
}
