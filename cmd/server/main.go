package main

import (
	"log"

	"github.com/open-jira/open-jira/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("starting server on port %d", cfg.Port)
}
