package dock

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"
)

// page builds a content area and a dock, each with a text of its own.
func page(s *State) func(*ui.Context) {
	return func(c *ui.Context) {
		View(c, s,
			func(c *ui.Context) { components.Text(c, "content") },
			func(c *ui.Context) { components.Text(c, "header") },
			func(c *ui.Context) { components.Text(c, "body") })
	}
}

// Closing never remounts the content: the dock is the thing that comes and
// goes, and the content keeps its parent, so every frame of the slide still
// shows it.
func TestClosingNeverRemountsTheContent(t *testing.T) {
	s := &State{Height: 240}
	tt := testui.Render(page(s), 1000, 600)
	tt.Frame()
	if !tt.HasText("body") {
		t.Fatalf("the dock is not there to close: %v", tt.Texts())
	}

	s.Toggle()
	for frame := range 30 {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if !tt.HasText("content") {
			t.Fatalf("the content vanished on frame %d of the slide", frame)
		}
	}
	if tt.HasText("body") {
		t.Error("the dock's body is still shown")
	}
	if !tt.HasText("header") {
		t.Errorf("the folded dock dropped its header: %v", tt.Texts())
	}

	// Opening it again brings the body back.
	s.Toggle()
	tt.Frame()
	for range 30 {
		tt.Advance(16 * time.Millisecond)
	}
	tt.Frame()
	if !tt.HasText("body") {
		t.Errorf("the body did not come back: %v", tt.Texts())
	}
}

// The dock's top edge is a drag: it opens the dock and sets its height within
// the limits, and a double click folds it away.
func TestEdgeDragsAndFolds(t *testing.T) {
	const win = 600
	s := &State{Height: 240, Label: "dock-edge"}
	tt := testui.Render(page(s), 1000, win)
	tt.Frame()

	// The edge is a band across the dock's top, over the header it covers.
	edge := func() (x, y float32) {
		t.Helper()
		r, ok := tt.Find("dock-edge")
		if !ok {
			t.Fatal("the edge is not on screen")
		}
		return r.X + r.W/2, r.Y + r.H/2
	}

	x, y := edge()
	tt.Move(x, y)
	tt.Press(x, y)
	tt.Move(x, y-60)
	tt.Release(x, y-60)
	tt.Frame()
	if s.Height <= 240 {
		t.Errorf("dragging up gave the dock %.0f, was 240", s.Height)
	}

	// Clamped, not unbounded: dragging down reaches the minimum and stops.
	s.Height = 400
	tt.Frame()
	x, y = edge()
	tt.Move(x, y)
	tt.Press(x, y)
	tt.Move(x, y+500)
	tt.Release(x, y+500)
	tt.Frame()
	if s.Height != MinHeight {
		t.Errorf("dragging down gave %.0f, want the minimum %.0f", s.Height, float32(MinHeight))
	}

	// A double click folds it away.
	x, y = edge()
	tt.ClickAt(x, y)
	tt.ClickAt(x, y)
	tt.Frame()
	if !s.Closed {
		t.Error("a double click did not fold the dock")
	}
}

// A drag opens and closes within the limits, opens a folded dock, and lets go
// when the pointer lifts away.
func TestDragBy(t *testing.T) {
	s := &State{Height: 240}
	s.dragBy(-60, true)
	if s.Height != 300 {
		t.Errorf("dragging up 60 gave %.0f, want 300", s.Height)
	}
	if s.drag != s.Height {
		t.Errorf("the drag holds %.0f while the dock is %.0f tall", s.drag, s.Height)
	}

	s.dragBy(400, true)
	if s.Height != MinHeight {
		t.Errorf("dragging down 400 gave %.0f, want the minimum %.0f", s.Height, float32(MinHeight))
	}

	s.dragBy(-1000, false)
	if s.drag != 0 {
		t.Errorf("the drag did not end: %v", s.drag)
	}

	// A dock with no height of its own begins a drag at the default rather than
	// at nothing, so the first pixel gives it a size.
	empty := &State{Closed: true, folded: true}
	empty.dragBy(-10, true)
	if empty.Height != DefaultHeight+10 || empty.Closed || empty.Folded() {
		t.Errorf("dragging a folded dock gave %.0f and closed=%v", empty.Height, empty.Closed)
	}
}

// What the page does on its own opens the dock, but leaves a dock the user
// folded folded; asking for it by name opens it.
func TestShowLeavesAFoldedDockFolded(t *testing.T) {
	s := &State{Height: 240}
	s.Toggle()
	if !s.Closed || !s.Folded() {
		t.Fatalf("the toggle left closed=%v folded=%v", s.Closed, s.Folded())
	}
	s.Show()
	if !s.Closed {
		t.Error("the page's own show undid the user's fold")
	}
	s.Expand()
	if s.Closed || s.Folded() {
		t.Errorf("expand left closed=%v folded=%v", s.Closed, s.Folded())
	}
}

// A dock with nothing to show is its header alone: an empty body would take the
// room a page could use, and say nothing in it.
func TestNilBodyLeavesTheHeaderAlone(t *testing.T) {
	const win = 600
	s := &State{Height: 240, Label: "dock-edge"}
	tt := testui.Render(func(c *ui.Context) {
		View(c, s,
			func(c *ui.Context) { components.Text(c, "content") },
			func(c *ui.Context) { components.Text(c, "header") }, nil)
	}, 1000, win)
	tt.Frame()

	if !tt.HasText("header") {
		t.Fatalf("the header is missing: %v", tt.Texts())
	}
	// The edge is the dock's top, so where it sits says how tall the dock is.
	r, ok := tt.Find("dock-edge")
	if !ok {
		t.Fatal("the edge is missing")
	}
	if want := float32(win - HeaderHeight - 4); r.Y != want {
		t.Errorf("the edge is at %.0f, want %.0f: the dock kept room for no body", r.Y, want)
	}
}

// A hairline on the dock's top edge tells it from the content above, and it
// travels with the edge as the dock opens.
func TestDockDrawsItsTopEdge(t *testing.T) {
	const (
		win  = 600
		wide = 1000
	)
	pal := theme.Palette(false)
	s := &State{Height: 240}
	tt := testui.Render(page(s), wide, win)
	tt.Frame()

	// The theme's 8-bit channels against the image's 16-bit ones.
	c16 := func(v uint8) uint32 { return uint32(v) * 257 }
	br, bg, bb := c16(pal.Border.R), c16(pal.Border.G), c16(pal.Border.B)
	line := func() int {
		img := tt.Image()
		for y := 1; y < win; y++ {
			r, g, b, _ := img.At(wide/2, y).RGBA()
			if r == br && g == bg && b == bb {
				return y
			}
		}
		return -1
	}
	if got, want := line(), win-HeaderHeight-240; got != want {
		t.Errorf("the edge is at %d, want the dock's own edge at %d", got, want)
	}

	// Halfway through folding, the line has moved down with the edge.
	s.Toggle()
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if moved := line(); moved <= win-HeaderHeight-240 {
		t.Errorf("the edge stayed at %d while the dock folded", moved)
	}
}
