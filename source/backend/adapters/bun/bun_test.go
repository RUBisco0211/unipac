package bun

import (
	"strings"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Every fixture in this file is output this adapter's commands actually printed
// on bun 1.4.2; the comments say which command produced it.

func names(pkgs []manager.Package) []string {
	out := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		out = append(out, pkg.Name+"@"+pkg.Version)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// `bun pm ls -g` with two globals, one of them scoped.
func TestParseGlobalTree(t *testing.T) {
	out := `/Users/rubisco/.bun/install/global node_modules (61 installed)
├── @quasar/cli@5.0.9
└── is-odd@3.0.1
`
	pkgs := parseGlobalTree("bun", out)
	if got, want := names(pkgs), []string{"@quasar/cli@5.0.9", "is-odd@3.0.1"}; !equal(got, want) {
		t.Fatalf("packages = %v, want %v", got, want)
	}
	for _, pkg := range pkgs {
		if pkg.Manager != "bun" || !pkg.Installed || pkg.Outdated || pkg.LatestVersion != "" {
			t.Errorf("%q = %+v, want an installed bun package with nothing else", pkg.Name, pkg)
		}
	}
}

// A scoped name splits at its last '@', not at the one it starts with.
func TestParseGlobalTreeSplitsScopedNames(t *testing.T) {
	out := `root node_modules (1 installed)
└── @deepseek-ai/dsh@0.2.2-rc.1
`
	pkgs := parseGlobalTree("bun", out)
	if len(pkgs) != 1 || pkgs[0].Name != "@deepseek-ai/dsh" || pkgs[0].Version != "0.2.2-rc.1" {
		t.Fatalf("packages = %+v, want @deepseek-ai/dsh at 0.2.2-rc.1", pkgs)
	}
}

// Nested tree lines carry their own prefix; the ones that reach the adapter do
// not, but a deeper tree must not turn into a package name.
func TestParseGlobalTreeSkipsWhatIsNotAPackage(t *testing.T) {
	out := `root node_modules (2 installed)
├── is-odd@3.0.1
│   └── is-number@6.0.0
`
	pkgs := parseGlobalTree("bun", out)
	if got := names(pkgs); len(got) != 1 || got[0] != "is-odd@3.0.1" {
		t.Fatalf("packages = %v, want is-odd only", got)
	}
}

// The app inherits the user's environment, and FORCE_COLOR makes bun colorize the
// tree it prints to a pipe. Parsing must see through it.
func TestParseGlobalTreeStripsColor(t *testing.T) {
	colored := "\x1b[2m└── \x1b[0m@quasar/cli\x1b[0m\x1b[2m@5.0.9\x1b[0m\n"
	pkgs := parseGlobalTree("bun", "root node_modules (59 installed)\n"+colored)
	if len(pkgs) != 1 || pkgs[0].Name != "@quasar/cli" || pkgs[0].Version != "5.0.9" {
		t.Fatalf("packages = %+v, want @quasar/cli at 5.0.9", pkgs)
	}
}

// `bun outdated -g` with two outdated globals: the separator row repeats between
// entries, and `Latest` is the newer version.
func TestParseOutdatedTable(t *testing.T) {
	out := `bun outdated v1.4.2 (744846f84)
|---------------------------------------|
| Package   | Current | Update | Latest |
|-----------|---------|--------|--------|
| is-number | 4.0.0   | 4.0.0  | 7.0.0  |
|-----------|---------|--------|--------|
| is-odd    | 2.0.0   | 2.0.0  | 3.0.1  |
|---------------------------------------|
`
	pkgs, err := parseOutdatedTable("bun", out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("packages = %+v, want two", pkgs)
	}
	first := pkgs[0]
	if first.Name != "is-number" || first.Version != "4.0.0" || first.LatestVersion != "7.0.0" {
		t.Errorf("first = %+v, want is-number 4.0.0 -> 7.0.0", first)
	}
	if !first.Outdated || !first.Installed || first.Manager != "bun" {
		t.Errorf("first = %+v, want an installed outdated bun package", first)
	}
	if pkgs[1].Name != "is-odd" || pkgs[1].LatestVersion != "3.0.1" {
		t.Errorf("second = %+v, want is-odd -> 3.0.1", pkgs[1])
	}
}

// Nothing outdated prints the banner alone.
func TestParseOutdatedTableWithNoRows(t *testing.T) {
	pkgs, err := parseOutdatedTable("bun", "bun outdated v1.4.2 (744846f84)\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("packages = %+v, want none", pkgs)
	}
}

// A row that is neither a rule nor a complete entry is reported, not dropped: a
// silently skipped row would leave a package showing no newer version.
func TestParseOutdatedTableReportsABrokenRow(t *testing.T) {
	out := `| Package | Current | Update | Latest |
| is-odd  | 2.0.0   |
`
	if _, err := parseOutdatedTable("bun", out); err == nil {
		t.Fatal("a row missing its columns parsed as a package")
	}
}

// An install names the version it was asked for; a search result's own version
// is used when the caller named none, and a bare name only when there is nothing
// else, where bun resolves it to latest.
func TestPackageSpecsNameTheVersion(t *testing.T) {
	chosen := packageSpecs([]manager.Package{{Name: "is-odd", Manager: "bun", Installed: true}}, manager.ActionOptions{Version: "2.0.0"})
	if !equal(chosen, []string{"is-odd@2.0.0"}) {
		t.Errorf("chosen version = %v", chosen)
	}

	found := packageSpecs([]manager.Package{{Name: "is-odd", Version: "1.0.0", Manager: "bun"}}, manager.ActionOptions{})
	if !equal(found, []string{"is-odd@1.0.0"}) {
		t.Errorf("search result = %v, want the version it was found at", found)
	}

	installed := packageSpecs([]manager.Package{{Name: "is-odd", Version: "1.0.0", Manager: "bun", Installed: true}}, manager.ActionOptions{})
	if !equal(installed, []string{"is-odd"}) {
		t.Errorf("installed package = %v, want a bare name", installed)
	}
}

// An update always names a version: without one bun resolves the range the
// global package.json holds, and an exact pin means "no changes".
func TestUpdateSpecsNameTheVersion(t *testing.T) {
	latest := updateSpecs([]manager.Package{{Name: "is-odd", Version: "2.0.0", LatestVersion: "3.0.1", Manager: "bun"}}, manager.ActionOptions{})
	if !equal(latest, []string{"is-odd@3.0.1"}) {
		t.Errorf("update = %v, want the scan's latest version", latest)
	}

	chosen := updateSpecs([]manager.Package{{Name: "is-odd", LatestVersion: "3.0.1", Manager: "bun"}}, manager.ActionOptions{Version: "2.0.0"})
	if !equal(chosen, []string{"is-odd@2.0.0"}) {
		t.Errorf("update = %v, want the version the caller chose", chosen)
	}

	unknown := updateSpecs([]manager.Package{{Name: "is-odd", Manager: "bun"}}, manager.ActionOptions{})
	if !equal(unknown, []string{"is-odd@latest"}) {
		t.Errorf("update = %v, want @latest when no version is known", unknown)
	}
}

// Removing names the packages, whatever version they are at.
func TestPackageNames(t *testing.T) {
	got := packageNames([]manager.Package{{Name: "@quasar/cli"}, {Name: "is-odd"}})
	if !equal(got, []string{"@quasar/cli", "is-odd"}) {
		t.Fatalf("names = %v", got)
	}
}

// Nothing installed globally is bun's exit-1 answer to both listings, not a
// failure: the inventory is simply empty. Each listing words the rest its own
// way, so only the shared phrase is matched.
func TestEmptyGlobalsAreNotAFailure(t *testing.T) {
	// bun has two ways of saying it has no global packages: a missing lockfile,
	// and a global directory with no package.json at all — which its listing and
	// its table word differently. Neither is a failure.
	for _, out := range []string{
		"error: missing lockfile, nothing to list\nnote: run 'bun install' first\n",
		"bun outdated v1.4.2 (744846f84)\nerror: missing lockfile, nothing outdated\nnote: run 'bun install' first\n",
		// A global directory with no package.json at all: the listing and the
		// table word it differently, and this machine prints these.
		"error: No package.json was found for directory \"/Users/me/.bun/install/global\"\nnote: Run \"bun init\" to initialize a project",
		"bun outdated v1.4.2 (744846f84)\nerror: missing package.json, nothing outdated\nerror: failed to initialize bun install: MissingPackageJSON",
	} {
		res := util.ExecResult{Stdout: out, ExitCode: 1}
		none, err := globalsState(res)
		if err != nil {
			t.Errorf("%q reported a failure: %v", strings.TrimSpace(out), err)
			continue
		}
		if !none {
			t.Errorf("%q is not read as an empty inventory", strings.TrimSpace(out))
		}
	}
}

// Anything else bun fails with is a failure, with bun's own words as the error.
func TestOtherFailuresAreReported(t *testing.T) {
	res := util.ExecResult{Stderr: "error: something else went wrong\n", ExitCode: 1}
	none, err := globalsState(res)
	if none {
		t.Fatal("an unrelated failure is read as an empty inventory")
	}
	if err == nil || !strings.Contains(err.Error(), "something else went wrong") {
		t.Fatalf("error = %v, want bun's own message", err)
	}
}

// The capabilities are the design, so they are pinned: bun shows, updates and
// removes what it manages, and does not advertise the three things it cannot
// answer.
func TestCapabilities(t *testing.T) {
	info := NewAdapter(t.Context()).Info()
	if info.ID != "bun" || info.Name != "Bun" || info.ExecName != "bun" {
		t.Errorf("info = %+v, want bun/Bun/bun", info)
	}
	caps := info.Capabilities
	if !caps.ListInstalled || !caps.ListOutdated {
		t.Errorf("capabilities = %+v, want the two listings", caps)
	}
	if !caps.Install || !caps.Update || !caps.Uninstall {
		t.Errorf("capabilities = %+v, want the three actions", caps)
	}
	// A package's page is read from the npm registry, which answers without bun;
	// search and a version list are what bun cannot offer.
	if !caps.Detail {
		t.Errorf("capabilities = %+v, want package detail on", caps)
	}
	if caps.Search || caps.ListVersions {
		t.Errorf("capabilities = %+v, want search and versions off", caps)
	}
}

// The capabilities the adapter does not advertise are not silently supported: a
// caller that asks anyway gets an error rather than an empty answer. Package
// detail is advertised, so it is not asked here — it would go to the registry.
func TestUnadvertisedCapabilitiesSaySo(t *testing.T) {
	a := NewAdapter(t.Context())
	if _, err := a.Search("is-odd"); err == nil {
		t.Error("search answered")
	}
	if _, err := a.ListVersions(manager.Package{Name: "is-odd"}); err == nil {
		t.Error("a version list answered")
	}
}
