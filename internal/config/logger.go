package config

import (
	"log/slog"
	"os"
)

func InitLogger(level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	l := slog.New(h)
	slog.SetDefault(l) // opcional: usar slog.Info(...) direto
	return l
}
