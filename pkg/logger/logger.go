package logger

import (
	"io"
	"log/slog"
	"os"

	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

var LogWriter io.Writer

func Setup(cfg config.LogConfig) *slog.Logger {
	var w io.Writer

	if cfg.Output == "file" && cfg.FilePath != "" {
		w = &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize, // megabytes
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge, // days
			Compress:   cfg.Compress,
		}
	} else {
		w = os.Stdout
	}

	// Store the writer globally so we can use it for other outputs (like the report)
	LogWriter = w

	handler := slog.NewTextHandler(w, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
