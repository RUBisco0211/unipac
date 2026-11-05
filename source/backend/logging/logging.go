package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/egoist/mygo"
)

var AppLogDir string
var LogFile *os.File

// logWriter is where the handlers write, kept so SetLevel can rebuild the
// default logger without reopening the file.
var logWriter io.Writer

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

	logWriter = io.MultiWriter(os.Stderr, LogFile)
	handler := slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: mygo.IsDev(),
	})
	slog.SetDefault(slog.New(handler))
	return nil
}

// Levels lists the log levels in order of increasing severity.
func Levels() []string {
	return []string{LevelDebug, LevelInfo, LevelWarn, LevelError}
}

// SetLevel applies a log level to the running logger. The file stays open, so
// the change takes effect immediately without a restart.
func SetLevel(level string) error {
	logLevel, ok := levelMap[level]
	if !ok {
		return fmt.Errorf("unknown log level: %q", level)
	}
	if logWriter == nil {
		return nil
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: mygo.IsDev(),
	})))
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
