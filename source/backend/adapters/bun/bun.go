// Package bun manages the packages a user installed with bun's global install
// (`bun add -g`). It reads them from `bun pm ls -g`, marks the outdated ones from
// `bun outdated -g`, and changes them with `bun add`/`update`/`remove` `-g`.
//
// Three things bun cannot do decide the capabilities it advertises: it has no
// search command, and its packages are npm's, so a second search would show every
// result twice; and `bun info`, which would answer a package's details and its
// versions, needs a package.json in the working directory, which the app does not
// have. `.agents/PACKAGE_MANAGER_ADAPTERS.md` records what would turn them on.
package bun

import (
	"context"
	"fmt"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

const managerID = "bun"

// missingLockfile is what bun says when it has never installed a package
// globally: `bun pm ls -g` and `bun outdated -g` both fail with exit code 1 and a
// line carrying this. It is an empty inventory, not a failure.
const missingLockfile = "missing lockfile"

// colorEnv asks bun for the colors the task console renders. It is set for the
// commands whose output the user reads, and never for the ones the adapter
// parses.
var colorEnv = []string{"FORCE_COLOR=1"}

type Adapter struct {
	ctx  context.Context
	info *manager.Info
}

func NewAdapter(ctx context.Context) manager.Adapter {
	return &Adapter{
		ctx: ctx,
		info: &manager.Info{
			ID:       managerID,
			Name:     "Bun",
			Website:  "https://bun.sh",
			ExecName: "bun",
			Capabilities: *manager.
				DefaultCapabilities().
				WithListOutdated(true).
				WithSearch(false).
				// `bun info` answers both of these only from inside a project,
				// which the app is not; see the package comment.
				WithGetPackageInfo(false).
				WithListVersions(false),
		},
	}
}

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

func (a *Adapter) Preflight() error {
	return manager.Preflight(a, a.ctx)
}

func (a *Adapter) Setup() error {
	return nil
}

// GetPackageInfo is not offered: it needs a package.json in the working
// directory.
func (a *Adapter) GetPackageInfo(manager.Package) (string, error) {
	return "", fmt.Errorf("package info not available for bun")
}

// ListInstalled reads the global dependency tree.
func (a *Adapter) ListInstalled() ([]manager.Package, error) {
	out, none, err := a.listGlobals([]string{"pm", "ls", "-g"})
	switch {
	case err != nil:
		return nil, err
	case none:
		return []manager.Package{}, nil
	}
	return parseGlobalTree(a.Info().ID, out), nil
}

// ListOutdated reads bun's table of outdated global packages.
func (a *Adapter) ListOutdated() ([]manager.Package, error) {
	out, none, err := a.listGlobals([]string{"outdated", "-g"})
	switch {
	case err != nil:
		return nil, err
	case none:
		return []manager.Package{}, nil
	}
	return parseOutdatedTable(a.Info().ID, out)
}

// listGlobals runs one of bun's global listings and tells "nothing installed"
// from a real failure.
func (a *Adapter) listGlobals(args []string) (out string, none bool, err error) {
	res, err := util.RunAllowExitCodes(a.ctx, a.info.ExecPath, args, nil, 0, 1)
	if err != nil {
		return "", false, err
	}
	none, err = globalsState(res)
	if none || err != nil {
		return "", none, err
	}
	return res.Stdout, false, nil
}

// globalsState reads a global listing's exit status. Both listings look alike
// when bun has never installed a package globally: exit code 1 and a message
// carrying "missing lockfile". Each words the rest of it its own way — "nothing
// to list" for the tree, "nothing outdated" for the table — so only the shared
// phrase is matched, and only on a non-zero exit.
func globalsState(res util.ExecResult) (none bool, err error) {
	if res.ExitCode == 0 {
		return false, nil
	}
	if strings.Contains(util.StripANSI(res.Output()), missingLockfile) {
		return true, nil
	}
	message := strings.TrimSpace(res.Output())
	if message == "" {
		message = fmt.Sprintf("bun exited with status %d", res.ExitCode)
	}
	return false, fmt.Errorf("bun: %s", message)
}

// Search is not offered: bun has no search command, and its packages are npm's,
// so searching twice would list every package twice.
func (a *Adapter) Search(string) ([]manager.Package, error) {
	return nil, fmt.Errorf("search not available for bun")
}

func (a *Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"add", "-g"}, packageSpecs(pkgs, opt)...)
	return a.runAction(args, "Installed bun package(s)")
}

func (a *Adapter) Uninstall(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"remove", "-g"}, packageNames(pkgs)...)
	return a.runAction(args, "Uninstalled bun package(s)")
}

// Update moves each package to the version it should go to.
//
// The version is always named: `bun update -g name` alone resolves the range the
// global package.json holds, and a global install pins an exact version, so it
// answers "no changes" and leaves the package where it is.
func (a *Adapter) Update(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"update", "-g"}, updateSpecs(pkgs, opt)...)
	return a.runAction(args, "Updated bun package(s)")
}

// ListVersions is not offered: it needs a project directory for `bun info`.
func (a *Adapter) ListVersions(manager.Package) ([]string, error) {
	return nil, fmt.Errorf("list versions not available for bun")
}

// runAction runs one global command and reports what bun said when it fails.
func (a *Adapter) runAction(args []string, successMsg string) (manager.ActionResult, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, args, colorEnv)
	if err != nil {
		msg := strings.TrimSpace(util.StripANSI(res.Output()))
		if msg == "" {
			msg = err.Error()
		}
		return manager.ErrorResult(msg), nil
	}
	return manager.SuccessResult(successMsg), nil
}

// packageSpecs names what an install asks for: the version the caller chose,
// else the one the row was found at, else whatever latest is when it runs.
//
// Naming the version matters: `bun add -g name` writes a caret range into the
// global package.json, where `name@version` writes the exact version the user
// asked for.
func packageSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
	specs := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		version := opt.Version
		if version == "" && !pkg.Installed {
			version = pkg.Version
		}
		specs = append(specs, packageSpec(pkg.Name, version))
	}
	return specs
}

// updateSpecs names what an update goes to: the version the caller chose, else
// the one the scan found, else latest.
func updateSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
	specs := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		version := opt.Version
		if version == "" {
			version = pkg.LatestVersion
		}
		if version == "" {
			version = "latest"
		}
		specs = append(specs, packageSpec(pkg.Name, version))
	}
	return specs
}

// packageSpec joins a name and a version; a dist-tag is spelled the same way as
// a version, `name@latest`.
func packageSpec(name, version string) string {
	if version == "" {
		return name
	}
	return name + "@" + version
}

func packageNames(pkgs []manager.Package) []string {
	names := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}
	return names
}

// parseGlobalTree reads `bun pm ls -g`:
//
//	/Users/me/.bun/install/global node_modules (61 installed)
//	├── @quasar/cli@5.0.9
//	└── is-odd@3.0.1
//
// The first line names the directory bun treats as global, and its count covers
// the transitive tree, so neither is a package; the adapter never builds that
// path itself, because bun does not always use the same one. Without --all the
// remaining lines are the top-level dependencies, which is the global package
// set.
func parseGlobalTree(managerID, out string) []manager.Package {
	lines := strings.Split(util.StripANSI(out), "\n")
	pkgs := make([]manager.Package, 0, len(lines))
	for i, line := range lines {
		if i == 0 {
			continue // the directory bun reports
		}
		name, version, ok := parseTreeLine(line)
		if !ok {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:      name,
			Version:   version,
			Manager:   managerID,
			Installed: true,
		})
	}
	return pkgs
}

// parseTreeLine reads one top-level line of the tree, `├── is-odd@3.0.1` or
// `└── @quasar/cli@5.0.9`, into a name and a version.
//
// Only entries drawn at the left edge count. A nested one is indented, under a
// `│` or under spaces, and is a dependency rather than a global package; the tree
// the adapter asks for has no nesting, and if bun ever prints some, reporting a
// dependency as a global package would offer an update for something the user
// never installed.
func parseTreeLine(line string) (name, version string, ok bool) {
	line = strings.TrimRight(line, " \t\r")
	switch {
	case strings.HasPrefix(line, "├── "):
	case strings.HasPrefix(line, "└── "):
	default:
		return "", "", false
	}
	line = strings.TrimSpace(line[len("├── "):])
	if line == "" {
		return "", "", false
	}
	// The last '@' separates the version: a name cannot contain one, and a
	// scoped name starts with one.
	at := strings.LastIndexByte(line, '@')
	if at <= 0 || at == len(line)-1 {
		return "", "", false
	}
	return line[:at], line[at+1:], true
}

// parseOutdatedTable reads `bun outdated -g`:
//
//	bun outdated v1.4.2 (744846f84)
//	|--------------------------------------|
//	| Package  | Current | Update | Latest |
//	|----------|---------|--------|--------|
//	| is-odd   | 2.0.0   | 2.0.0  | 3.0.1  |
//	|--------------------------------------|
//
// A row of dashes comes before and after the header and between entries, so a
// row whose cells are dashes is skipped. `Latest` is the newest version overall,
// which is the newer version the inventory shows; `Update` is the newest within
// the range the global package.json declares, and a global install pins an exact
// version, so it never differs from `Current`.
func parseOutdatedTable(managerID, out string) ([]manager.Package, error) {
	lines := strings.Split(util.StripANSI(out), "\n")
	pkgs := make([]manager.Package, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := tableCells(line)
		if isRule(cells) || cells[0] == "Package" {
			continue
		}
		if len(cells) < 4 {
			return nil, fmt.Errorf("failed to parse bun outdated row %q", line)
		}
		name, current, latest := cells[0], cells[1], cells[3]
		if name == "" && current == "" && latest == "" {
			continue
		}
		if name == "" || current == "" || latest == "" {
			return nil, fmt.Errorf("failed to parse bun outdated row %q", line)
		}
		pkgs = append(pkgs, manager.Package{
			Name:          name,
			Version:       current,
			LatestVersion: latest,
			Manager:       managerID,
			Installed:     true,
			Outdated:      current != latest,
		})
	}
	return pkgs, nil
}

// tableCells splits one `| a | b |` row into its trimmed cells.
func tableCells(line string) []string {
	fields := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, 0, len(fields))
	for _, field := range fields {
		cells = append(cells, strings.TrimSpace(field))
	}
	return cells
}

// isRule reports whether every cell of a row is a run of dashes, which is how bun
// draws the lines around its table.
func isRule(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}
