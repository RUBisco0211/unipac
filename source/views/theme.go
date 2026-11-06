// The palette every window draws in: Onevra's, with the theme color the user
// chose in Settings in place of Onevra's terracotta.
package views

import (
	"math"

	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// theme is the Shell theme of both windows. It runs on every frame, so a theme
// color saved in Settings shows as soon as a window draws again — no window
// needs to be told about the change.
func (u *UI) theme(c *ui.Context) *ui.Theme {
	t := theme.Onevra(c)
	accent, ok := parts.AccentColor(u.accent(), t.Dark)
	if !ok {
		return t
	}
	// The same three tints Onevra derives from its own accent, so a chosen
	// color behaves like the default one: hover and press darken on a light
	// frame and lighten on a dark one.
	t.Accent = accent
	if t.Dark {
		t.AccentHover = accent.Mix(ui.RGB(255, 255, 255), .12)
		t.AccentPressed = accent.Mix(ui.RGB(0, 0, 0), .12)
	} else {
		t.AccentHover = accent.Mix(ui.RGB(0, 0, 0), .12)
		t.AccentPressed = accent.Mix(ui.RGB(0, 0, 0), .25)
	}
	t.AccentText = readableOn(accent)
	t.Selection = accent.Alpha(.22)
	t.Focus = accent.Alpha(.6)
	return t
}

// accent is the theme color to draw in: what the settings page holds, which is
// the saved one until a write reports another. The page is the one place both
// windows agree on, so it is read here rather than kept in a second field.
func (u *UI) accent() string {
	if u.settings == nil {
		return ""
	}
	return u.settings.Accent()
}

// readableOn is the text color drawn on fill: white or near-black, whichever
// contrasts more, so a light accent keeps its labels readable and a dark one
// does not wash them out.
func readableOn(fill ui.Color) ui.Color {
	if luminance(fill) > .45 {
		return ui.Hex("#1c1a17")
	}
	return ui.RGB(255, 255, 255)
}

// luminance is the relative luminance of a color, as WCAG defines it.
func luminance(c ui.Color) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= .04045 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}
