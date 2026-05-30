package cargo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
			ID:       "cargo",
			Name:     "Cargo",
			ExecName: "cargo",
			Capabilities: *manager.
				DefaultCapabilities().
				WithSearch(true).
				WithGetPackageInfo(false).
				WithUpdate(false),
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
	return "", fmt.Errorf("get package info not available for cargo")
}

func (a Adapter) ListInstalled() ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"install", "--list"}, nil)
	if err != nil {
		return nil, err
	}
	return parseCargoInstalled(res.Stdout, a.Info().ID), nil
}

func (a Adapter) ListOutdated() ([]manager.Package, error) {
	return nil, fmt.Errorf("list outdated not available for cargo")
}

func (a Adapter) Search(keyword string) ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"search", keyword, "--limit", "20"}, nil)
	if err != nil {
		return nil, err
	}
	return parseCargoSearch(res.Stdout, a.Info().ID), nil
}

func (a Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	for _, args := range cargoInstallArgs(pkgs, opt) {
		if result, err := a.runAction(args, "Installed cargo package"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Installed cargo package(s)"), nil
}

func (a Adapter) Uninstall(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	for _, name := range cargoPackageNames(pkgs) {
		if result, err := a.runAction([]string{"uninstall", name}, "Uninstalled cargo package"); err != nil || !result.Success {
			return result, err
		}
	}
	return manager.SuccessResult("Uninstalled cargo package(s)"), nil
}

func (a Adapter) Update(names []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return manager.ErrorResult("Update not available for cargo"), nil
}

func (a Adapter) ListVersions(pkg manager.Package) ([]string, error) {
	return nil, fmt.Errorf("list versions not available for cargo")
}

var (
	cargoInstalledLine = regexp.MustCompile(`^(\S+)\s+v([^:]+):$`)
	cargoSearchLine    = regexp.MustCompile(`^([^=\s]+)\s*=\s*"([^"]+)"\s*(?:#\s*(.*))?$`)
)

func parseCargoInstalled(output string, managerID string) []manager.Package {
	lines := strings.Split(output, "\n")
	pkgs := make([]manager.Package, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := cargoInstalledLine.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:      matches[1],
			Version:   matches[2],
			Manager:   managerID,
			Installed: true,
			IsGUI:     false,
		})
	}
	return pkgs
}

func parseCargoSearch(output string, managerID string) []manager.Package {
	lines := strings.Split(output, "\n")
	pkgs := make([]manager.Package, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := cargoSearchLine.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		description := ""
		if len(matches) > 3 {
			description = matches[3]
		}
		pkgs = append(pkgs, manager.Package{
			Name:        matches[1],
			Version:     matches[2],
			Manager:     managerID,
			Installed:   false,
			IsGUI:       false,
			Description: description,
		})
	}
	return pkgs
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

func cargoInstallArgs(pkgs []manager.Package, opt manager.ActionOptions) [][]string {
	allArgs := make([][]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		args := []string{"install", pkg.Name}
		if opt.Version != "" {
			args = append(args, "--version", opt.Version)
		} else if pkg.Version != "" && !pkg.Installed {
			args = append(args, "--version", pkg.Version)
		}
		allArgs = append(allArgs, args)
	}
	return allArgs
}

func cargoPackageNames(pkgs []manager.Package) []string {
	names := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}
	return names
}
