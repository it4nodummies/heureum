package app

import (
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/store"
)

type App struct {
	Config *config.Config
	Store  *store.Store
}

func New(cfg *config.Config) (*App, error) {
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		return nil, err
	}
	if err := store.RunMigrations(cfg.DB); err != nil {
		return nil, err
	}
	return &App{Config: cfg, Store: s}, nil
}

func (a *App) Close() error {
	return a.Store.Close()
}
