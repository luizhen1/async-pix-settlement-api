package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort      string
	DatabaseURL  string
	KafkaBrokers []string
	KafkaTopic   string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		AppPort:      getEnv("APP_PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pixdb?sslmode=disable"),
		KafkaBrokers: splitCSV(getEnv("KAFKA_BROKERS", "localhost:29092")),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "pix-transfers"),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
