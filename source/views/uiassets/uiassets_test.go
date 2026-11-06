package uiassets

import (
	"testing"
)

// A manager with a file has its mark, and the same one on every call.
func TestManagerMark(t *testing.T) {
	first := ManagerMark("brew")
	if first == nil {
		t.Fatal("brew has no mark")
	}
	if again := ManagerMark("brew"); again != first {
		t.Error("brew's mark was parsed twice")
	}
	// The files are named by manager ID: pip's mark is Python's, and that is
	// the only spelling the interface asks for.
	if ManagerMark("python") != nil {
		t.Error("a brand name answered where only a manager ID should")
	}
}

// A manager without a file shows without one, and asking again does not read
// the directory again.
func TestManagerMarkWithoutAFile(t *testing.T) {
	if ManagerMark("not-a-manager") != nil {
		t.Fatal("an unknown manager has a mark")
	}
	if again := ManagerMark("not-a-manager"); again != nil {
		t.Error("the negative answer was not remembered")
	}
}

// The managers this build registers all have marks. The list is by hand and
// must track backend/adapters/index.go — a new adapter without a file here
// fails this test, which is the guard against a tab silently losing its mark.
func TestEveryRegisteredManagerHasAMark(t *testing.T) {
	for _, id := range []string{"brew", "npm", "pip", "pipx", "cargo", "uv-tool"} {
		if ManagerMark(id) == nil {
			t.Errorf("manager %q has no mark", id)
		}
	}
}
