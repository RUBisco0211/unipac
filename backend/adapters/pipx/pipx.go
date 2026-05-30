package pipx

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
			ID:       "pipx",
			Name:     "Pipx",
			ExecName: "pipx",
			Capabilities: *manager.
				DefaultCapabilities().
				WithListOutdated(false).
				WithSearch(false).
				WithListVersions(false),
		},
	}
}

func (a Adapter) Info() *manager.Info {
	return a.info
}

func (a Adapter) Preflight() error {
	return manager.Preflight(a, a.ctx)
}

func (a Adapter) Setup() error {
	return nil
}

func (a Adapter) GetPackageInfo(pkg manager.Package) (string, error) {
	venvs, err := a.listVenvs()
	if err != nil {
		return "", err
	}
	venv, ok := venvs[pkg.Name]
	if !ok {
		for _, item := range venvs {
			if item.Metadata.MainPackage.Package == pkg.Name {
				venv = item
				ok = true
				break
			}
		}
	}
	if !ok {
		return "", fmt.Errorf("pipx package not found: %s", pkg.Name)
	}
	bytes, err := json.Marshal(venv)
	if err != nil {
		return "", fmt.Errorf("failed to marshal pipx package info: %w", err)
	}
	return string(bytes), nil
}

func (a Adapter) ListInstalled() ([]manager.Package, error) {
	venvs, err := a.listVenvs()
	if err != nil {
		return nil, err
	}

	pkgs := make([]manager.Package, 0, len(venvs))
	for venvName, venv := range venvs {
		name := venv.Metadata.MainPackage.Package
		if name == "" {
			name = venvName
		}
		pkgs = append(pkgs, manager.Package{
			Name:      name,
			Version:   venv.Metadata.MainPackage.PackageVersion,
			Manager:   a.Info().ID,
			Installed: true,
			IsGUI:     false,
		})
	}
	return pkgs, nil
}

func (a Adapter) ListOutdated() ([]manager.Package, error) {
	return nil, fmt.Errorf("list outdated not available for pipx")
}

func (a Adapter) Search(keyword string) ([]manager.Package, error) {
	return nil, fmt.Errorf("search not available for pipx")
}

func (a Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	for _, spec := range pipxPackageSpecs(pkgs, opt) {
		if result, err := a.runAction([]string{"install", spec}, "Installed pipx package"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Installed pipx package(s)"), nil
}

func (a Adapter) Uninstall(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	for _, name := range pipxPackageNames(pkgs) {
		if result, err := a.runAction([]string{"uninstall", name}, "Uninstalled pipx package"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Uninstalled pipx package(s)"), nil
}

func (a Adapter) Update(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	for _, name := range pipxPackageNames(pkgs) {
		if result, err := a.runAction([]string{"upgrade", name}, "Updated pipx package"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Updated pipx package(s)"), nil
}

func (a Adapter) ListVersions(pkg manager.Package) ([]string, error) {
	return nil, fmt.Errorf("list versions not available for pipx")
}

type pipxVenv struct {
	Metadata struct {
		MainPackage struct {
			Package        string `json:"package"`
			PackageVersion string `json:"package_version"`
		} `json:"main_package"`
	} `json:"metadata"`
}

func (a Adapter) listVenvs() (map[string]pipxVenv, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"list", "--json"}, nil)
	if err != nil {
		return nil, err
	}

	var obj struct {
		Venvs map[string]pipxVenv `json:"venvs"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse pipx list output: %w", err)
	}
	if obj.Venvs == nil {
		return map[string]pipxVenv{}, nil
	}
	return obj.Venvs, nil
}

func (a Adapter) runAction(args []string, successMsg string) (manager.ActionResult, error) {
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

func pipxPackageSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
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

func pipxPackageNames(pkgs []manager.Package) []string {
	names := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}
	return names
}
