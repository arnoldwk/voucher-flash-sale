package logger

import (
	"log/slog"
	"os"
)

// InitLogger initializes and returns a structured JSON logger.
func InitLogger(serviceName string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	log := slog.New(handler).With("service", serviceName)
	slog.SetDefault(log)
	return log
}
