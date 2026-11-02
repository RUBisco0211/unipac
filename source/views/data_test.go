package views

import (
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

func TestBeginActionMarksOnlyItsPackages(t *testing.T) {
	d := fixture()
	npm := d.rows[1]
	brew := d.rows[0]

	if !d.beginAction("update", []manager.Package{npm}) {
		t.Fatal("the action did not start")
	}
	if got := d.actionFor(npm); got != "update" {
		t.Fatalf("npm action = %q, want update", got)
	}
	if got := d.actionFor(brew); got != "" {
		t.Fatalf("another package was marked: %q", got)
	}
	if d.beginAction("remove", []manager.Package{npm}) {
		t.Fatal("a second operation started on the same package")
	}
	if !d.beginAction("install", []manager.Package{brew}) {
		t.Fatal("an operation on another package was blocked")
	}

	d.finishAction([]manager.Package{npm, brew})
	if d.anyActive([]manager.Package{npm, brew}) {
		t.Fatal("finishing did not clear the packages")
	}
	if !d.beginAction("remove", []manager.Package{npm}) {
		t.Fatal("the package stayed busy after its action finished")
	}
}
