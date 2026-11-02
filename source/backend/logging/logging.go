package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/RUBisco0211/unipac/backend/util"
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
	logPath, err := util.SafeJoin(util.AppRoot, cfg.LogDir, "app.log")
	if err != nil {
		return fmt.Errorf("resolve app log path: %w", err)
	}
	AppLogDir = filepath.Dir(logPath)

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open app log %s: %w", logPath, err)
	}
	LogFile = file

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
	return nil
}

func Close() error {
	if LogFile == nil {
		return nil
	}
	err := LogFile.Close()
	LogFile = nil
	return err
}
