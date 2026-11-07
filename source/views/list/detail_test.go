package list

import (
	"errors"
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
)

// Opening a row shows its page in the panel and asks for the facts; the answer
// fills the panel in.
func TestRowClickOpensThePackagePage(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true}
	var asked []manager.Package
	l.OnDetail = func(pkg manager.Package) { asked = append(asked, pkg) }

	tt := render(t, l.Build, 1100, 500)
	if l.opened != nil {
		t.Fatal("a page opened before anything was clicked")
	}
	if tt.HasText(locales.DetailNone()) {
		t.Errorf("the panel is showing with nothing opened: %v", tt.Texts())
	}

	// A click on the table's first row opens it.
	// The click lands in the row, not on a control: the package's own
	// description is text the table passes to the row.
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click the row: %v", err)
	}
	tt.Frame()
	if l.opened == nil || l.opened.Name != "ripgrep" {
		t.Fatalf("opened = %+v, want ripgrep", l.opened)
	}
	if len(asked) != 1 || asked[0].Name != "ripgrep" {
		t.Fatalf("asked for %v, want ripgrep's page", asked)
	}
	// The panel slides in; past its motion the page is drawn.
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !tt.HasText(locales.DetailLoading()) {
		t.Fatalf("the panel did not arrive: %v", tt.Texts())
	}

	// The answer arrives and the page shows it.
	l.SetDetail(*l.opened, manager.Detail{
		Name: "ripgrep", Version: "14.1.1", Latest: "15.0.0",
		Summary: "line-oriented search tool", Homepage: "https://github.com/BurntSushi/ripgrep",
		Publisher: "Andrew Gallant", License: "MIT", Path: "/opt/homebrew/opt/ripgrep", Source: "brew",
	}, nil)
	tt.Frame()
	for _, want := range []string{
		"line-oriented search tool", "https://github.com/BurntSushi/ripgrep",
		"Andrew Gallant", "MIT", "/opt/homebrew/opt/ripgrep", "brew", "15.0.0",
		locales.DetailPublisher(), locales.DetailLicense(), locales.DetailLocation(),
	} {
		if !tt.HasText(want) {
			t.Errorf("the page is missing %q: %v", want, tt.Texts())
		}
	}
	// The path and the page are actable on.
	var openedURL, revealed string
	l.OnOpenURL = func(url string) { openedURL = url }
	l.OnReveal = func(path string) { revealed = path }
	tt.Frame()
	if err := tt.Click(locales.DetailHomepageOpen()); err != nil {
		t.Fatalf("open the homepage: %v", err)
	}
	if err := tt.Click(locales.DetailReveal()); err != nil {
		t.Fatalf("reveal the path: %v", err)
	}
	if openedURL != "https://github.com/BurntSushi/ripgrep" {
		t.Errorf("opened %q", openedURL)
	}
	if revealed != "/opt/homebrew/opt/ripgrep" {
		t.Errorf("revealed %q", revealed)
	}
}

// A page read for one package never lands on another: the user may click a second
// row while the first is still being read.
func TestDetailForAnotherPackageIsDropped(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true, OnDetail: func(manager.Package) {}}
	tt := render(t, l.Build, 1100, 500)
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if err := tt.Click("5.9.3"); err != nil {
		t.Fatalf("click the second row: %v", err)
	}
	tt.Frame()

	first := manager.Package{Name: "ripgrep", Manager: "brew", Version: "14.1.1"}
	l.SetDetail(first, manager.Detail{Name: "ripgrep", Summary: "the first one's page"}, nil)
	tt.Frame()
	if tt.HasText("the first one's page") {
		t.Error("the first package's page landed on the second")
	}
}

// A page that could not be read completely still shows what is known, and says
// what is missing.
func TestDetailFailureStillShowsTheLocalFacts(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true, OnDetail: func(manager.Package) {}}
	tt := render(t, l.Build, 1100, 500)
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)

	l.SetDetail(*l.opened, manager.Detail{
		Name: "ripgrep", Version: "14.1.1", Path: "/opt/homebrew/opt/ripgrep", Source: "brew",
	}, errors.New("registry.npmjs.org answered 504"))
	tt.Frame()
	if !tt.HasText("/opt/homebrew/opt/ripgrep") {
		t.Errorf("the local facts are missing: %v", tt.Texts())
	}
	if !tt.HasText("504") {
		t.Errorf("the failure is not named: %v", tt.Texts())
	}
}

// Closing the panel leaves the table alone, and a further click opens it again.
func TestDetailPanelClosesAndReopens(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true, OnDetail: func(manager.Package) {}}
	tt := render(t, l.Build, 1100, 500)
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if l.Panel.Closed {
		t.Fatal("the panel did not open")
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if err := tt.Click(locales.DetailClose()); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !l.Panel.Closed {
		t.Fatal("the panel stayed open")
	}
	// The same row opens it again without reading the page a second time.
	asked := 0
	l.OnDetail = func(manager.Package) { asked++ }
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click again: %v", err)
	}
	tt.Frame()
	if l.Panel.Closed || asked != 0 {
		t.Fatalf("reopening read the page again: closed=%v reads=%d", l.Panel.Closed, asked)
	}
}
