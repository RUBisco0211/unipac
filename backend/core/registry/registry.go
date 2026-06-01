package registry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unipac-wails/backend/adapters"
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/util"

	"github.com/samber/lo"
)

type Registry struct {
	ctx      context.Context
	adapters map[string]manager.Adapter
	mu       sync.RWMutex
}

var Instance *Registry

func Init(ctx context.Context) {
	Instance = newRegistry(ctx)
}

func newRegistry(ctx context.Context) *Registry {
	reg := &Registry{
		adapters: make(map[string]manager.Adapter, 0),
		ctx:      ctx,
	}
	Instance = reg

	// get all implemented manager adapters' constructor
	constructors := adapters.GetAdapterConstructors()
	for _, constructor := range constructors {
		reg.register(constructor(ctx))
	}

	return reg
}

// add the adapter into registry even when the manager is not available
// but should mark it as not enabled
func (reg *Registry) register(adp manager.Adapter) {
	if err := adp.Preflight(); err != nil {
		slog.ErrorContext(
			reg.ctx,
			fmt.Sprintf("Manager '%s' is not available: %v", adp.Info().Name, err),
		)
		adp.Info().Enabled = false
	} else {
		slog.InfoContext(
			reg.ctx,
			fmt.Sprintf("Manager '%s' is available and registered", adp.Info().Name),
		)
		adp.Info().Enabled = true
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.adapters[adp.Info().ID] = adp
}

func (reg *Registry) ListManagers() []manager.Info {
	adapters := reg.adapterSnapshot()
	return lo.Map(adapters, func(adp manager.Adapter, _ int) manager.Info {
		return *adp.Info()
	})
}

func (reg *Registry) getAdapter(id string) (manager.Adapter, error) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	adp, ok := reg.adapters[id]
	if !ok {
		return nil, fmt.Errorf("manager not found: %s", id)
	}
	return adp, nil
}

func (reg *Registry) getEnabledAdapter(id string) (manager.Adapter, error) {
	adp, err := reg.getAdapter(id)
	if err != nil {
		return nil, err
	}
	if !adp.Info().Enabled {
		return nil, fmt.Errorf("manager is not enabled: %s", id)
	}
	return adp, nil
}

func (reg *Registry) requireCapability(id string, check func(manager.Capabilities) bool, capability string) (manager.Adapter, error) {
	adp, err := reg.getEnabledAdapter(id)
	if err != nil {
		return nil, err
	}
	if !check(adp.Info().Capabilities) {
		return nil, fmt.Errorf("manager %s does not support %s", id, capability)
	}
	return adp, nil
}

// GetInstalledPackages returns the list of all installed packages across all registered adapters
func (reg *Registry) GetInstalledPackages() ([]manager.Package, error) {
	adapters := reg.adapterSnapshot()
	collectTasks := make([]util.Collector[manager.Package], 0, len(adapters))
	for _, adp := range adapters {
		if !adp.Info().Capabilities.ListInstalled || !adp.Info().Enabled {
			continue
		}
		adp := adp
		collectTasks = append(collectTasks, func() ([]manager.Package, error) {
			pkgs, err := adp.ListInstalled()
			if err != nil {
				slog.ErrorContext(reg.ctx, "Failed to list installed packages", "manager", adp.Info().Name, "error", err)
			}
			return pkgs, nil
		})
	}
	pkgs, err := util.CollectParallel(reg.ctx, collectTasks...)
	if err != nil {
		return pkgs, err
	}
	return reg.mergeOutdatedPackages(pkgs), nil
}

func (reg *Registry) mergeOutdatedPackages(pkgs []manager.Package) []manager.Package {
	index := make(map[string]int, len(pkgs))
	for i, pkg := range pkgs {
		index[packageKey(pkg.Manager, pkg.Name)] = i
	}

	for _, adp := range reg.adapterSnapshot() {
		if !adp.Info().Enabled || !adp.Info().Capabilities.ListOutdated {
			continue
		}
		outdated, err := adp.ListOutdated()
		if err != nil {
			slog.ErrorContext(reg.ctx, "Failed to list outdated packages", "manager", adp.Info().Name, "error", err)
			continue
		}
		for _, item := range outdated {
			item.Manager = adp.Info().ID
			item.Installed = true
			item.Outdated = true
			key := packageKey(item.Manager, item.Name)
			if pos, ok := index[key]; ok {
				if item.LatestVersion != "" {
					pkgs[pos].LatestVersion = item.LatestVersion
				}
				if item.Version != "" {
					pkgs[pos].Version = item.Version
				}
				pkgs[pos].Outdated = true
			} else {
				index[key] = len(pkgs)
				pkgs = append(pkgs, item)
			}
		}
	}
	return pkgs
}

func (reg *Registry) SearchPackages(keyword string) ([]manager.Package, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []manager.Package{}, nil
	}

	adapters := reg.adapterSnapshot()
	collectTasks := make([]util.Collector[manager.Package], 0, len(adapters))
	for _, adp := range adapters {
		if !adp.Info().Enabled || !adp.Info().Capabilities.Search {
			continue
		}
		adp := adp
		collectTasks = append(collectTasks, func() ([]manager.Package, error) {
			pkgs, err := adp.Search(keyword)
			if err != nil {
				slog.ErrorContext(reg.ctx, "Failed to search packages", "manager", adp.Info().Name, "error", err)
				return nil, nil
			}
			return pkgs, nil
		})
	}
	return util.CollectParallel(reg.ctx, collectTasks...)
}

func (reg *Registry) GetPackageInfo(managerID string, pkg manager.Package) (string, error) {
	adp, err := reg.requireCapability(managerID, func(cap manager.Capabilities) bool {
		return cap.GetPackageInfo
	}, "package info")
	if err != nil {
		return "", err
	}
	pkg.Manager = managerID
	return adp.GetPackageInfo(pkg)
}

func (reg *Registry) ListPackageVersions(managerID string, pkg manager.Package) ([]string, error) {
	adp, err := reg.requireCapability(managerID, func(cap manager.Capabilities) bool {
		return cap.ListVersions
	}, "version lookup")
	if err != nil {
		return nil, err
	}
	pkg.Manager = managerID
	return adp.ListVersions(pkg)
}

func (reg *Registry) InstallPackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	adp, err := reg.requireCapability(managerID, func(cap manager.Capabilities) bool {
		return cap.Install
	}, "install")
	if err != nil {
		return manager.ErrorResult(err.Error()), nil
	}
	return adp.Install(normalizeTargets(managerID, pkgs), opt)
}

func (reg *Registry) UninstallPackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	adp, err := reg.requireCapability(managerID, func(cap manager.Capabilities) bool {
		return cap.Uninstall
	}, "uninstall")
	if err != nil {
		return manager.ErrorResult(err.Error()), nil
	}
	return adp.Uninstall(normalizeTargets(managerID, pkgs), opt)
}

func (reg *Registry) UpdatePackages(managerID string, pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	adp, err := reg.requireCapability(managerID, func(cap manager.Capabilities) bool {
		return cap.Update
	}, "update")
	if err != nil {
		return manager.ErrorResult(err.Error()), nil
	}
	return adp.Update(normalizeTargets(managerID, pkgs), opt)
}

func (reg *Registry) BatchUninstallPackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return reg.runGroupedAction(pkgs, opt, reg.UninstallPackages)
}

func (reg *Registry) BatchUpdatePackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return reg.runGroupedAction(pkgs, opt, reg.UpdatePackages)
}

type groupedAction func(managerID string, pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error)

func (reg *Registry) runGroupedAction(pkgs []manager.Package, opt manager.ActionOptions, action groupedAction) (manager.ActionResult, error) {
	groups := lo.GroupBy(pkgs, func(pkg manager.Package) string {
		return pkg.Manager
	})

	messages := make([]string, 0, len(groups))
	for managerID, group := range groups {
		result, err := action(managerID, group, opt)
		if err != nil {
			return result, err
		}
		if !result.Success {
			return result, nil
		}
		if result.Message != "" {
			messages = append(messages, result.Message)
		}
	}
	return manager.SuccessResult(strings.Join(messages, "\n")), nil
}

func normalizeTargets(managerID string, pkgs []manager.Package) []manager.Package {
	normalized := make([]manager.Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		pkg.Manager = managerID
		normalized = append(normalized, pkg)
	}
	return normalized
}

func packageKey(managerID string, name string) string {
	return managerID + ":" + name
}

func (reg *Registry) adapterSnapshot() []manager.Adapter {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return lo.Values(reg.adapters)
}
