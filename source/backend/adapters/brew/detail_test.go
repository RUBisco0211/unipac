package brew

import (
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

// A formula's facts, with the install time Homebrew records and the tap as its
// closest thing to a publisher.
func TestFormulaDetail(t *testing.T) {
	f := formula{Name: "wget", FullName: "wget", Desc: "Internet file retriever",
		Homepage: "https://www.gnu.org/software/wget/", License: "GPL-3.0-or-later"}
	f.Versions.Stable = "1.25.0"
	f.Installed = append(f.Installed, struct {
		Version string `json:"version"`
	}{Version: "1.24.0"})

	detail := formulaDetail(f, manager.Package{Name: "wget", Version: "1.24.0"})
	if detail.Version != "1.24.0" || detail.Latest != "1.25.0" {
		t.Errorf("versions = %+v", detail)
	}
	if detail.Summary != "Internet file retriever" || detail.Homepage == "" {
		t.Errorf("facts = %+v", detail)
	}
	if detail.License != "GPL-3.0-or-later" {
		t.Errorf("license = %q", detail.License)
	}
	// A formula from Homebrew's own taps comes from the Homebrew project.
	if detail.Publisher != "Homebrew" {
		t.Errorf("publisher = %q", detail.Publisher)
	}
}

// A cask keeps its version as a string and records no install time, and a tap
// from someone else names its owner.
func TestCaskDetail(t *testing.T) {
	c := cask{Token: "android-platform-tools", Name: []any{"Android SDK Platform-Tools"},
		Desc: "Android SDK component", Homepage: "https://developer.android.com/tools",
		Version: "37.0.2", Installed: "36.0.2", Tap: "someuser/tap"}

	detail := caskDetail(c, manager.Package{Name: "android-platform-tools"})
	if detail.Name != "Android SDK Platform-Tools" {
		t.Errorf("name = %q", detail.Name)
	}
	if detail.Version != "36.0.2" || detail.Latest != "37.0.2" {
		t.Errorf("versions = %+v", detail)
	}
	if detail.Publisher != "someuser" {
		t.Errorf("publisher = %q", detail.Publisher)
	}
}

// The license field is a string for most formulae and a list for those that offer
// a choice of terms.
func TestLicenseText(t *testing.T) {
	if got := licenseText("MIT"); got != "MIT" {
		t.Errorf("string license = %q", got)
	}
	if got := licenseText([]any{"MIT", "Apache-2.0"}); got != "MIT, Apache-2.0" {
		t.Errorf("list license = %q", got)
	}
	if got := licenseText(nil); got != "" {
		t.Errorf("missing license = %q", got)
	}
}
