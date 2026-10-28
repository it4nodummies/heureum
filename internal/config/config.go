package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
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
	// WebhookAllowPrivateHosts lists hostnames, IPs and/or CIDRs that outgoing
	// webhooks may target even though they are private/loopback/link-local
	// (APP_WEBHOOK_ALLOW_PRIVATE_HOSTS, comma-separated). Empty by default:
	// every non-public target is refused at create and at delivery (SSRF).
	WebhookAllowPrivateHosts []string
	// TrustedProxies lists the reverse proxies (IPs and/or CIDRs,
	// APP_TRUSTED_PROXIES, comma-separated) whose X-Forwarded-For header is
	// believed when resolving the client IP for rate limiting. Empty by
	// default: the header is ignored and RemoteAddr is the client.
	TrustedProxies []string
}

// minProductionSecretLen is the minimum APP_SECRET length accepted when
// APP_ENV=production (32 bytes: enough entropy for HMAC-SHA256 / AES-256 keys).
const minProductionSecretLen = 32

func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("APP_PORT", "8080"))
	if err != nil || port == 0 {
		port = 8080
	}

	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil || smtpPort == 0 {
		smtpPort = 587
	}

	env := getEnv("APP_ENV", "development")
	isProduction := env == "production"

	// Signup policy: an explicit APP_SIGNUP always wins; when unset, production
	// is closed by default (the perimeter must not be open on a fresh
	// self-hosted instance) and every other env stays open (dev/e2e flows).
	// While the instance has no active admin a registration is always accepted
	// and becomes admin regardless (bootstrap, see auth.Service.RegisterWithPolicy).
	var signupOpen bool
	switch os.Getenv("APP_SIGNUP") {
	case "":
		signupOpen = !isProduction
	case "open":
		signupOpen = true
	case "closed":
		signupOpen = false
	default:
		return nil, fmt.Errorf("APP_SIGNUP must be \"open\" or \"closed\", got %q", os.Getenv("APP_SIGNUP"))
	}

	cfg := &Config{
		Port:       port,
		Env:        env,
		Secret:     os.Getenv("APP_SECRET"),
		BaseURL:    getEnv("APP_BASE_URL", fmt.Sprintf("http://localhost:%d", port)),
		UploadsDir: getEnv("APP_UPLOADS_DIR", "./data/uploads"),
		SignupOpen: signupOpen,
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

		WebhookAllowPrivateHosts: getEnvList("APP_WEBHOOK_ALLOW_PRIVATE_HOSTS"),
		TrustedProxies:           getEnvList("APP_TRUSTED_PROXIES"),
	}

	// A malformed trusted-proxy entry is a startup error: silently dropping it
	// would either ignore a real proxy (every client shares one bucket) or,
	// worse, never be noticed.
	for _, e := range cfg.TrustedProxies {
		if _, _, err := net.ParseCIDR(e); err == nil {
			continue
		}
		if net.ParseIP(e) == nil {
			return nil, fmt.Errorf("APP_TRUSTED_PROXIES entry %q is neither a CIDR nor an IP address", e)
		}
	}

	if cfg.Secret == "" {
		return nil, errors.New("APP_SECRET is required")
	}
	// APP_SECRET signs JWTs and (from R27) derives encryption keys: refuse to
	// start a production instance on a weak secret. Dev keeps APP_SECRET=dev.
	if isProduction && len(cfg.Secret) < minProductionSecretLen {
		return nil, fmt.Errorf("APP_SECRET must be at least %d characters when APP_ENV=production (got %d)",
			minProductionSecretLen, len(cfg.Secret))
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

// getEnvList reads a comma-separated env var into a slice: entries are
// trimmed and empty ones dropped, so "" and " , " both yield nil.
func getEnvList(key string) []string {
	var out []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
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
