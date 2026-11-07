package parts

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"
)

// detail is the backdrop a dialog blurs: stripes of text, which are what makes a
// blurred backdrop measurable — a blur turns their hard edges into gradients.
func detail(c *ui.Context) {
	ui.Column(c).Fill().Padding(12).Gap(6).Children(func() {
		for i := 0; i < 8; i++ {
			components.Text(c, "The quick brown fox jumps over the lazy dog, twice over").
				FontSize(13)
		}
	})
}

// rough sums the change between side-by-side pixels of the window: a sharp frame
// scores high, a blurred one low, whether or not it is also dimmed.
func rough(tt *ui.Tester) int {
	img := tt.Image()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	total := 0
	for y := 0; y < h; y++ {
		for x := 1; x < w; x++ {
			a, b := img.At(x-1, y), img.At(x, y)
			r1, g1, b1, _ := a.RGBA()
			r2, g2, b2, _ := b.RGBA()
			total += absDiff(r1, r2) + absDiff(g1, g2) + absDiff(b1, b2)
		}
	}
	return total
}

func absDiff(a, b uint32) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// A dialog blurs the window behind it. The dim alone would only darken it, which
// keeps the edges behind the panel as sharp as they were; the blur is what makes
// the frame behind a dialog calm.
func TestDialogBlursItsBackdrop(t *testing.T) {
	open := false
	var tt *ui.Tester
	tt = ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Onevra(c))
		detail(c)
		Dialog(c, &open, components.DialogOptions{Label: "Test"}, func(panel ui.Element) {
			panel.Width(320).Children(func() { components.Text(c, "a dialog") })
		})
	}, 640, 320)
	tt.Frame()
	before := rough(tt)

	open = true
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	after := rough(tt)

	if after >= before*3/4 {
		t.Errorf("the frame behind the dialog is as sharp as it was: %d, was %d", after, before)
	}
}

// A closed dialog leaves the window as it was.
func TestClosedDialogLeavesNoLayer(t *testing.T) {
	open := false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Onevra(c))
		detail(c)
		Dialog(c, &open, components.DialogOptions{Label: "Test"}, func(panel ui.Element) {
			panel.Children(func() { components.Text(c, "a dialog") })
		})
	}, 640, 320)
	tt.Frame()
	if tt.HasText("a dialog") {
		t.Errorf("a closed dialog drew its panel: %v", tt.Texts())
	}
}
