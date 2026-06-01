package config

import (
	"errors"
	"fmt"
	"os"
	"unipac-wails/backend/cache"
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/logging"
	"unipac-wails/backend/util"

	"github.com/samber/lo"
)

// Config is the main configuration struct for the application, containing all necessary configuration for different modules and features
type Config struct {
	Cache    cache.Config            `json:"cache"`
	Log      logging.Config          `json:"log"`
	Managers map[string]manager.Info `json:"adapters"`
}

// Instance exported
var Instance Config
var filePath string

func Init() error {
	var err error
	filePath, err = util.SafeJoin(util.AppRoot, "unipac.json")
	Instance, err = Load()
	if err != nil {
		return err
	}
	return nil
}

func defaultConfig() Config {
	return Config{
		Cache:    cache.DefaultConfig(),
		Log:      logging.DefaultConfig(),
		Managers: map[string]manager.Info{},
	}
}

// Load loads the configuration from the specified config file.
// If the file does not exist, create a new one with default values.
// If the file exists but cannot be parsed, overwrite it with default values.
func Load() (Config, error) {
	// read from local config JSON file
	cfg, err := util.ReadJson[Config](filePath)
	if err != nil {
		// permission denied
		if errors.Is(err, os.ErrPermission) {
			return Config{}, fmt.Errorf("permission denied: %w", err)
		}
		// file not found or unmarshall error
		// create default config
		cfg := defaultConfig()
		if err = Save(cfg); err != nil {
			return cfg, fmt.Errorf("failed to save default config: %w", err)
		}
		return cfg, nil
	}
	return cfg, nil
}

func SyncManagers(managers []manager.Info) error {
	Instance.Managers = lo.SliceToMap(managers, func(item manager.Info) (string, manager.Info) {
		return item.ID, item
	})
	if err := Save(Instance); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// Save saves the configuration to the config file
func Save(cfg Config) error {
	return util.WriteJson(filePath, cfg)
}
