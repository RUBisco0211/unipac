package views

import (
	"testing"

	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/RUBisco0211/unipac/source/views/settings"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// accentService reports a saved theme color, which is all the theme needs.
type accentService struct {
	wiredService
	accent string
}

func (s accentService) Accent() string    { return s.accent }
func (s accentService) Accents() []string { return []string{"", "blue"} }

// accentTheme builds the frame's palette for the saved color, in the given
// appearance.
func accentTheme(t *testing.T, accent string, dark bool) *ui.Theme {
	t.Helper()
	u := &UI{settings: settings.New(accentService{accent: accent}, testui.Data())}
	var got *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) { got = u.theme(c) }, 100, 100)
	tt.SetDark(dark)
	tt.Frame()
	if got == nil {
		t.Fatal("the theme was not built")
	}
	return got
}

// The default keeps the toolkit's own accent, which is what the app has always
// drawn in.
func TestThemeKeepsOnevrasAccentByDefault(t *testing.T) {
	for _, dark := range []bool{false, true} {
		got := accentTheme(t, "", dark)
		want := theme.Palette(dark).Accent
		if got.Accent != want {
			t.Errorf("dark=%v accent = %v, want the toolkit's %v", dark, got.Accent, want)
		}
	}
}

// A chosen color replaces the accent, with the tints the controls use derived
// from it: hover and press darken on a light frame and lighten on a dark one.
func TestThemeFollowsTheChosenAccent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		got := accentTheme(t, "blue", dark)
		want, ok := parts.AccentColor("blue", dark)
		if !ok {
			t.Fatal("blue has no color")
		}
		if got.Accent != want {
			t.Errorf("dark=%v accent = %v, want %v", dark, got.Accent, want)
		}
		if got.AccentHover == got.Accent || got.AccentPressed == got.Accent {
			t.Errorf("dark=%v the accent's tints were not derived: %v %v", dark, got.AccentHover, got.AccentPressed)
		}
		if got.Selection == (ui.Color{}) || got.Focus == (ui.Color{}) {
			t.Errorf("dark=%v selection and focus do not follow the accent", dark)
		}
		// The text drawn on the accent has to stay readable on it, whichever
		// color the user picked.
		if got.AccentText == got.Accent {
			t.Errorf("dark=%v accent text is the accent itself", dark)
		}
	}
}

// A color this build does not know draws the default rather than nothing.
func TestUnknownAccentDrawsTheDefault(t *testing.T) {
	got := accentTheme(t, "chartreuse", false)
	if want := theme.Palette(false).Accent; got.Accent != want {
		t.Errorf("accent = %v, want the toolkit's %v", got.Accent, want)
	}
}
