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

func TestLoadConfigSMTPFromEnv(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USER", "mailer")
	t.Setenv("SMTP_PASS", "secret")
	t.Setenv("SMTP_FROM", "noreply@example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SMTPHost != "smtp.example.com" {
		t.Errorf("SMTPHost = %s, want smtp.example.com", cfg.SMTPHost)
	}
	if cfg.SMTPPort != 2525 {
		t.Errorf("SMTPPort = %d, want 2525", cfg.SMTPPort)
	}
	if cfg.SMTPUser != "mailer" {
		t.Errorf("SMTPUser = %s, want mailer", cfg.SMTPUser)
	}
	if cfg.SMTPPass != "secret" {
		t.Errorf("SMTPPass = %s, want secret", cfg.SMTPPass)
	}
	if cfg.SMTPFrom != "noreply@example.com" {
		t.Errorf("SMTPFrom = %s, want noreply@example.com", cfg.SMTPFrom)
	}
}

func TestLoadConfigSMTPPortDefault(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SMTPHost != "" {
		t.Errorf("default SMTPHost = %q, want empty", cfg.SMTPHost)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("default SMTPPort = %d, want 587", cfg.SMTPPort)
	}
}

func TestLoadConfigAuthRateLimit(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Setenv("APP_AUTH_RATELIMIT", "42")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthRateLimit != 42 {
		t.Errorf("AuthRateLimit = %d, want 42", cfg.AuthRateLimit)
	}
}

func TestLoadConfigAuthRateLimitDefault(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthRateLimit != 10 {
		t.Errorf("default AuthRateLimit = %d, want 10", cfg.AuthRateLimit)
	}
}

func TestLoadConfigAuthRateLimitDisabled(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Setenv("APP_AUTH_RATELIMIT", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthRateLimit != 0 {
		t.Errorf("AuthRateLimit = %d, want 0 (disabled)", cfg.AuthRateLimit)
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
