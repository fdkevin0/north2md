package south2md

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// InitLogger initializes the global slog logger with a text handler.
func InitLogger(debug bool) {
	level := slog.LevelWarn
	if debug {
		level = slog.LevelDebug
	}

	w := os.Stderr

	// Set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      level,
			TimeFormat: time.DateTime,
		}),
	))
}

// Logger is the global logger instance
var Logger = slog.Default()
