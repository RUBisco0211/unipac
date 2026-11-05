package brew

import (
	"context"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

// A cask from an untrusted tap makes `brew list --cask --versions` fail as a
// whole; its --json form is the one that lists every cask anyway, so the adapter
// reads that and takes the linked version.
func TestParseCaskList(t *testing.T) {
	output := `{"formulae":[],"casks":[
		{"token":"android-platform-tools","versions":["36.0.2"],"pinned_version":null},
		{"token":"bettercmdtab","versions":["26.7,20260813144806"],"pinned_version":null},
		{"token":"flux-markdown","versions":["1.2.0"],"pinned_version":null}
	]}`
	casks, err := parseCaskList(output)
	if err != nil {
		t.Fatalf("parseCaskList: %v", err)
	}
	if len(casks) != 3 {
		t.Fatalf("casks = %d, want 3: %+v", len(casks), casks)
	}
	for _, want := range []struct{ name, version string }{
		{"android-platform-tools", "36.0.2"},
		{"bettercmdtab", "26.7,20260813144806"},
		{"flux-markdown", "1.2.0"},
	} {
		var found *manager.Package
		for i := range casks {
			if casks[i].Name == want.name {
				found = &casks[i]
			}
		}
		if found == nil {
			t.Fatalf("%s is missing from %+v", want.name, casks)
		}
		if found.Version != want.version {
			t.Errorf("%s version = %q, want %q", want.name, found.Version, want.version)
		}
		if !found.Installed || !found.IsGUI {
			t.Errorf("%s = %+v, want an installed cask", want.name, found)
		}
	}
}

// The line parser takes the name and the linked version: a package with an older
// version still installed used to come out as one version holding both.
func TestLineParserTakesTheLinkedVersion(t *testing.T) {
	a := &Adapter{ctx: context.Background(), info: &manager.Info{ID: "brew"}}
	parse := a.lineParserFunc(false)

	pkg, err := parse("aom 3.14.1 3.15.3")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pkg.Name != "aom" || pkg.Version != "3.15.3" {
		t.Fatalf("package = %+v, want aom at 3.15.3", pkg)
	}
	if !pkg.Installed || pkg.IsGUI || pkg.Manager != "brew" {
		t.Errorf("package = %+v, want an installed formula of brew", pkg)
	}

	one, err := parse("mole 1.68.0")
	if err != nil || one.Version != "1.68.0" {
		t.Fatalf("package = %+v, %v", one, err)
	}

	if _, err := parse("aom"); err == nil {
		t.Error("a line without a version parsed")
	}
}
