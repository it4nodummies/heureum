package store

import (
	"testing"

	gomysql "github.com/go-sql-driver/mysql"

	"github.com/it4nodummies/heureum/internal/config"
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

func TestEnsureMultiStatements(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name: "no parameters",
			dsn:  "user:pass@tcp(127.0.0.1:3306)/heureum",
		},
		{
			name: "other parameters already present",
			dsn:  "user:pass@tcp(127.0.0.1:3306)/heureum?parseTime=true&loc=UTC",
		},
		{
			name: "multiStatements already true",
			dsn:  "user:pass@tcp(127.0.0.1:3306)/heureum?multiStatements=true",
		},
		{
			name:    "malformed DSN",
			dsn:     "not a valid dsn###",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ensureMultiStatements(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ensureMultiStatements(%q) error = nil, want error", tt.dsn)
				}
				return
			}
			if err != nil {
				t.Fatalf("ensureMultiStatements(%q) unexpected error: %v", tt.dsn, err)
			}

			// Assert on the parsed result, not exact string equality — FormatDSN
			// does not guarantee parameter order.
			gotCfg, err := gomysql.ParseDSN(got)
			if err != nil {
				t.Fatalf("ParseDSN(%q) (round-trip of ensureMultiStatements output) error: %v", got, err)
			}
			if !gotCfg.MultiStatements {
				t.Errorf("ensureMultiStatements(%q) = %q, MultiStatements = false, want true", tt.dsn, got)
			}

			wantCfg, err := gomysql.ParseDSN(tt.dsn)
			if err != nil {
				t.Fatalf("ParseDSN(%q) (original DSN) error: %v", tt.dsn, err)
			}
			wantCfg.MultiStatements = true
			if gotCfg.FormatDSN() != wantCfg.FormatDSN() {
				t.Errorf("ensureMultiStatements(%q) changed fields other than MultiStatements:\ngot  %+v\nwant %+v", tt.dsn, gotCfg, wantCfg)
			}
		})
	}
}
