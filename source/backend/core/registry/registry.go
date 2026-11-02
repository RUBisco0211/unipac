package registry

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/RUBisco0211/unipac/source/backend/adapters"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/samber/lo"
)

type Registry struct {
	ctx context.Context
	// mu guards the adapter map and the Info values it holds. Published
	// adapters are never changed: a settings change builds new ones, so a scan
	// that is already running keeps reading the objects it started with.
	mu           sync.RWMutex
	adapters     map[string]manager.Adapter
	constructors map[string]manager.AdapterConstructor
	// settings is what each adapter was last configured with, so Apply can tell
	// a changed executable from one that only needs its switch flipped.
	settings map[string]manager.Settings
	// write serializes rebuilds, so two settings saves cannot interleave.
	write sync.Mutex
}

var Instance *Registry

// Init builds the registry from the adapter constructors and the user's saved
// settings, preflighting every manager.
func Init(ctx context.Context, settings map[string]manager.Settings) {
	Instance = newRegistry(ctx, settings)
}

func newRegistry(ctx context.Context, settings map[string]manager.Settings) *Registry {
	reg := &Registry{
		ctx:          ctx,
		adapters:     make(map[string]manager.Adapter, 0),
		constructors: make(map[string]manager.AdapterConstructor, 0),
		settings:     make(map[string]manager.Settings, len(settings)),
	}

	// Every constructor is kept by the ID of the adapter it builds, so a later
	// settings change can replace one adapter without rebuilding the others.
	for _, constructor := range adapters.GetAdapterConstructors() {
		reg.constructors[constructor(ctx).Info().ID] = constructor
	}
	reg.rebuild(settings, true)

	return reg
}

// Apply installs saved settings. A manager whose executable did not change is
// not preflighted again, so turning one off costs no subprocesses; a new path
// is checked before it is used.
func (reg *Registry) Apply(settings map[string]manager.Settings) {
	reg.rebuild(settings, false)
}

// Reprobe preflights every manager again, so a tool installed while the app was
// open is picked up.
func (reg *Registry) Reprobe() {
	reg.mu.RLock()
	settings := maps.Clone(reg.settings)
	reg.mu.RUnlock()
	reg.rebuild(settings, true)
}

// rebuild publishes a fresh set of adapters for settings. probeAll re-runs
// Preflight for every manager; otherwise one whose executable did not change
// keeps the path and availability it had.
//
// Building new adapters rather than editing the published ones is what makes
// the concurrent scan safe: the map and the Info values are only read after
// publication, and they are replaced under the write lock.
func (reg *Registry) rebuild(settings map[string]manager.Settings, probeAll bool) {
	reg.write.Lock()
	defer reg.write.Unlock()

	reg.mu.RLock()
	previous := reg.adapters
	previousSettings := reg.settings
	constructors := reg.constructors
	reg.mu.RUnlock()

	next := make(map[string]manager.Adapter, len(constructors))
	nextSettings := make(map[string]manager.Settings, len(constructors))
	for id, constructor := range constructors {
		nextSettings[id] = settings[id]

		adp := constructor(reg.ctx)
		info := adp.Info()
		info.ExecPath = settings[id].ExecPath

		reused := false
		if !probeAll {
			if old, ok := previous[id]; ok && settings[id].ExecPath == previousSettings[id].ExecPath {
				info.ExecPath = old.Info().ExecPath
				info.Available = old.Info().Available
				reused = true
			}
		}
		if !reused {
			if err := adp.Preflight(); err != nil {
				slog.ErrorContext(
					reg.ctx,
					fmt.Sprintf("Manager '%s' is not available: %v", info.Name, err),
				)
				info.Available = false
			} else {
				slog.InfoContext(
					reg.ctx,
					fmt.Sprintf("Manager '%s' is available and registered", info.Name),
				)
				info.Available = true
			}
		}
		info.Enabled = info.Available && !settings[id].Disabled
		next[id] = adp
	}

	reg.mu.Lock()
	reg.adapters = next
	reg.settings = nextSettings
	reg.mu.Unlock()
}

// ListManagers returns every registered manager in a stable alphabetical
// order. The adapters live in a map, whose iteration order changes between
// calls; sorting here keeps the settings page and the manager filter from
// reshuffling on every rebuild.
func (reg *Registry) ListManagers() []manager.Info {
	reg.mu.RLock()
	managers := lo.Map(lo.Values(reg.adapters), func(adp manager.Adapter, _ int) manager.Info {
		return *adp.Info()
	})
	reg.mu.RUnlock()

	slices.SortStableFunc(managers, func(a, b manager.Info) int {
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return managers
}

// snapshot returns the published adapters, so a caller can work through them
// without holding the lock while a command runs.
func (reg *Registry) snapshot() []manager.Adapter {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	list := make([]manager.Adapter, 0, len(reg.adapters))
	for _, adp := range reg.adapters {
		list = append(list, adp)
	}
	return list
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

// GetInstalledPackages returns the list of all installed packages across the
// enabled adapters. reliable is false when enabled managers were asked and all
// of them failed, so a caller replacing a snapshot can keep the last good one
// instead of storing an empty picture.
func (reg *Registry) GetInstalledPackages() (pkgs []manager.Package, reliable bool, err error) {
	list := reg.snapshot()
	collectTasks := make([]util.Collector[manager.Package], 0, len(list))
	var succeeded atomic.Int32
	attempted := 0
	for _, adp := range list {
		info := adp.Info()
		if !info.Capabilities.ListInstalled || !info.Enabled {
			continue
		}
		adp := adp
		attempted++
		collectTasks = append(collectTasks, func() ([]manager.Package, error) {
			pkgs, err := adp.ListInstalled()
			if err != nil {
				slog.ErrorContext(reg.ctx, "Failed to list installed packages", "manager", adp.Info().Name, "error", err)
				return pkgs, nil
			}
			succeeded.Add(1)
			return pkgs, nil
		})
	}
	pkgs, err = util.CollectParallel(reg.ctx, collectTasks...)
	if err != nil {
		return pkgs, false, err
	}
	// No enabled manager means there is nothing to fail: the empty result is
	// the truth, and the cache should follow it.
	reliable = attempted == 0 || succeeded.Load() > 0
	return reg.mergeOutdatedPackages(pkgs, list), reliable, nil
}

func (reg *Registry) mergeOutdatedPackages(pkgs []manager.Package, list []manager.Adapter) []manager.Package {
	index := make(map[string]int, len(pkgs))
	for i, pkg := range pkgs {
		index[packageKey(pkg.Manager, pkg.Name)] = i
	}

	for _, adp := range list {
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

	list := reg.snapshot()
	collectTasks := make([]util.Collector[manager.Package], 0, len(list))
	for _, adp := range list {
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
