package main

import (
	"context"
	"fmt"
	"log/slog"
	"unipac-wails/backend/cache"
	"unipac-wails/backend/config"
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/core/registry"
	"unipac-wails/backend/logging"
	"unipac-wails/backend/util"

	"github.com/samber/lo"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

// startup called when the app starts
// context injected
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// init app root directory
	if err := util.InitAppRoot("unipac"); err != nil {
		slog.ErrorContext(ctx, "Failed to init app root dir", "error", err)
		if err = util.WailsDialog(ctx, util.DialogOptions{
			Type:  wailsrt.ErrorDialog,
			Title: "Error",
			Message: fmt.Sprintf("Failed to init app root dir: %s. "+
				"Please check if UniPac has the right to create directories",
				err.Error()),
		}); err != nil {
			panic(err)
		}
		return
	}

	// register adapters
	a.InitManagerRegistry()

	// load config
	if err := config.Init(); err != nil {
		slog.ErrorContext(ctx, "Failed to load config", "error", err)
		_ = util.WailsDialog(ctx, util.DialogOptions{
			Type:    wailsrt.ErrorDialog,
			Title:   "Error",
			Message: fmt.Sprintf("Failed to load app config: %s.", err.Error()),
		})
		panic(err)
	}

	// init logging
	if err := logging.Init(config.Instance.Log); err != nil {
		slog.ErrorContext(ctx, "Failed to initialize logging", "error", err)
		_ = util.WailsDialog(ctx, util.DialogOptions{
			Type:    wailsrt.ErrorDialog,
			Title:   "Error",
			Message: fmt.Sprintf("Failed to init logging: %s.", err.Error()),
		})
		panic(err)
	}
	slog.InfoContext(ctx, "Logging initialized")

	// init cache
	if err := cache.Init(ctx, config.Instance.Cache); err != nil {
		slog.ErrorContext(ctx, "Failed to initialize cache", "error", err)
		_ = util.WailsDialog(ctx, util.DialogOptions{
			Type:    wailsrt.ErrorDialog,
			Title:   "Error",
			Message: fmt.Sprintf("Failed to init cache: %s.", err.Error()),
		})
		panic(err)
	}
	slog.InfoContext(ctx, "Cache initialized")

	if err := a.UpdateCache(); err != nil {
		slog.ErrorContext(ctx, "Failed to update cache", "error", err)
		_ = util.WailsDialog(ctx, util.DialogOptions{
			Type:    wailsrt.ErrorDialog,
			Title:   "Error",
			Message: fmt.Sprintf("Failed to update cache: %s.", err.Error()),
		})
		panic(err)
	}
}

func (a *App) shutdown(_ context.Context) {
	Cleanup()
}

func (a *App) InitManagerRegistry() {
	slog.InfoContext(a.ctx, "initializing manager registry")
	registry.Init(a.ctx)
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

func (a *App) UninstallPackage(managerID string, name string, opt manager.ActionOptions) (manager.ActionResult, error) {
	slog.InfoContext(a.ctx, "uninstalling package", "manager", managerID, "package", name)
	result, err := registry.Instance.UninstallPackages(managerID, []manager.Package{{Name: name}}, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "package uninstall failed", "manager", managerID, "package", name, "error", err)
	} else {
		slog.InfoContext(a.ctx, "package uninstall completed", "manager", managerID, "package", name, "success", result.Success)
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
