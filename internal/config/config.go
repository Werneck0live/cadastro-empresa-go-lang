package config

import (
	"log/slog"
	"os"
	"time"
)

type Config struct {
	Port              string
	MongoURI          string
	MongoDB           string
	RabbitURI         string
	RabbitQueue       string
	LogLevel          slog.Level
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

func Load() *Config {
	return &Config{
		Port:              getenv("PORT", "8080"),
		MongoURI:          getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:           getenv("MONGO_DB", "empresasdb"),
		RabbitURI:         getenv("RABBIT_URI", "amqp://guest:guest@localhost:5672/"),
		RabbitQueue:       getenv("RABBIT_QUEUE", "empresas_log"),
		LogLevel:          parseLevel(getenv("LOG_LEVEL", "info")),
		ReadHeaderTimeout: parseDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		ShutdownTimeout:   parseDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseDuration(env string, def time.Duration) time.Duration {
	if v := os.Getenv(env); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
