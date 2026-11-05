package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/cache"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/logging"
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
