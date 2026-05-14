package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

type DBConfig struct {
	Driver string
	DSN    string
}

type RedisConfig struct {
	URL string
}

type Config struct {
	Port    int
	Env     string
	Secret  string
	BaseURL string
	DB      DBConfig
	Redis   RedisConfig
}

func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("APP_PORT", "8080"))
	if err != nil || port == 0 {
		port = 8080
	}

	cfg := &Config{
		Port:    port,
		Env:     getEnv("APP_ENV", "development"),
		Secret:  os.Getenv("APP_SECRET"),
		BaseURL: getEnv("APP_BASE_URL", fmt.Sprintf("http://localhost:%d", port)),
		DB: DBConfig{
			Driver: getEnv("DB_DRIVER", "postgres"),
			DSN:    os.Getenv("DB_DSN"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
	}

	if cfg.Secret == "" {
		return nil, errors.New("APP_SECRET is required")
	}
	if cfg.DB.DSN == "" {
		return nil, errors.New("DB_DSN is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
