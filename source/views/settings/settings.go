// Package settings is the settings page: startup behavior, language, log level
// and one row per package manager.
package settings

import (
	"strings"

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

	OnSetAutoScan func(on bool)
	OnSetLogLevel func(level string)
	OnSetLanguage func(tag string)
	OnSetTheme    func(value string)
	OnSetHold     func(on bool)
	// OnSetNotifications saves whether a finished task is announced.
	OnSetNotifications func(on bool)
	// OnOpenPath opens a file or folder in the desktop's own application.
	OnOpenPath func(path string)
	// OnClearHistory drops the tasks that have an outcome.
	OnClearHistory func()
	// TitleActions builds the buttons this page puts in the title bar.
	TitleActions func(*ui.Context)

	// AutoScan is the saved startup behavior; the shell reads it to decide
	// whether to scan on launch.
	AutoScan bool
	// HoldToConfirm is how a destructive confirmation is answered, a hold or a
	// click; the shell reads it for the quit dialog.
	HoldToConfirm bool
	// Notifications is whether a finished task is announced; the shell reads it
	// before showing one.
	Notifications bool
	logLevel      string
	levels        []string
	language      string
	theme         string
	// languages are the tags UniPac is written in, in the app's own order.
	languages []string

	// about is what the build is and where it keeps its files, loaded once when
	// the page is built.
	about inventory.About
	// busy is true while a save runs; controls stay in place and are disabled
	// instead of vanishing.
	busy bool
	err  string
}

// New builds the page from the values the app started with.
func New(service inventory.Service, d *inventory.Data) *View {
	return &View{
		Data:          d,
		AutoScan:      service.AutoScanOnLaunch(),
		HoldToConfirm: service.HoldToConfirm(),
		Notifications: service.Notifications(),
		logLevel:      service.LogLevel(),
		levels:        service.LogLevels(),
		language:      service.Language(),
		languages:     service.Languages(),
		theme:         service.Theme(),
		about:         service.About(),
	}
}

// Snapshot is the settings state the backend reports after a write. It comes
// from the backend either way, so a failed write puts the controls back.
type Snapshot struct {
	AutoScan      bool
	LogLevel      string
	Language      string
	Theme         string
	HoldToConfirm bool
	Notifications bool
}

// BeginSave marks the page busy for one settings write. The shell owns the guard
// that keeps two writes from racing; this only disables the controls.
func (s *View) BeginSave() {
	s.busy = true
	s.err = ""
}

// EndSave installs what the backend now reports and the write's error, if any.
func (s *View) EndSave(err error, saved Snapshot) {
	s.busy = false
	s.AutoScan = saved.AutoScan
	s.logLevel = saved.LogLevel
	s.language = saved.Language
	s.theme = saved.Theme
	s.HoldToConfirm = saved.HoldToConfirm
	s.Notifications = saved.Notifications
	if err != nil {
		s.err = err.Error()
	}
}

func (s *View) Build(c *ui.Context) {
	components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(760).Gap(28).Padding(0, 16, 24, 16).Children(func() {
				if s.err != "" {
					components.Text(c, locales.SettingsSaveError(s.err)).
						FontSize(12).TextColor(c.Theme().Danger).Wrap()
				}
				s.general(c)
				s.tasks(c)
				s.aboutSection(c)
			})
		})
	})
}

func (s *View) general(c *ui.Context) {
	s.section(c, locales.SettingsGeneral(), "", nil, func() {
		s.themeRow(c)
		parts.Divider(c)
		s.languageRow(c)
		parts.Divider(c)
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
		s.switchRow(c, locales.SettingsNotifications(), locales.SettingsNotificationsDesc(), &s.Notifications, func(on bool) {
			if s.OnSetNotifications != nil {
				s.OnSetNotifications(on)
			}
		})
		parts.Divider(c)
		s.logLevelRow(c)
	})
}

// field is one line of a key-value block: what it is on the leading side, what it
// says across the rest of the row, so a long path has room and badges can wrap.
func (s *View) field(c *ui.Context, label string, value func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(10, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		components.Text(c, label).FontSize(12).TextColor(t.TextMuted).Width(100).Shrink(0)
		ui.Box(c).Grow(1).MinWidth(0).Children(value)
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

// themeRow is the appearance: the system's, or one the user pinned. Onevra's
// palette follows the frame's dark state, so nothing else has to change with it.
func (s *View) themeRow(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		components.Text(c, locales.SettingsAppearance()).Grow(1).MinWidth(0)
		before := s.theme
		sel := components.Select(c, &s.theme)
		sel.Trigger.Label(locales.SettingsAppearance()).Width(150).Children(func() {
			components.Text(c, appearanceLabel(s.theme)).SingleLine()
		})
		sel.Trigger.Disabled(s.busy)
		sel.Popup(func(panel ui.Element) {
			for _, value := range []string{"", "light", "dark"} {
				option := sel.Item(value)
				option.Label(appearanceLabel(value)).Children(func() {
					components.Text(c, appearanceLabel(value)).SingleLine()
				})
			}
		})
		if s.theme != before && s.OnSetTheme != nil {
			s.OnSetTheme(s.theme)
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
		components.Text(c, locales.SettingsLanguage()).Grow(1).MinWidth(0)
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

// tasks is the maintenance section: the store keeps history, and this is where it
// is thrown away. Clearing is destructive, so it answers to the same setting as
// uninstalling — a hold, or a click when the user turned holding off.
func (s *View) tasks(c *ui.Context) {
	s.section(c, locales.SettingsTasks(), locales.SettingsTasksDesc(), func() {
		if s.HoldToConfirm {
			var held bool
			hold := components.HoldToConfirm(c, &held, locales.SettingsClearHistoryHold(),
				components.HoldToConfirmOptions{Tone: components.HoldDanger})
			hold.Disabled(s.busy)
			if hold.Changed() && s.OnClearHistory != nil {
				s.OnClearHistory()
			}
			return
		}
		clear := components.Button(c, locales.SettingsClearHistory(), components.ButtonOutline)
		clear.TextColor(c.Theme().Danger)
		clear.Disabled(s.busy)
		if clear.Clicked() && s.OnClearHistory != nil {
			s.OnClearHistory()
		}
	}, nil)
}

// aboutSection is what this build is and where it keeps its files, with a way to
// open either folder and to copy the lot into a bug report.
func (s *View) aboutSection(c *ui.Context) {
	s.section(c, locales.SettingsAbout(), "", func() {
		copy := components.Button(c, locales.AboutCopy(), components.ButtonGhost)
		if copy.Clicked() {
			c.WriteClipboard(s.diagnostics())
			c.Toast(locales.ToastDiagnosticsCopied())
		}
	}, func() {
		s.aboutRow(c, locales.AboutVersion(), s.about.Version, "")
		parts.Divider(c)
		s.aboutRow(c, locales.AboutRuntime(), s.about.Go, "")
		for _, module := range s.about.Modules {
			parts.Divider(c)
			name, version, _ := strings.Cut(module, " ")
			s.aboutRow(c, name, version, "")
		}
		parts.Divider(c)
		s.aboutRow(c, locales.AboutDataDir(), s.about.DataDir, locales.AboutDataDir())
		parts.Divider(c)
		s.aboutRow(c, locales.AboutLogDir(), s.about.LogDir, locales.AboutLogDir())
	})
}

// aboutRow is one line of the About section: what it is, what it is, and a way to
// open it when it is a folder. A path is one line and clips, so it carries a
// tooltip with the whole value.
func (s *View) aboutRow(c *ui.Context, label, value, folder string) {
	t := c.Theme()
	s.field(c, label, func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				if value == "" {
					components.Text(c, "-").FontSize(12).TextColor(t.TextMuted)
					return
				}
				components.Tooltip(c, components.Text(c, value).FontSize(12).SingleLine(), value)
			})
			if folder == "" || s.OnOpenPath == nil {
				return
			}
			path := value
			open := components.Button(c, locales.AboutOpenAction(), components.ButtonGhost)
			open.Label(locales.AboutOpen(folder))
			open.Disabled(path == "")
			if open.Clicked() {
				s.OnOpenPath(path)
			}
		})
	})
}

// diagnostics is what a user pastes into a bug report: the build, where it keeps
// its files, and what each manager claims.
func (s *View) diagnostics() string {
	var b strings.Builder
	line := func(label, value string) {
		b.WriteString(label + ": " + value + "\n")
	}
	line("UniPac", s.about.Version)
	line(locales.AboutRuntime(), s.about.Go)
	for _, module := range s.about.Modules {
		name, version, _ := strings.Cut(module, " ")
		line(name, version)
	}
	line(locales.AboutDataDir(), s.about.DataDir)
	line(locales.AboutLogDir(), s.about.LogDir)
	counts := make(map[string]int, len(s.Data.Managers))
	for _, pkg := range s.Data.Rows {
		counts[pkg.Manager]++
	}
	for _, info := range s.Data.Managers {
		state := "off"
		switch {
		case !info.Available:
			state = "missing"
		case info.Enabled:
			state = "on"
		}
		fields := []string{info.Name, info.Version, state,
			locales.SettingsPackageCount(counts[info.ID])}
		if missing := parts.UnsupportedCapabilities(info.Capabilities); missing != "" {
			fields = append(fields, missing)
		}
		b.WriteString(strings.Join(fields, " · ") + "\n")
	}
	return b.String()
}

// appearanceLabel names one appearance choice. The empty value is the system's.
func appearanceLabel(value string) string {
	switch value {
	case "light":
		return locales.AppearanceLight()
	case "dark":
		return locales.AppearanceDark()
	default:
		return locales.AppearanceSystem()
	}
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
