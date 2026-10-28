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

// ── Round 27: perimeter defaults ──────────────────────────────────────────────

const prodSecret = "production-secret-at-least-32-characters-long"

func TestLoad_ProductionDefaultsSignupClosed(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_SECRET", prodSecret)
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	// APP_SIGNUP deliberately unset.

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SignupOpen {
		t.Errorf("SignupOpen = true in production with APP_SIGNUP unset, want false (closed by default)")
	}
}

func TestLoad_NonProductionDefaultsSignupOpen(t *testing.T) {
	for _, env := range []string{"", "development", "test"} {
		t.Run("APP_ENV="+env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			t.Setenv("APP_SECRET", "dev")
			t.Setenv("DB_DSN", "file::memory:?cache=shared")

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !cfg.SignupOpen {
				t.Errorf("SignupOpen = false with APP_ENV=%q and APP_SIGNUP unset, want true", env)
			}
		})
	}
}

func TestLoad_ExplicitSignupWinsOverEnv(t *testing.T) {
	cases := []struct {
		env, signup string
		want        bool
	}{
		{"production", "open", true},
		{"production", "closed", false},
		{"development", "open", true},
		{"development", "closed", false},
	}
	for _, c := range cases {
		t.Run(c.env+"/"+c.signup, func(t *testing.T) {
			t.Setenv("APP_ENV", c.env)
			t.Setenv("APP_SIGNUP", c.signup)
			t.Setenv("APP_SECRET", prodSecret)
			t.Setenv("DB_DSN", "file::memory:?cache=shared")

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.SignupOpen != c.want {
				t.Errorf("SignupOpen = %v with APP_ENV=%s APP_SIGNUP=%s, want %v", cfg.SignupOpen, c.env, c.signup, c.want)
			}
		})
	}
}

func TestLoad_RejectsUnknownSignupValue(t *testing.T) {
	t.Setenv("APP_SIGNUP", "maybe")
	t.Setenv("APP_SECRET", prodSecret)
	t.Setenv("DB_DSN", "file::memory:?cache=shared")

	if _, err := Load(); err == nil {
		t.Error("expected error for APP_SIGNUP=maybe (only open|closed are valid)")
	}
}

func TestLoad_ProductionRejectsShortSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_SECRET", "short-secret")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")

	if _, err := Load(); err == nil {
		t.Error("expected error: APP_SECRET shorter than 32 bytes must be rejected in production")
	}

	// A 32-byte secret is the minimum accepted.
	t.Setenv("APP_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load(); err != nil {
		t.Errorf("Load() with a 32-byte secret in production: unexpected error %v", err)
	}
}

func TestLoad_NonProductionAcceptsShortSecret(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_SECRET", "dev")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")

	if _, err := Load(); err != nil {
		t.Errorf("Load() in development with APP_SECRET=dev: unexpected error %v", err)
	}
}

func TestLoad_WebhookAllowPrivateHosts(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Setenv("APP_WEBHOOK_ALLOW_PRIVATE_HOSTS", " forgejo.home.arpa, 192.168.1.0/24 ,,10.0.0.5 ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"forgejo.home.arpa", "192.168.1.0/24", "10.0.0.5"}
	if len(cfg.WebhookAllowPrivateHosts) != len(want) {
		t.Fatalf("WebhookAllowPrivateHosts = %v, want %v", cfg.WebhookAllowPrivateHosts, want)
	}
	for i := range want {
		if cfg.WebhookAllowPrivateHosts[i] != want[i] {
			t.Errorf("WebhookAllowPrivateHosts[%d] = %q, want %q", i, cfg.WebhookAllowPrivateHosts[i], want[i])
		}
	}
}

func TestLoad_WebhookAllowPrivateHostsDefaultEmpty(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Setenv("APP_WEBHOOK_ALLOW_PRIVATE_HOSTS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.WebhookAllowPrivateHosts) != 0 {
		t.Errorf("default WebhookAllowPrivateHosts = %v, want empty (no private target allowed)", cfg.WebhookAllowPrivateHosts)
	}
}
