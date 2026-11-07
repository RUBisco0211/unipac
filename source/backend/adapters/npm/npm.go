package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

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
			ID:       "npm",
			Name:     "NPM",
			Website:  "https://www.npmjs.com",
			ExecName: "npm",
			Capabilities: *manager.
				DefaultCapabilities().
				WithListOutdated(true).
				WithSearch(true).
				WithListVersions(true),
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

func (a *Adapter) ListInstalled() ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"ls", "-g", "--depth=0", "--json"}, nil)
	if err != nil {
		return nil, err
	}

	var obj struct {
		Dependencies map[string]struct {
			Version    string `json:"version"`
			Overridden bool   `json:"overridden"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse npm installed output: %w", err)
	}

	pkgs := make([]manager.Package, 0, len(obj.Dependencies))
	for name, dep := range obj.Dependencies {
		if dep.Version == "" {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:      name,
			Version:   dep.Version,
			Manager:   a.Info().ID,
			Installed: true,
		})
	}
	return pkgs, nil
}

func (a *Adapter) ListOutdated() ([]manager.Package, error) {
	res, err := util.RunAllowExitCodes(a.ctx, a.info.ExecPath, []string{"outdated", "-g", "--json"}, nil, 0, 1)
	if err != nil {
		return nil, err
	}
	if res.Stdout == "" {
		return []manager.Package{}, nil
	}

	var obj map[string]struct {
		Current string `json:"current"`
		Latest  string `json:"latest"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse npm outdated output: %w", err)
	}

	pkgs := make([]manager.Package, 0, len(obj))
	for name, item := range obj {
		pkgs = append(pkgs, manager.Package{
			Name:          name,
			Version:       item.Current,
			LatestVersion: item.Latest,
			Manager:       a.Info().ID,
			Installed:     true,
			Outdated:      true,
		})
	}
	return pkgs, nil
}

func (a *Adapter) Search(keyword string) ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath,
		[]string{"search", keyword, "--json", "--searchlimit", strconv.Itoa(manager.MaxSearchResults)},
		nil)
	if err != nil {
		return nil, err
	}
	if res.Stdout == "" {
		return []manager.Package{}, nil
	}

	var items []struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &items); err != nil {
		return nil, fmt.Errorf("failed to parse npm search output: %w", err)
	}

	pkgs := make([]manager.Package, 0, len(items))
	for _, item := range items {
		pkgs = append(pkgs, manager.Package{
			Name:        item.Name,
			Version:     item.Version,
			Manager:     a.Info().ID,
			Installed:   false,
			Description: item.Description,
		})
	}
	return pkgs, nil
}

func (a *Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	args := []string{"install", "-g"}
	args = append(args, npmPackageSpecs(pkgs, opt)...)
	return a.runAction(args, "Installed npm package(s)")
}

func (a *Adapter) Uninstall(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"uninstall", "-g"}, packageNames(pkgs)...)
	return a.runAction(args, "Uninstalled npm package(s)")
}

// Update installs the newest version of each package.
//
// It installs rather than running `npm update`: update resolves the whole global
// tree, so one unrelated package whose manifest carries a workspaces-only spec
// makes every update fail with EUNSUPPORTEDPROTOCOL, whatever the package asked
// for. Installing a named package resolves that package alone.
func (a *Adapter) Update(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"install", "-g"}, updateSpecs(pkgs, opt)...)
	return a.runAction(args, "Updated npm package(s)")
}

// updateSpecs name the version each update goes to: the one the caller asked
// for, else the one the scan found, else whatever latest is when it runs.
func updateSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
	specs := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		version := opt.Version
		if version == "" {
			version = pkg.LatestVersion
		}
		if version == "" {
			specs = append(specs, pkg.Name+"@latest")
			continue
		}
		specs = append(specs, pkg.Name+"@"+version)
	}
	return specs
}

func (a *Adapter) ListVersions(pkg manager.Package) ([]string, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"view", pkg.Name, "versions", "--json"}, nil)
	if err != nil {
		return nil, err
	}
	return parseVersions([]byte(res.Stdout))
}

// parseVersions reads what `npm view <name> versions --json` prints: the array of
// every published version, or the single version of a package published once. It
// keeps npm's order, which is oldest first; the picker that shows the list sorts
// it the way it shows it.
func parseVersions(output []byte) ([]string, error) {
	var versions []string
	if err := json.Unmarshal(output, &versions); err != nil {
		var single string
		if singleErr := json.Unmarshal(output, &single); singleErr != nil {
			return nil, fmt.Errorf("failed to parse npm versions output: %w", err)
		}
		versions = []string{single}
	}
	return versions, nil
}

func (a *Adapter) runAction(args []string, successMsg string) (manager.ActionResult, error) {
	// npm turns color off for a pipe; the console can render it, so ask for it.
	res, err := util.Run(a.ctx, a.info.ExecPath, args, []string{"FORCE_COLOR=1"})
	if err != nil {
		msg := util.StripANSI(res.Output())
		if msg == "" {
			msg = err.Error()
		}
		return manager.ErrorResult(msg), nil
	}
	return manager.SuccessResult(successMsg), nil
}

func npmPackageSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
	specs := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		if opt.Version != "" {
			specs = append(specs, pkg.Name+"@"+opt.Version)
		} else if pkg.Version != "" && !pkg.Installed {
			specs = append(specs, pkg.Name+"@"+pkg.Version)
		} else {
			specs = append(specs, pkg.Name)
		}
	}
	return specs
}

func packageNames(pkgs []manager.Package) []string {
	names := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}
	return names
}
