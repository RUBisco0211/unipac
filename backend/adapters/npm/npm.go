package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/util"
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

func (a *Adapter) Preflight() error {
	return manager.Preflight(a, a.ctx)
}

func (a *Adapter) Setup() error {
	return nil
}

func (a *Adapter) GetPackageInfo(pkg manager.Package) (string, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"view", pkg.Name, "--json"}, nil)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
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
			IsGUI:     false,
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
			IsGUI:         false,
		})
	}
	return pkgs, nil
}

func (a *Adapter) Search(keyword string) ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"search", keyword, "--json"}, nil)
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

func (a *Adapter) Update(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"update", "-g"}, packageNames(pkgs)...)
	return a.runAction(args, "Updated npm package(s)")
}

func (a *Adapter) ListVersions(pkg manager.Package) ([]string, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"view", pkg.Name, "versions", "--json"}, nil)
	if err != nil {
		return nil, err
	}

	var versions []string
	if err := json.Unmarshal([]byte(res.Stdout), &versions); err != nil {
		var single string
		if singleErr := json.Unmarshal([]byte(res.Stdout), &single); singleErr != nil {
			return nil, fmt.Errorf("failed to parse npm versions output: %w", err)
		}
		versions = []string{single}
	}
	return versions, nil
}

func (a *Adapter) runAction(args []string, successMsg string) (manager.ActionResult, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, args, nil)
	if err != nil {
		msg := res.Output()
		if msg == "" {
			msg = err.Error()
		}
		return manager.ErrorResult(msg), nil
	}
	return manager.SuccessResult(successMsg), nil
}

func npmPackageSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
	version, _ := opt.Version, opt.Version != ""
	specs := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		if version != "" {
			specs = append(specs, pkg.Name+"@"+version)
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
