package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/cache"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/logging"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/backend/util"
)

// Config is the main configuration struct for the application, containing all necessary configuration for different modules and features
type Config struct {
	Cache    cache.Config                `json:"cache"`
	Log      logging.Config              `json:"log"`
	Startup  Startup                     `json:"startup"`
	Managers map[string]manager.Settings `json:"managers"`
	// Language is the tag chosen in Settings, "" to follow the system.
	Language string `json:"language,omitempty"`
	// Theme is the appearance chosen in Settings: ThemeLight, ThemeDark, or ""
	// to follow the system.
	Theme string `json:"theme,omitempty"`
	// Accent is the theme color chosen in Settings, one of Accents; "" keeps the
	// toolkit's own accent.
	Accent string `json:"accent,omitempty"`
	// HoldToConfirm is nil in a config written before the setting existed, which
	// behaves like true: the destructive confirmations hold rather than click.
	HoldToConfirm *bool `json:"hold_to_confirm,omitempty"`
	// Notify is which finished operations are announced, one switch per task
	// kind. Its fields are nil in a config written before the setting was split,
	// which is what Notifies falls back on.
	Notify Notify `json:"notify,omitempty"`
	// Notifications is the single switch a config written before the split holds.
	// It is read when Notify has nothing to say and dropped on the next write, so
	// the file converges on the per-operation switches.
	Notifications *bool `json:"notifications,omitempty"`
}

// HoldsToConfirm reports whether a destructive confirmation is a hold rather
// than a click.
func (c Config) HoldsToConfirm() bool {
	return c.HoldToConfirm == nil || *c.HoldToConfirm
}

// Notify is what the app announces: each finished operation by how it ended, and
// whether a scan that finds updates says so. A nil field follows the single
// switch the config used to hold, and that switch being absent means on, so an
// old file keeps announcing.
type Notify struct {
	// Enabled is the master switch: off means the app announces nothing at all.
	// Nil in a file written before it existed, which behaves like on.
	Enabled   *bool         `json:"enabled,omitempty"`
	Install   NotifyOutcome `json:"install,omitempty"`
	Update    NotifyOutcome `json:"update,omitempty"`
	Uninstall NotifyOutcome `json:"uninstall,omitempty"`
	// Updates is the "an update is available" switch, nil in a file written
	// before it existed, which behaves like on.
	Updates *bool `json:"updates,omitempty"`
}

// NotifyOutcome is one operation's two switches: it finished, or it failed.
type NotifyOutcome struct {
	Success *bool `json:"success,omitempty"`
	Failure *bool `json:"failure,omitempty"`
}

// NotifyKinds is the operations Settings offers switches for, in the order the
// category shows them.
func NotifyKinds() []tasks.Kind {
	return []tasks.Kind{tasks.KindInstall, tasks.KindUpdate, tasks.KindUninstall}
}

// Notifies reports whether a finished operation of kind is announced, when it
// succeeded or when it failed. A kind this build does not know is never
// announced.
func (c Config) Notifies(kind tasks.Kind, succeeded bool) bool {
	if chosen := c.Notify.switchFor(kind, succeeded); chosen != nil {
		return *chosen
	}
	return c.Notifications == nil || *c.Notifications
}

// NotifiesEnabled reports whether the app announces anything at all. It is what
// the other switches are read under, so turning it off silences them without
// forgetting them.
func (c Config) NotifiesEnabled() bool {
	if c.Notify.Enabled != nil {
		return *c.Notify.Enabled
	}
	return c.Notifications == nil || *c.Notifications
}

// NotifiesUpdates reports whether a scan that finds packages with a newer version
// says so.
func (c Config) NotifiesUpdates() bool {
	if c.Notify.Updates != nil {
		return *c.Notify.Updates
	}
	return c.Notifications == nil || *c.Notifications
}

// outcome is the saved switches of kind, nil when the file does not have them.
func (n *Notify) outcome(kind tasks.Kind) *NotifyOutcome {
	switch kind {
	case tasks.KindInstall:
		return &n.Install
	case tasks.KindUpdate:
		return &n.Update
	case tasks.KindUninstall:
		return &n.Uninstall
	}
	return nil
}

// switchFor is the saved switch of kind and outcome, nil when the file does not
// have one.
func (n *Notify) switchFor(kind tasks.Kind, succeeded bool) *bool {
	outcome := n.outcome(kind)
	if outcome == nil {
		return nil
	}
	if succeeded {
		return outcome.Success
	}
	return outcome.Failure
}

// switchField is where a switch is saved, nil for a kind this build does not
// know.
func (n *Notify) switchField(kind tasks.Kind, succeeded bool) **bool {
	outcome := n.outcome(kind)
	if outcome == nil {
		return nil
	}
	if succeeded {
		return &outcome.Success
	}
	return &outcome.Failure
}

// Startup holds what the app does when it opens.
type Startup struct {
	// AutoScan is nil in a config written before the setting existed, which
	// behaves like true.
	AutoScan *bool `json:"auto_scan,omitempty"`
}

// ScansOnLaunch reports whether the app should scan for installed packages as
// soon as it opens.
func (s Startup) ScansOnLaunch() bool {
	return s.AutoScan == nil || *s.AutoScan
}

// Instance exported
var Instance Config
var filePath string

func Init() error {
	var err error
	filePath, err = util.SafeJoin(util.AppRoot, "unipac.json")
	if err != nil {
		return err
	}
	Instance, err = Load()
	if err != nil {
		return err
	}
	return nil
}

func defaultConfig() Config {
	autoScan := true
	return Config{
		Cache:    cache.DefaultConfig(),
		Log:      logging.DefaultConfig(),
		Startup:  Startup{AutoScan: &autoScan},
		Managers: map[string]manager.Settings{},
	}
}

// Load loads the configuration from the specified config file.
//
// A missing file is created with default values. A file that does not parse is
// kept first: the config is a file people edit by hand, and replacing it without
// a word would throw away every setting over a stray comma. It is renamed to
// `unipac.json.bad-<time>` and the app starts on defaults, so the edited file can
// be recovered and fixed.
func Load() (Config, error) {
	// read from local config JSON file
	cfg, err := util.ReadJson[Config](filePath)
	if err != nil {
		// permission denied
		if errors.Is(err, os.ErrPermission) {
			return Config{}, fmt.Errorf("permission denied: %w", err)
		}
		cfg := defaultConfig()
		// Only a file that is there and unreadable is worth keeping: a missing
		// one needs nothing.
		if !errors.Is(err, os.ErrNotExist) {
			kept, keepErr := keepUnreadable()
			if keepErr != nil {
				return cfg, fmt.Errorf("failed to set aside the unreadable config %s: %w", filePath, keepErr)
			}
			slog.Warn("config: the config file did not parse; it was kept and defaults are in use",
				"path", filePath, "kept", kept, "error", err)
		}
		if err = Save(cfg); err != nil {
			return cfg, fmt.Errorf("failed to save default config: %w", err)
		}
		return cfg, nil
	}
	if cfg.Managers == nil {
		cfg.Managers = map[string]manager.Settings{}
	}
	return cfg, nil
}

// keepUnreadable renames the config file that did not parse out of the way and
// returns its new path, beside the original so it is found where it was left.
func keepUnreadable() (string, error) {
	dir, name := filepath.Split(filePath)
	kept := filepath.Join(dir, fmt.Sprintf("%s.bad-%s", name, time.Now().Format("20060102-150405")))
	if err := os.Rename(filePath, kept); err != nil {
		return "", err
	}
	return kept, nil
}

// ManagerSettings returns the saved per-manager settings. It never returns a
// nil map, so callers can read and copy from it without checking.
func ManagerSettings() map[string]manager.Settings {
	if Instance.Managers == nil {
		return map[string]manager.Settings{}
	}
	return Instance.Managers
}

// SetManagerEnabled turns one manager on or off and saves the config.
func SetManagerEnabled(id string, enabled bool) error {
	s := Instance.Managers[id]
	s.Disabled = !enabled
	return setManager(id, s)
}

// SetManagerExecPath overrides one manager's executable and saves the config.
// An empty path goes back to detecting it.
func SetManagerExecPath(id string, path string) error {
	s := Instance.Managers[id]
	s.ExecPath = strings.TrimSpace(path)
	return setManager(id, s)
}

// setManager commits one manager's settings only after they are on disk, so a
// failed write leaves the running app on the values it was started with.
func setManager(id string, s manager.Settings) error {
	managers := make(map[string]manager.Settings, len(Instance.Managers)+1)
	for key, value := range Instance.Managers {
		managers[key] = value
	}
	managers[id] = s

	cfg := Instance
	cfg.Managers = managers
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// SetAutoScan saves whether the app scans for installed packages on launch.
func SetAutoScan(on bool) error {
	cfg := Instance
	cfg.Startup.AutoScan = &on
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// SetHoldToConfirm saves whether destructive confirmations hold or click.
func SetHoldToConfirm(on bool) error {
	cfg := Instance
	cfg.HoldToConfirm = &on
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// The appearances Settings offers, the empty one following the system.
const (
	ThemeLight = "light"
	ThemeDark  = "dark"
)

// The theme colors Settings offers, the empty one keeping the toolkit's accent.
const (
	AccentDefault  = ""
	AccentBlue     = "blue"
	AccentPurple   = "purple"
	AccentPink     = "pink"
	AccentGreen    = "green"
	AccentGraphite = "graphite"
)

// Accents is the theme colors Settings offers, in the order the choice shows
// them. It is the source of the list: the interface draws the colors, and this
// is what it asks for.
func Accents() []string {
	return []string{AccentDefault, AccentBlue, AccentPurple, AccentPink, AccentGreen, AccentGraphite}
}

// AccentChosen reports whether value is one of the theme colors this build
// knows.
func AccentChosen(value string) bool {
	return slices.Contains(Accents(), value)
}

// SetAccent saves the theme color, "" for the toolkit's own. Applying it to the
// running windows is the caller's job: the palette is rebuilt every frame. A
// value the app does not know is rejected rather than saved, so a hand edit or a
// stale client cannot leave a color the interface cannot draw.
func SetAccent(value string) error {
	if !AccentChosen(value) {
		return fmt.Errorf("unknown accent: %q", value)
	}
	cfg := Instance
	cfg.Accent = value
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// ThemeChosen reports whether value is one of the appearances this build knows.
func ThemeChosen(value string) bool {
	switch value {
	case "", ThemeLight, ThemeDark:
		return true
	default:
		return false
	}
}

// SetNotify saves whether a finished operation of kind is announced, when it
// succeeded or when it failed. Saving one switch settles all of them: the single
// switch an older file carries is materialized first, so dropping it keeps every
// answer and the file converges on the shape it is read in now. A kind this build
// does not know is rejected rather than saved.
func SetNotify(kind tasks.Kind, succeeded, on bool) error {
	cfg := Instance
	if cfg.Notify.switchField(kind, succeeded) == nil {
		return fmt.Errorf("unknown operation: %q", kind)
	}
	settleNotify(&cfg)
	*cfg.Notify.switchField(kind, succeeded) = &on
	return saveSettings(cfg)
}

// SetNotifyEnabled saves the master switch.
func SetNotifyEnabled(on bool) error {
	cfg := Instance
	settleNotify(&cfg)
	cfg.Notify.Enabled = &on
	return saveSettings(cfg)
}

// SetNotifyUpdates saves whether a scan that finds updates says so.
func SetNotifyUpdates(on bool) error {
	cfg := Instance
	settleNotify(&cfg)
	cfg.Notify.Updates = &on
	return saveSettings(cfg)
}

// settleNotify fills every switch the config does not answer yet with what it
// currently reports, and drops the single switch it replaced. One write is what
// takes a file written before the per-operation switches to the current shape.
func settleNotify(cfg *Config) {
	for _, kind := range NotifyKinds() {
		for _, succeeded := range []bool{true, false} {
			field := cfg.Notify.switchField(kind, succeeded)
			if field == nil || *field != nil {
				continue
			}
			value := cfg.Notifies(kind, succeeded)
			*field = &value
		}
	}
	if cfg.Notify.Enabled == nil {
		value := cfg.NotifiesEnabled()
		cfg.Notify.Enabled = &value
	}
	if cfg.Notify.Updates == nil {
		value := cfg.NotifiesUpdates()
		cfg.Notify.Updates = &value
	}
	cfg.Notifications = nil
}

// saveSettings writes the config and installs it as the running one.
func saveSettings(cfg Config) error {
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// SetTheme saves the appearance, "" to follow the system. Applying it to the
// running window is the caller's job. A value the app does not know is
// rejected rather than saved, so a hand edit or a stale client cannot leave a
// config the window cannot show.
func SetTheme(value string) error {
	if !ThemeChosen(value) {
		return fmt.Errorf("unknown appearance: %q", value)
	}
	cfg := Instance
	cfg.Theme = value
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// SetLanguage saves the language UniPac shows, "" to follow the system.
// Applying it to the running app is the caller's job.
func SetLanguage(tag string) error {
	cfg := Instance
	cfg.Language = tag
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// SetLogLevel saves the log level. Applying it to the running logger is the
// caller's job.
func SetLogLevel(level string) error {
	cfg := Instance
	cfg.Log.Level = level
	if err := Save(cfg); err != nil {
		return err
	}
	Instance = cfg
	return nil
}

// Save saves the configuration to the config file
func Save(cfg Config) error {
	return util.WriteJson(filePath, cfg)
}
