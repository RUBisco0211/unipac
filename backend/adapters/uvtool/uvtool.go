package uvtool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/util"
)

const installConcurrency = 3

type Adapter struct {
	ctx  context.Context
	info *manager.Info
}

func NewAdapter(ctx context.Context) manager.Adapter {
	return &Adapter{
		ctx: ctx,
		info: &manager.Info{
			ID:       managerID,
			Name:     "uv tool",
			ExecName: "uv",
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
	records, err := a.listToolRecords()
	if err != nil {
		return "", err
	}
	record, ok := findTool(records, pkg.Name)
	if !ok {
		return "", fmt.Errorf("uv tool package not found: %s", pkg.Name)
	}
	bytes, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("failed to marshal uv tool package info: %w", err)
	}
	return string(bytes), nil
}

func (a *Adapter) ListInstalled() ([]manager.Package, error) {
	records, err := a.listToolRecords()
	if err != nil {
		return nil, err
	}
	return normalizeInstalled(records), nil
}

func (a *Adapter) ListOutdated() ([]manager.Package, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"tool", "list", "--outdated"}, uvToolEnv())
	if err != nil {
		return nil, err
	}
	return normalizeOutdated(parseOutdatedTools(res.Stdout)), nil
}

func (a *Adapter) Search(keyword string) ([]manager.Package, error) {
	return nil, fmt.Errorf("search not available for uv tool")
}

func (a *Adapter) Install(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	specs := uvToolPackageSpecs(pkgs, opt)
	if len(specs) == 0 {
		return manager.SuccessResult("No uv tool packages to install"), nil
	}
	return a.runInstallActions(specs)
}

func (a *Adapter) Uninstall(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	names := uvToolPackageNames(pkgs)
	if len(names) == 0 {
		return manager.SuccessResult("No uv tool packages to uninstall"), nil
	}
	args := append([]string{"tool", "uninstall"}, names...)
	return a.runAction(args, "Uninstalled uv tool package(s)")
}

func (a *Adapter) Update(pkgs []manager.Package, _ manager.ActionOptions) (manager.ActionResult, error) {
	names := uvToolPackageNames(pkgs)
	if len(names) == 0 {
		return manager.SuccessResult("No uv tool packages to update"), nil
	}
	args := append([]string{"tool", "upgrade"}, names...)
	return a.runAction(args, "Updated uv tool package(s)")
}

func (a *Adapter) ListVersions(pkg manager.Package) ([]string, error) {
	return nil, fmt.Errorf("list versions not available for uv tool")
}

func (a *Adapter) listToolRecords() ([]toolRecord, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, []string{"tool", "list", "--show-version-specifiers", "--show-paths", "--show-python"}, uvToolEnv())
	if err != nil {
		return nil, err
	}
	return parseInstalledTools(res.Stdout), nil
}

func (a *Adapter) runAction(args []string, successMsg string) (manager.ActionResult, error) {
	res, err := util.Run(a.ctx, a.info.ExecPath, args, uvToolEnv())
	if err != nil {
		msg := res.Output()
		if msg == "" {
			msg = err.Error()
		}
		return manager.ErrorResult(msg), nil
	}
	return manager.SuccessResult(successMsg), nil
}

func (a *Adapter) runInstallActions(specs []string) (manager.ActionResult, error) {
	type failure struct {
		spec string
		msg  string
	}

	jobs := make(chan string)
	failures := make([]failure, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	workers := installConcurrency
	if len(specs) < workers {
		workers = len(specs)
	}

	for range workers {
		wg.Go(func() {
			for spec := range jobs {
				res, err := util.Run(a.ctx, a.info.ExecPath, []string{"tool", "install", spec}, uvToolEnv())
				if err == nil {
					continue
				}
				msg := res.Output()
				if msg == "" {
					msg = err.Error()
				}
				mu.Lock()
				failures = append(failures, failure{spec: spec, msg: msg})
				mu.Unlock()
			}
		})
	}

	for _, spec := range specs {
		select {
		case <-a.ctx.Done():
			close(jobs)
			wg.Wait()
			return manager.ErrorResult(a.ctx.Err().Error()), nil
		case jobs <- spec:
		}
	}
	close(jobs)
	wg.Wait()

	if len(failures) > 0 {
		messages := make([]string, 0, len(failures))
		for _, item := range failures {
			messages = append(messages, item.spec+": "+item.msg)
		}
		return manager.ErrorResult(strings.Join(messages, "\n")), nil
	}

	return manager.SuccessResult("Installed uv tool package(s)"), nil
}

func uvToolPackageSpecs(pkgs []manager.Package, opt manager.ActionOptions) []string {
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

func uvToolPackageNames(pkgs []manager.Package) []string {
	names := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}
	return names
}

func uvToolEnv() []string {
	return []string{"UV_NO_PROGRESS=1"}
}
