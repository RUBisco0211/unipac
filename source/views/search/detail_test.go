package search

import (
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
)

// A search result opens the same page the other tables do, and its facts are
// asked for: a result is not installed here, so there is no local copy to read.
func TestSearchResultOpensThePackagePage(t *testing.T) {
	s := &View{Data: fixture()}
	s.ran = true
	s.resultQuery = "ripgrep"
	s.results = []manager.Package{
		{Name: "ripgrep", Manager: "brew", Description: "line-oriented search tool"},
	}
	var asked []manager.Package
	s.OnDetail = func(pkg manager.Package) { asked = append(asked, pkg) }

	tt := render(t, s.Build, 1000, 600)
	if err := tt.Click("line-oriented search tool"); err != nil {
		t.Fatalf("click the row: %v", err)
	}
	tt.Frame()
	if len(asked) != 1 || asked[0].Name != "ripgrep" {
		t.Fatalf("asked for %v, want ripgrep's page", asked)
	}
	pkg, ok := s.Page.Pkg()
	if !ok || pkg.Name != "ripgrep" {
		t.Fatalf("the page shows %+v", pkg)
	}

	s.SetDetail(pkg, manager.Detail{
		Name: "ripgrep", Latest: "15.0.0", Summary: "searches for a regex pattern",
		Homepage: "https://github.com/BurntSushi/ripgrep", Publisher: "Andrew Gallant",
		License: "MIT", Source: "brew",
	}, nil)
	tt.Frame()
	for _, want := range []string{"searches for a regex pattern", "Andrew Gallant", "MIT", "15.0.0"} {
		if !tt.HasText(want) {
			t.Errorf("the page is missing %q: %v", want, tt.Texts())
		}
	}
	if !tt.HasText(locales.DetailClose()) {
		t.Errorf("the page has no close button: %v", tt.Texts())
	}
}
