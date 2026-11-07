// Package parts holds the presentational pieces the pages share: package table
// columns and cells, badges, the loading bar, placeholders, row action slots,
// separators and the labels a task's kind or message gets. Nothing here keeps
// state; a part is built, drawn and forgotten.
package parts

import (
	"math"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/uiassets"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// accentColors is what each theme color Settings offers draws in: the light
// frame's accent, and the one the dark frame lifts it to, which is how Onevra
// keeps its own accent legible on a dark surface.
var accentColors = map[string]struct{ light, dark ui.Color }{
	"blue":     {ui.Hex("#2f6feb"), ui.Hex("#7aa7ff")},
	"purple":   {ui.Hex("#7a4ff2"), ui.Hex("#b39bff")},
	"pink":     {ui.Hex("#d63384"), ui.Hex("#f58ac2")},
	"green":    {ui.Hex("#0f7a52"), ui.Hex("#6ecf9f")},
	"graphite": {ui.Hex("#4b5563"), ui.Hex("#9aa3ae")},
}

// AccentColor is the color a saved theme color draws in, and false for the
// toolkit's own accent — what an empty or unknown choice shows.
func AccentColor(id string, dark bool) (ui.Color, bool) {
	pair, ok := accentColors[id]
	if !ok {
		return ui.Color{}, false
	}
	if dark {
		return pair.dark, true
	}
	return pair.light, true
}

// markNote annotates a package's name with what the page knows about it. A mark
// and a word, not a pill: this says something about the name rather than giving
// the row a label of its own.
func markNote(c *ui.Context, mark inventory.Mark) {
	t := c.Theme()
	var icon *ui.SVG
	var text string
	var color ui.Color
	switch mark {
	case inventory.MarkInstalled:
		icon, text, color = lucide.Check(), locales.SearchInstalled(), t.Success
	case inventory.MarkUpdate:
		icon, text, color = lucide.CircleArrowUp(), locales.SearchUpdateAvailable(), t.Accent
	default:
		return
	}
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		components.Icon(c, icon).Size(12, 12).TextColor(color)
		components.Text(c, text).FontSize(12).TextColor(color)
	})
}

// TitleButton builds an icon button in the title bar. Every one of them sits in
// the same box, so the tasks button and a page's own line up: the button is
// square around its glyph, and the box keeps its height off the row's.
func TitleButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	t := c.Theme()
	var button ui.Element
	ui.Box(c).Shrink(0).Width(t.Space(8)).Height(t.Space(7)).Center().Children(func() {
		button = components.IconButton(c, icon, label, components.IconButtonPlain)
	})
	return button
}

// ManagerLabel shows a manager as its mark and its name, which is how a table
// row says which manager a package belongs to. The mark is the brand's colored
// artwork at a row's size — smaller than the settings sidebar draws it, because
// a row is denser than a navigation item — and a manager without one shows its
// name alone.
func ManagerLabel(c *ui.Context, id, label string) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		if logo := uiassets.ManagerLogo(id); logo != nil {
			ui.Image(c, logo).Size(14, 14).Fit(ui.Contain).Radius(14 * .2).Shrink(0)
		}
		components.Text(c, label).SingleLine()
	})
}

// LoadingBar draws a thin indeterminate bar along the top edge of e: a
// highlight sweeping across while the table inside it loads or reloads.
//
// It paints on e rather than adding an element, because an absolute one measures
// the window, not the box it is built in: a page with a side panel had the bar
// run under the panel. Painting on the box keeps it within the page's share by
// construction, and it takes no room, as before.
func LoadingBar(c *ui.Context, e ui.Element) {
	t := c.Theme()
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 {
			return
		}
		// Keep the bar moving on its own; nothing else asks for these frames.
		p.AnimationFrameRate(60)
		const (
			height = 2
			span   = 0.28 // the highlight's share of the width
			period = 1.2  // seconds per sweep
		)
		track := ui.Rect{X: r.X, Y: r.Y, W: r.W, H: height}
		p.Fill(track, t.Border.Alpha(0.35), 0)
		seconds := float64(p.Now().UnixNano()) / 1e9
		phase := math.Mod(seconds, period) / period
		width := r.W * span
		x := r.X - width + (r.W+width)*float32(phase)
		// The sweep starts and ends beyond the box, as a marquee does, so clamp
		// it here rather than trusting the box to clip its last and first
		// fractions: a bar that paints one pixel past the table is the bug this
		// element was moved onto the box to fix.
		start, end := max(r.X, x), min(r.X+r.W, x+width)
		if end > start {
			p.Fill(ui.Rect{X: start, Y: r.Y, W: end - start, H: height}, t.Accent, 0)
		}
	})
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
		{Title: locales.TableManager(), ID: "manager", Width: 108, Sortable: true, Fixed: true, Align: ui.Center},
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

// Cell renders field, indexing Fields. mark is what the page says about the
// package beside its name, which only a search result has.
func Cell(c *ui.Context, pkg manager.Package, field int, managerName func(string) string, mark inventory.Mark) {
	t := c.Theme()
	switch field {
	case 0:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			components.Icon(c, lucide.Package).Size(16, 16).TextColor(t.TextMuted)
			ui.Column(c).Gap(1).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					components.Tooltip(c, components.Text(c, pkg.Name).SingleLine(), pkg.Name)
					markNote(c, mark)
				})
				description := pkg.Description
				if description == "" {
					description = locales.TableNoDescription()
				}
				components.Tooltip(c, components.Text(c, description).SingleLine().TextColor(t.TextMuted), description)
			})
		})
	case 1:
		VersionMark(c, pkg.Version)
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
			version := OrDash(latest)
			components.Tooltip(c, components.Text(c, version).SingleLine().TextColor(color), version)
		})
	case 3:
		ManagerLabel(c, pkg.Manager, managerName(pkg.Manager))
	}
}

// VersionMark is a version with the tag that says what it is: a package's
// installed version in a row, and a manager's own version on its settings page.
// It keeps the tooltip, because a version is read at a glance and checked only
// when something looks wrong.
func VersionMark(c *ui.Context, version string) ui.Element {
	t := c.Theme()
	return ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		components.Icon(c, lucide.Tag).Size(14, 14).TextColor(t.TextMuted)
		value := OrDash(version)
		components.Tooltip(c, components.Text(c, value).SingleLine().TextColor(t.TextMuted), value)
	})
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

// ManagerLogo is a manager's brand mark, when it has one, ready for a slot that
// shows images in their own colors. The marks are assets the interface embeds,
// read by manager ID, so no manager is named here; nil leaves the slot as it
// was, for a manager that ships no artwork.
func ManagerLogo(id string) ui.ImageSource { return uiassets.ManagerLogo(id) }

// Capability is one thing a manager may offer, named the way the pages show it
// and the copied diagnostics spell it.
type Capability struct {
	Name      string
	Supported bool
}

// Capabilities names what a manager advertises, in the order the adapter guide's
// capability matrix lists them.
func Capabilities(caps manager.Capabilities) []Capability {
	return []Capability{
		{locales.CapabilityInstalled(), caps.ListInstalled},
		{locales.CapabilityOutdated(), caps.ListOutdated},
		{locales.CapabilityPackageInfo(), caps.Detail},
		{locales.CapabilitySearch(), caps.Search},
		{locales.CapabilityInstall(), caps.Install},
		{locales.CapabilityUninstall(), caps.Uninstall},
		{locales.CapabilityUpdate(), caps.Update},
		{locales.CapabilityVersions(), caps.ListVersions},
	}
}

// CapabilityBadges shows what a manager offers, one badge per capability: a check
// in green for what it does and a cross in red for what it does not, so a gap is
// visible at a glance instead of read out of a sentence.
func CapabilityBadges(c *ui.Context, caps manager.Capabilities) {
	ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
		for _, item := range Capabilities(caps) {
			tone, icon := components.BadgeSuccess, lucide.Check()
			if !item.Supported {
				tone, icon = components.BadgeDanger, lucide.X()
			}
			components.Badge(c, item.Name, components.BadgeOptions{Tone: tone, Icon: icon})
		}
	})
}

// UnsupportedCapabilities names the actions a manager does not advertise, for a
// text that has no room for badges, and is empty when the manager offers every
// one.
func UnsupportedCapabilities(caps manager.Capabilities) string {
	missing := make([]string, 0, 8)
	for _, item := range Capabilities(caps) {
		if !item.Supported {
			missing = append(missing, item.Name)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return locales.SettingsUnsupported(strings.Join(missing, locales.CommonListSeparator()))
}
