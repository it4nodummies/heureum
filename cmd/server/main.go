package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api"
	"github.com/it4nodummies/heureum/internal/config"
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
