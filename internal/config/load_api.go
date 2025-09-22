package config

import (
	"log/slog"
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
		Port:              getenvAny("8080", "PORT", "API_PORT"),
		MongoURI:          getenvAny("mongodb://localhost:27017", "MONGO_URI"),
		MongoDB:           getenv("MONGO_DB", "empresasdb"),
		RabbitURI:         getenvAny("amqp://guest:guest@localhost:5672/", "RABBITMQ_URL", "RABBIT_URI"),
		RabbitQueue:       getenvAny("empresas_log", "RABBITMQ_QUEUE", "RABBIT_QUEUE"),
		LogLevel:          parseLevel(getenv("LOG_LEVEL", "info")),
		ReadHeaderTimeout: parseDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		ShutdownTimeout:   parseDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}
