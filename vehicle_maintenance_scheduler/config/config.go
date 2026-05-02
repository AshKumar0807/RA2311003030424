package config

import (
	"fmt"
	"os"
	logging "github.com/student/ROLL_NUMBER/logging_middleware"
)

type Config struct {
	Port        string
	LogAPIToken string
	GinMode     string
}

func Load(logger *logging.Logger) (*Config, error) {
	logger.Info("config", "loading application configuration from environment")
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		LogAPIToken: os.Getenv("LOG_API_TOKEN"),
		GinMode:     getEnv("GIN_MODE", "debug"),
	}
	if cfg.LogAPIToken == "" {
		logger.Warn("config", "LOG_API_TOKEN not set — log API calls will be unauthenticated")
	} else {
		logger.Info("config", "LOG_API_TOKEN loaded successfully")
	}
	logger.Info("config", fmt.Sprintf("configuration loaded: port=%s gin_mode=%s", cfg.Port, cfg.GinMode))
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
