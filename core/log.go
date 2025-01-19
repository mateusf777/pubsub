package core

import (
	"log/slog"
	"os"
)

// logger is initialized with error level for the core package.
var logger *slog.Logger

func init() {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	logger = slog.New(logHandler)
}

// SetLogLevel allows core user to configure a different level.
func SetLogLevel(level slog.Level) {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger = slog.New(logHandler)
}
