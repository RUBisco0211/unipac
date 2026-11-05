// Package parts holds the presentational pieces the pages share: package table
// columns and cells, badges, the loading bar, placeholders, row action slots,
// separators and the labels a task's kind or message gets. Nothing here keeps
// state; a part is built, drawn and forgotten.
package parts

import (
	"math"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// managerHue derives a stable hue from a manager ID, matching the hash the old
// interface used so the same manager keeps the same color.
func managerHue(id string) float64 {
	hash := 0
	for _, r := range id {
		hash = (hash*31 + int(r)) % 360
	}
	return float64((hash + 360) % 360)
}

// hslToRGB converts HSL to RGB. Hue is in degrees, saturation and lightness in
// 0..1.
func hslToRGB(h, s, l float64) ui.Color {
	h = math.Mod(h, 360) / 360
	if s <= 0 {
		v := uint8(math.Round(l * 255))
		return ui.RGBA(v, v, v, 1)
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	channel := func(t float64) uint8 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		var v float64
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 0.5:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		default:
			v = p
		}
		return uint8(math.Round(v * 255))
	}
	return ui.RGBA(channel(h+1.0/3), channel(h), channel(h-1.0/3), 1)
}

// ManagerBadge renders the manager label in a badge tinted by the manager ID.
// It reuses components.Badge so both badges in a row share their geometry, and
// only overrides the colors: Onevra's Badge offers status tones, not a
// per-manager hue.
func ManagerBadge(c *ui.Context, id, label string) {
	t := c.Theme()
	saturation, lightness := 0.68, 0.42
	backgroundAlpha, borderAlpha := float32(0.12), float32(0.22)
	if t.Dark {
		saturation, lightness = 0.72, 0.68
		backgroundAlpha, borderAlpha = 0.16, 0.28
	}
	tint := hslToRGB(managerHue(id), saturation, lightness)
	badge := components.Badge(c, label)
	badge.AlignSelf(ui.Center).
		Background(tint.Alpha(backgroundAlpha)).
		TextColor(tint).
		Border(1, tint.Alpha(borderAlpha))
}

// LoadingBar draws a thin indeterminate bar: a highlight sweeping across while
// the table loads or reloads.
func LoadingBar(c *ui.Context) ui.Element {
	t := c.Theme()
	// Absolute, so showing it never moves what is under it.
	e := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(2).
		Background(t.Border.Alpha(0.35)).Clip()
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 || r.H <= 0 {
			return
		}
		// Keep the bar moving on its own; nothing else asks for these frames.
		p.AnimationFrameRate(60)
		const (
			span   = 0.28 // the highlight's share of the width
			period = 1.2  // seconds per sweep
		)
		seconds := float64(p.Now().UnixNano()) / 1e9
		phase := math.Mod(seconds, period) / period
		width := r.W * span
		x := r.X - width + (r.W+width)*float32(phase)
		p.Fill(ui.Rect{X: x, Y: r.Y, W: width, H: r.H}, t.Accent, 0)
	})
	return e
}

// Placeholder fills a page with a centred icon and a line of text.
func Placeholder(c *ui.Context, text string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(t.Space(3)).Padding(24).Children(func() {
		components.Icon(c, lucide.Package).Size(t.Space(10), t.Space(10)).
			TextColor(t.TextMuted.Alpha(0.5))
		components.Text(c, text).TextColor(t.TextMuted)
	})
}

// Fields are the columns shared by the installed, outdated and search
// tables, in order: package, installed version, latest version, manager.
func Fields() []ui.TableColumn {
	return []ui.TableColumn{
		{Title: locales.TablePackage(), ID: "name", MinWidth: 180, Sortable: true, Fixed: true},
		{Title: locales.TableInstalled(), ID: "version", Width: 100, Fixed: true},
		{Title: locales.TableLatest(), ID: "latest", Width: 100, Fixed: true},
		{Title: locales.TableManager(), ID: "manager", Width: 108, Sortable: true, Fixed: true},
	}
}

// SearchFields are the columns of a search result. A package found by search is
// not installed, so there is no installed version to show.
func SearchFields() []ui.TableColumn {
	fields := Fields()
	return append(fields[:1], fields[2:]...)
}

// SearchFieldIndex maps a search column back to its Fields index.
var SearchFieldIndex = []int{0, 2, 3}

// Cell renders field, indexing Fields.
func Cell(c *ui.Context, pkg manager.Package, field int, managerName func(string) string) {
	t := c.Theme()
	switch field {
	case 0:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			components.Icon(c, lucide.Package).Size(16, 16).TextColor(t.TextMuted)
			ui.Column(c).Gap(1).Children(func() {
				components.Tooltip(c, components.Text(c, pkg.Name).SingleLine(), pkg.Name)
				description := pkg.Description
				if description == "" {
					description = locales.TableNoDescription()
				}
				components.Text(c, description).SingleLine().TextColor(t.TextMuted)
			})
		})
	case 1:
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			components.Icon(c, lucide.Tag).Size(14, 14).TextColor(t.TextMuted)
			version := OrDash(pkg.Version)
			components.Tooltip(c, components.Text(c, version).SingleLine().TextColor(t.TextMuted), version)
		})
	case 2:
		latest := pkg.LatestVersion
		if latest == "" {
			latest = pkg.Version
		}
		color := t.TextMuted
		if pkg.Outdated {
			color = t.Accent
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			components.Icon(c, lucide.GitBranch).Size(14, 14).TextColor(color)
			components.Text(c, OrDash(latest)).SingleLine().TextColor(color)
		})
	case 3:
		ManagerBadge(c, pkg.Manager, managerName(pkg.Manager))
	}
}

// OrDash keeps empty cells readable.
func OrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// ActionSlot wraps a control that may be disabled. MyGo shows no tooltip
// anchored on a disabled element, so the reason hangs on the wrapper instead.
func ActionSlot(c *ui.Context) ui.Element {
	return ui.Row(c).Shrink(0).AlignItems(ui.Center)
}

// ActionTooltip explains a disabled action; an enabled one needs no reason.
func ActionTooltip(c *ui.Context, slot ui.Element, enabled bool, reason string) {
	if enabled || reason == "" {
		return
	}
	components.Tooltip(c, slot, reason)
}

// Divider is a hairline rule between sections.
func Divider(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(c.Theme().Border.Alpha(.5))
}

// TaskMessage shows the one message UniPac writes itself in the window's
// language.
func TaskMessage(task tasks.Snapshot) string {
	switch task.Message {
	case tasks.InterruptedMessage:
		return locales.TasksInterrupted()
	case tasks.CancelledMessage:
		return locales.TasksCancelledByUser()
	default:
		return task.Message
	}
}

// Operation names a task kind in the user's language.
func Operation(kind tasks.Kind) string {
	switch kind {
	case tasks.KindInstall:
		return locales.TasksKindInstall()
	case tasks.KindUpdate:
		return locales.TasksKindUpdate()
	case tasks.KindUninstall:
		return locales.TasksKindUninstall()
	default:
		return string(kind)
	}
}
