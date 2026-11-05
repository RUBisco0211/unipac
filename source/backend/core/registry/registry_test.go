package registry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

// fakeAdapter counts preflights so a test can prove a settings change does not
// spawn a subprocess for a manager it did not touch.
type fakeAdapter struct {
	info       *manager.Info
	available  bool
	preflights int
	packages   []manager.Package
	listErr    error
	searches   int
	searchHits []manager.Package
}

func (f *fakeAdapter) Info() *manager.Info { return f.info }

func (f *fakeAdapter) Preflight() error {
	f.preflights++
	if !f.available {
		return errors.New("not found")
	}
	return nil
}

func (f *fakeAdapter) Setup() error { return nil }

func (f *fakeAdapter) GetPackageInfo(manager.Package) (string, error) { return "", nil }

func (f *fakeAdapter) ListInstalled() ([]manager.Package, error) { return f.packages, f.listErr }

func (f *fakeAdapter) ListOutdated() ([]manager.Package, error) { return nil, nil }

func (f *fakeAdapter) Search(string) ([]manager.Package, error) {
	f.searches++
	return f.searchHits, nil
}

func (f *fakeAdapter) Install([]manager.Package, manager.ActionOptions) (manager.ActionResult, error) {
	return manager.SuccessResult(""), nil
}

func (f *fakeAdapter) Uninstall([]manager.Package, manager.ActionOptions) (manager.ActionResult, error) {
	return manager.SuccessResult(""), nil
}

func (f *fakeAdapter) Update([]manager.Package, manager.ActionOptions) (manager.ActionResult, error) {
	return manager.SuccessResult(""), nil
}

func (f *fakeAdapter) ListVersions(manager.Package) ([]string, error) { return nil, nil }

// newTestRegistry builds an empty registry whose constructors return the given
// adapters. A real constructor makes a new adapter per call; these return the
// same one so a test can read its counters.
func newTestRegistry(adapters ...*fakeAdapter) *Registry {
	reg := &Registry{
		ctx:          context.Background(),
		adapters:     map[string]manager.Adapter{},
		constructors: map[string]manager.AdapterConstructor{},
		settings:     map[string]manager.Settings{},
	}
	for _, adp := range adapters {
		adp := adp
		reg.constructors[adp.Info().ID] = func(context.Context) manager.Adapter { return adp }
	}
	return reg
}

func fake(available bool) *fakeAdapter {
	return &fakeAdapter{
		info: &manager.Info{
			ID:           "fake",
			Name:         "Fake",
			ExecName:     "fake",
			Capabilities: manager.Capabilities{ListInstalled: true},
		},
		available: available,
	}
}

func newFakeRegistry(available bool) (*Registry, *fakeAdapter) {
	adp := fake(available)
	reg := newTestRegistry(adp)
	reg.rebuild(map[string]manager.Settings{"fake": {}}, true)
	return reg, adp
}

func TestRegisterMarksAvailabilityAndState(t *testing.T) {
	_, adp := newFakeRegistry(true)
	if !adp.Info().Available || !adp.Info().Enabled {
		t.Fatalf("available manager = %+v, want available and enabled", adp.Info())
	}

	_, adp = newFakeRegistry(false)
	if adp.Info().Available || adp.Info().Enabled {
		t.Fatalf("missing manager = %+v, want unavailable and disabled", adp.Info())
	}
}

// A saved preference survives a missing executable: the manager stays
// unavailable, and turning it on cannot make it usable.
func TestUnavailableManagerStaysDisabled(t *testing.T) {
	reg, adp := newFakeRegistry(false)
	reg.Apply(map[string]manager.Settings{"fake": {}})
	if adp.Info().Enabled {
		t.Fatal("an unavailable manager was enabled")
	}
}

// Flipping the switch must not preflight again; a new executable must.
func TestApplyOnlyPreflightsChangedPaths(t *testing.T) {
	reg, adp := newFakeRegistry(true)
	if adp.preflights != 1 {
		t.Fatalf("preflights after register = %d, want 1", adp.preflights)
	}

	reg.Apply(map[string]manager.Settings{"fake": {Disabled: true}})
	if adp.Info().Enabled {
		t.Fatal("disabled manager is still enabled")
	}
	if adp.preflights != 1 {
		t.Fatalf("toggling preflighted again: %d", adp.preflights)
	}

	reg.Apply(map[string]manager.Settings{"fake": {ExecPath: "/opt/fake"}})
	if adp.Info().ExecPath != "/opt/fake" {
		t.Fatalf("exec path = %q, want /opt/fake", adp.Info().ExecPath)
	}
	if adp.preflights != 2 {
		t.Fatalf("a new path did not preflight: %d", adp.preflights)
	}
	if !adp.Info().Enabled {
		t.Fatal("enabled state did not follow the applied settings")
	}
}

// The adapters live in a map, so without sorting the settings page and the
// manager filter would reshuffle on every rebuild.
func TestListManagersIsAlphabetical(t *testing.T) {
	var fakes []*fakeAdapter
	for _, name := range []string{"uv tool", "Pip", "Homebrew", "npm", "Cargo"} {
		adp := fake(true)
		adp.info.ID = strings.ReplaceAll(strings.ToLower(name), " ", "-")
		adp.info.Name = name
		fakes = append(fakes, adp)
	}
	reg := newTestRegistry(fakes...)
	reg.rebuild(map[string]manager.Settings{}, true)

	var got []string
	for _, m := range reg.ListManagers() {
		got = append(got, m.Name)
	}
	want := []string{"Cargo", "Homebrew", "npm", "Pip", "uv tool"}
	if !slices.Equal(got, want) {
		t.Fatalf("manager order = %v, want %v", got, want)
	}
}

// reliable is what lets App.UpdateCache tell "nothing is installed" from "every
// manager failed", so an empty but trustworthy scan clears the cache while a
// failed one keeps it.
func TestGetInstalledPackagesReportsReliability(t *testing.T) {
	// No enabled manager: nothing can fail, so an empty result is the truth.
	reg, _ := newFakeRegistry(true)
	reg.Apply(map[string]manager.Settings{"fake": {Disabled: true}})
	if _, reliable, err := reg.GetInstalledPackages(); err != nil || !reliable {
		t.Fatalf("all disabled: reliable = %v, err = %v; want true, nil", reliable, err)
	}

	// The only enabled manager fails.
	failing := fake(true)
	failing.info.ID = "broken"
	failing.listErr = errors.New("boom")
	reg = newTestRegistry(failing)
	reg.rebuild(map[string]manager.Settings{"broken": {}}, true)
	if _, reliable, err := reg.GetInstalledPackages(); err != nil || reliable {
		t.Fatalf("all failed: reliable = %v, err = %v; want false, nil", reliable, err)
	}

	// One of them works.
	working := fake(true)
	working.info.ID = "works"
	working.packages = []manager.Package{{Name: "ripgrep", Manager: "works"}}
	reg = newTestRegistry(failing, working)
	reg.rebuild(map[string]manager.Settings{"broken": {}, "works": {}}, true)
	pkgs, reliable, err := reg.GetInstalledPackages()
	if err != nil || !reliable || len(pkgs) != 1 {
		t.Fatalf("one works: %d packages, reliable = %v, err = %v", len(pkgs), reliable, err)
	}
}

// blockingAdapter stalls ListInstalled so a settings change can land while a
// scan is running, which is the case that used to mutate Info under the scan.
type blockingAdapter struct {
	*fakeAdapter
	started chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (b *blockingAdapter) ListInstalled() ([]manager.Package, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return nil, nil
}

// A settings change must not wait for a scan, and must not touch the adapters
// the scan is using. Run this with -race to check the second half.
func TestApplyWhileScanning(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	reg := &Registry{
		ctx:          context.Background(),
		adapters:     map[string]manager.Adapter{},
		constructors: map[string]manager.AdapterConstructor{},
		settings:     map[string]manager.Settings{},
	}
	reg.constructors["fake"] = func(context.Context) manager.Adapter {
		base := fake(true)
		return &blockingAdapter{fakeAdapter: base, started: started, release: release, once: &once}
	}
	reg.rebuild(map[string]manager.Settings{"fake": {}}, true)

	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		_, _, _ = reg.GetInstalledPackages()
	}()
	<-started

	applied := make(chan struct{})
	go func() {
		defer close(applied)
		reg.Apply(map[string]manager.Settings{"fake": {Disabled: true}})
	}()
	select {
	case <-applied:
	case <-time.After(2 * time.Second):
		t.Fatal("a settings change waited for the scan")
	}

	close(release)
	<-scanDone

	for _, m := range reg.ListManagers() {
		if m.ID == "fake" && m.Enabled {
			t.Fatal("the setting did not take effect")
		}
	}
}

func TestSearchPackagesFiltersManagersInTheBackend(t *testing.T) {
	a := fake(true)
	a.info.ID, a.info.Name = "a", "A"
	a.info.Available, a.info.Enabled = true, true
	a.info.Capabilities.Search = true
	a.searchHits = []manager.Package{{Name: "from-a", Manager: "a"}}
	b := fake(true)
	b.info.ID, b.info.Name = "b", "B"
	b.info.Available, b.info.Enabled = true, true
	b.info.Capabilities.Search = true
	b.searchHits = []manager.Package{{Name: "from-b", Manager: "b"}}
	reg := newTestRegistry(a, b)
	reg.rebuild(map[string]manager.Settings{"a": {}, "b": {}}, true)

	result, err := reg.SearchPackages("query", []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if a.searches != 1 || b.searches != 0 {
		t.Fatalf("searches a=%d b=%d, want only a", a.searches, b.searches)
	}
	if len(result.Packages) != 1 || result.Packages[0].Name != "from-a" {
		t.Fatalf("result = %+v", result)
	}

	a.searches, b.searches = 0, 0
	_, _ = reg.SearchPackages("query", nil)
	if a.searches != 1 || b.searches != 1 {
		t.Fatalf("all managers searched a=%d b=%d", a.searches, b.searches)
	}

	a.searches, b.searches = 0, 0
	result, _ = reg.SearchPackages("query", []string{})
	if len(result.Packages) != 0 || a.searches != 0 || b.searches != 0 {
		t.Fatalf("empty set searched a=%d b=%d, result %+v", a.searches, b.searches, result)
	}
}

// A manager with more matches than the limit gives the first of them, and the
// result says it was cut, so the page can say so instead of looking complete.
func TestSearchPackagesCutsAManagerAtTheLimit(t *testing.T) {
	many := make([]manager.Package, manager.MaxSearchResults+5)
	for i := range many {
		many[i] = manager.Package{Name: fmt.Sprintf("pkg-%03d", i), Manager: "a"}
	}
	a := fake(true)
	a.info.ID, a.info.Name = "a", "A"
	a.info.Available, a.info.Enabled = true, true
	a.info.Capabilities.Search = true
	a.searchHits = many
	reg := newTestRegistry(a)
	reg.rebuild(map[string]manager.Settings{"a": {}}, true)

	result, err := reg.SearchPackages("pkg", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) != manager.MaxSearchResults {
		t.Errorf("packages = %d, want the limit of %d", len(result.Packages), manager.MaxSearchResults)
	}
	if !result.Truncated {
		t.Error("the result does not say it was cut")
	}

	a.searchHits = many[:3]
	result, err = reg.SearchPackages("pkg", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) != 3 || result.Truncated {
		t.Errorf("result = %d packages, truncated %v; want 3 and not truncated",
			len(result.Packages), result.Truncated)
	}
}
