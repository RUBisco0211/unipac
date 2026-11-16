package parts

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// dialogBlur is how much the window behind a dialog blurs, in DIPs. The dim alone
// leaves the table behind a dialog as legible as the dialog's own text, which
// makes both harder to read; blurring it is what puts the dialog in front.
const dialogBlur = 8

// dialogFade is how a dialog's layers enter: Onevra's dialogs fade their dim over
// the same span, and the blur fades with it.
var dialogFade = ui.ElementTransition{
	Duration: 180 * time.Millisecond,
	Colors:   true,
	Enter:    &ui.Motion{Opacity: 0},
}

// Dialog is an Onevra dialog whose backdrop blurs the window behind it. Every
// dialog in the app goes through it, so they look alike.
//
// The blur is a layer of its own under the dialog's dim: an overlay's layers are
// painted in order, and a material samples what was painted before it, so the
// blur sees the page and the dim darkens the blurred result rather than the other
// way round. It takes no input, so clicks still reach the dialog's backdrop and
// dismiss it as before.
func Dialog(c *ui.Context, open *bool, options components.DialogOptions, build func(panel ui.Element)) {
	if *open {
		ui.Overlay(c, func() {
			ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
				PassThrough().Material(glass.Blur{Radius: dialogBlur}).Transition(dialogFade)
		})
	}
	components.Dialog(c, open, options, func(panel ui.Element) {
		centerPanel(c, panel, options.Inset)
		if build != nil {
			build(panel)
		}
	})
}

// panelGuess is the height assumed for a panel that has not been laid out yet,
// so the first frame of the opening fade already sits close to its place.
const panelGuess = 160

// centerPanel puts a dialog's panel in the middle of the window. Onevra biases
// it upward — of the space left over, twice as much goes below the panel as
// above it — which reads as too high for the app's short questions.
//
// A top margin is what moves it: the spacers around the panel are 1:2, so a
// margin of m moves the panel down by 2m/3, and a quarter of the spare space
// centers it. The height is the panel's own from the pass before; a dialog keeps
// its height while it opens, so this settles on the second frame, inside the
// fade. MaxHeight drops by the same margin, so a panel as tall as the room
// allows still fits between the window's edges.
func centerPanel(c *ui.Context, panel ui.Element, inset float32) {
	_, h := c.Size()
	if inset <= 0 {
		inset = 20
	}
	room := h - 2*inset
	if room <= 0 {
		return
	}
	height := min(panel.LayoutBounds().H, room)
	if height <= 0 {
		height = min(panelGuess, room)
	}
	margin := (room - height) / 4
	panel.Margin(margin, 0, 0, 0).MaxHeight(room - margin)
}
