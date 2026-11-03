package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/views"
	"github.com/egoist/mygo"
)

// demoService drives the views with simulated packages and tasks. It exists so
// the interface, the task page and the animations can be exercised without a
// package manager on the machine.
//
// Start it with `go run ./source -demo` or `UNIPAC_DEMO=1 go run ./source`.
type demoService struct {
	mu sync.Mutex

	packages []manager.Package
	// catalog is what Discover can install. It is separate from packages so an
	// install can move an entry into the inventory.
	catalog []manager.Package

	managers  []manager.Info
	overrides map[string]manager.Settings

	tasks    []*demoTask
	next     int
	listener func()
	// attempts counts simulated actions per kind/manager/package. The first
	// update of libpng fails on purpose, so Retry has something to retry.
	attempts map[string]int
	// slots keeps a batch readable: two tasks run, the rest wait.
	slots chan struct{}

	autoScan bool
	logLevel string
	language string
	// lineDelay is how long each console line takes. Tests shorten it.
	lineDelay time.Duration
}

type demoTask struct {
	id          string
	kind        tasks.Kind
	state       tasks.State
	managerID   string
	managerName string
	pkg         string
	version     string
	newVersion  string
	isGUI       bool
	command     string
	execPath    string
	message     string
	started     time.Time
	finished    time.Time
	output      []tasks.Line
	truncated   bool
	ctx         context.Context
	cancel      context.CancelFunc
}

var _ views.Service = (*demoService)(nil)

func demoMode() bool {
	if os.Getenv("UNIPAC_DEMO") == "1" {
		return true
	}
	for _, arg := range os.Args[1:] {
		if arg == "-demo" || arg == "--demo" {
			return true
		}
	}
	return false
}

func newDemoService() *demoService {
	return &demoService{
		packages:  demoPackages(),
		catalog:   demoCatalog(),
		managers:  demoManagers(),
		overrides: map[string]manager.Settings{},
		attempts:  map[string]int{},
		slots:     make(chan struct{}, 2),
		autoScan:  true,
		logLevel:  "info",
		lineDelay: 520 * time.Millisecond,
	}
}

func demoCapabilities(search bool) manager.Capabilities {
	caps := manager.DefaultCapabilities().WithListOutdated(true).WithListVersions(true)
	caps.Search = search
	return *caps
}

func demoManagers() []manager.Info {
	return []manager.Info{
		{
			ID: "brew", Name: "Homebrew", ExecName: "brew",
			ExecPath: "/opt/homebrew/bin/brew", Available: true, Enabled: true,
			Capabilities: demoCapabilities(true),
		},
		{
			ID: "npm", Name: "NPM", ExecName: "npm",
			ExecPath: "/opt/homebrew/bin/npm", Available: true, Enabled: true,
			Capabilities: demoCapabilities(true),
		},
		{
			ID: "pip", Name: "Pip", ExecName: "pip3",
			ExecPath: "/opt/homebrew/bin/pip3", Available: true, Enabled: true,
			Capabilities: demoCapabilities(true),
		},
	}
}

func demoPackages() []manager.Package {
	return []manager.Package{
		{Name: "ripgrep", Version: "14.1.1", LatestVersion: "15.0.0", Manager: "brew", Installed: true, Outdated: true,
			Description: "Line-oriented search tool"},
		{Name: "git", Version: "2.45.2", Manager: "brew", Installed: true,
			Description: "Distributed version control"},
		{Name: "node", Version: "22.5.1", LatestVersion: "22.6.0", Manager: "brew", Installed: true, Outdated: true,
			Description: "JavaScript runtime"},
		{Name: "iTerm2", Version: "3.5.4", LatestVersion: "3.5.5", Manager: "brew", Installed: true, Outdated: true, IsGUI: true,
			Description: "Terminal emulator (GUI package)"},
		{Name: "libpng", Version: "1.6.43", LatestVersion: "1.6.44", Manager: "brew", Installed: true, Outdated: true,
			Description: "PNG image library; its first update fails, so Retry can be tried"},
		{Name: "typescript", Version: "5.5.4", LatestVersion: "5.6.2", Manager: "npm", Installed: true, Outdated: true,
			Description: "TypeScript compiler"},
		{Name: "eslint", Version: "9.9.0", Manager: "npm", Installed: true,
			Description: "JavaScript linter"},
		{Name: "requests", Version: "2.31.0", Manager: "pip", Installed: true,
			Description: "HTTP library for Python"},
	}
}

func demoCatalog() []manager.Package {
	return []manager.Package{
		{Name: "fzf", Version: "0.55.0", LatestVersion: "0.55.0", Manager: "brew",
			Description: "Command-line fuzzy finder"},
		{Name: "httpie", Version: "3.2.4", LatestVersion: "3.2.4", Manager: "brew",
			Description: "Human-friendly HTTP client"},
		{Name: "prettier", Version: "3.3.3", LatestVersion: "3.3.3", Manager: "npm",
			Description: "Opinionated code formatter"},
		{Name: "vitest", Version: "2.0.5", LatestVersion: "2.0.5", Manager: "npm",
			Description: "Vite-native test runner"},
		{Name: "black", Version: "24.8.0", LatestVersion: "24.8.0", Manager: "pip",
			Description: "Python code formatter"},
	}
}

// --- Inventory and settings -------------------------------------------------

func (s *demoService) ListManagers() []manager.Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]manager.Info(nil), s.managers...)
}

func (s *demoService) GetCachedPackages() ([]manager.Package, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enabled := make(map[string]bool, len(s.managers))
	for _, m := range s.managers {
		enabled[m.ID] = m.Enabled
	}
	out := make([]manager.Package, 0, len(s.packages))
	for _, pkg := range s.packages {
		if enabled[pkg.Manager] {
			out = append(out, pkg)
		}
	}
	return out, nil
}

func (s *demoService) UpdateCache() error {
	// Long enough to see the thin refresh bar, short enough not to be in the
	// way of a demo.
	time.Sleep(220 * time.Millisecond)
	return nil
}

func (s *demoService) SearchPackages(keyword string, managerIDs []string) ([]manager.Package, error) {
	time.Sleep(180 * time.Millisecond)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return []manager.Package{}, nil
	}
	restricted := managerIDs != nil
	allowed := make(map[string]bool, len(managerIDs))
	for _, id := range managerIDs {
		allowed[id] = true
	}
	if restricted && len(managerIDs) == 0 {
		return []manager.Package{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	enabled := make(map[string]bool, len(s.managers))
	search := make(map[string]bool, len(s.managers))
	for _, m := range s.managers {
		enabled[m.ID] = m.Enabled
		search[m.ID] = m.Capabilities.Search
	}
	installed := make(map[string]bool, len(s.packages))
	for _, pkg := range s.packages {
		installed[pkg.Manager+":"+pkg.Name] = true
	}

	out := make([]manager.Package, 0)
	for _, pkg := range s.catalog {
		if !enabled[pkg.Manager] || !search[pkg.Manager] {
			continue
		}
		if restricted && !allowed[pkg.Manager] {
			continue
		}
		if installed[pkg.Manager+":"+pkg.Name] {
			continue
		}
		if strings.Contains(strings.ToLower(pkg.Name), keyword) ||
			strings.Contains(strings.ToLower(pkg.Description), keyword) {
			pkg.Installed, pkg.Outdated = false, false
			out = append(out, pkg)
		}
	}
	return out, nil
}

func (s *demoService) ManagerOverrides() map[string]manager.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]manager.Settings, len(s.overrides))
	for id, settings := range s.overrides {
		out[id] = settings
	}
	return out
}

func (s *demoService) SetManagerEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.managers {
		if s.managers[i].ID == id {
			s.managers[i].Enabled = enabled
			break
		}
	}
	settings := s.overrides[id]
	settings.Disabled = !enabled
	s.overrides[id] = settings
	return nil
}

func (s *demoService) SetManagerExecPath(id, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.managers {
		if s.managers[i].ID == id {
			s.managers[i].ExecPath = path
			break
		}
	}
	settings := s.overrides[id]
	settings.ExecPath = path
	s.overrides[id] = settings
	return nil
}

func (s *demoService) ReloadManagers() error { return nil }

func (s *demoService) Language() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.language
}

func (s *demoService) Languages() []string {
	return mygo.App.Languages()
}

func (s *demoService) SetLanguage(tag string) error {
	if tag != "" && !slices.Contains(mygo.App.Languages(), tag) {
		return fmt.Errorf("unknown language: %q", tag)
	}
	s.mu.Lock()
	s.language = tag
	s.mu.Unlock()
	mygo.App.SetLocale(tag)
	return nil
}

func (s *demoService) AutoScanOnLaunch() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.autoScan
}

func (s *demoService) SetAutoScanOnLaunch(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoScan = on
	return nil
}

func (s *demoService) LogLevel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logLevel
}

func (s *demoService) LogLevels() []string {
	return []string{"debug", "info", "warn", "error"}
}

func (s *demoService) SetLogLevel(level string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logLevel = level
	return nil
}

// --- Actions ----------------------------------------------------------------

func (s *demoService) InstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return s.simulate(tasks.KindInstall, pkg, opt)
}

func (s *demoService) UpdatePackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return s.simulate(tasks.KindUpdate, pkg, opt)
}

func (s *demoService) UninstallPackage(pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return s.simulate(tasks.KindUninstall, pkg, opt)
}

func (s *demoService) BatchUpdatePackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return s.runBatch(tasks.KindUpdate, pkgs, opt)
}

func (s *demoService) BatchUninstallPackages(pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	return s.runBatch(tasks.KindUninstall, pkgs, opt)
}

func (s *demoService) runBatch(kind tasks.Kind, pkgs []manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	if len(pkgs) == 0 {
		return manager.SuccessResult(""), nil
	}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		failures []string
	)
	for _, pkg := range pkgs {
		wg.Go(func() {
			result, _ := s.simulate(kind, pkg, opt)
			if result.Success {
				return
			}
			mu.Lock()
			failures = append(failures, pkg.Name+": "+result.Message)
			mu.Unlock()
		})
	}
	wg.Wait()
	if len(failures) > 0 {
		return manager.ErrorResult(strings.Join(failures, "\n")), nil
	}
	return manager.SuccessResult(""), nil
}

// simulate runs one fake command. It blocks for the duration of the task, as a
// real App action does, and reports each console line through the task listener.
func (s *demoService) simulate(kind tasks.Kind, pkg manager.Package, opt manager.ActionOptions) (manager.ActionResult, error) {
	task := s.begin(kind, pkg, opt)

	select {
	case s.slots <- struct{}{}:
	case <-task.ctx.Done():
		s.finish(task, false, tasks.InterruptedMessage)
		if s.cancelled(task) {
			return manager.ErrorResult(tasks.CancelledMessage), nil
		}
		return manager.ErrorResult(tasks.InterruptedMessage), nil
	}
	defer func() { <-s.slots }()

	s.mu.Lock()
	if task.state != tasks.StateQueued {
		// StopTasks got here first.
		s.mu.Unlock()
		return manager.ErrorResult(tasks.InterruptedMessage), nil
	}
	task.state = tasks.StateRunning
	s.mu.Unlock()
	s.notify()

	script := s.script(kind, pkg, opt)
	s.setCommand(task, script.command, script.execPath)
	for _, line := range script.lines {
		select {
		case <-task.ctx.Done():
			s.finish(task, false, tasks.InterruptedMessage)
			if s.cancelled(task) {
				return manager.ErrorResult(tasks.CancelledMessage), nil
			}
			return manager.ErrorResult(tasks.InterruptedMessage), nil
		case <-time.After(s.lineDelay):
		}
		s.appendLine(task, line)
	}

	if script.fail {
		s.finish(task, false, script.message)
		return manager.ErrorResult(script.message), nil
	}
	s.finish(task, true, script.message)
	s.applyInventory(kind, pkg)
	return manager.SuccessResult(script.message), nil
}

type demoScript struct {
	command  string
	execPath string
	lines    []string
	fail     bool
	message  string
}

func (s *demoService) script(kind tasks.Kind, pkg manager.Package, opt manager.ActionOptions) demoScript {
	s.mu.Lock()
	attempt := s.attempts[string(kind)+":"+pkg.Manager+":"+pkg.Name]
	s.mu.Unlock()

	execPath := "/opt/homebrew/bin/brew"
	switch pkg.Manager {
	case "npm":
		execPath = "/opt/homebrew/bin/npm"
	case "pip":
		execPath = "/opt/homebrew/bin/pip3"
	}

	name := pkg.Name
	version := pkg.Version
	latest := pkg.LatestVersion
	if latest == "" {
		latest = pkg.Version
	}

	switch pkg.Manager {
	case "npm":
		return s.npmScript(kind, name, version, latest, execPath)
	case "pip":
		return s.pipScript(kind, name, version, latest, execPath)
	default:
		script := s.brewScript(kind, pkg, version, latest, execPath)
		if kind == tasks.KindUpdate && name == "libpng" && attempt == 1 {
			script.fail = true
			script.message = "SHA256 mismatch"
			script.lines = append(script.lines,
				"\x1b[31;1mError:\x1b[0m SHA256 mismatch for \x1b[1mlibpng\x1b[0m",
				"\x1b[31m==> Formula installation failed\x1b[0m",
			)
		}
		return script
	}
}

func (s *demoService) brewScript(kind tasks.Kind, pkg manager.Package, version, latest, execPath string) demoScript {
	name := pkg.Name
	cask := ""
	if pkg.IsGUI {
		cask = "--cask "
	}
	versionLine := fmt.Sprintf("%s -> %s", version, latest)
	switch kind {
	case tasks.KindInstall:
		return demoScript{
			command:  "brew install " + cask + name,
			execPath: execPath,
			message:  "Installed " + name,
			lines: []string{
				"\x1b[34m==>\x1b[0m Downloading \x1b[1m" + name + "\x1b[0m",
				"\x1b[34m==>\x1b[0m Pouring " + name + "--" + latest + ".bottle.tar.gz",
				"==> Caveats",
				"\x1b[32m🍺\x1b[0m  " + name + " was successfully installed!",
			},
		}
	case tasks.KindUninstall:
		return demoScript{
			command:  "brew uninstall " + cask + name,
			execPath: execPath,
			message:  "Uninstalled " + name,
			lines: []string{
				"Uninstalling " + name + " " + version,
				"==> Removing files...",
				"\x1b[32m🍺\x1b[0m  " + name + " was successfully uninstalled",
			},
		}
	default:
		return demoScript{
			command:  "brew upgrade " + cask + name,
			execPath: execPath,
			message:  "Updated " + name,
			lines: []string{
				"\x1b[34m==>\x1b[0m Updating Homebrew...",
				"\x1b[34m==>\x1b[0m Upgrading \x1b[1m" + name + "\x1b[0m " + versionLine,
				"\x1b[34m==>\x1b[0m Pouring " + name + "--" + latest + ".bottle.tar.gz",
				"\x1b[32m🍺\x1b[0m  /opt/homebrew/Cellar/" + name + "/" + latest + ": 120 files, 5.2MB",
			},
		}
	}
}

func (s *demoService) npmScript(kind tasks.Kind, name, version, latest, execPath string) demoScript {
	switch kind {
	case tasks.KindInstall:
		return demoScript{
			command:  "npm install -g " + name,
			execPath: execPath,
			message:  "Installed " + name,
			lines: []string{
				"npm \x1b[33mwarn\x1b[0m deprecated " + name + "@" + latest + ": use the newer package",
				"added 87 packages, and audited 88 packages in 2s",
				"\x1b[32mfound 0 vulnerabilities\x1b[0m",
				"\x1b[32m" + name + "@" + latest + "\x1b[0m installed globally",
			},
		}
	case tasks.KindUninstall:
		return demoScript{
			command:  "npm uninstall -g " + name,
			execPath: execPath,
			message:  "Uninstalled " + name,
			lines: []string{
				"removed 87 packages in 340ms",
				"\x1b[32m" + name + " removed\x1b[0m",
			},
		}
	default:
		return demoScript{
			command:  "npm update -g " + name,
			execPath: execPath,
			message:  "Updated " + name,
			lines: []string{
				"\x1b[36mchanged\x1b[0m 1 package: " + name + "@" + version + " -> " + latest,
				"\x1b[32mfound 0 vulnerabilities\x1b[0m",
			},
		}
	}
}

func (s *demoService) pipScript(kind tasks.Kind, name, version, latest, execPath string) demoScript {
	switch kind {
	case tasks.KindInstall:
		return demoScript{
			command:  "pip install " + name,
			execPath: execPath,
			message:  "Installed " + name,
			lines: []string{
				"Collecting " + name,
				"  Downloading " + name + "-" + latest + "-py3-none-any.whl",
				"Installing collected packages: " + name,
				"\x1b[32mSuccessfully installed " + name + "-" + latest + "\x1b[0m",
			},
		}
	case tasks.KindUninstall:
		return demoScript{
			command:  "pip uninstall -y " + name,
			execPath: execPath,
			message:  "Uninstalled " + name,
			lines: []string{
				"Found existing installation: " + name + " " + version,
				"Uninstalling " + name + "-" + version + ":",
				"  Successfully uninstalled " + name + "-" + version,
			},
		}
	default:
		return demoScript{
			command:  "pip install --upgrade " + name,
			execPath: execPath,
			message:  "Updated " + name,
			lines: []string{
				"Requirement already satisfied: " + name,
				"Installing collected packages: " + name,
				"\x1b[32mSuccessfully installed " + name + "-" + latest + "\x1b[0m",
			},
		}
	}
}

// --- Task state -------------------------------------------------------------

func (s *demoService) begin(kind tasks.Kind, pkg manager.Package, opt manager.ActionOptions) *demoTask {
	s.mu.Lock()
	s.next++
	ctx, cancel := context.WithCancel(context.Background())
	name := pkg.Manager
	for _, m := range s.managers {
		if m.ID == pkg.Manager {
			name = m.Name
			break
		}
	}
	oldVersion, newVersion := pkg.Version, pkg.LatestVersion
	switch kind {
	case tasks.KindInstall:
		oldVersion = ""
		if opt.Version != "" {
			newVersion = opt.Version
		}
	case tasks.KindUninstall:
		newVersion = ""
	case tasks.KindUpdate:
		if newVersion == "" {
			newVersion = pkg.Version
		}
	}
	key := string(kind) + ":" + pkg.Manager + ":" + pkg.Name
	s.attempts[key]++

	task := &demoTask{
		id:          fmt.Sprintf("demo-%d", s.next),
		kind:        kind,
		state:       tasks.StateQueued,
		managerID:   pkg.Manager,
		managerName: name,
		pkg:         pkg.Name,
		version:     oldVersion,
		newVersion:  newVersion,
		isGUI:       pkg.IsGUI,
		started:     time.Now(),
		ctx:         ctx,
		cancel:      cancel,
	}
	s.tasks = append(s.tasks, task)
	s.mu.Unlock()
	s.notify()
	return task
}

func (s *demoService) setCommand(task *demoTask, command, execPath string) {
	s.mu.Lock()
	if task.state == tasks.StateFailed {
		s.mu.Unlock()
		return
	}
	task.command, task.execPath = command, execPath
	s.mu.Unlock()
	s.notify()
}

func (s *demoService) appendLine(task *demoTask, line string) {
	s.mu.Lock()
	if task.state != tasks.StateRunning {
		s.mu.Unlock()
		return
	}
	task.output = append(task.output, tasks.Line{Text: line})
	s.mu.Unlock()
	s.notify()
}

func (s *demoService) finish(task *demoTask, ok bool, message string) {
	s.mu.Lock()
	if task.state == tasks.StateSucceeded || task.state == tasks.StateFailed || task.state == tasks.StateCancelled {
		s.mu.Unlock()
		return
	}
	task.state = tasks.StateSucceeded
	if !ok {
		task.state = tasks.StateFailed
	}
	task.message = message
	task.finished = time.Now()
	cancel := task.cancel
	task.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.notify()
}

func (s *demoService) applyInventory(kind tasks.Kind, pkg manager.Package) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch kind {
	case tasks.KindInstall:
		for _, existing := range s.packages {
			if existing.Manager == pkg.Manager && existing.Name == pkg.Name {
				return
			}
		}
		pkg.Installed, pkg.Outdated = true, false
		if pkg.LatestVersion != "" {
			pkg.Version = pkg.LatestVersion
		}
		s.packages = append(s.packages, pkg)
	case tasks.KindUpdate:
		for i := range s.packages {
			if s.packages[i].Manager == pkg.Manager && s.packages[i].Name == pkg.Name {
				if pkg.LatestVersion != "" {
					s.packages[i].Version = pkg.LatestVersion
				}
				s.packages[i].Outdated = false
				return
			}
		}
	case tasks.KindUninstall:
		kept := s.packages[:0]
		for _, existing := range s.packages {
			if existing.Manager == pkg.Manager && existing.Name == pkg.Name {
				continue
			}
			kept = append(kept, existing)
		}
		s.packages = kept
	}
}

func (s *demoService) SetTaskListener(fn func()) {
	s.mu.Lock()
	s.listener = fn
	s.mu.Unlock()
}

// TaskOutput returns a task's console; the demo keeps it in memory only.
func (s *demoService) TaskOutput(id string) tasks.Output {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range s.tasks {
		if task.id == id {
			return tasks.Output{
				Lines:     append([]tasks.Line(nil), task.output...),
				Truncated: task.truncated,
				Seq:       len(task.output),
			}
		}
	}
	return tasks.Output{}
}

// CancelTask stops a running or queued simulated task.
func (s *demoService) CancelTask(id string) error {
	s.mu.Lock()
	var task *demoTask
	for _, candidate := range s.tasks {
		if candidate.id == id {
			task = candidate
			break
		}
	}
	if task == nil {
		s.mu.Unlock()
		return fmt.Errorf("task not found")
	}
	if task.state != tasks.StateQueued && task.state != tasks.StateRunning {
		s.mu.Unlock()
		return fmt.Errorf("task is not running")
	}
	task.state = tasks.StateCancelled
	task.message = tasks.CancelledMessage
	task.finished = time.Now()
	cancel := task.cancel
	task.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.notify()
	return nil
}

func (s *demoService) cancelled(task *demoTask) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return task.state == tasks.StateCancelled
}

func (s *demoService) notify() {
	s.mu.Lock()
	fn := s.listener
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (s *demoService) ListTasks() []tasks.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]tasks.Snapshot, 0, len(s.tasks))
	for _, task := range s.tasks {
		out = append(out, tasks.Snapshot{
			ID:          task.id,
			Kind:        task.kind,
			State:       task.state,
			ManagerID:   task.managerID,
			ManagerName: task.managerName,
			Package:     task.pkg,
			Version:     task.version,
			NewVersion:  task.newVersion,
			IsGUI:       task.isGUI,
			Command:     task.command,
			ExecPath:    task.execPath,
			StartedAt:   task.started,
			FinishedAt:  task.finished,
			Message:     task.message,
			Truncated:   task.truncated,
			OutputSeq:   len(task.output),
		})
	}
	sortDemoTasks(out)
	return out
}

func sortDemoTasks(list []tasks.Snapshot) {
	rank := func(state tasks.State) int {
		switch state {
		case tasks.StateRunning:
			return 0
		case tasks.StateQueued:
			return 1
		default:
			return 2
		}
	}
	slices.SortStableFunc(list, func(a, b tasks.Snapshot) int {
		if ra, rb := rank(a.State), rank(b.State); ra != rb {
			return ra - rb
		}
		if rank(a.State) <= 1 {
			return a.StartedAt.Compare(b.StartedAt)
		}
		if !a.FinishedAt.Equal(b.FinishedAt) {
			return b.FinishedAt.Compare(a.FinishedAt)
		}
		return b.StartedAt.Compare(a.StartedAt)
	})
}

func (s *demoService) UnfinishedTasks() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, task := range s.tasks {
		if task.state == tasks.StateQueued || task.state == tasks.StateRunning {
			n++
		}
	}
	return n
}

func (s *demoService) StopTasks() {
	s.mu.Lock()
	for _, task := range s.tasks {
		if task.state != tasks.StateQueued && task.state != tasks.StateRunning {
			continue
		}
		task.state = tasks.StateCancelled
		task.message = tasks.InterruptedMessage
		task.finished = time.Now()
		if task.cancel != nil {
			task.cancel()
			task.cancel = nil
		}
	}
	s.mu.Unlock()
	s.notify()
}
