package parts

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
