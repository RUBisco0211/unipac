package views

import (
	"math"

	"github.com/RUBisco0211/unipac/backend/core/manager"
	"github.com/RUBisco0211/unipac/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// packageKey identifies a package across managers.
func packageKey(pkg manager.Package) string {
	return pkg.Manager + ":" + pkg.Name
}

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

// managerBadge renders the manager label in a badge tinted by the manager ID.
// It reuses components.Badge so both badges in a row share their geometry, and
// only overrides the colors: Onevra's Badge offers status tones, not a
// per-manager hue.
func managerBadge(c *ui.Context, id, label string) {
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

// loadingBar draws a thin indeterminate bar: a highlight sweeping across while
// the table loads or reloads.
func loadingBar(c *ui.Context) ui.Element {
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

// placeholder fills a page with a centred icon and a line of text.
func placeholder(c *ui.Context, text string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(t.Space(3)).Padding(24).Children(func() {
		components.Icon(c, lucide.Package).Size(t.Space(10), t.Space(10)).
			TextColor(t.TextMuted.Alpha(0.5))
		components.Text(c, text).TextColor(t.TextMuted)
	})
}

// skeletonRows stands in for the table while the inventory loads.
func skeletonRows(c *ui.Context) {
	ui.Column(c).Fill().Gap(16).Padding(10, 16).Children(func() {
		for range 9 {
			components.Skeleton(c, components.SkeletonOptions{Lines: 2, Avatar: true})
		}
	})
}

// packageFields are the columns shared by the installed, outdated and search
// tables, in order: package, installed version, latest version, type, manager.
func packageFields() []ui.TableColumn {
	return []ui.TableColumn{
		{Title: locales.TablePackage(), ID: "name", MinWidth: 180, Sortable: true, Fixed: true},
		{Title: locales.TableInstalled(), ID: "version", Width: 100, Fixed: true},
		{Title: locales.TableLatest(), ID: "latest", Width: 100, Fixed: true},
		{Title: locales.TableType(), ID: "type", Width: 92, Sortable: true, Fixed: true},
		{Title: locales.TableManager(), ID: "manager", Width: 108, Sortable: true, Fixed: true},
	}
}

// searchFields are the columns of a search result. A package found by search is
// not installed, so there is no installed version to show.
func searchFields() []ui.TableColumn {
	fields := packageFields()
	return append(fields[:1], fields[2:]...)
}

// searchFieldIndex maps a search column back to its packageFields index.
var searchFieldIndex = []int{0, 2, 3, 4}

// packageFieldCell renders field, indexing packageFields.
func packageFieldCell(c *ui.Context, pkg manager.Package, field int, managerName func(string) string) {
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
			version := orDash(pkg.Version)
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
			components.Text(c, orDash(latest)).SingleLine().TextColor(color)
		})
	case 3:
		typeBadge(c, pkg.IsGUI)
	case 4:
		managerBadge(c, pkg.Manager, managerName(pkg.Manager))
	}
}

// typeBadge tells a GUI package from a command-line tool by tone and icon, so
// the two read apart at a glance.
func typeBadge(c *ui.Context, isGUI bool) {
	options := components.BadgeOptions{
		Tone: components.BadgeNeutral,
		Icon: lucide.Terminal(),
	}
	label := locales.TypeCli()
	if isGUI {
		label = locales.TypeGui()
		options.Tone = components.BadgeAccent
		options.Icon = lucide.Monitor()
	}
	components.Badge(c, label, options).AlignSelf(ui.Center)
}

// orDash keeps empty cells readable.
func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
