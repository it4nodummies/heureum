package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/open-jira/open-jira/internal/api"
	"github.com/open-jira/open-jira/internal/config"
	applog "github.com/open-jira/open-jira/internal/log"
	"github.com/open-jira/open-jira/internal/store"
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
	if err := store.RunMigrations(cfg.DB); err != nil {
		logger.Error("failed to run migrations", "error", err)
		log.Fatal(err)
	}
	router := api.NewRouter(cfg, s.DB)
	logger.Info("starting server", "port", cfg.Port, "env", cfg.Env)
	if err := http.ListenAndServe(":"+strconv.Itoa(cfg.Port), router); err != nil {
		logger.Error("server error", "error", err)
	}
}
