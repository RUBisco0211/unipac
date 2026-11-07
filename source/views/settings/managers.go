// Manager categories: one page per manager the backend registered, showing what
// it is, what it holds and what it can do, and the two settings that change it —
// whether UniPac uses it, and which executable it runs.
//
// The rows are built from inventory.Data, never from a list the interface keeps:
// a manager the registry drops loses its page, and one it adds gets a page with
// no code here changing.
package settings

import (
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// managerState is what the manager categories keep between frames: one row per
// manager, the drafts being typed into a path field, and the counts the
// inventory gives.
type managerState struct {
	rows []managerRow
	seen int
	// counts is how many installed packages each manager contributes, built from
	// the inventory version it was counted at.
	counts     map[string]int
	countsSeen int
	// detecting says the running job is a full re-probe, so only that action
	// reports it as detecting.
	detecting bool
}

// managerRow is one manager's retained state: the switch the user sees, the path
// they are typing, and the path last saved.
type managerRow struct {
	info    manager.Info
	enabled bool
	path    string
	saved   string
}

// sync rebuilds the rows when the backend reported a different manager list. An
// unapplied path edit is carried over, so a rebuild never eats typing.
func (m *managerState) sync(data *inventory.Data) {
	if m.rows != nil && m.seen == data.ManagersVersion {
		return
	}
	m.seen = data.ManagersVersion

	drafts := make(map[string]string, len(m.rows))
	for _, row := range m.rows {
		if row.path != row.saved {
			drafts[row.info.ID] = row.path
		}
	}
	rows := make([]managerRow, 0, len(data.Managers))
	for _, info := range data.Managers {
		saved := data.Overrides[info.ID]
		row := managerRow{
			info:    info,
			enabled: !saved.Disabled,
			path:    saved.ExecPath,
			saved:   saved.ExecPath,
		}
		if draft, ok := drafts[info.ID]; ok {
			row.path = draft
		}
		rows = append(rows, row)
	}
	m.rows = rows
}

// invalidate makes the next build rebuild the rows from the backend, which is how
// a failed or dropped save puts the controls back.
func (m *managerState) invalidate() {
	m.seen = -1
}

// countsFor counts the inventory's rows per manager, rebuilt only when the
// inventory changed: a refresh must move the numbers, and a frame that changes
// nothing must not walk the rows again.
func (m *managerState) countsFor(data *inventory.Data) map[string]int {
	if m.counts != nil && m.countsSeen == data.Version {
		return m.counts
	}
	counts := make(map[string]int, len(data.Managers))
	for _, pkg := range data.Rows {
		counts[pkg.Manager]++
	}
	m.counts, m.countsSeen = counts, data.Version
	return counts
}

// managerCategories is one category per manager, in registry order, under the
// managers section of the sidebar.
func (s *View) managerCategories() []components.SettingsCategory {
	categories := make([]components.SettingsCategory, 0, len(s.managers.rows))
	for i := range s.managers.rows {
		categories = append(categories, s.managerCategory(&s.managers.rows[i]))
	}
	return categories
}

// managerCategory is one manager's page, in two areas: what a user can change
// about it, then what the page reports about it. They are apart because they are
// read differently — a form on one side, a report on the other — and each area is
// a group, which is what gives it its heading and the rule under it.
func (s *View) managerCategory(row *managerRow) components.SettingsCategory {
	info := row.info
	id := info.ID
	return components.SettingsCategory{
		// The prefix keeps a manager ID from colliding with a category of the
		// app's own settings.
		ID:      "manager:" + id,
		Title:   info.Name,
		Section: locales.NavManagers(),
		// A colored brand mark rather than a themed glyph: the category is the
		// manager, and its artwork names it faster than its name does.
		Image: parts.ManagerLogo(id),
		Groups: []components.SettingsGroup{
			{ID: "settings", Title: locales.SettingsManagersConfigure(), Items: s.managerSettingItems(row)},
			{ID: "info", Title: locales.SettingsManagersInfo(), Items: s.managerInfoItems(row)},
		},
	}
}

// managerSettingItems is what a user can change about one manager: whether UniPac
// uses it, and which executable it runs. The switch comes first, above the path,
// because turning a manager off is the coarser of the two decisions.
//
// The rows carry keywords the titles do not: a manager's ID is what an adapter
// calls itself, and it is worth finding "brew" even though the page shows the
// name the backend reported.
func (s *View) managerSettingItems(row *managerRow) []components.SettingsItem {
	info := row.info
	id := info.ID
	return []components.SettingsItem{
		{
			ID:       "enabled",
			Title:    locales.SettingsManagerEnabled(),
			Keywords: []string{id, "use", "enable", "disable"},
			Build: func(c *ui.Context) {
				before := row.enabled
				components.Switch(c, &row.enabled, locales.SettingsEnableManager(info.Name), components.SwitchOptions{HideLabel: true})
				if row.enabled != before && s.OnSetManagerEnabled != nil {
					s.OnSetManagerEnabled(id, row.enabled)
				}
			},
		},
		{
			ID:       "executable",
			Title:    locales.SettingsManagerExecutable(),
			Keywords: []string{id, "executable", "path", "shell"},
			// A path is long: it takes the row's width rather than the space left
			// of it.
			Stacked: true,
			Build:   func(c *ui.Context) { s.executableAction(c, row) },
		},
	}
}

// managerInfoItems is what the page says about one manager: whether its
// executable answered, what version it is, how many packages it contributes, and
// what it can do. Nothing here changes anything.
func (s *View) managerInfoItems(row *managerRow) []components.SettingsItem {
	info := row.info
	id := info.ID
	return []components.SettingsItem{
		{
			ID:       "status",
			Title:    locales.SettingsManagerStatus(),
			Keywords: []string{id, "status", "detected"},
			// A badge rather than a colored word: answered or not is the same
			// question the capability badges answer, so it is read the same way —
			// a check in green, or a cross in red.
			Build: func(c *ui.Context) {
				text, tone, icon := locales.SettingsManagerAvailable(), components.BadgeSuccess, lucide.Check()
				if !info.Available {
					text, tone, icon = locales.SettingsNotFound(), components.BadgeDanger, lucide.X()
				}
				components.Badge(c, text, components.BadgeOptions{Tone: tone, Icon: icon}).Shrink(0)
			},
		},
		{
			ID:       "version",
			Title:    locales.SettingsManagerVersion(),
			Keywords: []string{id, "version", "release"},
			// The version wears the same tag a package row gives its own.
			Build: func(c *ui.Context) { parts.VersionMark(c, info.Version).Shrink(0) },
		},
		{
			ID:       "packages",
			Title:    locales.SettingsManagerPackages(),
			Keywords: []string{id, "packages", "count", "installed"},
			Build:    func(c *ui.Context) { factValue(c, s.packageCount(id)) },
		},
		// The address is the link itself, clicked in place as on a web page,
		// rather than a button beside it.
		linkItem("website", locales.SettingsManagerWebsite(),
			[]string{id, "website", "homepage", "docs", "documentation"},
			func(c *ui.Context) {
				if info.Website == "" || s.OnOpenURL == nil {
					components.Text(c, parts.OrDash(info.Website)).FontSize(12).TextColor(c.Theme().TextMuted)
					return
				}
				// A link is a text node: its visible words are its address.
				url := info.Website
				link := components.Button(c, url, components.ButtonLink)
				components.Tooltip(c, link, locales.SettingsManagerWebsiteOpen())
				if link.Clicked() {
					s.OnOpenURL(url)
				}
			},
		),
		{
			ID:       "capabilities",
			Title:    locales.SettingsCapabilities(),
			Keywords: []string{id, "capabilities", "support", "actions"},
			// The badges wrap, so they need the row's whole width.
			Stacked: true,
			Build:   func(c *ui.Context) { parts.CapabilityBadges(c, info.Capabilities) },
		},
	}
}

// packageCount says how many installed packages a manager contributes, which is
// how a user sees that detection works and what turning a manager off would take
// away.
func (s *View) packageCount(id string) string {
	return locales.SettingsPackageCount(s.managers.countsFor(s.Data)[id])
}

// executableAction is the path field: the saved override, with the detected path
// as its muted placeholder, so an empty field means "detect automatically".
// Apply stays visible and disabled until the draft differs, and the field
// submits on Enter.
func (s *View) executableAction(c *ui.Context, row *managerRow) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		box := components.TextBox(c, &row.path, components.TextBoxOptions{}).
			Label(locales.SettingsExecPathLabel(row.info.Name)).
			Placeholder(pathHint(row)).
			PlaceholderColor(t.TextMuted).
			Grow(1).MinWidth(0)
		if box.Submitted() {
			s.applyPath(row)
		}
		apply := components.Button(c, locales.SettingsApply(), components.ButtonGhost)
		apply.Label(locales.SettingsApplyPathFor(row.info.Name))
		apply.Disabled(strings.TrimSpace(row.path) == row.saved)
		if apply.Clicked() {
			s.applyPath(row)
		}
	})
}

// pathHint shows what automatic detection resolves to, so an empty field is not
// an empty answer.
func pathHint(row *managerRow) string {
	if row.info.ExecPath != "" {
		return row.info.ExecPath
	}
	return locales.SettingsPathAuto()
}

func (s *View) applyPath(row *managerRow) {
	path := strings.TrimSpace(row.path)
	if path == row.saved || s.OnSetManagerPath == nil {
		return
	}
	s.OnSetManagerPath(row.info.ID, path)
}

// sectionAction builds the trailing control of the managers section: probe every
// manager again, for a tool installed while the app was open. The section name is
// the value locates the section, because it is the heading the section shows.
func (s *View) sectionAction(managersSection string) func(*ui.Context, string) {
	return func(c *ui.Context, section string) {
		if section != managersSection {
			return
		}
		label := locales.SettingsRedetect()
		if s.managers.detecting {
			label = locales.SettingsDetecting()
		}
		button := components.IconButton(c, lucide.RefreshCcw(), label, components.IconButtonCompact)
		if button.Clicked() && s.OnReloadManagers != nil {
			s.managers.detecting = true
			s.OnReloadManagers()
		}
	}
}
