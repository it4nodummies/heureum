package store

import (
	"testing"

	"github.com/open-jira/open-jira/internal/config"
)

func TestNewSQLite(t *testing.T) {
	cfg := config.DBConfig{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
	}
	s, err := New(cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	if s.Driver != "sqlite" {
		t.Errorf("Driver = %s, want sqlite", s.Driver)
	}
	if s.DB == nil {
		t.Error("DB is nil")
	}
}

func TestNewUnsupportedDriver(t *testing.T) {
	cfg := config.DBConfig{
		Driver: "cassandra",
		DSN:    "host=localhost",
	}
	_, err := New(cfg, "test")
	if err == nil {
		t.Error("expected error for unsupported driver")
	}
}
