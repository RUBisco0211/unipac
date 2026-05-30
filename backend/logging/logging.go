package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"unipac-wails/backend/util"
)

var AppLogDir string
var LogFile *os.File

type Config struct {
	LogDir string `json:"log_dir,omitempty"`
	Level  string `json:"level,omitempty"`
}

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

var levelMap = map[string]slog.Level{
	LevelDebug: slog.LevelDebug,
	LevelInfo:  slog.LevelInfo,
	LevelWarn:  slog.LevelWarn,
	LevelError: slog.LevelError,
}

func DefaultConfig() Config {
	return Config{
		LogDir: "logs",
		Level:  LevelInfo,
	}
}

func Init(cfg Config) error {
	AppLogDir, err := util.SafeJoin(util.AppRoot, cfg.LogDir)
	if err != nil {
		return err
	}

	LogFile, err = os.OpenFile(
		filepath.Join(AppLogDir, "app.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		if err = LogFile.Close(); err != nil {
			return err
		}
		return err
	}

	logLevel, ok := levelMap[cfg.Level]
	if !ok {
		logLevel = slog.LevelInfo
	}

	multiWriter := io.MultiWriter(os.Stderr, LogFile)
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: util.IsDev(),
	})
	slog.SetDefault(slog.New(handler))
	return err
}

func Close() error {
	return LogFile.Close()
}
