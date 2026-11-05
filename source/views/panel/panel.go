// Package panel is a side panel beside a page's content: it slides in from the
// window's edge, its width is a drag of that edge, and a double click puts it
// away. It is Kero's shape: a row of the content, which grows, and the panel,
// which is a fixed width animated to and from nothing. The content's parent
// never changes, so opening and closing the panel never remounts it.
package panel

import (
	"math"
	"time"

	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// The panel's own width, its limits, and what the content beside it keeps when
// the window is too narrow for both.
const (
	DefaultWidth = 380
	MinWidth     = 280
	MaxWidth     = 720
	MinContent   = 360
)

// Motion moves the panel. A curve, not a spring: a spring's tail crawls under a
// pixel a frame, which the whole pixels of a sliding edge turn into a hop. This
// is the curve Kero's sidebars move on.
var Motion = ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.CubicBezier(0.25, 1, 0.5, 1)}

// State is one panel's: its width, whether it is closed, and a drag in flight.
// Keep it in the page's retained state.
type State struct {
	// ID names the panel in the element tree, so two panels on one page animate
	// apart. It may be empty for a page with one.
	ID string
	// Width is the panel's own width in DIPs, which a drag of its edge sets.
	// Zero means DefaultWidth.
	Width float32
	// Closed puts the panel away; it is a closed flag so a state built without it
	// shows the panel.
	Closed bool
	// drag is the width as a drag began, 0 while none.
	drag float32
}

// Toggle opens or closes the panel.
func (s *State) Toggle() { s.Closed = !s.Closed }

// Show opens the panel, which a page does when the user asks for what it holds.
func (s *State) Show() { s.Closed = false }

// width is the panel's own width, within the limits and never more than the
// window can give while the content keeps its share.
func (s *State) width(c *ui.Context) float32 {
	width := s.Width
	if width <= 0 {
		width = DefaultWidth
	}
	width = min(max(width, MinWidth), MaxWidth)
	if win, _ := c.Size(); win > 0 {
		width = min(width, max(MinWidth, win-MinContent))
	}
	return width
}

// key names one of the panel's elements, so two panels on a page stay apart.
func (s *State) key(part string) string {
	if s.ID == "" {
		return "panel-" + part
	}
	return "panel-" + s.ID + "-" + part
}

// View builds content and the panel beside it, filling the space it is given.
// content may be nil for an empty side; the panel is built only while it is
// open or sliding.
func View(c *ui.Context, s *State, content, side func(*ui.Context)) {
	if s == nil {
		panic("panel.View: nil state")
	}
	width := s.width(c)
	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	progress := clamp01(row.AnimateTo(s.key("progress"), target(s.Closed), Motion))
	// Whole pixels: the edge slides, and a sub-pixel width reads as a hop.
	shown := float32(math.Round(float64(width * progress)))
	row.Children(func() {
		// The content grows into whatever the panel leaves, and keeps the same
		// parent whether the panel is there or not. It is clipped on the trailing
		// edge: a table wider than the room, or a row of toolbar controls that do
		// not wrap, would otherwise draw under the panel, which paints over them
		// only where its own background is opaque.
		ui.Column(c).Key(s.key("content")).Grow(1).MinWidth(0).ClipX().Children(func() {
			if content != nil {
				content(c)
			}
		})
		if shown > 0 {
			// The panel keeps its width as it slides: this row narrowing is what
			// moves it out past the window's edge, clipped. A hairline on its own
			// edge is what tells it from the content beside it.
			t := c.Theme()
			pane := ui.Row(c).Key(s.key("pane")).Width(shown).Shrink(0).AlignItems(ui.Stretch).ClipX()
			pane.Draw(func(p *ui.Painter, r ui.Rect) {
				if r.W > 0 && r.H > 0 {
					p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 1, H: r.H}, t.Border, 0)
				}
			})
			pane.Children(func() {
				ui.Column(c).Width(width).Shrink(0).AlignItems(ui.Stretch).
					Opacity(0.4 + 0.6*progress).Children(func() {
					if side != nil {
						side(c)
					}
				})
			})
			s.edge(c, shown)
		}
	})
}

// edge is the panel's edge: a drag of it sets the width, a double click puts the
// panel away.
func (s *State) edge(c *ui.Context, shown float32) {
	drag := ui.Box(c).Key(s.key("edge")).Absolute().Right(shown - 5).Top(0).Bottom(0).
		Width(9).Cursor(ui.CursorResizeColumn).KeepFocus()
	dx, _, held := drag.Dragged()
	s.dragBy(dx, held, drag.Pressed())
	if drag.DoubleClicked() {
		s.Closed = true
	}
}

// dragBy applies a drag of the edge: dx is how far the pointer moved, held says
// it is still down, and pressed whether it is over the edge at all. The width
// follows the edge within its limits, and a drag opens a closed panel, since the
// user is reaching into it.
func (s *State) dragBy(dx float32, held, pressed bool) {
	if !held {
		if s.drag != 0 && !pressed {
			s.drag = 0
		}
		return
	}
	if s.drag == 0 {
		s.drag = s.Width
		if s.drag <= 0 {
			s.drag = DefaultWidth
		}
	}
	// Dragging left widens the panel: the edge moves with the pointer.
	s.drag -= dx
	s.Width = min(max(s.drag, MinWidth), MaxWidth)
	s.Closed = false
}

// TitleAction builds the panel's toggle for a title bar, naming what the press
// will do.
func TitleAction(c *ui.Context, s *State, labels Labels) {
	label, glyph := labels.Hide, glyphOf(labels.HideIcon, lucide.PanelRightClose)
	if s.Closed {
		label, glyph = labels.Show, glyphOf(labels.ShowIcon, lucide.PanelRightOpen)
	}
	button := parts.TitleButton(c, glyph, label)
	components.Tooltip(c, button, label)
	if button.Clicked() {
		s.Toggle()
	}
}

// Labels word a panel's toggle; the icons default to the side panel's pair.
type Labels struct {
	Show, Hide string
	// ShowIcon and HideIcon may be nil for the default icons.
	ShowIcon, HideIcon func() *ui.SVG
}

// glyphOf falls back to the panel's own icon when a label set names none.
func glyphOf(chosen func() *ui.SVG, fallback func() *ui.SVG) *ui.SVG {
	if chosen != nil {
		return chosen()
	}
	return fallback()
}

// target is where the animation is heading: 1 shown, 0 away.
func target(closed bool) float32 {
	if closed {
		return 0
	}
	return 1
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }
