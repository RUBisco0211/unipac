// Package settings is the settings page: startup behavior, language, log level
// and one row per package manager.
package settings

import (
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// View is the settings page. Sections are stacked on one scrollable
// page and separated by rules rather than cards: startup behavior, then one
// row per package manager.
//
// Rows are rebuilt only when the manager list changes, so the path a user is
// typing survives the frames in between.
type View struct {
	Data *inventory.Data

	OnSetEnabled  func(id string, enabled bool)
	OnSetPath     func(id, path string)
	OnReload      func()
	OnSetAutoScan func(on bool)
	OnSetLogLevel func(level string)
	OnSetLanguage func(tag string)
	OnSetHold     func(on bool)
	// TitleActions builds the buttons this page puts in the title bar.
	TitleActions func(*ui.Context)

	// AutoScan is the saved startup behavior; the shell reads it to decide
	// whether to scan on launch.
	AutoScan bool
	// HoldToConfirm is how a destructive confirmation is answered, a hold or a
	// click; the shell reads it for the quit dialog.
	HoldToConfirm bool
	logLevel      string
	levels        []string
	language      string
	// languages are the tags UniPac is written in, in the app's own order.
	languages []string

	rows []managerRow
	seen int
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

// managerRow is one manager's retained state: the switch the user sees, the
// path they are typing, and the path last saved.
type managerRow struct {
	info    manager.Info
	enabled bool
	path    string
	saved   string
}

// New builds the page from the values the app started with.
func New(service inventory.Service, d *inventory.Data) *View {
	return &View{
		Data:          d,
		AutoScan:      service.AutoScanOnLaunch(),
		HoldToConfirm: service.HoldToConfirm(),
		logLevel:      service.LogLevel(),
		levels:        service.LogLevels(),
		language:      service.Language(),
		languages:     service.Languages(),
	}
}

// Snapshot is the settings state the backend reports after a write. It comes
// from the backend either way, so a failed write puts the controls back.
type Snapshot struct {
	AutoScan      bool
	LogLevel      string
	Language      string
	HoldToConfirm bool
}

// BeginSave marks the page busy for one settings write and reports false when
// another is still running: a second change would race the config write, so the
// controls go back to the saved state instead of being silently dropped.
func (s *View) BeginSave() bool {
	if s.busy {
		s.Invalidate()
		return false
	}
	s.busy = true
	s.err = ""
	return true
}

// EndSave installs what the backend now reports and the write's error, if any.
func (s *View) EndSave(err error, saved Snapshot) {
	s.busy = false
	s.detecting = false
	s.AutoScan = saved.AutoScan
	s.logLevel = saved.LogLevel
	s.language = saved.Language
	s.HoldToConfirm = saved.HoldToConfirm
	if err != nil {
		s.err = err.Error()
		s.Invalidate()
	}
}

func (s *View) Build(c *ui.Context) {
	s.Sync()
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
func (s *View) Sync() {
	if s.rows != nil && s.seen == s.Data.ManagersVersion {
		return
	}
	s.seen = s.Data.ManagersVersion

	drafts := make(map[string]string, len(s.rows))
	for _, row := range s.rows {
		if row.path != row.saved {
			drafts[row.info.ID] = row.path
		}
	}
	rows := make([]managerRow, 0, len(s.Data.Managers))
	for _, m := range s.Data.Managers {
		saved := s.Data.Overrides[m.ID]
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
func (s *View) Invalidate() {
	s.seen = -1
}

func (s *View) general(c *ui.Context) {
	s.section(c, locales.SettingsGeneral(), "", nil, func() {
		s.switchRow(c, locales.SettingsStartupScan(), locales.SettingsStartupScanDesc(), &s.AutoScan, func(on bool) {
			if s.OnSetAutoScan != nil {
				s.OnSetAutoScan(on)
			}
		})
		parts.Divider(c)
		s.switchRow(c, locales.SettingsHoldToConfirm(), locales.SettingsHoldToConfirmDesc(), &s.HoldToConfirm, func(on bool) {
			if s.OnSetHold != nil {
				s.OnSetHold(on)
			}
		})
		parts.Divider(c)
		s.logLevelRow(c)
		parts.Divider(c)
		s.languageRow(c)
	})
}

func (s *View) managers(c *ui.Context) {
	s.section(c, locales.SettingsManagers(), locales.SettingsManagersDesc(), func() {
		label := locales.SettingsRedetect()
		if s.detecting {
			label = locales.SettingsDetecting()
		}
		button := components.Button(c, label, components.ButtonGhost)
		button.Disabled(s.busy)
		if button.Clicked() && s.OnReload != nil {
			s.detecting = true
			s.OnReload()
		}
	}, func() {
		for i := range s.rows {
			if i > 0 {
				parts.Divider(c)
			}
			s.managerControl(c, &s.rows[i])
		}
	})
}

// section builds a titled block: a heading with an optional trailing action, an
// optional explanation, a rule under it, and the settings themselves.
func (s *View) section(c *ui.Context, title, description string, action, body func()) {
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

// switchRow is a labeled switch: the setting explains itself on the leading
// side and the control sits on the trailing edge.
//
// A switch reports its own change on the element it returns, not on the row
// that wraps it, so the value is compared across the build instead.
func (s *View) switchRow(c *ui.Context, title, description string, value *bool, changed func(bool)) {
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

func (s *View) logLevelRow(c *ui.Context) {
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
		if s.logLevel != before && s.OnSetLogLevel != nil {
			s.OnSetLogLevel(s.logLevel)
		}
	})
}

func (s *View) languageRow(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			components.Text(c, locales.SettingsLanguage())
			components.Text(c, locales.SettingsLanguageDesc()).
				FontSize(12).TextColor(c.Theme().TextMuted).Wrap()
		})
		before := s.language
		sel := components.Select(c, &s.language)
		sel.Trigger.Label(locales.SettingsLanguage()).Width(150).Children(func() {
			components.Text(c, languageLabel(s.language)).SingleLine()
		})
		sel.Trigger.Disabled(s.busy)
		sel.Popup(func(panel ui.Element) {
			item := sel.Item("")
			item.Label(locales.SettingsLanguageSystem()).Children(func() {
				components.Text(c, locales.SettingsLanguageSystem()).SingleLine()
			})
			for _, tag := range s.languages {
				option := sel.Item(tag)
				option.Label(languageLabel(tag)).Children(func() {
					components.Text(c, languageLabel(tag)).SingleLine()
				})
			}
		})
		if s.language != before && s.OnSetLanguage != nil {
			s.OnSetLanguage(s.language)
		}
	})
}

// managerControl is one manager: what it is and what it holds, the executable
// path, what it cannot do, and the switch that turns it on or off.
//
// The row is three lines rather than one. The name and its switch own the first,
// so the path gets the row's whole width on the second, and a manager that cannot
// answer an action says so on the third instead of leaving the page to explain a
// disabled button somewhere else.
func (s *View) managerControl(c *ui.Context, row *managerRow) {
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(12, 0).Gap(8).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Text(c, row.info.Name).SingleLine()
				// A dot, not a column: the count is a note about the name, and
				// the name is what the row is.
				components.Text(c, "·").TextColor(t.TextMuted)
				components.Text(c, s.packageCount(row.info.ID)).
					FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			if !row.info.Available {
				components.Text(c, locales.SettingsNotFound()).
					FontSize(11).TextColor(t.Danger).SingleLine()
			}
			before := row.enabled
			sw := components.Switch(c, &row.enabled, locales.SettingsEnableManager(row.info.Name), components.SwitchOptions{HideLabel: true})
			sw.Disabled(s.busy)
			if row.enabled != before && s.OnSetEnabled != nil {
				s.OnSetEnabled(row.info.ID, row.enabled)
			}
		})

		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
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
		})

		if missing := unsupportedCapabilities(row.info.Capabilities); missing != "" {
			components.Text(c, missing).FontSize(12).TextColor(t.TextMuted).Wrap()
		}
	})
}

// packageCount says how many installed packages a manager contributes, which is
// how a user sees that detection works and what turning a manager off would take
// away.
func (s *View) packageCount(id string) string {
	return locales.SettingsPackageCount(s.managerCounts()[id])
}

// managerCounts counts the inventory's rows per manager, rebuilt only when the
// inventory changed: a refresh must move the numbers, and a frame that changes
// nothing must not walk the rows again.
func (s *View) managerCounts() map[string]int {
	if s.counts != nil && s.countsSeen == s.Data.Version {
		return s.counts
	}
	counts := make(map[string]int, len(s.Data.Managers))
	for _, pkg := range s.Data.Rows {
		counts[pkg.Manager]++
	}
	s.counts, s.countsSeen = counts, s.Data.Version
	return counts
}

// unsupportedCapabilities names the actions a manager does not advertise, in the
// order the adapter guide's capability matrix lists them, and is empty when the
// manager offers every one: a row says what is missing rather than listing what
// is there.
func unsupportedCapabilities(cap manager.Capabilities) string {
	missing := make([]string, 0, 8)
	for _, item := range []struct {
		supported bool
		label     func() string
	}{
		{cap.ListInstalled, locales.CapabilityInstalled},
		{cap.ListOutdated, locales.CapabilityOutdated},
		{cap.GetPackageInfo, locales.CapabilityPackageInfo},
		{cap.Search, locales.CapabilitySearch},
		{cap.Install, locales.CapabilityInstall},
		{cap.Uninstall, locales.CapabilityUninstall},
		{cap.Update, locales.CapabilityUpdate},
		{cap.ListVersions, locales.CapabilityVersions},
	} {
		if !item.supported {
			missing = append(missing, item.label())
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return locales.SettingsUnsupported(strings.Join(missing, locales.CommonListSeparator()))
}

// pathHint shows what automatic detection resolves to, so an empty field is not
// an empty answer.
func (s *View) pathHint(row *managerRow) string {
	if row.info.ExecPath != "" {
		return row.info.ExecPath
	}
	return locales.SettingsPathAuto()
}

func (s *View) applyPath(row *managerRow) {
	path := strings.TrimSpace(row.path)
	if path == row.saved || s.OnSetPath == nil {
		return
	}
	s.OnSetPath(row.info.ID, path)
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

// languageLabel names a language in that language, so a reader can find their
// own even when the app currently shows another. The empty tag is the system
// choice, which the app's own language names.
func languageLabel(tag string) string {
	if tag == "" {
		return locales.SettingsLanguageSystem()
	}
	// CLDR knows zh-CN as 中文; the app's Chinese catalog is Simplified, so name
	// it by its script instead.
	if tag == "zh-CN" {
		tag = "zh-Hans"
	}
	if name := i18n.Autonym(tag); name != "" {
		return name
	}
	return tag
}
