package config

import (
	"os"
	"testing"
)

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("APP_PORT", "9090")
	os.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	os.Setenv("APP_BASE_URL", "http://localhost:9090")
	os.Setenv("DB_DRIVER", "sqlite")
	os.Setenv("DB_DSN", "file::memory:?cache=shared")
	os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.Secret != "test-secret-min-32-chars-long!!" {
		t.Errorf("Secret mismatch")
	}
	if cfg.BaseURL != "http://localhost:9090" {
		t.Errorf("BaseURL = %s, want http://localhost:9090", cfg.BaseURL)
	}
	if cfg.DB.Driver != "sqlite" {
		t.Errorf("DB.Driver = %s, want sqlite", cfg.DB.Driver)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	os.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	os.Setenv("DB_DSN", "file::memory:?cache=shared")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("default Port = %d, want 8080", cfg.Port)
	}
}

func TestLoadConfigMissingSecret(t *testing.T) {
	_, err := Load()
	if err == nil {
		t.Error("expected error for missing APP_SECRET")
	}
}
