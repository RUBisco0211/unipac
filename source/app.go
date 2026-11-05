package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/RUBisco0211/unipac/source/backend/cache"
	"github.com/RUBisco0211/unipac/source/backend/command"
	"github.com/RUBisco0211/unipac/source/backend/config"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/core/registry"
	"github.com/RUBisco0211/unipac/source/backend/logging"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/egoist/mygo"
	"github.com/samber/lo"
)

// App is the application service. Its exported methods are called directly by
// the views; there is no binding layer between them.
type App struct {
	ctx context.Context
	// runCtx is the parent of every action. Cancelling it stops the commands a
	// quit interrupts, and wakes workers waiting for a free slot.
	runCtx context.Context
	cancel context.CancelFunc
	// locks arbitrates package-manager commands: a task waits for its manager's
	// slot, and a cache scan waits for every enabled manager's slot.
	locks *command.Limiter
}

// App is the only implementation of the port the views declare. The build fails
// here, not in the views, when a method's signature drifts from it.
var _ inventory.Service = (*App)(nil)

func NewApp() *App {
	ctx := context.Background()
	runCtx, cancel := context.WithCancel(ctx)
	return &App{
		ctx:    ctx,
		runCtx: runCtx,
		cancel: cancel,
		locks:  command.NewLimiter(command.MaxConcurrentTasks),
	}
}

// Startup prepares the app root, config, logging, cache and manager registry.
// It returns the first failure with the stage that produced it; entry points
// decide whether to report it and quit.
func (a *App) Startup(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.ctx = ctx
	a.runCtx, a.cancel = context.WithCancel(ctx)

	if err := util.InitAppRoot("unipac"); err != nil {
		return a.fail("init app root dir", err)
	}

	if err := config.Init(); err != nil {
		return a.fail("load app config", err)
	}
	// The config is the source of the language choice; an unknown tag follows
	// the system rather than making the app unusable.
	language := config.Instance.Language
	if language != "" && !slices.Contains(mygo.App.Languages(), language) {
		language = ""
	}
	mygo.App.SetLocale(language)

	if err := logging.Init(config.Instance.Log); err != nil {
		return a.fail("init logging", err)
	}
	slog.InfoContext(a.ctx, "logging initialized")

	if err := cache.Init(a.ctx, config.Instance.Cache); err != nil {
		return a.fail("init cache", err)
	}
	slog.InfoContext(a.ctx, "cache initialized")

	if err := tasks.Init(a.ctx); err != nil {
		return a.fail("init task history", err)
	}
	slog.InfoContext(a.ctx, "task history initialized")

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

// Cleanup closes the task store and the log file. It is safe to call more than
// once.
func Cleanup() {
	tasks.Default.Close()
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

// Language is the saved language tag, "" to follow the system. A tag this
// build no longer ships reads as the system choice, so the Settings select can
// still show and replace it.
func (a *App) Language() string {
	tag := config.Instance.Language
	if tag != "" && !slices.Contains(mygo.App.Languages(), tag) {
		return ""
	}
	return tag
}

// Languages lists the languages UniPac is written in.
func (a *App) Languages() []string {
	return mygo.App.Languages()
}

// SetLanguage saves the chosen language and applies it at once. An empty tag
// follows the system again.
func (a *App) SetLanguage(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag != "" && !slices.Contains(mygo.App.Languages(), tag) {
		return fmt.Errorf("unknown language: %q", tag)
	}
	slog.InfoContext(a.ctx, "changing language", "language", tag)
	if err := config.SetLanguage(tag); err != nil {
		slog.ErrorContext(a.ctx, "failed to save language", "error", err)
		return err
	}
	mygo.App.SetLocale(tag)
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
	ids := make([]string, 0)
	for _, info := range registry.Instance.ListManagers() {
		if info.Enabled {
			ids = append(ids, info.ID)
		}
	}
	release, err := a.locks.AcquireAll(a.runCtx, ids)
	if err != nil {
		return err
	}
	defer release()
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

func (a *App) SearchPackages(keyword string, managerIDs []string) (manager.SearchResult, error) {
	slog.InfoContext(a.ctx, "searching packages", "keyword", keyword, "managers", managerIDs)
	result, err := registry.Instance.SearchPackages(keyword, managerIDs)
	if err != nil {
		slog.ErrorContext(a.ctx, "package search failed", "keyword", keyword, "error", err)
		return manager.SearchResult{}, err
	}
	slog.InfoContext(a.ctx, "package search completed", "keyword", keyword,
		"count", len(result.Packages), "truncated", result.Truncated)
	return result, nil
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

func (a *App) InstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	slog.InfoContext(a.ctx, "installing package", "manager", pkg.Manager, "package", pkg.Name, "version", opt.Version)
	result, err := a.runPackageTask(tasks.KindInstall, pkg, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "package install failed", "manager", pkg.Manager, "package", pkg.Name, "error", err)
	} else {
		slog.InfoContext(a.ctx, "package install completed", "manager", pkg.Manager, "package", pkg.Name, "success", result.Success)
	}
	return result, err
}

func (a *App) UninstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	slog.InfoContext(a.ctx, "uninstalling package", "manager", pkg.Manager, "package", pkg.Name)
	result, err := a.runPackageTask(tasks.KindUninstall, pkg, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "package uninstall failed", "manager", pkg.Manager, "package", pkg.Name, "error", err)
	} else {
		slog.InfoContext(a.ctx, "package uninstall completed", "manager", pkg.Manager, "package", pkg.Name, "success", result.Success)
	}
	return result, err
}

func (a *App) UpdatePackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	slog.InfoContext(a.ctx, "updating package", "manager", pkg.Manager, "package", pkg.Name)
	result, err := a.runPackageTask(tasks.KindUpdate, pkg, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "package update failed", "manager", pkg.Manager, "package", pkg.Name, "error", err)
	} else {
		slog.InfoContext(a.ctx, "package update completed", "manager", pkg.Manager, "package", pkg.Name, "success", result.Success)
	}
	return result, err
}

// BatchUninstallPackages fans a selection out into one task per package. The
// adapter never receives a batch, so every task has its own command, state and
// console.
func (a *App) BatchUninstallPackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	names := packageNames(pkgs)
	slog.InfoContext(a.ctx, "batch uninstalling packages", "count", len(pkgs), "packages", names)
	result, err := a.runPackageBatch(tasks.KindUninstall, pkgs, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "batch uninstall failed", "error", err)
	} else {
		slog.InfoContext(a.ctx, "batch uninstall completed", "success", result.Success)
	}
	return result, err
}

// BatchUpdatePackages fans a selection out into one task per package.
func (a *App) BatchUpdatePackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	names := packageNames(pkgs)
	slog.InfoContext(a.ctx, "batch updating packages", "count", len(pkgs), "packages", names)
	result, err := a.runPackageBatch(tasks.KindUpdate, pkgs, opt)
	if err != nil {
		slog.ErrorContext(a.ctx, "batch update failed", "error", err)
	} else {
		slog.InfoContext(a.ctx, "batch update completed", "success", result.Success)
	}
	return result, err
}

// ListTasks returns the tasks of this session and the stored history, with the
// console output of the ones this process ran.
func (a *App) ListTasks() []tasks.Snapshot {
	return tasks.Default.List()
}

// SetTaskListener registers a callback for task changes, so the page can
// repaint while a command prints. It is called off the UI thread.
func (a *App) SetTaskListener(fn func()) {
	tasks.Default.SetListener(fn)
}

// TaskOutput returns a task's console lines, including a task from a previous
// session whose output lives in the store.
func (a *App) TaskOutput(id string) tasks.Output {
	return tasks.Default.Output(id)
}

// CancelTask stops one running or queued task and records it as cancelled.
func (a *App) CancelTask(id string) error {
	return tasks.Default.Cancel(id)
}

// UnfinishedTasks reports queued and running tasks. The window asks before it
// closes and before the app quits.
func (a *App) UnfinishedTasks() int {
	return tasks.Default.PendingCount()
}

// StopTasks cancels every unfinished task and kills the commands still
// running, so the child processes are not orphaned.
func (a *App) StopTasks() {
	tasks.Default.StopAll(tasks.InterruptedMessage)
	a.cancel()
}

// runPackageTask creates one task for one package and runs it.
func (a *App) runPackageTask(kind tasks.Kind, pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	task := a.beginTask(kind, pkg, opt)
	// The task's own context wakes a queued acquire when the user cancels it.
	release, err := a.locks.Acquire(task.Context(), pkg.Manager)
	if err != nil {
		if task.Cancelled() {
			return manager.ErrorResult(tasks.CancelledMessage), nil
		}
		task.Finish(false, tasks.InterruptedMessage)
		return manager.ErrorResult(err.Error()), err
	}
	defer release()

	if !task.Start() {
		return manager.ErrorResult(tasks.CancelledMessage), nil
	}
	result, err := a.runRegistryAction(task.Context(), kind, pkg, opt)
	if task.Cancelled() {
		return manager.ErrorResult(tasks.CancelledMessage), nil
	}
	task.Finish(err == nil && result.Success, actionMessage(result, err))
	return result, err
}

// batchJob is one package's task in a batch.
type batchJob struct {
	task *tasks.Task
	pkg  manager.Package
}

// runPackageBatch creates a task per package first, so the page shows the whole
// selection as queued, then runs them with the action limit.
func (a *App) runPackageBatch(kind tasks.Kind, pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	if len(pkgs) == 0 {
		return manager.SuccessResult(""), nil
	}
	jobs := make([]batchJob, 0, len(pkgs))
	for _, pkg := range pkgs {
		jobs = append(jobs, batchJob{task: a.beginTask(kind, pkg, opt), pkg: pkg})
	}

	workers := min(command.MaxConcurrentTasks, len(jobs))
	queue := make(chan batchJob)
	var (
		mu        sync.Mutex
		failures  []string
		cancelled int
		wg        sync.WaitGroup
	)
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for job := range queue {
				release, err := a.locks.Acquire(job.task.Context(), job.pkg.Manager)
				if err != nil {
					if job.task.Cancelled() {
						mu.Lock()
						cancelled++
						mu.Unlock()
						continue
					}
					job.task.Finish(false, tasks.InterruptedMessage)
					continue
				}
				if !job.task.Start() {
					release()
					mu.Lock()
					cancelled++
					mu.Unlock()
					continue
				}
				result, runErr := a.runRegistryAction(job.task.Context(), kind, job.pkg, opt)
				release()
				if job.task.Cancelled() {
					mu.Lock()
					cancelled++
					mu.Unlock()
					continue
				}
				job.task.Finish(runErr == nil && result.Success, actionMessage(result, runErr))

				if runErr == nil && result.Success {
					continue
				}
				message := actionMessage(result, runErr)
				if message == "" {
					message = "failed"
				}
				mu.Lock()
				failures = append(failures, job.pkg.Name+": "+message)
				mu.Unlock()
			}
		}()
	}
	for _, job := range jobs {
		queue <- job
	}
	close(queue)
	wg.Wait()

	if len(failures) > 0 {
		return manager.ErrorResult(strings.Join(failures, "\n")), nil
	}
	switch {
	case cancelled == len(jobs):
		return manager.ErrorResult(tasks.CancelledMessage), nil
	case cancelled > 0:
		return manager.ErrorResult(locales.TasksBatchCancelled()), nil
	default:
		return manager.SuccessResult(""), nil
	}
}

// runRegistryAction calls the registry for one task. Every call carries exactly
// one package: batch commands are gone from the UI's path.
func (a *App) runRegistryAction(ctx context.Context, kind tasks.Kind, pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	switch kind {
	case tasks.KindInstall:
		return registry.Instance.InstallPackages(ctx, pkg.Manager, []manager.Package{pkg}, opt)
	case tasks.KindUpdate:
		return registry.Instance.UpdatePackages(ctx, pkg.Manager, []manager.Package{pkg}, opt)
	case tasks.KindUninstall:
		return registry.Instance.UninstallPackages(ctx, pkg.Manager, []manager.Package{pkg}, opt)
	default:
		return manager.ErrorResult("unknown task kind: " + string(kind)), nil
	}
}

// beginTask records a queued task, naming the manager as the interface shows it
// rather than exposing an ID.
func (a *App) beginTask(kind tasks.Kind, pkg manager.Package, opt manager.ActionOptions) *tasks.Task {
	oldVersion, newVersion := taskVersions(kind, pkg, opt)
	return tasks.Default.Begin(a.runCtx, kind, tasks.Target{
		ManagerID:   pkg.Manager,
		ManagerName: a.managerLabel(pkg.Manager),
		Name:        pkg.Name,
		Version:     oldVersion,
		NewVersion:  newVersion,
	})
}

// taskVersions is what a task records it changed from and to. An install has no
// old version and goes to the one it asked for: a search result carries that in
// Version, since it is not installed yet and has no latest of its own.
func taskVersions(kind tasks.Kind, pkg manager.Package, opt manager.ActionOptions) (old, next string) {
	old, next = pkg.Version, pkg.LatestVersion
	switch kind {
	case tasks.KindInstall:
		old = ""
		if next == "" {
			next = pkg.Version
		}
		if opt.Version != "" {
			next = opt.Version
		}
	case tasks.KindUninstall:
		next = ""
	case tasks.KindUpdate:
		// A requested version belongs to an install; an update goes to the
		// latest version the list knows.
		if next == "" {
			next = pkg.Version
		}
	}
	return old, next
}

// managerLabel resolves a manager ID to the backend-provided display name. An
// unknown ID stays as it is, so a task never loses its manager.
func (a *App) managerLabel(id string) string {
	for _, info := range registry.Instance.ListManagers() {
		if info.ID == id {
			return info.Name
		}
	}
	return id
}

// actionMessage is what the task records when it ends: the adapter's result or
// the error that stopped it.
func actionMessage(result manager.ActionResult, err error) string {
	if err != nil {
		return util.StripANSI(err.Error())
	}
	return util.StripANSI(result.Message)
}

func packageNames(pkgs []manager.Package) []string {
	return lo.Map(pkgs, func(pkg manager.Package, _ int) string { return pkg.Name })
}
