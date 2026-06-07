package logger

import (
	"log/slog"
	"os"
)

func New(appEnv string) *slog.Logger {
	var handler slog.Handler
	if appEnv == "local" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	return slog.New(handler)
}
