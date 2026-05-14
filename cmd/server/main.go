package main

import (
	"log"
	"os"

	"github.com/open-jira/open-jira/internal/config"
	applog "github.com/open-jira/open-jira/internal/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	logger := applog.New(cfg.Env)
	logger.Info("starting server", "port", cfg.Port, "env", cfg.Env)
	os.Exit(0)
}
