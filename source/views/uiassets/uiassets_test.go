package uiassets

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A manager with a file has its mark, and the same one on every call.
func TestManagerLogo(t *testing.T) {
	first := ManagerLogo("brew")
	if first == nil {
		t.Fatal("brew has no mark")
	}
	if again := ManagerLogo("brew"); again != first {
		t.Error("brew's mark was decoded twice")
	}
	// The files are named by manager ID: pip's mark is Python's, and that is
	// the only spelling the interface asks for.
	if ManagerLogo("python") != nil {
		t.Error("a brand name answered where only a manager ID should")
	}
	// Cargo's mark is a bitmap, the others are vectors; both arrive as images.
	if w, h := ui.ImageSize(ManagerLogo("cargo")); w <= 0 || h <= 0 {
		t.Errorf("cargo's bitmap mark has no size: %v×%v", w, h)
	}
}

// A manager without a file shows without one, and asking again does not read
// the directory again.
func TestManagerLogoWithoutAFile(t *testing.T) {
	if ManagerLogo("not-a-manager") != nil {
		t.Fatal("an unknown manager has a mark")
	}
	if again := ManagerLogo("not-a-manager"); again != nil {
		t.Error("the negative answer was not remembered")
	}
}

// The managers this build registers all have marks. The list is by hand and
// must track backend/adapters/index.go — a new adapter without a file here
// fails this test, which is the guard against a tab silently losing its mark.
func TestEveryRegisteredManagerHasAMark(t *testing.T) {
	for _, id := range []string{"brew", "bun", "npm", "pip", "pipx", "cargo", "uv-tool"} {
		if ManagerLogo(id) == nil {
			t.Errorf("manager %q has no mark", id)
		}
	}
}
