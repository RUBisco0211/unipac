// Package app holds UniPac's application service: the coarse operations the
// views call. It has no dependency on Wails or on any UI toolkit, so the Wails
// entry point and the MyGo entry point share the same implementation.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/RUBisco0211/unipac/backend/cache"
	"github.com/RUBisco0211/unipac/backend/config"
	"github.com/RUBisco0211/unipac/backend/core/manager"
	"github.com/RUBisco0211/unipac/backend/core/registry"
	"github.com/RUBisco0211/unipac/backend/logging"
	"github.com/RUBisco0211/unipac/backend/util"
	"github.com/samber/lo"
)

// App is the application service. Its exported methods are called directly by
// the views; there is no binding layer between them.
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{ctx: context.Background()}
}

// Startup prepares the app root, config, logging, cache and manager registry.
// It returns the first failure with the stage that produced it; entry points
// decide whether to report it and quit.
func (a *App) Startup(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.ctx = ctx

	if err := util.InitAppRoot("unipac"); err != nil {
		return a.fail("init app root dir", err)
	}

	if err := config.Init(); err != nil {
		return a.fail("load app config", err)
	}

	if err := logging.Init(config.Instance.Log); err != nil {
		return a.fail("init logging", err)
	}
	slog.InfoContext(a.ctx, "logging initialized")

	if err := cache.Init(a.ctx, config.Instance.Cache); err != nil {
		return a.fail("init cache", err)
	}
	slog.InfoContext(a.ctx, "cache initialized")

	slog.InfoContext(a.ctx, "initializing manager registry")
	registry.Init(a.ctx, config.Instance.Managers)
	if err := config.SyncManagers(registry.Instance.ListManagers()); err != nil {
		return a.fail("sync manager config", err)
	}
	slog.InfoContext(a.ctx, "manager registry initialized")

	return nil
}

func (a *App) fail(stage string, err error) error {
	slog.ErrorContext(a.ctx, "startup failed", "stage", stage, "error", err)
	return fmt.Errorf("%s: %w", stage, err)
}

// Shutdown releases resources held by the app.
func (a *App) Shutdown() {
	Cleanup()
}

// Cleanup closes the log file. It is safe to call more than once.
func Cleanup() {
	if logging.LogFile == nil {
		return
	}
	if err := logging.Close(); err != nil {
		slog.Error("Failed to close log file", "error", err)
	}
}

// Context returns the context the app was started with.
func (a *App) Context() context.Context {
	return a.ctx
}

func (a *App) ListManagers() []manager.Info {
	slog.DebugContext(a.ctx, "listing all managers")
	return registry.Instance.ListManagers()
}

func (a *App) ListEnabledManagers() []manager.Info {
	slog.DebugContext(a.ctx, "listing enabled managers")
	managers := registry.Instance.ListManagers()
	return lo.Filter(managers, func(info manager.Info, _ int) bool {
		return info.Enabled
	})
}

func (a *App) GetCachedPackages() ([]manager.Package, error) {
	slog.DebugContext(a.ctx, "getting cached packages")
	pkgs, err := cache.Default.List()
	if err != nil {
		slog.ErrorContext(a.ctx, "failed to list installed packages from cache", "error", err)
		return nil, err
	}
	slog.DebugContext(a.ctx, "cached packages retrieved", "count", len(pkgs))
	return pkgs, nil
}

func (a *App) UpdateCache() error {
	slog.InfoContext(a.ctx, "updating package cache")
	pkgs, err := registry.Instance.GetInstalledPackages()
	if err != nil {
		slog.ErrorContext(a.ctx, "failed to get installed packages from registry", "error", err)
		return fmt.Errorf("failed to get installed packages from registry: %w", err)
	}

	if err = cache.Default.UpdateCache(pkgs); err != nil {
		slog.ErrorContext(a.ctx, "failed to update cache", "error", err)
		return fmt.Errorf("failed to update cache for installed packages: %w", err)
	}
	slog.InfoContext(a.ctx, "package cache updated", "count", len(pkgs))
	return nil
}

func (a *App) SearchPackages(keyword string) ([]manager.Package, error) {
	slog.InfoContext(a.ctx, "searching packages", "keyword", keyword)
	pkgs, err := registry.Instance.SearchPackages(keyword)
	if err != nil {
		slog.ErrorContext(a.ctx, "package search failed", "keyword", keyword, "error", err)
		return nil, err
	}
	slog.InfoContext(a.ctx, "package search completed", "keyword", keyword, "count", len(pkgs))
	return pkgs, nil
}

func (a *App) GetPackageInfo(managerID string, name string) (string, error) {
	slog.InfoContext(a.ctx, "getting package info", "manager", managerID, "package", name)
	info, err := registry.Instance.GetPackageInfo(managerID, manager.Package{Name: name})
	if err != nil {
		slog.ErrorContext(a.ctx, "failed to get package info", "manager", managerID, "package", name, "error", err)
		return "", err
	}
	return info, nil
}

func (a *App) ListPackageVersions(managerID string, name string) ([]string, error) {
	slog.InfoContext(a.ctx, "listing package versions", "manager", managerID, "package", name)
	versions, err := registry.Instance.ListPackageVersions(managerID, manager.Package{Name: name})
	if err != nil {
		slog.ErrorContext(a.ctx, "failed to list package versions", "manager", managerID, "package", name, "error", err)
		return nil, err
	}
	return versions, nil
}

func (a *App) InstallPackage(managerID string, name string, opt manager.ActionOptions) (manager.ActionResult, error) {
	slog.InfoContext(a.ctx, "installing package", "manager", managerID, "package", name, "version", opt.Version)
	result, err := registry.Instance.InstallPackages(managerID, []manager.Package{{Name: name}}, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "package install failed", "manager", managerID, "package", name, "error", err)
	} else {
		slog.InfoContext(a.ctx, "package install completed", "manager", managerID, "package", name, "success", result.Success)
	}
	return result, err
}

func (a *App) UninstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	slog.InfoContext(a.ctx, "uninstalling package", "manager", pkg.Manager, "package", pkg.Name)
	result, err := registry.Instance.UninstallPackages(pkg.Manager, []manager.Package{pkg}, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "package uninstall failed", "manager", pkg.Manager, "package", pkg.Name, "error", err)
	} else {
		slog.InfoContext(a.ctx, "package uninstall completed", "manager", pkg.Manager, "package", pkg.Name, "success", result.Success)
	}
	return result, err
}

func (a *App) UpdatePackage(managerID string, name string, opt manager.ActionOptions) (manager.ActionResult, error) {
	slog.InfoContext(a.ctx, "updating package", "manager", managerID, "package", name)
	result, err := registry.Instance.UpdatePackages(managerID, []manager.Package{{Name: name}}, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "package update failed", "manager", managerID, "package", name, "error", err)
	} else {
		slog.InfoContext(a.ctx, "package update completed", "manager", managerID, "package", name, "success", result.Success)
	}
	return result, err
}

func (a *App) BatchUninstallPackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	names := lo.Map(pkgs, func(p manager.Package, _ int) string { return p.Name })
	slog.InfoContext(a.ctx, "batch uninstalling packages", "count", len(pkgs), "packages", names)
	result, err := registry.Instance.BatchUninstallPackages(pkgs, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "batch uninstall failed", "error", err)
	} else {
		slog.InfoContext(a.ctx, "batch uninstall completed", "success", result.Success)
	}
	return result, err
}

func (a *App) BatchUpdatePackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	names := lo.Map(pkgs, func(p manager.Package, _ int) string { return p.Name })
	slog.InfoContext(a.ctx, "batch updating packages", "count", len(pkgs), "packages", names)
	result, err := registry.Instance.BatchUpdatePackages(pkgs, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "batch update failed", "error", err)
	} else {
		slog.InfoContext(a.ctx, "batch update completed", "success", result.Success)
	}
	return result, err
}
