package app

import (
	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/store"
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
