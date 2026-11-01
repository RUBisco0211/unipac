package views

import (
	"image/png"
	"os"
	"testing"

	"github.com/RUBisco0211/unipac/backend/core/manager"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// TestScreenshot renders the real builders to a PNG so the interface can be
// reviewed without a desktop session. It writes only when UNIPAC_SCREENSHOT
// names the output file.
func TestScreenshot(t *testing.T) {
	out := os.Getenv("UNIPAC_SCREENSHOT")
	if out == "" {
		t.Skip("set UNIPAC_SCREENSHOT=/path/to.png to render")
	}

	d := &data{
		version: 1,
		names:   map[string]string{"brew": "Homebrew", "npm": "NPM", "pip": "Pip", "uv-tool": "uv tool"},
		managers: []manager.Info{
			{ID: "brew", Name: "Homebrew", Enabled: true, ExecPath: "/opt/homebrew/bin/brew",
				Capabilities: manager.Capabilities{ListInstalled: true, ListOutdated: true, Search: true, Install: true, Uninstall: true, Update: true, GetPackageInfo: true, ListVersions: true}},
		},
		rows: []manager.Package{
			{Name: "ripgrep", Version: "14.1.1", Manager: "brew", Installed: true, Description: "Search tool like grep and The Silver Searcher"},
			{Name: "ffmpeg", Version: "7.1", LatestVersion: "8.0", Manager: "brew", Installed: true, Outdated: true, Description: "Play, record, convert, and stream audio and video"},
			{Name: "git", Version: "2.51.0", Manager: "brew", Installed: true, Description: "Distributed revision control system"},
			{Name: "node", Version: "24.10.0", Manager: "brew", Installed: true, Description: "Platform built on V8 to build network applications"},
			{Name: "python@3.13", Version: "3.13.7", Manager: "brew", Installed: true, Description: "Interpreted, interactive, object-oriented programming language"},
			{Name: "typescript", Version: "5.9.3", LatestVersion: "6.0.0", Manager: "npm", Installed: true, Outdated: true, Description: "TypeScript is a language for application scale JavaScript"},
			{Name: "pnpm", Version: "10.20.0", Manager: "npm", Installed: true, Description: "Fast, disk space efficient package manager"},
			{Name: "ruff", Version: "0.14.0", Manager: "uv-tool", Installed: true, Description: "An extremely fast Python linter and code formatter"},
			{Name: "httpie", Version: "3.2.4", Manager: "pip", Installed: true, Description: "HTTPie: command-line HTTP client for the API era"},
		},
	}
	u := &UI{data: d, history: []Page{PagePackages}}
	u.installed = &listView{data: d, allowRemove: true}
	u.outdated = &listView{data: d, outdatedOnly: true}
	u.search = &searchView{data: d}
	u.page = PagePackages
	switch os.Getenv("UNIPAC_SCREENSHOT_PAGE") {
	case "settings":
		u.page = PageSettings
	case "search":
		u.page = PageSearch
	case "outdated":
		u.page = PageOutdated
	}
	if os.Getenv("UNIPAC_SCREENSHOT_DIALOG") != "" {
		u.page = PagePackages
		u.installed.sync()
		u.installed.uninstallOpen = true
		u.installed.uninstallTarget = u.installed.rows[0]
	}
	if os.Getenv("UNIPAC_SCREENSHOT_SELECTED") != "" {
		u.installed.sync()
		// Selection lives in chosen; body() rebuilds the table's selection from
		// it every frame, so writing to selection here would be discarded.
		u.installed.choose(u.installed.rows[1], true)
		u.installed.choose(u.installed.rows[2], true)
	}

	dark := os.Getenv("UNIPAC_SCREENSHOT_DARK") != ""
	const w, h = 1180, 720
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(dark))
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Width(220).FillHeight().
				Background(c.Theme().Surface.Alpha(1)).
				Children(func() { u.left(c) })
			ui.Column(c).Grow(1).FillHeight().Children(func() { u.content(c) })
		})
	}, w, h)

	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatalf("encode: %v", err)
	}
	t.Logf("wrote %s", out)
}
