package views

import (
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// settingsView is the settings page. Sections are stacked on one scrollable
// page and separated by rules rather than cards: startup behavior, then one
// row per package manager.
//
// Rows are rebuilt only when the manager list changes, so the path a user is
// typing survives the frames in between.
type settingsView struct {
	data *data

	onSetEnabled  func(id string, enabled bool)
	onSetPath     func(id, path string)
	onReload      func()
	onSetAutoScan func(on bool)
	onSetLogLevel func(level string)

	autoScan bool
	logLevel string
	levels   []string

	rows []managerRow
	seen int
	// busy is true while a save or a preflight runs; controls stay in place and
	// are disabled instead of vanishing.
	busy bool
	// detecting says the running job is a full re-probe, so only that action
	// reports it as detecting.
	detecting bool
	err       string
}

// managerRow is one manager's retained state: the switch the user sees, the
// path they are typing, and the path last saved.
type managerRow struct {
	info    manager.Info
	enabled bool
	path    string
	saved   string
}

// newSettings builds the page from the values the app started with.
func newSettings(service Service, d *data) *settingsView {
	return &settingsView{
		data:     d,
		autoScan: service.AutoScanOnLaunch(),
		logLevel: service.LogLevel(),
		levels:   service.LogLevels(),
	}
}

func (s *settingsView) view(c *ui.Context) {
	s.sync()
	components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(760).Gap(28).Padding(0, 16, 24, 16).Children(func() {
				if s.err != "" {
					components.Text(c, locales.SettingsSaveError(s.err)).
						FontSize(12).TextColor(c.Theme().Danger).Wrap()
				}
				s.general(c)
				s.managers(c)
			})
		})
	})
}

// sync rebuilds the rows when the backend reported a different manager list.
// An unapplied path edit is carried over, so a rebuild never eats typing.
func (s *settingsView) sync() {
	if s.rows != nil && s.seen == s.data.managersVersion {
		return
	}
	s.seen = s.data.managersVersion

	drafts := make(map[string]string, len(s.rows))
	for _, row := range s.rows {
		if row.path != row.saved {
			drafts[row.info.ID] = row.path
		}
	}
	rows := make([]managerRow, 0, len(s.data.managers))
	for _, m := range s.data.managers {
		saved := s.data.overrides[m.ID]
		row := managerRow{
			info:    m,
			enabled: !saved.Disabled,
			path:    saved.ExecPath,
			saved:   saved.ExecPath,
		}
		if draft, ok := drafts[m.ID]; ok {
			row.path = draft
		}
		rows = append(rows, row)
	}
	s.rows = rows
}

// invalidate makes the next build rebuild the rows from the backend, which is
// how a failed save puts the switches back.
func (s *settingsView) invalidate() {
	s.seen = -1
}

func (s *settingsView) general(c *ui.Context) {
	s.section(c, locales.SettingsGeneral(), "", nil, func() {
		s.switchRow(c, locales.SettingsStartupScan(), locales.SettingsStartupScanDesc(), &s.autoScan, func(on bool) {
			if s.onSetAutoScan != nil {
				s.onSetAutoScan(on)
			}
		})
		divider(c)
		s.logLevelRow(c)
	})
}

func (s *settingsView) managers(c *ui.Context) {
	s.section(c, locales.SettingsManagers(), locales.SettingsManagersDesc(), func() {
		label := locales.SettingsRedetect()
		if s.detecting {
			label = locales.SettingsDetecting()
		}
		button := components.Button(c, label, components.ButtonGhost)
		button.Disabled(s.busy)
		if button.Clicked() && s.onReload != nil {
			s.detecting = true
			s.onReload()
		}
	}, func() {
		for i := range s.rows {
			if i > 0 {
				divider(c)
			}
			s.managerControl(c, &s.rows[i])
		}
	})
}

// section builds a titled block: a heading with an optional trailing action, an
// optional explanation, a rule under it, and the settings themselves.
func (s *settingsView) section(c *ui.Context, title, description string, action, body func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			components.Text(c, title).FontSize(15).Bold()
			ui.Spacer(c)
			if action != nil {
				action()
			}
		})
		if description != "" {
			components.Text(c, description).FontSize(12).TextColor(t.TextMuted).Wrap()
		}
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		if body != nil {
			ui.Column(c).FillWidth().Children(body)
		}
	})
}

func divider(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(c.Theme().Border.Alpha(.5))
}

// switchRow is a labeled switch: the setting explains itself on the leading
// side and the control sits on the trailing edge.
//
// A switch reports its own change on the element it returns, not on the row
// that wraps it, so the value is compared across the build instead.
func (s *settingsView) switchRow(c *ui.Context, title, description string, value *bool, changed func(bool)) {
	ui.Row(c).FillWidth().Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			components.Text(c, title)
			if description != "" {
				components.Text(c, description).FontSize(12).TextColor(c.Theme().TextMuted).Wrap()
			}
		})
		before := *value
		sw := components.Switch(c, value, title, components.SwitchOptions{HideLabel: true})
		sw.Disabled(s.busy)
		if *value != before {
			changed(*value)
		}
	})
}

func (s *settingsView) logLevelRow(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			components.Text(c, locales.SettingsLogLevel())
			components.Text(c, locales.SettingsLogLevelDesc()).
				FontSize(12).TextColor(c.Theme().TextMuted).Wrap()
		})
		before := s.logLevel
		sel := components.Select(c, &s.logLevel)
		sel.Trigger.Label(locales.SettingsLogLevel()).Width(130).Children(func() {
			components.Text(c, logLevelLabel(s.logLevel)).SingleLine()
		})
		sel.Trigger.Disabled(s.busy)
		sel.Popup(func(panel ui.Element) {
			for _, level := range s.levels {
				sel.Item(level).Label(logLevelLabel(level)).Children(func() {
					components.Text(c, logLevelLabel(level)).SingleLine()
				})
			}
		})
		if s.logLevel != before && s.onSetLogLevel != nil {
			s.onSetLogLevel(s.logLevel)
		}
	})
}

// managerControl is one manager: its name and state, the executable path, and
// the switch that turns it on or off.
func (s *settingsView) managerControl(c *ui.Context, row *managerRow) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(12, 0).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(150).Shrink(0).Gap(2).Children(func() {
			components.Text(c, row.info.Name).SingleLine()
			if !row.info.Available {
				components.Text(c, locales.SettingsNotFound()).
					FontSize(11).TextColor(t.Danger).SingleLine()
			}
		})

		box := components.TextBox(c, &row.path, components.TextBoxOptions{}).
			Label(locales.SettingsExecPathLabel(row.info.Name)).
			Placeholder(s.pathHint(row)).
			PlaceholderColor(t.TextMuted).
			FillWidth().MinWidth(0)
		if box.Submitted() {
			s.applyPath(row)
		}

		apply := components.Button(c, locales.SettingsApply(), components.ButtonGhost)
		apply.Label(locales.SettingsApplyPathFor(row.info.Name))
		apply.Disabled(s.busy || strings.TrimSpace(row.path) == row.saved)
		if apply.Clicked() {
			s.applyPath(row)
		}

		before := row.enabled
		sw := components.Switch(c, &row.enabled, locales.SettingsEnableManager(row.info.Name), components.SwitchOptions{HideLabel: true})
		sw.Disabled(s.busy)
		if row.enabled != before && s.onSetEnabled != nil {
			s.onSetEnabled(row.info.ID, row.enabled)
		}
	})
}

// pathHint shows what automatic detection resolves to, so an empty field is not
// an empty answer.
func (s *settingsView) pathHint(row *managerRow) string {
	if row.info.ExecPath != "" {
		return row.info.ExecPath
	}
	return locales.SettingsPathAuto()
}

func (s *settingsView) applyPath(row *managerRow) {
	path := strings.TrimSpace(row.path)
	if path == row.saved || s.onSetPath == nil {
		return
	}
	s.onSetPath(row.info.ID, path)
}

// logLevelLabels names the levels the backend offers. A level this build does
// not know reads as unknown rather than showing the raw token.
var logLevelLabels = map[string]func() string{
	"debug": locales.LogLevelDebug,
	"info":  locales.LogLevelInfo,
	"warn":  locales.LogLevelWarn,
	"error": locales.LogLevelError,
}

func logLevelLabel(level string) string {
	if label, ok := logLevelLabels[level]; ok {
		return label()
	}
	return locales.LogLevelUnknown()
}
