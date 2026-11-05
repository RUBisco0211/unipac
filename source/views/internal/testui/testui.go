// Package testui builds the fixtures the view tests share: a package
// inventory, a task list and a headless window. It is a normal package rather
// than a _test.go file because the pages are separate packages and each one's
// tests need the same fixtures, without any of them importing another page.
package testui

import (
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// Data is a small inventory: brew and npm enabled, cargo turned off, one
// outdated package and one current one.
func Data() *inventory.Data {
	return &inventory.Data{
		Names:   map[string]string{"brew": "Homebrew", "npm": "NPM"},
		Version: 1,
		Managers: []manager.Info{
			{ID: "brew", Name: "Homebrew", Enabled: true, Available: true, ExecPath: "/opt/homebrew/bin/brew",
				Capabilities: manager.Capabilities{ListInstalled: true, ListOutdated: true, Search: true, Update: true, Uninstall: true}},
			{ID: "npm", Name: "NPM", Enabled: true, Available: true,
				Capabilities: manager.Capabilities{ListInstalled: true, Update: true, Uninstall: true}},
			{ID: "cargo", Name: "Cargo", Enabled: false,
				Capabilities: manager.Capabilities{ListInstalled: true, Search: true, Install: true, Uninstall: true}},
		},
		Rows: []manager.Package{
			{Name: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true, Description: "Line-oriented search tool"},
			{Name: "typescript", Version: "5.9.3", LatestVersion: "6.0.0", Manager: "npm", Installed: true, Outdated: true},
		},
	}
}

// Tasks is one task of each state: running, queued and failed.
func Tasks() []tasks.Snapshot {
	base := time.Date(2026, 11, 2, 14, 3, 22, 0, time.UTC)
	return []tasks.Snapshot{
		{
			ID:          "3",
			Kind:        tasks.KindUpdate,
			State:       tasks.StateRunning,
			ManagerID:   "brew",
			ManagerName: "Homebrew",
			Package:     "typescript",
			Version:     "14.1.1",
			NewVersion:  "15.0.0",
			StartedAt:   base,
			Command:     "brew upgrade typescript",
			ExecPath:    "/opt/homebrew/bin/brew",
			OutputSeq:   2,
		},
		{
			ID:          "2",
			Kind:        tasks.KindInstall,
			State:       tasks.StateQueued,
			ManagerID:   "npm",
			ManagerName: "NPM",
			Package:     "eslint",
			NewVersion:  "9.9.0",
			StartedAt:   base.Add(-time.Second),
		},
		{
			ID:          "1",
			Kind:        tasks.KindUninstall,
			State:       tasks.StateFailed,
			ManagerID:   "pip",
			ManagerName: "Pip",
			Package:     "requests",
			Version:     "2.31.0",
			StartedAt:   base.Add(-time.Minute),
			FinishedAt:  base,
			Message:     "pip failed",
			Command:     "pip uninstall requests",
			ExecPath:    "/usr/bin/pip3",
		},
	}
}

// Render builds a headless window over build, in the light theme.
func Render(build func(*ui.Context), w, h int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		build(c)
	}, w, h)
}
