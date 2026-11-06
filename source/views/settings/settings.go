// Package settings is the settings page: the app's own settings and the package
// managers' overrides, in one searchable surface built with Onevra's Settings
// component.
//
// The surface is a list of categories, grouped into two sidebar sections — the
// app's settings, then one category per manager the backend registered. The
// manager categories come from the registry in inventory.Data, never from a list
// the interface keeps, so installing a new adapter is enough to show it here.
//
// Values are bound to fields on this struct, which the shell owns across frames:
// the page is one page for the whole config file, so one write guard and one
// error line cover both the app's settings and the managers'.
package settings

import (
	"strings"

	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// surfaceMinWidth is how wide the surface needs to be for its two columns: the
// category sidebar at Onevra's width, and a content column wide enough for a
// label beside the control that answers it. Below it the sidebar becomes a
// selector and every row stacks, which is what a window too narrow for both
// columns gets instead of a squeezed one.
const surfaceMinWidth = 620

// untitled is a zero-width space. Onevra's surface always draws its title above
// the search field, and this page's name belongs to the window that holds it, so
// the row stays as the clearance the window's own controls need while saying
// nothing. Drop it when SettingsOptions gains a way to hide the title.
const untitled = "\u200b"

// View is the settings page. It holds every value the surface shows and the
// drafts being typed into a manager's path field.
type View struct {
	Data *inventory.Data

	OnSetAutoScan      func(on bool)
	OnSetLogLevel      func(level string)
	OnSetLanguage      func(tag string)
	OnSetTheme         func(value string)
	OnSetAccent        func(value string)
	OnSetHold          func(on bool)
	OnSetNotifications func(on bool)
	// OnOpenPath opens a file or folder in the desktop's own application.
	OnOpenPath func(path string)
	// OnOpenURL opens a web address in the browser.
	OnOpenURL func(url string)
	// OnClearHistory drops the tasks that have an outcome.
	OnClearHistory func()

	OnSetManagerEnabled func(id string, enabled bool)
	OnSetManagerPath    func(id, path string)
	OnReloadManagers    func()

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
	languages     []string
	theme         string
	// accent is the theme color, and accents the ones the backend offers, in the
	// order the choice shows them.
	accent  string
	accents []string

	// about is what the build is and where it keeps its files, loaded once when
	// the page is built.
	about inventory.About
	// TopInset is the chrome the window draws over this page's top, which is the
	// window's drag strip when content runs behind it. The page pads its error
	// line by it; zero is a window that insets the page instead.
	TopInset float32
	// busy is true while a save runs; controls stay in place and are disabled
	// instead of vanishing.
	busy bool
	err  string
	// clearOpen asks for a confirmation of the history clear, which is a dialog
	// rather than a hold: clearing history is not irreversible like uninstalling
	// is, and a dialog can say exactly what will be dropped.
	clearOpen bool

	// search is the surface's own navigation: the chosen category, and the query
	// that reaches every category at once.
	search components.SettingsState
	// surface measures the box the settings surface fills, which is the width
	// its layout answers to — not the window's, which the sidebar shares.
	surface ui.Handle
	// managers is what the manager categories keep between frames: one row per
	// registered manager, the path being typed, and the package counts.
	managers managerState
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
		accent:        service.Accent(),
		accents:       service.Accents(),
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
	Accent        string
	HoldToConfirm bool
	Notifications bool
}

// BeginSave marks the page busy for one settings write. The shell owns the guard
// that keeps two writes from racing; this only disables the controls.
func (s *View) BeginSave() {
	s.busy = true
	s.err = ""
}

// Revert puts the controls back on what the backend last reported, without
// ending the write: a change dropped because another write is running is not
// an error, and the page stays busy until that write lands. The manager rows
// are rebuilt from the backend for the same reason a dropped path edit must not
// stay on screen.
func (s *View) Revert(saved Snapshot) {
	s.AutoScan = saved.AutoScan
	s.logLevel = saved.LogLevel
	s.language = saved.Language
	s.theme = saved.Theme
	s.accent = saved.Accent
	s.HoldToConfirm = saved.HoldToConfirm
	s.Notifications = saved.Notifications
	s.managers.invalidate()
}

// EndSave installs what the backend now reports and the write's error, if any.
func (s *View) EndSave(err error, saved Snapshot) {
	s.busy = false
	s.managers.detecting = false
	s.Revert(saved)
	if err != nil {
		s.err = err.Error()
	}
}

// Accent is the theme color both windows draw in. The page holds it because the
// window reads it every frame, and a reverted write has to put the old color
// back.
func (s *View) Accent() string { return s.accent }

// Build draws the surface: one category per group of settings, and one per
// manager, under a search field that reaches every row of both.
func (s *View) Build(c *ui.Context) {
	s.managers.sync(s.Data)
	managersSection := locales.NavManagers()
	narrow := s.narrow(c)
	ui.Column(c).Fill().MinWidth(0).Children(func() {
		if s.err != "" {
			ui.Row(c).FillWidth().Shrink(0).Padding(s.TopInset+12, 16, 0, 16).Children(func() {
				components.Text(c, locales.SettingsSaveError(s.err)).
					FontSize(12).TextColor(c.Theme().Danger).Wrap()
			})
		}
		// The surface fills its parent: it brings its own two columns, its own
		// search field and its own scroll view.
		surface := ui.Column(c).Fill().MinWidth(0).Bind(&s.surface)
		surface.Children(func() {
			components.Settings(c, &s.search, s.categories(), components.SettingsOptions{
				Title:  untitled,
				Narrow: narrow,
				// The only section action is the re-probe: it is about the
				// managers as a whole, so it belongs beside that section's
				// heading rather than on every manager's page.
				SectionActions: s.sectionAction(managersSection),
			})
		})
	})
	s.clearDialog(c)
}

// narrow reports whether the surface has less width than its two columns need.
// The measured box is the last committed frame, so a resize settles a frame
// later; a first frame, with no box yet, is drawn as wide, which the app's
// minimum window always allows.
func (s *View) narrow(c *ui.Context) bool {
	if b := s.surface.Bounds(c); b.W > 0 {
		return b.W < surfaceMinWidth
	}
	return false
}

// categories is the whole catalog, in sidebar order. Sections are contiguous
// runs, which is how the component groups them: the app's settings first, then
// every manager the registry reported.
func (s *View) categories() []components.SettingsCategory {
	section := locales.NavSettings()
	categories := []components.SettingsCategory{
		{
			ID:      "general",
			Title:   locales.SettingsGeneral(),
			Section: section,
			Icon:    lucide.Settings(),
			Groups:  []components.SettingsGroup{{ID: "general", Items: s.generalItems()}},
		},
		{
			ID:          "tasks",
			Title:       locales.SettingsTasks(),
			Section:     section,
			Description: locales.SettingsTasksDesc(),
			Icon:        lucide.ListTodo(),
			Groups: []components.SettingsGroup{{ID: "history", Items: []components.SettingsItem{{
				ID:       "history",
				Title:    locales.SettingsHistory(),
				Keywords: []string{"tasks", "history", "console", "clear"},
				Build:    s.clearHistoryAction,
			}}}},
		},
		{
			ID:      "about",
			Title:   locales.SettingsAbout(),
			Section: section,
			Icon:    lucide.Info(),
			Groups:  []components.SettingsGroup{{ID: "about", Items: s.aboutItems()}},
		},
	}
	return append(categories, s.managerCategories()...)
}

// generalItems is the app's own behavior: appearance, language, and the switches
// that change how it starts and how it answers a destructive question.
func (s *View) generalItems() []components.SettingsItem {
	return []components.SettingsItem{
		{
			ID:       "appearance",
			Title:    locales.SettingsAppearance(),
			Keywords: []string{"theme", "appearance", "light", "dark"},
			Build:    s.appearanceAction,
		},
		{
			ID:          "accent",
			Title:       locales.SettingsAccent(),
			Description: locales.SettingsAccentDesc(),
			Keywords:    []string{"color", "colour", "accent", "highlight"},
			Build:       s.accentAction,
		},
		{
			ID:       "language",
			Title:    locales.SettingsLanguage(),
			Keywords: []string{"language", "locale"},
			Build:    s.languageAction,
		},
		{
			ID:          "scan",
			Title:       locales.SettingsStartupScan(),
			Description: locales.SettingsStartupScanDesc(),
			Keywords:    []string{"scan", "startup", "launch"},
			Build: func(c *ui.Context) {
				s.switchAction(c, &s.AutoScan, locales.SettingsStartupScanToggle(), s.OnSetAutoScan)
			},
		},
		{
			ID:          "hold",
			Title:       locales.SettingsHoldToConfirm(),
			Description: locales.SettingsHoldToConfirmDesc(),
			Keywords:    []string{"confirm", "hold", "click", "destructive"},
			Build: func(c *ui.Context) {
				s.switchAction(c, &s.HoldToConfirm, locales.SettingsHoldToConfirmToggle(), s.OnSetHold)
			},
		},
		{
			ID:          "notifications",
			Title:       locales.SettingsNotifications(),
			Description: locales.SettingsNotificationsDesc(),
			Keywords:    []string{"notifications", "announce", "finished"},
			Build: func(c *ui.Context) {
				s.switchAction(c, &s.Notifications, locales.SettingsNotificationsToggle(), s.OnSetNotifications)
			},
		},
		{
			ID:          "log",
			Title:       locales.SettingsLogLevel(),
			Description: locales.SettingsLogLevelDesc(),
			Keywords:    []string{"log", "level", "debug", "verbose"},
			Build:       s.logLevelAction,
		},
	}
}

// switchAction is a setting that answers true or false.
//
// A switch reports its own change on the element it returns, not on the row that
// wraps it, so the value is compared across the build instead.
func (s *View) switchAction(c *ui.Context, value *bool, label string, set func(bool)) {
	before := *value
	// The switch's accessible name is its own phrase, not the row title: the
	// title text beside it already says the setting, and a control named exactly
	// like its caption reads twice to a screen reader.
	sw := components.Switch(c, value, label, components.SwitchOptions{HideLabel: true})
	sw.Disabled(s.busy)
	if *value != before && set != nil {
		set(*value)
	}
}

// appearanceAction is the appearance: the system's, or one the user pinned.
// Onevra's palette follows the frame's dark state, so nothing else has to change
// with it.
func (s *View) appearanceAction(c *ui.Context) {
	before := s.theme
	sel := components.Select(c, &s.theme)
	sel.Trigger.Label(locales.SettingsAppearance()).Width(150).Children(func() {
		components.Text(c, appearanceLabel(s.theme)).SingleLine()
	})
	sel.Trigger.Disabled(s.busy)
	sel.Popup(func(panel ui.Element) {
		for _, value := range []string{"", "light", "dark"} {
			sel.Item(value).Label(appearanceLabel(value)).Children(func() {
				components.Text(c, appearanceLabel(value)).SingleLine()
			})
		}
	})
	if s.theme != before && s.OnSetTheme != nil {
		s.OnSetTheme(s.theme)
	}
}

// accentAction is the theme color: the color selections, focus and active
// controls are drawn in. Each choice shows the color it stands for, so the list
// reads as a palette rather than as names.
func (s *View) accentAction(c *ui.Context) {
	before := s.accent
	sel := components.Select(c, &s.accent)
	sel.Trigger.Label(locales.SettingsAccent()).Width(150).Children(func() {
		accentChoice(c, s.accent)
	})
	sel.Trigger.Disabled(s.busy)
	sel.Popup(func(panel ui.Element) {
		for _, value := range s.accents {
			sel.Item(value).Label(accentLabel(value)).Children(func() {
				accentChoice(c, value)
			})
		}
	})
	if s.accent != before && s.OnSetAccent != nil {
		s.OnSetAccent(s.accent)
	}
}

// accentChoice is one theme color in the list: a dot of the color it names, and
// what Settings calls it. The default keeps the toolkit's accent, so its dot is
// the frame's own.
func accentChoice(c *ui.Context, value string) {
	t := c.Theme()
	dot := ui.Hex("#bd481b")
	if t.Dark {
		dot = ui.Hex("#ed986e")
	}
	if accent, ok := parts.AccentColor(value, t.Dark); ok {
		dot = accent
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(10).Height(10).Radius(5).Shrink(0).Background(dot)
		components.Text(c, accentLabel(value)).SingleLine()
	})
}

func (s *View) logLevelAction(c *ui.Context) {
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
}

func (s *View) languageAction(c *ui.Context) {
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
}

// clearHistoryAction opens the question. Clearing is destructive, so it answers
// rather than acts: the dialog says what will be dropped before anything is.
func (s *View) clearHistoryAction(c *ui.Context) {
	clear := components.Button(c, locales.SettingsClearHistory(), components.ButtonOutline)
	clear.TextColor(c.Theme().Danger)
	clear.Disabled(s.busy)
	if clear.Clicked() {
		s.clearOpen = true
	}
}

// clearDialog says what clearing drops and asks once. The confirm is a danger
// button, not a hold: the history is a record, not the package the record is
// about, and it stops growing back the moment the app runs again.
func (s *View) clearDialog(c *ui.Context) {
	t := c.Theme()
	components.Dialog(c, &s.clearOpen, components.DialogOptions{Label: locales.SettingsClearHistory()}, func(panel ui.Element) {
		panel.Width(420).Padding(20).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, locales.SettingsClearHistory()).FontSize(16).Bold()
				components.Text(c, locales.SettingsClearHistoryMessage()).
					TextColor(t.TextMuted).Wrap()
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					if components.Button(c, locales.CommonCancel(), components.ButtonOutline).Clicked() {
						s.clearOpen = false
					}
					confirm := components.Button(c, locales.SettingsClearHistoryConfirm(), components.ButtonOutline)
					confirm.TextColor(t.Danger)
					confirm.Disabled(s.busy)
					if confirm.Clicked() && !s.busy {
						s.clearOpen = false
						if s.OnClearHistory != nil {
							s.OnClearHistory()
						}
					}
				})
			})
		})
	})
}

// aboutItems is what this build is and where it keeps its files. Each fact is a
// row of its own so the search field can reach it; a fact that is a place rather
// than a word carries the way to it, and the path itself is the row's caption so
// a long one wraps instead of pushing the row wide.
func (s *View) aboutItems() []components.SettingsItem {
	items := []components.SettingsItem{
		{
			ID:       "version",
			Title:    locales.AboutVersion(),
			Keywords: []string{"version", "build"},
			Build:    func(c *ui.Context) { factValue(c, parts.OrDash(s.about.Version)) },
		},
		{
			ID:       "runtime",
			Title:    locales.AboutRuntime(),
			Keywords: []string{"go", "runtime", "built"},
			Build:    func(c *ui.Context) { factValue(c, parts.OrDash(s.about.Go)) },
		},
	}
	for _, module := range s.about.Modules {
		name, version, _ := strings.Cut(module, " ")
		items = append(items, components.SettingsItem{
			ID:       "module:" + name,
			Title:    name,
			Keywords: []string{"module", "dependency", "toolkit"},
			Build:    func(c *ui.Context) { factValue(c, parts.OrDash(version)) },
		})
	}
	return append(items,
		components.SettingsItem{
			ID:          "data",
			Title:       locales.AboutDataDir(),
			Description: s.about.DataDir,
			Keywords:    []string{"data", "folder", "path", "storage"},
			Build: func(c *ui.Context) {
				if s.about.DataDir == "" || s.OnOpenPath == nil {
					return
				}
				path := s.about.DataDir
				open := components.Button(c, locales.AboutOpenAction(), components.ButtonGhost).Shrink(0)
				open.Label(locales.AboutOpen(locales.AboutDataDir()))
				if open.Clicked() {
					s.OnOpenPath(path)
				}
			},
		},
		components.SettingsItem{
			ID:          "repository",
			Title:       locales.AboutRepository(),
			Description: s.about.Repository,
			Keywords:    []string{"github", "repository", "source", "browser"},
			Build: func(c *ui.Context) {
				if s.about.Repository == "" || s.OnOpenURL == nil {
					return
				}
				url := s.about.Repository
				open := components.Button(c, locales.AboutOpenAction(), components.ButtonGhost).Shrink(0)
				open.Label(locales.AboutRepositoryOpen())
				if open.Clicked() {
					s.OnOpenURL(url)
				}
			},
		},
		// The report is the one thing here that is done rather than read, so it
		// is the last row of the category, drawn as its own button.
		components.SettingsItem{
			ID:       "copy",
			Title:    locales.AboutCopy(),
			Keywords: []string{"copy", "diagnostics", "report", "bug", "clipboard"},
			Content:  s.copyDiagnosticsAction,
		},
	)
}

// factValue is a read-only answer in a row's trailing slot: one line, and its own
// width, so the title beside it gives way instead of squeezing the answer into an
// ellipsis.
func factValue(c *ui.Context, value string) ui.Element {
	return components.Text(c, value).SingleLine().Shrink(0)
}

// copyDiagnosticsAction is the row that puts the build and every manager's state
// on the clipboard, which is what a bug report opens with. It is an outlined
// button rather than the borderless one the places beside it use: those open
// something, this one is the page's only action and has to look like one.
func (s *View) copyDiagnosticsAction(c *ui.Context) {
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		// The label is the button's own, so the icon sits inside one child row:
		// Button appends its children after the label with no gap of its own.
		copy := components.Button(c, "", components.ButtonOutline).Label(locales.AboutCopy()).Shrink(0)
		copy.Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				components.Icon(c, lucide.Copy()).Size(14, 14)
				components.Text(c, locales.AboutCopy()).SingleLine()
			})
		})
		if copy.Clicked() {
			c.WriteClipboard(s.diagnostics())
			c.Toast(locales.ToastDiagnosticsCopied())
		}
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
	line(locales.AboutRepository(), s.about.Repository)
	counts := s.managers.countsFor(s.Data)
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

// accentLabels name the theme colors Settings offers. A color this build does
// not know reads as the default rather than showing the raw token.
var accentLabels = map[string]func() string{
	"":         locales.AccentDefault,
	"blue":     locales.AccentBlue,
	"purple":   locales.AccentPurple,
	"pink":     locales.AccentPink,
	"green":    locales.AccentGreen,
	"graphite": locales.AccentGraphite,
}

func accentLabel(value string) string {
	if label, ok := accentLabels[value]; ok {
		return label()
	}
	return locales.AccentDefault()
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
