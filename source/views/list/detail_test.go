package list

import (
	"errors"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
)

// The row the user clicked, as the page holds it.
func openedPkg(t *testing.T, l *View) manager.Package {
	t.Helper()
	pkg, ok := l.Page.Pkg()
	if !ok {
		t.Fatal("no package is open")
	}
	return pkg
}

// A row opens its package's page in a dialog and asks for the facts; the answer
// fills the page in.
func TestRowClickOpensThePackagePage(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true}
	var asked []manager.Package
	l.OnDetail = func(pkg manager.Package) { asked = append(asked, pkg) }

	tt := render(t, l.Build, 1100, 620)
	if _, ok := l.Page.Pkg(); ok {
		t.Fatal("a page opened before anything was clicked")
	}

	// The click lands in the row, not on a control: the package's own description
	// is text the table passes to the row.
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click the row: %v", err)
	}
	tt.Frame()
	pkg := openedPkg(t, l)
	if pkg.Name != "ripgrep" {
		t.Fatalf("opened %+v, want ripgrep", pkg)
	}
	if len(asked) != 1 || asked[0].Name != "ripgrep" {
		t.Fatalf("asked for %v, want ripgrep's page", asked)
	}
	if !l.Page.Open {
		t.Error("the dialog did not open")
	}

	// The answer arrives and the page shows it.
	l.SetDetail(pkg, manager.Detail{
		Name: "ripgrep", Version: "14.1.1", Latest: "15.0.0",
		Summary: "line-oriented search tool", Homepage: "https://github.com/BurntSushi/ripgrep",
		Publisher: "Andrew Gallant", License: "MIT", Path: "/opt/homebrew/opt/ripgrep", Source: "brew",
	}, nil)
	tt.Frame()
	for _, want := range []string{
		"line-oriented search tool", "https://github.com/BurntSushi/ripgrep",
		"Andrew Gallant", "MIT", "/opt/homebrew/opt/ripgrep", "15.0.0",
		locales.DetailPublisher(), locales.DetailLicense(), locales.DetailLocation(),
	} {
		if !tt.HasText(want) {
			t.Errorf("the page is missing %q: %v", want, tt.Texts())
		}
	}

	// The page and the path are actable on.
	var openedURL, revealed string
	l.OnOpenURL = func(url string) { openedURL = url }
	l.OnReveal = func(path string) { revealed = path }
	tt.Frame()
	// The address is the link itself, and the path asks the file manager to show
	// the copy.
	if err := tt.Click("https://github.com/BurntSushi/ripgrep"); err != nil {
		t.Fatalf("open the homepage: %v", err)
	}
	if err := tt.Click("/opt/homebrew/opt/ripgrep"); err != nil {
		t.Fatalf("reveal the path: %v", err)
	}
	if openedURL != "https://github.com/BurntSushi/ripgrep" {
		t.Errorf("opened %q", openedURL)
	}
	if revealed != "/opt/homebrew/opt/ripgrep" {
		t.Errorf("revealed %q", revealed)
	}

	// Closing the dialog puts the page away.
	if err := tt.Click(locales.DetailClose()); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if l.Page.Open {
		t.Error("the dialog stayed open")
	}
}

// A page read for one package never lands on another: the user may click a second
// row while the first is still being read.
func TestDetailForAnotherPackageIsDropped(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true, OnDetail: func(manager.Package) {}}
	tt := render(t, l.Build, 1100, 620)
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	first := openedPkg(t, l)
	// The dialog is modal: the next row is reached by closing it first, which is
	// also when an answer for the package left behind can still arrive.
	if err := tt.Click(locales.DetailClose()); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if err := tt.Click("5.9.3"); err != nil {
		t.Fatalf("click the second row: %v", err)
	}
	tt.Frame()

	l.SetDetail(first, manager.Detail{Name: "ripgrep", Summary: "the first one's page"}, nil)
	tt.Frame()
	if tt.HasText("the first one's page") {
		t.Error("the first package's page landed on the second")
	}
}

// A page that could not be read completely still shows what is known, and stays
// quiet about the rest.
func TestDetailFailureStillShowsTheLocalFacts(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true, OnDetail: func(manager.Package) {}}
	tt := render(t, l.Build, 1100, 620)
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()

	l.SetDetail(openedPkg(t, l), manager.Detail{
		Name: "ripgrep", Version: "14.1.1", Path: "/opt/homebrew/opt/ripgrep", Source: "brew",
	}, errors.New("crates.io answered 504"))
	tt.Frame()
	if !tt.HasText("/opt/homebrew/opt/ripgrep") {
		t.Errorf("the local facts are missing: %v", tt.Texts())
	}
	// A field the registry did not answer is a dash, and the failure is the
	// shell's to log: the page does not turn into an error message.
	if tt.HasText("504") || tt.HasText("crates.io answered") {
		t.Errorf("the page shows the reader an error: %v", tt.Texts())
	}
}

// The same row opens the page again without reading it a second time, and a
// different row replaces what the dialog shows.
func TestPackagePageFollowsTheRow(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true, OnDetail: func(manager.Package) {}}
	tt := render(t, l.Build, 1100, 620)
	if err := tt.Click("Line-oriented search tool"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if !l.Page.Open {
		t.Fatal("the page did not open")
	}

	asked := 0
	l.OnDetail = func(manager.Package) { asked++ }
	if err := tt.Click(locales.DetailClose()); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if err := tt.Click("5.9.3"); err != nil {
		t.Fatalf("click the second row: %v", err)
	}
	tt.Frame()
	if pkg := openedPkg(t, l); pkg.Name != "typescript" {
		t.Fatalf("opening the second row left %q in the dialog", pkg.Name)
	}
	if asked != 1 {
		t.Fatalf("the second package was read %d times, want once", asked)
	}
}

// A manager whose listing prints no description has its rows described once a
// page is read, so the table says what the package is from then on.
func TestReadingAPageDescribesItsRow(t *testing.T) {
	l := &View{Data: fixture(), AllowRemove: true, OnDetail: func(manager.Package) {}}
	tt := render(t, l.Build, 1100, 620)
	if err := tt.Click("5.9.3"); err != nil {
		t.Fatalf("click the row with no description: %v", err)
	}
	tt.Frame()

	l.Data.Describe(openedPkg(t, l), "TypeScript is a language for application scale JavaScript")
	l.SetDetail(openedPkg(t, l), manager.Detail{
		Name: "typescript", Version: "5.9.3", Summary: "TypeScript is a language for application scale JavaScript",
	}, nil)
	tt.Frame()
	if !tt.HasText("TypeScript is a language for application scale JavaScript") {
		t.Errorf("the row is not described: %v", tt.Texts())
	}
}
