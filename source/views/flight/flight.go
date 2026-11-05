// Package flight animates the package icon that flies from the control the user
// pressed to the tasks button, so a started task is visibly somewhere.
package flight

import (
	"time"

	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// Origin is where the user pressed the control that started a task, in window
// coordinates. A package icon flies from there to the tasks button.
type Origin struct {
	X, Y float32
}

// OriginOf is el's press point, or its centre when the pointer has already left
// (a hold-to-confirm gesture releases over the same control, so this is mostly
// the exact press point).
func OriginOf(el ui.Element) Origin {
	r := el.Bounds()
	if x, y, over := el.PointerPosition(); over {
		return Origin{X: r.X + x, Y: r.Y + y}
	}
	return Origin{X: r.X + r.W/2, Y: r.Y + r.H/2}
}

// Flight is the package icon on its way from a task's control to the tasks
// button in the title bar.
type Flight struct {
	active bool
	// fresh means the next build must jump to the start before animating; it is
	// what makes a second flight begin at the new control rather than at the
	// previous flight's end.
	fresh   bool
	started time.Time
	sx, sy  float32
	ex, ey  float32
}

// flightDuration is short on purpose: the icon is a signal that the task
// started, not something to watch. A fixed curve, rather than a spring, also
// means it disappears as it lands instead of settling on the button.
const flightDuration = 300 * time.Millisecond

// ToTasks starts the flight for a task that just began. target is the tasks
// button's last laid-out rectangle.
func (f *Flight) ToTasks(at Origin, target ui.Rect) {
	r := target
	if r.W <= 0 || r.H <= 0 || (at.X == 0 && at.Y == 0) {
		return
	}
	*f = Flight{
		active:  true,
		fresh:   true,
		started: time.Now(),
		sx:      at.X,
		sy:      at.Y,
		ex:      r.X + r.W/2,
		ey:      r.Y + r.H/2,
	}
}

// Active reports whether an icon is still on its way, which a test needs to
// assert when the flight ends.
func (f *Flight) Active() bool {
	return f.active
}

// Draw paints the flying package over the page. It is absolute, so it takes no
// layout room, and paint-only frames would not move it; AnimateTo keeps
// rebuilding the little subtree until it arrives.
func (f *Flight) Draw(c *ui.Context, root ui.Element) {
	if !f.active {
		return
	}
	if time.Since(f.started) > 2*flightDuration {
		// A stall or a disabled animation must not leave the icon behind.
		f.active = false
		return
	}
	t := c.Theme()
	sx, sy, ok := root.WindowToLocal(f.sx, f.sy)
	if !ok {
		f.active = false
		return
	}
	ex, ey, _ := root.WindowToLocal(f.ex, f.ey)

	size := t.Space(7)
	half := size / 2
	box := ui.Box(c).Key("job-flight").Absolute().
		Left(sx - half).Top(sy - half).Width(size).Height(size).
		Radius(half).Background(t.Accent).Center().
		Label(locales.TasksFlight())
	box.Children(func() {
		components.Icon(c, lucide.Package()).Size(t.Space(4), t.Space(4)).TextColor(t.AccentText)
	})

	if f.fresh {
		// The animation state is new for this flight, so place it at the press
		// point before asking it to travel.
		box.Jump("job-flight-x", sx)
		box.Jump("job-flight-y", sy)
		f.fresh = false
	}
	animation := ui.Animation{Duration: flightDuration, Ease: ui.EaseOut}
	x := box.AnimateTo("job-flight-x", ex, animation)
	y := box.AnimateTo("job-flight-y", ey, animation)
	box.Left(x - half).Top(y - half)

	if box.Arrived("job-flight-x") {
		f.active = false
	}
}
