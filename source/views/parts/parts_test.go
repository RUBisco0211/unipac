package parts

import (
	"strings"
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/theme"
)

// TestLoadingBarPaints checks the indeterminate bar actually draws something:
// the sweep is time-based, so the clock is advanced into the middle of a pass.
func TestLoadingBarPaints(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		box := ui.Box(c).FillWidth().Height(12)
		LoadingBar(c, box)
	}, 240, 12)
	tt.Advance(600 * time.Millisecond)

	img := tt.Image()
	b := img.Bounds()
	colors := map[uint32]struct{}{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			colors[uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(bl>>8)] = struct{}{}
		}
	}
	if len(colors) < 2 {
		t.Fatalf("loading bar painted one colour only: %v", colors)
	}
}

// It paints on the box it is given, so a page beside a panel keeps the bar to
// its own share: an absolute element measured the window instead.
func TestLoadingBarStopsAtItsBox(t *testing.T) {
	const (
		boxWidth = 200
		window   = 600
	)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		ui.Row(c).Fill().Children(func() {
			content := ui.Column(c).Width(boxWidth).Shrink(0).FillHeight()
			content.Children(func() {
				box := ui.Box(c).FillWidth().Height(40)
				LoadingBar(c, box)
			})
			// Beside it, plain page: nothing of the bar may show here.
			ui.Column(c).Grow(1)
		})
	}, window, 40)
	tt.Advance(600 * time.Millisecond)

	img := tt.Image()
	beside := img.At(window-1, 1)
	for x := boxWidth + 1; x < window; x++ {
		for y := 0; y < 3; y++ {
			if img.At(x, y) != beside {
				t.Fatalf("the bar reaches x=%d, y=%d, past its %d wide box", x, y, boxWidth)
			}
		}
	}
	// And inside the box it does paint, or the test above would pass on a bar
	// that never showed.
	inside := img.At(boxWidth/2, 0)
	if inside == beside {
		t.Errorf("nothing was painted on the box: %v", inside)
	}
}

// Every title bar icon button sits in the same box, so the tasks button and a
// page's own share a height and a centre whatever their icons are.
func TestTitleButtonsShareOneBox(t *testing.T) {
	tt := testui.Render(func(c *ui.Context) {
		ui.Row(c).FillWidth().FillHeight().AlignItems(ui.Center).Gap(2).Children(func() {
			TitleButton(c, lucide.ListTodo(), "tasks-button")
			TitleButton(c, lucide.PanelRightClose(), "page-button")
		})
	}, 200, 48)
	tt.Frame()

	tasks, ok := tt.Find("tasks-button")
	if !ok {
		t.Fatal("the tasks button is missing")
	}
	page, ok := tt.Find("page-button")
	if !ok {
		t.Fatal("the page button is missing")
	}
	if tasks.H != page.H || tasks.Y != page.Y {
		t.Errorf("buttons differ: tasks %+v, page %+v", tasks, page)
	}
	// The box is what they line up in: the width of the row's own spacing, and a
	// height of one unit less, centred on the bar.
	if want := theme.Palette(false).Space(8); tasks.W != want {
		t.Errorf("button width = %.0f, want %.0f", tasks.W, want)
	}
}

// The capability badges name every capability in the adapter guide's order, and
// the text form says which are missing.
func TestCapabilityBadges(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		CapabilityBadges(c, manager.Capabilities{ListInstalled: true, Search: true, Update: true})
	}, 600, 120)
	tt.Frame()

	for _, want := range []string{
		locales.CapabilityInstalled(), locales.CapabilityOutdated(), locales.CapabilityPackageInfo(),
		locales.CapabilitySearch(), locales.CapabilityInstall(), locales.CapabilityUninstall(),
		locales.CapabilityUpdate(), locales.CapabilityVersions(),
	} {
		if !tt.HasText(want) {
			t.Errorf("capability %q is missing: %v", want, tt.Texts())
		}
	}
}

// The text form names the gaps for a place with no room for badges, in the same
// order, and says nothing when there are none.
func TestUnsupportedCapabilities(t *testing.T) {
	all := manager.Capabilities{
		ListInstalled: true, ListOutdated: true, Detail: true, Search: true,
		Install: true, Uninstall: true, Update: true, ListVersions: true,
	}
	if got := UnsupportedCapabilities(all); got != "" {
		t.Errorf("a manager offering everything says %q", got)
	}
	want := locales.SettingsUnsupported(strings.Join([]string{
		locales.CapabilityOutdated(), locales.CapabilityPackageInfo(),
		locales.CapabilityInstall(), locales.CapabilityUninstall(),
		locales.CapabilityVersions(),
	}, locales.CommonListSeparator()))
	if got := UnsupportedCapabilities(manager.Capabilities{ListInstalled: true, Search: true, Update: true}); got != want {
		t.Errorf("unsupported list = %q, want %q", got, want)
	}
}

func TestPackagePageSummarySelects(t *testing.T) {
	pkg := manager.Package{Name: "ripgrep", Manager: "brew", Version: "14.1.1"}
	page := &PackagePage{}
	page.Show(pkg)
	// One paragraph that wraps: the summary as a manager sends it.
	const summary = "ripgrep is a line-oriented search tool that recursively searches the current directory for a regex pattern, respecting .gitignore and skipping hidden and binary files, so it is fast and safe to run in a large repository."
	page.Apply(pkg, manager.Detail{Summary: summary}, nil)
	tt := testui.Render(func(c *ui.Context) {
		page.Build(c, PackagePageOptions{})
	}, 1024, 740)
	tt.Frame()

	// One text of two lines: a drag from the first into the second takes both.
	block, ok := tt.Find(summary)
	if !ok {
		t.Fatalf("the summary is missing: %v", tt.Texts())
	}
	if block.H < 24 {
		t.Fatalf("the summary is one line tall (%.0f): it does not wrap", block.H)
	}
	tt.Move(block.X+2, block.Y+block.H/4)
	tt.Press(block.X+2, block.Y+block.H/4)
	tt.Move(block.X+200, block.Y+block.H*3/4)
	tt.Release(block.X+200, block.Y+block.H*3/4)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Frame()
	if got := tt.Clipboard(); got != summary {
		t.Errorf("the selection copied %q, want the whole summary", got)
	}

	// The page's own controls still work: the selection scope does not swallow
	// the close button.
	if err := tt.Click(locales.DetailClose()); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if page.Open {
		t.Error("the close button did not close the page")
	}
}
