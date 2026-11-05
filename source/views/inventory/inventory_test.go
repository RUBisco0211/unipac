package inventory

import (
	"slices"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

// inventoryFixture is minimal on purpose: this package cannot import its own
// test fixtures, and these tests only need two packages from two managers.
func inventoryFixture() *Data {
	return &Data{
		Names:   map[string]string{"brew": "Homebrew", "npm": "NPM"},
		Version: 1,
		Managers: []manager.Info{
			{ID: "brew", Name: "Homebrew", Enabled: true},
			{ID: "npm", Name: "NPM", Enabled: true},
		},
		Rows: []manager.Package{
			{Name: "ripgrep", Manager: "brew", Installed: true},
			{Name: "typescript", Manager: "npm", Installed: true},
		},
	}
}

// Saving an unrelated setting republishes the same manager list; the settings
// page and search filter must not treat that as a manager change.
func TestSetManagersOnlyBumpsVersionOnChange(t *testing.T) {
	d := inventoryFixture()
	d.SetManagers(d.Managers, d.Overrides)
	version := d.ManagersVersion
	d.SetManagers(d.Managers, d.Overrides)
	if d.ManagersVersion != version {
		t.Fatalf("identical manager list bumped version %d -> %d", version, d.ManagersVersion)
	}
	next := slices.Clone(d.Managers)
	next[0].Enabled = false
	d.SetManagers(next, d.Overrides)
	if d.ManagersVersion == version {
		t.Fatal("a manager change did not bump the version")
	}
}

func TestBeginActionMarksOnlyItsPackages(t *testing.T) {
	d := inventoryFixture()
	npm := d.Rows[1]
	brew := d.Rows[0]

	if !d.BeginTask("update", []manager.Package{npm}) {
		t.Fatal("the task did not start")
	}
	if got := d.Running(npm); got != "update" {
		t.Fatalf("npm task = %q, want update", got)
	}
	if got := d.Running(brew); got != "" {
		t.Fatalf("another package was marked: %q", got)
	}
	if d.BeginTask("remove", []manager.Package{npm}) {
		t.Fatal("a second operation started on the same package")
	}
	if !d.BeginTask("install", []manager.Package{brew}) {
		t.Fatal("an operation on another package was blocked")
	}

	d.EndTask([]manager.Package{npm, brew})
	if d.AnyRunning([]manager.Package{npm, brew}) {
		t.Fatal("finishing did not clear the packages")
	}
	if !d.BeginTask("remove", []manager.Package{npm}) {
		t.Fatal("the package stayed busy after its task finished")
	}
}

// A request that arrives mid-scan must not be dropped: the follow-up scan is
// what stops a task from leaving pre-task rows on screen.
func TestRefreshQueuesOneFollowUp(t *testing.T) {
	d := &Data{}

	if !d.BeginRefresh() {
		t.Fatal("first refresh did not start")
	}
	if d.BeginRefresh() {
		t.Fatal("second refresh started while one was running")
	}
	if !d.refreshPending {
		t.Fatal("the mid-scan request was dropped")
	}
	if !d.FinishRefresh(nil, nil, nil) {
		t.Fatal("finishing did not report the queued scan")
	}
	if d.refreshPending {
		t.Fatal("the queued scan was not cleared")
	}
	if !d.BeginRefresh() {
		t.Fatal("the queued scan did not start")
	}
}
