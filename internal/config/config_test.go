package config

import (
	"testing"
)

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("APP_BASE_URL", "http://localhost:9090")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")

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
	if cfg.Env != "development" {
		t.Errorf("Env = %s, want development", cfg.Env)
	}
	if cfg.DB.DSN != "file::memory:?cache=shared" {
		t.Errorf("DB.DSN = %s, want file::memory:?cache=shared", cfg.DB.DSN)
	}
	if cfg.Redis.URL != "redis://localhost:6379/0" {
		t.Errorf("Redis.URL = %s, want redis://localhost:6379/0", cfg.Redis.URL)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("default Port = %d, want 8080", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("default Env = %s, want development", cfg.Env)
	}
	if cfg.DB.Driver != "postgres" {
		t.Errorf("default DB.Driver = %s, want postgres", cfg.DB.Driver)
	}
	if cfg.Redis.URL != "redis://localhost:6379/0" {
		t.Errorf("default Redis.URL = %s, want redis://localhost:6379/0", cfg.Redis.URL)
	}
	if cfg.UploadsDir != "./data/uploads" {
		t.Errorf("default UploadsDir = %s, want ./data/uploads", cfg.UploadsDir)
	}
	if !cfg.SignupOpen {
		t.Errorf("default SignupOpen = %v, want true", cfg.SignupOpen)
	}
}

func TestLoadConfigUploadsDirAndSignupFromEnv(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Setenv("APP_UPLOADS_DIR", "/var/data/attachments")
	t.Setenv("APP_SIGNUP", "closed")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UploadsDir != "/var/data/attachments" {
		t.Errorf("UploadsDir = %s, want /var/data/attachments", cfg.UploadsDir)
	}
	if cfg.SignupOpen {
		t.Errorf("SignupOpen = %v, want false when APP_SIGNUP=closed", cfg.SignupOpen)
	}
}

func TestLoadConfigMissingSecret(t *testing.T) {
	_, err := Load()
	if err == nil {
		t.Error("expected error for missing APP_SECRET")
	}
}

func TestLoadConfigMissingDSN(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	_, err := Load()
	if err == nil {
		t.Error("expected error for missing DB_DSN")
	}
}
