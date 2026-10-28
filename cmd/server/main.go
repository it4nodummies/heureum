package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api"
	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/domain/git"
	applog "github.com/it4nodummies/heureum/internal/log"
	"github.com/it4nodummies/heureum/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	logger := applog.New(cfg.Env)
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		log.Fatal(err)
	}
	defer s.Close()
	if err := store.RunMigrations(s); err != nil {
		logger.Error("failed to run migrations", "error", err)
		log.Fatal(err)
	}
	// Git provider lockdown (R27): rows created before token encryption and
	// the separate webhook URL token are fixed up in place, idempotently.
	// Counts only — token values are never logged.
	gitStats, err := git.NewConfigService(s.DB, git.KeyFromSecret(cfg.Secret)).EnsureBackfill()
	if err != nil {
		logger.Error("git provider backfill failed", "error", err)
		log.Fatal(err)
	}
	logger.Info("git provider backfill", "encrypted_tokens", gitStats.Encrypted, "url_tokens_generated", gitStats.URLTokens)
	if gitStats.EmptyWebhookSecrets > 0 {
		logger.Warn("git providers with an empty webhook_secret: their inbound webhooks are rejected until the provider is re-saved with a secret", "count", gitStats.EmptyWebhookSecrets)
	}
	router := api.NewRouter(cfg, s.DB)
	logger.Info("starting server", "port", cfg.Port, "env", cfg.Env)
	if err := http.ListenAndServe(":"+strconv.Itoa(cfg.Port), router); err != nil {
		logger.Error("server error", "error", err)
	}
}
