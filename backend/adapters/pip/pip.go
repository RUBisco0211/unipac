package pip

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
			ID:       "pip",
			Name:     "Pip",
			ExecName: "pip3",
			Capabilities: *manager.
				DefaultCapabilities().
				WithListOutdated(true).
				WithSearch(false).
				WithListVersions(false),
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
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"show", pkg.Name}, nil)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}

func (a *Adapter) ListInstalled() ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"list", "--format=json"}, nil)
	if err != nil {
		return nil, err
	}

	var items []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &items); err != nil {
		return nil, fmt.Errorf("failed to parse pip installed output: %w", err)
	}

	pkgs := make([]manager.Package, 0, len(items))
	for _, item := range items {
		pkgs = append(pkgs, manager.Package{
			Name:      item.Name,
			Version:   item.Version,
			Manager:   a.Info().ID,
			Installed: true,
			IsGUI:     false,
		})
	}
	return pkgs, nil
}

func (a *Adapter) ListOutdated() ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"list", "--outdated", "--format=json"}, nil)
	if err != nil {
		return nil, err
	}

	var items []struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		LatestVersion string `json:"latest_version"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &items); err != nil {
		return nil, fmt.Errorf("failed to parse pip outdated output: %w", err)
	}

	pkgs := make([]manager.Package, 0, len(items))
	for _, item := range items {
		pkgs = append(pkgs, manager.Package{
			Name:          item.Name,
			Version:       item.Version,
			LatestVersion: item.LatestVersion,
			Manager:       a.Info().ID,
			Installed:     true,
			Outdated:      true,
			IsGUI:         false,
		})
	}
	return pkgs, nil
}

func (a *Adapter) Search(keyword string) ([]manager.Package, error) {
	return nil, fmt.Errorf("search not available for pip")
}

func (a *Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"install"}, pipPackageSpecs(pkgs, opt)...)
	return a.runAction(args, "Installed pip package(s)")
}

func (a *Adapter) Uninstall(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"uninstall", "-y"}, pipPackageNames(pkgs)...)
	return a.runAction(args, "Uninstalled pip package(s)")
}

func (a *Adapter) Update(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	args := append([]string{"install", "--upgrade"}, pipPackageNames(pkgs)...)
	return a.runAction(args, "Updated pip package(s)")
}

func (a *Adapter) ListVersions(pkg manager.Package) ([]string, error) {
	return nil, fmt.Errorf("list versions not available for pip")
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

func pipPackageSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
	specs := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		if opt.Version != "" {
			specs = append(specs, pkg.Name+"=="+opt.Version)
		} else if pkg.Version != "" && !pkg.Installed {
			specs = append(specs, pkg.Name+"=="+pkg.Version)
		} else {
			specs = append(specs, pkg.Name)
		}
	}
	return specs
}

func pipPackageNames(pkgs []manager.Package) []string {
	names := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}
	return names
}
