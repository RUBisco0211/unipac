// The settings window: Onevra's settings surface in a window of its own, beside
// the window that lists packages.
//
// Settings are a config file the user changes now and then, not a place to
// browse packages from, so they do not take a page in the main window. A window
// of their own can also stay open while a manager's path is applied and the
// lists in the other window answer, which is the one thing the two share.
package views

import (
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/shells/plain"
)

// showSettings brings up the settings window, creating it the first time it is
// asked for.
//
// The window is built on demand and destroyed when it is closed, rather than
// created hidden and shown: MyGo asks every window to close when the app quits,
// so a window that hides instead of closing would have to tell a quit from a
// click and answer both. MyGo remembers its bounds under StateKey either way;
// the surface's own state, such as the chosen category, lives on the page and
// survives the window.
func (u *UI) showSettings() {
	if u.settingsShell == nil {
		u.settingsShell = plain.NewShell(plain.ShellOptions{
			Title:     locales.NavSettings(),
			Theme:     u.theme,
			StateKey:  "settings",
			Width:     720,
			Height:    600,
			MinWidth:  420,
			MinHeight: 400,
			// The surface names itself in its sidebar's heading, so the strip
			// above it would otherwise say "Settings" twice, 48 points apart. An
			// empty leading slot keeps the strip draggable and the native window
			// title, which the window menu reads, unchanged.
			HeaderLeading: func(*ui.Context) {},
			// The surface runs to the window's top edge, so the window's controls
			// sit on the surface rather than on a band of background above it.
			// The surface's own heading row is the clearance they need.
			ContentBehindHeader: true,
			Content:             u.settingsContent,
			Toasts:              toastOptions,
		})
		u.settingsShell.Window().OnClosed(func() { u.settingsShell = nil })
	}
	win := u.settingsShell.Window()
	win.Show()
	win.Focus()
}

// settingsContent is the settings window's content: the surface alone, running
// behind the window's drag strip. The shell supplies the theme and the drag
// behaviour, so the page brings only its categories.
func (u *UI) settingsContent(c *ui.Context) {
	if u.settings == nil {
		return
	}
	// The page cannot know what chrome sits over its top: the window does.
	u.settings.TopInset = plain.HeaderHeight(c)
	u.settings.Build(c)
}

// invalidateSettings repaints the settings window when it is open, which is what
// moves the package counts a refresh changed and what a manager's save publishes
// through.
func (u *UI) invalidateSettings() {
	if u.settingsShell != nil {
		u.settingsShell.Window().Invalidate()
	}
}
