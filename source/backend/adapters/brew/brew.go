package brew

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

type Adapter struct {
	ctx  context.Context
	info *manager.Info
}

func NewAdapter(ctx context.Context) manager.Adapter {
	return &Adapter{
		ctx: ctx,
		info: &manager.Info{
			ID:       "brew",
			Name:     "Homebrew",
			Website:  "https://brew.sh",
			ExecName: "brew",
			Capabilities: *manager.
				DefaultCapabilities().
				WithSearch(true).
				WithListOutdated(true),
		},
	}
}

func (a *Adapter) lineParserFunc() util.LineHandler[manager.Package] {
	return func(line string) (manager.Package, error) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			slog.ErrorContext(a.ctx,
				"unexpected line format",
				"line", line,
			)
			return manager.Package{}, fmt.Errorf("unexpected line format: %s", line)
		}
		return manager.Package{
			Name: fields[0],
			// A package with an older version still installed lists them all,
			// oldest first: `brew list --versions aom` prints `aom 3.14.1 3.15.3`
			// and the last is the one linked.
			Version:   fields[len(fields)-1],
			Installed: true,
			Manager:   a.Info().ID,
		}, nil
	}
}

// Info returns the manager info
func (a *Adapter) Info() *manager.Info {
	return a.info
}

// WithContext returns a copy of the adapter whose commands run with ctx, so
// one action's output goes to that action's console.
func (a *Adapter) WithContext(ctx context.Context) manager.Adapter {
	clone := *a
	clone.ctx = ctx
	return &clone
}

// Preflight checks if brew is available in the system
func (a *Adapter) Preflight() error {
	return manager.Preflight(a, a.ctx)
}

// Setup performs any necessary setup for the manager (no-op for brew)
func (a *Adapter) Setup() error {
	return nil
}

// ListInstalled returns the list of installed packages for brew
func (a *Adapter) ListInstalled() ([]manager.Package, error) {
	caskCollector := func() ([]manager.Package, error) {
		return a.listCasks()
	}

	formulaCollector := func() ([]manager.Package, error) {
		formulas, err := util.ExecAndCollect(a.ctx,
			a.info.ExecPath,
			[]string{"list", "--formula", "--versions"},
			nil,
			a.lineParserFunc(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list formulas: %w", err)
		}
		return formulas, nil
	}

	return util.CollectParallel(a.ctx, caskCollector, formulaCollector)
}

// listCasks lists the installed casks with their versions.
//
// The --json form is what makes this survive a cask from a tap the user has not
// trusted: `brew list --cask --versions` without it loads every cask through
// Homebrew's Ruby front end, which refuses such a cask outright and fails the
// whole listing, so one tap's trust setting hid every cask. The --json form
// reads the Caskroom instead and lists them all.
func (a *Adapter) listCasks() ([]manager.Package, error) {
	output, err := util.Exec(a.ctx,
		a.info.ExecPath,
		[]string{"list", "--cask", "--versions", "--json"},
		nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list casks: %w", err)
	}
	casks, err := parseCaskList(output)
	if err != nil {
		return nil, err
	}
	for i := range casks {
		casks[i].Manager = a.Info().ID
	}
	return casks, nil
}

// parseCaskList reads the casks `brew list --cask --versions --json` printed.
func parseCaskList(output string) ([]manager.Package, error) {
	var list caskList
	if err := json.Unmarshal([]byte(output), &list); err != nil {
		return nil, fmt.Errorf("failed to parse brew cask list: %w", err)
	}
	casks := make([]manager.Package, 0, len(list.Casks))
	for _, c := range list.Casks {
		if c.Token == "" {
			continue
		}
		pkg := manager.Package{Name: c.Token, Installed: true}
		if len(c.Versions) > 0 {
			pkg.Version = c.Versions[len(c.Versions)-1]
		}
		casks = append(casks, pkg)
	}
	return casks, nil
}

// ListOutdated returns the list of outdated packages for brew
func (a *Adapter) ListOutdated() ([]manager.Package, error) {
	output, err := util.Exec(a.ctx,
		a.info.ExecPath,
		[]string{"outdated", "--json=v2"},
		[]string{"HOMEBREW_NO_AUTO_UPDATE=1"})
	if err != nil {
		return nil, fmt.Errorf("failed to list outdated packages: %w", err)
	}

	var obj jsonOutput
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse brew outdated output: %w", err)
	}

	// Process the JSON output to extract outdated packages
	var outdated []manager.Package
	for _, formula := range obj.Formulae {
		outdated = append(outdated, manager.Package{
			Name:          formula.Name,
			Version:       formula.InstalledVersions[0],
			LatestVersion: formula.CurrentVersion,
			Manager:       a.Info().ID,
			Outdated:      true,
		})
	}

	for _, cask := range obj.Casks {
		outdated = append(outdated, manager.Package{
			Name:          cask.Name,
			Version:       cask.InstalledVersions[0],
			LatestVersion: cask.CurrentVersion,
			Manager:       a.Info().ID,
			Outdated:      true,
		})
	}
	return outdated, nil
}

// Search returns the list of packages matching the keyword for brew
func (a *Adapter) Search(keyword string) ([]manager.Package, error) {
	res, err := util.RunAllowExitCodes(a.ctx, a.info.ExecPath,
		[]string{"search", keyword},
		[]string{"HOMEBREW_NO_AUTO_UPDATE=1"},
		0, 1,
	)
	if err != nil {
		return nil, fmt.Errorf("brew search failed: %w", err)
	}
	if res.Stdout == "" {
		return []manager.Package{}, nil
	}

	entries := parseBrewSearch(res.Stdout)
	if len(entries) == 0 {
		return []manager.Package{}, nil
	}

	// collect all package names into a single brew info call
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	// `brew search` has no limit, so a one-letter keyword can match thousands of
	// names. Asking for all their details in one command makes brew load every
	// one of them before printing anything; ask in batches instead, and keep what
	// came back, so a batch that fails costs its own names and not the search.
	info := make(pkgLookup, len(names))
	for start := 0; start < len(names); start += searchDetailsBatch {
		end := min(start+searchDetailsBatch, len(names))
		details, err := a.searchDetails(names[start:end])
		if err != nil {
			slog.WarnContext(a.ctx,
				"brew info failed, returning those search results without details",
				"keyword", keyword,
				"names", end-start,
				"error", err,
			)
			continue
		}
		maps.Copy(info, details)
	}
	return buildSearchResults(entries, info), nil
}

// searchDetailsBatch is how many names one `brew info` is asked about. Brew
// answers in a single JSON document, and a document of a few thousand entries is
// where its startup starts to dominate the search.
const searchDetailsBatch = 100

// searchDetails reads the version and description of names from one `brew info`.
func (a *Adapter) searchDetails(names []string) (pkgLookup, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath,
		append([]string{"info", "--json=v2"}, names...),
		[]string{"HOMEBREW_NO_AUTO_UPDATE=1"},
	)
	if err != nil {
		return nil, err
	}
	return parseBrewInfo(res.Stdout), nil
}

// searchEntry is an intermediate result from parsing brew search output
type searchEntry struct {
	Name string
}

// parseBrewSearch parses the flat list output from `brew search <keyword>`.
func parseBrewSearch(output string) []searchEntry {
	lines := strings.Split(output, "\n")
	entries := make([]searchEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, searchEntry{Name: line})
	}
	return entries
}

// pkgDetail holds version and description looked up from brew info
type pkgDetail struct {
	version string
	desc    string
}

// pkgLookup maps a search-result name to its detail from brew info
type pkgLookup map[string]pkgDetail

// parseBrewInfo parses the JSON output from `brew info --json=v2 [names...]`
// and builds a lookup map keyed by formula name / cask token.
func parseBrewInfo(output string) pkgLookup {
	var obj brewInfoOutput
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		return nil
	}
	m := make(pkgLookup, len(obj.Formulae)+len(obj.Casks))
	for _, f := range obj.Formulae {
		m[f.Name] = pkgDetail{version: f.Versions.Stable, desc: f.Desc}
	}
	for _, c := range obj.Casks {
		m[c.Token] = pkgDetail{version: c.Version, desc: c.Desc}
	}
	return m
}

// buildSearchResults merges search entries with info details into manager.Package slice.
func buildSearchResults(entries []searchEntry, info pkgLookup) []manager.Package {
	pkgs := make([]manager.Package, 0, len(entries))
	for _, e := range entries {
		pkg := manager.Package{
			Name:    e.Name,
			Manager: "brew",
		}
		if info != nil {
			if d, ok := info[e.Name]; ok {
				pkg.Version = d.version
				pkg.Description = d.desc
			}
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

// Install installs the specified packages using brew
func (a *Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	caskOf, err := a.caskNames(pkgs)
	if err != nil {
		return manager.ErrorResult(err.Error()), nil
	}
	formulas, casks := splitBrewPackageNames(pkgs, caskOf)

	if len(formulas) > 0 {
		args := append([]string{"install"}, formulas...)
		if result, err := a.runAction(args, "Installed Homebrew formula(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	if len(casks) > 0 {
		args := append([]string{"install", "--cask"}, casks...)
		if result, err := a.runAction(args, "Installed Homebrew cask(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Installed Homebrew package(s)"), nil
}

// Uninstall uninstalls the specified packages using brew
func (a *Adapter) Uninstall(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	caskOf, err := a.caskNames(pkgs)
	if err != nil {
		return manager.ErrorResult(err.Error()), nil
	}
	formulas, casks := splitBrewPackageNames(pkgs, caskOf)

	if len(formulas) > 0 {
		args := append([]string{"uninstall"}, formulas...)
		if result, err := a.runAction(args, "Uninstalled Homebrew formula(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	if len(casks) > 0 {
		args := append([]string{"uninstall", "--cask"}, casks...)
		if result, err := a.runAction(args, "Uninstalled Homebrew cask(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Uninstalled Homebrew package(s)"), nil
}

// Update updates the specified packages using brew
func (a *Adapter) Update(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	caskOf, err := a.caskNames(pkgs)
	if err != nil {
		return manager.ErrorResult(err.Error()), nil
	}
	formulas, casks := splitBrewPackageNames(pkgs, caskOf)

	if len(formulas) > 0 {
		args := append([]string{"upgrade"}, formulas...)
		if result, err := a.runAction(args, "Updated Homebrew formula(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	if len(casks) > 0 {
		args := append([]string{"upgrade", "--cask"}, casks...)
		if result, err := a.runAction(args, "Updated Homebrew cask(s)"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Updated Homebrew package(s)"), nil
}

// ListVersions is not offered, and the capability says so.
//
// `brew info --json=v2` reports one stable version and a head, never the
// versions a formula has published before: those live in the tap's git history,
// and installing one means fetching a tarball by URL or finding a versioned
// formula that is a package of its own (`python@3.12`). A choice of one is not a
// choice, so the interface has no version picker for Homebrew.
func (a *Adapter) ListVersions(pkg manager.Package) ([]string, error) {
	return nil, fmt.Errorf("list versions not available for brew")
}

func (a *Adapter) runAction(args []string, successMsg string) (manager.ActionResult, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, args, []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_COLOR=1"})
	if err != nil {
		msg := util.StripANSI(res.Output())
		if msg == "" {
			msg = err.Error()
		}
		return manager.ErrorResult(msg), nil
	}
	return manager.SuccessResult(successMsg), nil
}

// splitBrewPackageNames divides the packages by the command that installs each:
// casks take --cask, formulae do not.
func splitBrewPackageNames(pkgs []manager.Package, casks map[string]bool) ([]string, []string) {
	formulas := make([]string, 0, len(pkgs))
	caskNames := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		if casks[pkg.Name] {
			caskNames = append(caskNames, pkg.Name)
		} else {
			formulas = append(formulas, pkg.Name)
		}
	}
	return formulas, caskNames
}

// caskNames says which of the packages are casks.
//
// Nothing in a package carries that: a cask and a formula are the same kind of
// row here, and only Homebrew knows which a name is. The installed ones come
// from the cask list, which reads the Caskroom and so needs no tap trust; a
// package not installed yet, as one picked from search, is asked about.
func (a *Adapter) caskNames(pkgs []manager.Package) (map[string]bool, error) {
	// These calls work out what to run; they are not what the user asked for, so
	// they stay out of the task's console.
	quiet := *a
	quiet.ctx = util.WithoutReporting(a.ctx)
	casks := make(map[string]bool, len(pkgs))
	if installed, err := quiet.listCasks(); err == nil {
		for _, pkg := range installed {
			casks[pkg.Name] = true
		}
	} else {
		slog.DebugContext(a.ctx, "brew: cannot read the installed casks", "error", err)
	}
	// An installed package that is not a cask is a formula; only the ones being
	// installed need asking about.
	rest := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		if !pkg.Installed && !casks[pkg.Name] {
			rest = append(rest, pkg.Name)
		}
	}
	if len(rest) == 0 {
		return casks, nil
	}
	args := append([]string{"info", "--json=v2"}, rest...)
	output, err := util.Exec(quiet.ctx, a.info.ExecPath, args, []string{"HOMEBREW_NO_AUTO_UPDATE=1"})
	if err != nil {
		// A name Homebrew refuses to load, as a cask of an untrusted tap, goes to
		// the formula command, and brew says what is wrong with it.
		slog.DebugContext(a.ctx, "brew: cannot read the package types", "error", err)
		return casks, nil
	}
	var list caskList
	if err := json.Unmarshal([]byte(output), &list); err != nil {
		slog.DebugContext(a.ctx, "brew: cannot parse the package types", "error", err)
		return casks, nil
	}
	for _, c := range list.Casks {
		casks[c.Token] = true
	}
	return casks, nil
}
