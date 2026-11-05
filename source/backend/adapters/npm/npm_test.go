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

// `npm view <name> versions --json` answers an array for a package with more
// than one version and a bare string for one published once. Both are the list,
// in npm's order: sorting it is the picker's job, not the adapter's.
func TestParseVersions(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
		want  []string
	}{
		{
			name:  "every version, oldest first",
			stdin: `["1.0.0","1.1.0","2.0.0-rc.1"]`,
			want:  []string{"1.0.0", "1.1.0", "2.0.0-rc.1"},
		},
		{
			name:  "a single version",
			stdin: `"1.0.0"`,
			want:  []string{"1.0.0"},
		},
	}
	for _, tc := range cases {
		got, err := parseVersions([]byte(tc.stdin))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: versions = %v, want %v", tc.name, got, tc.want)
		}
	}

	if _, err := parseVersions([]byte("npm error E404")); err == nil {
		t.Error("an error page parsed as a version list")
	}
}
