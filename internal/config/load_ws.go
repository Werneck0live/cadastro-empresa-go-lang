package config

import (
	"log/slog"
	"time"
)

type WSConfig struct {
	Addr              string // :8090
	RabbitURI         string
	RabbitQueue       string
	LogLevel          slog.Level
	ReadHeaderTimeout time.Duration // p/ http.Server
	ShutdownTimeout   time.Duration
	ConsumerPrefetch  int // ajuste de QoS no consumidor
}

func LoadWSConfig() *WSConfig {
	return &WSConfig{
		Addr:              getenv("WS_ADDR", ":8090"),
		RabbitURI:         getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitQueue:       getenv("RABBITMQ_QUEUE", "empresas_log"),
		LogLevel:          parseLevel(getenv("LOG_LEVEL", "info")),
		ReadHeaderTimeout: parseDuration("WS_READ_HEADER_TIMEOUT", 5*time.Second),
		ShutdownTimeout:   parseDuration("WS_SHUTDOWN_TIMEOUT", 10*time.Second),
		ConsumerPrefetch:  parseInt("WS_PREFETCH", 50),
	}
}
