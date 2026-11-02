package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/RUBisco0211/unipac/source/backend/cache"
	"github.com/RUBisco0211/unipac/source/backend/config"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/core/registry"
	"github.com/RUBisco0211/unipac/source/backend/logging"
	"github.com/RUBisco0211/unipac/source/backend/util"
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
	registry.Init(a.ctx, config.ManagerSettings())
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

// ManagerOverrides returns the per-manager settings the user saved. The
// settings page shows them as the values behind its controls.
func (a *App) ManagerOverrides() map[string]manager.Settings {
	return config.ManagerSettings()
}

// SetManagerEnabled turns one manager on or off and applies it to the running
// registry, so the lists, search and actions follow at once.
func (a *App) SetManagerEnabled(id string, enabled bool) error {
	slog.InfoContext(a.ctx, "changing manager state", "manager", id, "enabled", enabled)
	if err := config.SetManagerEnabled(id, enabled); err != nil {
		slog.ErrorContext(a.ctx, "failed to save manager state", "manager", id, "error", err)
		return err
	}
	registry.Instance.Apply(config.ManagerSettings())
	return nil
}

// SetManagerExecPath saves one manager's executable path and preflights it. An
// empty path goes back to detecting the executable on PATH.
func (a *App) SetManagerExecPath(id string, path string) error {
	slog.InfoContext(a.ctx, "changing manager executable", "manager", id, "path", path)
	if err := config.SetManagerExecPath(id, path); err != nil {
		slog.ErrorContext(a.ctx, "failed to save manager executable", "manager", id, "error", err)
		return err
	}
	registry.Instance.Apply(config.ManagerSettings())
	return nil
}

// ReloadManagers preflights every manager again, so a tool installed while the
// app was open is picked up.
func (a *App) ReloadManagers() error {
	slog.InfoContext(a.ctx, "reloading manager registry")
	registry.Instance.Reprobe()
	return nil
}

// AutoScanOnLaunch reports whether the app scans for installed packages when
// it opens.
func (a *App) AutoScanOnLaunch() bool {
	return config.Instance.Startup.ScansOnLaunch()
}

// SetAutoScanOnLaunch saves whether the app scans for installed packages when
// it opens.
func (a *App) SetAutoScanOnLaunch(on bool) error {
	slog.InfoContext(a.ctx, "changing launch scan", "enabled", on)
	return config.SetAutoScan(on)
}

// LogLevel returns the level the app logs at.
func (a *App) LogLevel() string {
	if level := config.Instance.Log.Level; level != "" {
		return level
	}
	return logging.LevelInfo
}

// LogLevels lists the levels the app can log at, least to most severe.
func (a *App) LogLevels() []string {
	return logging.Levels()
}

// SetLogLevel saves a log level and applies it to the running logger. The
// config is written first, so a failed save cannot leave the running app at a
// level the next start would not use.
func (a *App) SetLogLevel(level string) error {
	slog.InfoContext(a.ctx, "changing log level", "level", level)
	if !slices.Contains(logging.Levels(), level) {
		return fmt.Errorf("unknown log level: %q", level)
	}
	if err := config.SetLogLevel(level); err != nil {
		slog.ErrorContext(a.ctx, "failed to save log level", "level", level, "error", err)
		return err
	}
	if err := logging.SetLevel(level); err != nil {
		slog.ErrorContext(a.ctx, "failed to apply log level", "level", level, "error", err)
		return err
	}
	return nil
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
	pkgs, reliable, err := registry.Instance.GetInstalledPackages()
	if err != nil {
		slog.ErrorContext(a.ctx, "failed to get installed packages from registry", "error", err)
		return fmt.Errorf("failed to get installed packages from registry: %w", err)
	}
	if !reliable {
		// Every enabled manager failed. Keep the last good snapshot instead of
		// replacing it with an empty one, and say so rather than looking idle.
		slog.WarnContext(a.ctx, "no enabled manager could list packages; keeping the cached snapshot")
		return fmt.Errorf("no package manager could list its packages")
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
