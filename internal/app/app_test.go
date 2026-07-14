package app

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/config"
)

func TestNewSQLiteApp(t *testing.T) {
	t.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_DSN", "file::memory:?cache=shared")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer a.Close()

	if a.Store == nil {
		t.Error("Store is nil")
	}
	if a.Store.DB == nil {
		t.Error("DB is nil")
	}

	var count int64
	a.Store.DB.Raw("SELECT count(*) FROM users").Scan(&count)
}
