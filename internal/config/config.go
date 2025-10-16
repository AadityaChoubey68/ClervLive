package config

import (
	"fmt"
	"os"
)

type Config struct {
	Address   string
	MaxMemory int64
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}

func LoadConfig() Config {
	address := getEnv("ADDRESS", ":8080")
	maxMemoryMB := getEnvInt("MAX_MEMORY_MB", 2048)

	return Config{
		Address:   address,
		MaxMemory: int64(maxMemoryMB) * 1024 * 1024,
	}
}
