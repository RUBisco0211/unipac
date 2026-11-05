package parts

import (
	"testing"
	"time"

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
		ui.Column(c).Fill().Children(func() { LoadingBar(c).Width(240) })
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
