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
	components.Dialog(c, open, options, build)
}
