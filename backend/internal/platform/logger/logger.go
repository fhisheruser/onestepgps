// Package logger builds the application's structured logger on top of the
// standard library's log/slog, so no third-party logging dependency is needed.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a slog.Logger writing to stdout. Text output is easier to read
// during development; JSON is what log aggregators want in production.
func New(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if strings.EqualFold(format, "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
