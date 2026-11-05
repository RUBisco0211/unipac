package panel

import (
	"testing"
	"time"

	"github.com/RUBisco0211/unipac/source/views/internal/testui"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/theme"
)

// page builds a content area and a panel, each with a text of its own.
func page(s *State) func(*ui.Context) {
	return func(c *ui.Context) {
		View(c, s, func(c *ui.Context) {
			components.Text(c, "content")
		}, func(c *ui.Context) {
			components.Text(c, "side")
		})
	}
}

// Closing the panel never remounts the content: the panel is the thing that
// comes and goes, and the content keeps its parent, so every frame of the slide
// still shows it.
func TestClosingNeverRemountsTheContent(t *testing.T) {
	s := &State{Width: 380}
	tt := testui.Render(page(s), 1000, 400)
	tt.Frame()
	if !tt.HasText("side") {
		t.Fatalf("the panel is not there to close: %v", tt.Texts())
	}

	s.Toggle()
	for frame := range 30 {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if !tt.HasText("content") {
			t.Fatalf("the content vanished on frame %d of the slide", frame)
		}
	}
	if tt.HasText("side") {
		t.Error("the panel is still shown")
	}

	// Opening it again brings it back.
	s.Toggle()
	tt.Frame()
	for range 30 {
		tt.Advance(16 * time.Millisecond)
	}
	tt.Frame()
	if !tt.HasText("side") {
		t.Errorf("the panel did not come back: %v", tt.Texts())
	}
}

// The panel's edge is a drag: it widens and narrows the panel within its
// limits, and a double click puts the panel away.
func TestEdgeDragsAndCloses(t *testing.T) {
	const width = 1000
	s := &State{Width: 380}
	tt := testui.Render(page(s), width, 400)
	tt.Frame()

	// The edge sits just inside the panel's left edge, which the window limits
	// as the content keeps its share.
	edgeAt := func() float32 { return width - min(s.Width, width-MinContent) + 2 }
	y := float32(200)

	x := edgeAt()
	tt.Press(x, y)
	tt.Move(x-60, y)
	tt.Advance(16 * time.Millisecond)
	tt.Release(x-60, y)
	tt.Frame()
	if s.Width <= 380 {
		t.Errorf("dragging left gave the panel %.0f, was 380", s.Width)
	}

	// Clamped, not unbounded: dragging right reaches the minimum and stops.
	s.Width = 600
	tt.Frame()
	x = edgeAt()
	tt.Press(x, y)
	tt.Move(x+400, y)
	tt.Advance(16 * time.Millisecond)
	tt.Release(x+400, y)
	tt.Frame()
	if s.Width != MinWidth {
		t.Errorf("dragging right gave %.0f, want the minimum %.0f", s.Width, float32(MinWidth))
	}

}

// A drag widens and narrows within the limits, opens a closed panel, and lets go
// when the pointer lifts away.
func TestDragBy(t *testing.T) {
	s := &State{Width: 380}
	s.dragBy(-60, true, true)
	if s.Width != 440 {
		t.Errorf("dragging left 60 gave %.0f, want 440", s.Width)
	}
	if s.drag != s.Width {
		t.Errorf("the drag holds %.0f while the panel is %.0f wide", s.drag, s.Width)
	}

	s.dragBy(400, true, true)
	if s.Width != MinWidth {
		t.Errorf("dragging right 400 gave %.0f, want the minimum %.0f", s.Width, float32(MinWidth))
	}

	s.dragBy(-1000, false, false)
	if s.drag != 0 {
		t.Errorf("the drag did not end: %v", s.drag)
	}

	// A panel with no width of its own begins a drag at the default rather than at
	// nothing, so the first pixel gives it a size.
	empty := &State{Closed: true}
	empty.dragBy(-10, true, true)
	if empty.Width != DefaultWidth+10 || empty.Closed {
		t.Errorf("dragging an empty panel gave %.0f and closed=%v", empty.Width, empty.Closed)
	}
}

// The title bar's button names what the press will do, and toggles.
func TestTitleActionToggles(t *testing.T) {
	s := &State{}
	labels := Labels{Show: "show-panel", Hide: "hide-panel"}
	tt := testui.Render(func(c *ui.Context) {
		ui.Row(c).Children(func() { TitleAction(c, s, labels) })
	}, 200, 48)
	tt.Frame()

	if !tt.HasText(labels.Hide) {
		t.Fatalf("an open panel's toggle does not offer to hide it: %v", tt.Texts())
	}
	if err := tt.Click(labels.Hide); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if !s.Closed {
		t.Error("the press did not close the panel")
	}
	if !tt.HasText(labels.Show) {
		t.Errorf("the toggle still offers to hide a closed panel: %v", tt.Texts())
	}
}

// A hairline on the panel's edge tells it from the content beside it, and it
// travels with the edge as the panel slides.
func TestPanelDrawsItsEdge(t *testing.T) {
	const width = 1000
	pal := theme.Palette(false)
	s := &State{Width: 380}
	tt := testui.Render(page(s), width, 400)
	tt.Frame()

	// The theme's 8-bit channels against the image's 16-bit ones.
	c16 := func(v uint8) uint32 { return uint32(v) * 257 }
	br, bg, bb := c16(pal.Border.R), c16(pal.Border.G), c16(pal.Border.B)
	line := func() int {
		img := tt.Image()
		for x := 1; x < width; x++ {
			r, g, b, _ := img.At(x, 200).RGBA()
			if r == br && g == bg && b == bb {
				return x
			}
		}
		return -1
	}
	if got := line(); got != width-380 {
		t.Errorf("the edge is at %d, want the panel's own edge at %d", got, width-380)
	}

	// Halfway through closing, the line has moved with the edge, not stayed put.
	s.Toggle()
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	moved := line()
	if moved <= width-380 {
		t.Errorf("the edge stayed at %d while the panel slid", moved)
	}
}
