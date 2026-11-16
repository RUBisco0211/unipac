// Package dock is the console dock along the bottom of a page: a header row
// that is always there, a body that opens above it, and a top edge that is a
// drag. It is Kero's side panel turned on its side: the content grows into
// whatever the dock leaves, and the dock is a fixed height animated to and from
// its header. The content's parent never changes, so opening and closing the
// dock never remounts it.
//
// The header stays while the dock is folded, so a folded dock still names what
// it holds and offers the control that opens it again.
package dock

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The dock's own height, its limits, the header it keeps when folded, and what
// the content above it keeps when the window is too short for both.
const (
	DefaultHeight = 240
	MinHeight     = 96
	MaxHeight     = 640
	// HeaderHeight is the summary row the dock always shows, and the height the
	// body opens under.
	HeaderHeight = 40
	MinContent   = 240
)

// Motion moves the dock. A curve, not a spring: a spring's tail crawls under a
// pixel a frame, which the whole pixels of a moving edge turn into a hop. This
// is the curve Kero's sidebars move on.
var Motion = ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.CubicBezier(0.25, 1, 0.5, 1)}

// State is one dock's: its height, whether it is folded, and a drag in flight.
// Keep it in the page's retained state.
type State struct {
	// ID names the dock in the element tree, so two docks on one page animate
	// apart. It may be empty for a page with one.
	ID string
	// Height is the body's height in DIPs, which a drag of the top edge sets.
	// Zero means DefaultHeight.
	Height float32
	// Closed folds the body away, leaving the header. It is a closed flag so a
	// state built without it shows the body.
	Closed bool
	// folded says the user folded it, so what the page does on its own — choosing
	// a task, say — leaves it folded. Toggle and Expand clear it.
	folded bool
	// Label names the edge for assistive technology, which a page words for what
	// the dock holds.
	Label string
	// drag is the height as a drag began, 0 while none.
	drag float32
}

// Toggle opens or folds the dock.
func (s *State) Toggle() {
	s.Closed = !s.Closed
	s.folded = s.Closed
}

// Expand opens the dock, which a page does when the user asks for what it holds.
// Unlike Show it overrides a fold: the user asked for the body by name.
func (s *State) Expand() {
	s.Closed = false
	s.folded = false
}

// Show opens the dock unless the user folded it, which a page does on its own —
// choosing a task, say — without undoing that choice.
func (s *State) Show() {
	if !s.folded {
		s.Closed = false
	}
}

// Folded reports whether the user folded the dock, so a page can tell an empty
// body from one that is put away.
func (s *State) Folded() bool { return s.folded }

// height is the body's height, within the limits and never more than the window
// can give while the content above keeps its share.
func (s *State) height(c *ui.Context) float32 {
	height := s.Height
	if height <= 0 {
		height = DefaultHeight
	}
	height = min(max(height, MinHeight), MaxHeight)
	if _, win := c.Size(); win > 0 {
		height = min(height, max(MinHeight, win-MinContent-HeaderHeight))
	}
	return height
}

// key names one of the dock's elements, so two docks on a page stay apart.
func (s *State) key(part string) string {
	if s.ID == "" {
		return "dock-" + part
	}
	return "dock-" + s.ID + "-" + part
}

// View builds content above and the dock below it, filling the space it is
// given. header is always built; body only while the dock is open or sliding,
// and a nil body — nothing to show yet — leaves the dock its header alone.
func View(c *ui.Context, s *State, content, header, body func(*ui.Context)) {
	if s == nil {
		panic("dock.View: nil state")
	}
	height := s.height(c)
	column := ui.Column(c).Fill().AlignItems(ui.Stretch)
	progress := clamp01(column.AnimateTo(s.key("progress"), target(s.Closed), Motion))
	// Whole pixels: the edge moves, and a sub-pixel height reads as a hop.
	shown := float32(0)
	if body != nil {
		shown = float32(math.Round(float64(height * progress)))
	}
	column.Children(func() {
		// The content grows into whatever the dock leaves, and keeps the same
		// parent whether the dock is open or not. It is clipped on the bottom
		// edge: a table taller than the room would otherwise draw under the dock,
		// which paints over it only where its own background is opaque.
		ui.Column(c).Key(s.key("content")).Grow(1).MinHeight(0).ClipY().Children(func() {
			if content != nil {
				content(c)
			}
		})
		s.pane(c, shown, header, body)
	})
}

// pane is the dock's box: its edge, its header, and its body while it is open.
func (s *State) pane(c *ui.Context, shown float32, header, body func(*ui.Context)) {
	t := c.Theme()
	// The box is not clipped, so the edge can straddle its top: the grab band is
	// half in the content above, where the pointer expects to find it.
	box := ui.Column(c).Key(s.key("pane")).Shrink(0)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 || r.H <= 0 {
			return
		}
		p.Fill(r, t.Text.Alpha(0.03), 0)
		// The header's own line tells it from the body; the top edge is what
		// separates the dock from the content.
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 1}, t.Border, 0)
		if shown > 0 {
			p.Fill(ui.Rect{X: r.X, Y: r.Y + HeaderHeight, W: r.W, H: 1}, t.Border.Alpha(0.6), 0)
		}
	})
	box.Children(func() {
		ui.Column(c).Shrink(0).Height(HeaderHeight).ClipY().Children(func() {
			if header != nil {
				header(c)
			}
		})
		if shown > 0 {
			// The body keeps its height as it opens: this column narrowing is what
			// uncovers it, clipped.
			ui.Column(c).Shrink(0).Height(shown).ClipY().Children(func() {
				if body != nil {
					body(c)
				}
			})
		}
		s.edge(c)
	})
}

// edge is the dock's top edge: a drag of it sets the height, a double click
// folds the dock away.
func (s *State) edge(c *ui.Context) {
	drag := ui.Box(c).Key(s.key("edge")).Absolute().Left(0).Right(0).Top(-4).
		Height(8).Cursor(ui.CursorResizeNS).KeepFocus()
	drag.Label(s.Label)
	// The edge follows the vertical axis alone: a sideways drag of it is the
	// pointer's own.
	_, dy, held := drag.Dragged()
	s.dragBy(dy, held)
	// The double click comes after the drag because it is a press of an element
	// the drag makes interactive, and before the frame ends because the press it
	// ends is still held in the next one.
	if drag.DoubleClicked() {
		s.Closed = true
		s.folded = true
	}
}

// dragBy applies a drag of the edge: dy is how far the pointer moved, held says
// it is still down. The height follows the edge within its limits, and a drag
// that moves opens a folded dock, since the user is reaching into it.
//
// A press that does not move leaves the fold alone: the press that folds the
// dock with a double click is still held for the frame after it, and would
// otherwise open what it just folded.
func (s *State) dragBy(dy float32, held bool) {
	if !held {
		s.drag = 0
		return
	}
	if s.drag == 0 {
		s.drag = s.Height
		if s.drag <= 0 {
			s.drag = DefaultHeight
		}
	}
	// Dragging up opens the dock: the edge moves with the pointer.
	s.drag -= dy
	s.Height = min(max(s.drag, MinHeight), MaxHeight)
	if dy != 0 {
		s.Closed = false
		s.folded = false
	}
}

// target is where the animation is heading: 1 open, 0 folded.
func target(closed bool) float32 {
	if closed {
		return 0
	}
	return 1
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }
