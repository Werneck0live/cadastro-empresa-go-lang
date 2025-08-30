package config

import "os"

type Config struct {
	Port        string
	MongoURI    string
	MongoDB     string
	RabbitURI   string
	RabbitQueue string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		MongoURI:    getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:     getEnv("MONGO_DB", "empresasdb"),
		RabbitURI:   getEnv("RABBIT_URI", "amqp://guest:guest@localhost:5672/"),
		RabbitQueue: getEnv("RABBIT_QUEUE", "empresas_log"),
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
