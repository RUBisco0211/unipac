// Package managers is the package managers page: one tab per manager, showing
// what it is, what it holds and what it can do, and the two settings that change
// it — whether UniPac uses it, and which executable it runs.
package managers

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

// View is the package managers page. It owns the rows while it is shown: the
// drafts being typed into a path field and the tab the user is on.
type View struct {
	Data *inventory.Data

	OnSetEnabled func(id string, enabled bool)
	OnSetPath    func(id, path string)
	OnReload     func()
	// TitleActions builds the buttons this page puts in the title bar, beside
	// its title and before the tasks button.
	TitleActions func(*ui.Context)

	rows []managerRow
	seen int
	// tab is the chosen manager's tab and tabID the manager it belongs to, so a
	// rescan that reorders or adds managers keeps the tab the user was on.
	tab   int
	tabID string
	// counts is how many installed packages each manager contributes, built from
	// the inventory version it was counted at.
	counts     map[string]int
	countsSeen int
	// busy is true while a save or a preflight runs; controls stay in place and
	// are disabled instead of vanishing.
	busy bool
	// detecting says the running job is a full re-probe, so only that action
	// reports it as detecting.
	detecting bool
	err       string
}

// managerRow is one manager's retained state: the switch the user sees, the path
// they are typing, and the path last saved.
type managerRow struct {
	info    manager.Info
	enabled bool
	path    string
	saved   string
}

// BeginSave marks the page busy for one settings write. The shell owns the guard
// that keeps two writes from racing; this only disables the controls.
func (m *View) BeginSave() {
	m.busy = true
	m.err = ""
}

// EndSave installs the write's outcome. A failure puts the rows back on the
// values the backend reports, so a path the backend refused does not stay on
// screen as if it had been saved.
func (m *View) EndSave(err error) {
	m.busy = false
	m.detecting = false
	if err != nil {
		m.err = err.Error()
		m.Invalidate()
	}
}

// Invalidate makes the next build rebuild the rows from the backend, which is
// how a failed save puts the controls back.
func (m *View) Invalidate() {
	m.seen = -1
}

func (m *View) Build(c *ui.Context) {
	m.Sync()
	t := c.Theme()
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(760).Gap(16).Padding(4, 16, 24, 16).Children(func() {
				if m.err != "" {
					components.Text(c, locales.SettingsSaveError(m.err)).
						FontSize(12).TextColor(t.Danger).Wrap()
				}
				m.tabs(c)
			})
		})
	})
}

// Sync rebuilds the rows when the backend reported a different manager list. An
// unapplied path edit is carried over, so a rebuild never eats typing.
func (m *View) Sync() {
	if m.rows != nil && m.seen == m.Data.ManagersVersion {
		return
	}
	m.seen = m.Data.ManagersVersion

	drafts := make(map[string]string, len(m.rows))
	for _, row := range m.rows {
		if row.path != row.saved {
			drafts[row.info.ID] = row.path
		}
	}
	rows := make([]managerRow, 0, len(m.Data.Managers))
	for _, info := range m.Data.Managers {
		saved := m.Data.Overrides[info.ID]
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
	// The chosen tab belongs to a manager, not to a position.
	m.tab = 0
	for i, row := range m.rows {
		if row.info.ID == m.tabID {
			m.tab = i
			break
		}
	}
}

// tabs puts one tab per manager above the chosen one's panel: a page of five
// managers is five screens of settings, and one of them is being changed at a
// time. The tabs are centred, since the panel below them is the only content.
func (m *View) tabs(c *ui.Context) {
	if len(m.rows) == 0 {
		return
	}
	if m.tab < 0 || m.tab >= len(m.rows) {
		m.tab = 0
	}
	labels := make([]string, len(m.rows))
	for i := range m.rows {
		labels[i] = m.rows[i].info.Name
	}
	// The tab carries the manager's mark before its name, read by the manager's
	// ID; a manager without a mark has its name alone.
	tabs := components.TabsWithLeading(c, &m.tab, components.TabsUnderline,
		func(c *ui.Context, i int, size float32) {
			parts.ManagerIcon(c, m.rows[i].info.ID, size)
		}, labels...)
	tabs.List.FillWidth().Justify(ui.Center)
	if m.tab >= 0 && m.tab < len(m.rows) {
		m.tabID = m.rows[m.tab].info.ID
	}
	ui.Column(c).FillWidth().Children(func() {
		m.panel(c, &m.rows[m.tab])
	})
}

// panel is one manager's settings as a list of answers: whether it runs, what it
// is, what it can do, and the two controls that change it. A label and a value
// rather than a form, because most of the page is information and the parts that
// are not are obvious as controls. The switch sits second, under the state it
// explains, so turning a manager off is where the eye lands first.
func (m *View) panel(c *ui.Context, row *managerRow) {
	t := c.Theme()
	ui.Column(c).FillWidth().Children(func() {
		m.field(c, locales.SettingsManagerStatus(), func() {
			status, color := locales.SettingsManagerAvailable(), t.Success
			if !row.info.Available {
				status, color = locales.SettingsNotFound(), t.Danger
			}
			components.Text(c, status).FontSize(12).TextColor(color)
		})
		parts.Divider(c)
		m.field(c, locales.SettingsManagerEnabled(), func() {
			before := row.enabled
			sw := components.Switch(c, &row.enabled, locales.SettingsEnableManager(row.info.Name), components.SwitchOptions{HideLabel: true})
			sw.Disabled(m.busy)
			if row.enabled != before && m.OnSetEnabled != nil {
				m.OnSetEnabled(row.info.ID, row.enabled)
			}
		})
		parts.Divider(c)
		m.field(c, locales.SettingsManagerVersion(), func() {
			components.Text(c, parts.OrDash(row.info.Version)).FontSize(12)
		})
		parts.Divider(c)
		m.field(c, locales.SettingsManagerExecutable(), func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				box := components.TextBox(c, &row.path, components.TextBoxOptions{}).
					Label(locales.SettingsExecPathLabel(row.info.Name)).
					Placeholder(m.pathHint(row)).
					PlaceholderColor(t.TextMuted).
					Grow(1).MinWidth(0)
				if box.Submitted() {
					m.applyPath(row)
				}
				apply := components.Button(c, locales.SettingsApply(), components.ButtonGhost)
				apply.Label(locales.SettingsApplyPathFor(row.info.Name))
				apply.Disabled(m.busy || strings.TrimSpace(row.path) == row.saved)
				if apply.Clicked() {
					m.applyPath(row)
				}
			})
		})
		parts.Divider(c)
		m.field(c, locales.SettingsManagerPackages(), func() {
			components.Text(c, m.packageCount(row.info.ID)).FontSize(12)
		})
		parts.Divider(c)
		m.field(c, locales.SettingsCapabilities(), func() {
			parts.CapabilityBadges(c, row.info.Capabilities)
		})
	})
}

// field is one line of a key-value block: what it is on the leading side, what it
// says across the rest of the row, so a long path has room and badges can wrap.
func (m *View) field(c *ui.Context, label string, value func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(10, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		components.Text(c, label).FontSize(12).TextColor(t.TextMuted).Width(100).Shrink(0)
		ui.Box(c).Grow(1).MinWidth(0).Children(value)
	})
}

// packageCount says how many installed packages a manager contributes, which is
// how a user sees that detection works and what turning a manager off would take
// away.
func (m *View) packageCount(id string) string {
	return locales.SettingsPackageCount(m.managerCounts()[id])
}

// managerCounts counts the inventory's rows per manager, rebuilt only when the
// inventory changed: a refresh must move the numbers, and a frame that changes
// nothing must not walk the rows again.
func (m *View) managerCounts() map[string]int {
	if m.counts != nil && m.countsSeen == m.Data.Version {
		return m.counts
	}
	counts := make(map[string]int, len(m.Data.Managers))
	for _, pkg := range m.Data.Rows {
		counts[pkg.Manager]++
	}
	m.counts, m.countsSeen = counts, m.Data.Version
	return counts
}

// pathHint shows what automatic detection resolves to, so an empty field is not
// an empty answer.
func (m *View) pathHint(row *managerRow) string {
	if row.info.ExecPath != "" {
		return row.info.ExecPath
	}
	return locales.SettingsPathAuto()
}

func (m *View) applyPath(row *managerRow) {
	path := strings.TrimSpace(row.path)
	if path == row.saved || m.OnSetPath == nil {
		return
	}
	m.OnSetPath(row.info.ID, path)
}

// TitleAction is the page's button in the title bar: probe every manager again,
// for a tool installed while the app was open.
func (m *View) TitleAction(c *ui.Context) {
	label := locales.SettingsRedetect()
	if m.detecting {
		label = locales.SettingsDetecting()
	}
	button := parts.TitleButton(c, lucide.RefreshCcw(), label)
	button.Disabled(m.busy)
	if button.Clicked() && m.OnReload != nil {
		m.detecting = true
		m.OnReload()
	}
}
