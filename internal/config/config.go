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
	Port       int
	Env        string
	Secret     string
	BaseURL    string
	UploadsDir string
	SignupOpen bool
	DB         DBConfig
	Redis      RedisConfig
	SMTPHost   string
	SMTPPort   int
	SMTPUser   string
	SMTPPass   string
	SMTPFrom   string
	// AuthRateLimit is the number of auth requests (login+register, shared)
	// allowed per client IP in a fixed 5-minute window. 0 disables limiting.
	AuthRateLimit int
}

func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("APP_PORT", "8080"))
	if err != nil || port == 0 {
		port = 8080
	}

	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil || smtpPort == 0 {
		smtpPort = 587
	}

	cfg := &Config{
		Port:       port,
		Env:        getEnv("APP_ENV", "development"),
		Secret:     os.Getenv("APP_SECRET"),
		BaseURL:    getEnv("APP_BASE_URL", fmt.Sprintf("http://localhost:%d", port)),
		UploadsDir: getEnv("APP_UPLOADS_DIR", "./data/uploads"),
		SignupOpen: getEnv("APP_SIGNUP", "open") != "closed",
		DB: DBConfig{
			Driver: getEnv("DB_DRIVER", "postgres"),
			DSN:    os.Getenv("DB_DSN"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      smtpPort,
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		SMTPFrom:      os.Getenv("SMTP_FROM"),
		AuthRateLimit: getEnvInt("APP_AUTH_RATELIMIT", 10),
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

// getEnvInt reads an integer env var, returning fallback when unset or empty.
// A present-but-invalid value also falls back. Note that an explicit "0" is a
// valid value (e.g. APP_AUTH_RATELIMIT=0 disables rate limiting).
func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}
