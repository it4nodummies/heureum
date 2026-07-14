package contract

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api"
	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/domain/auth"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/store"
)

// newTestServerDB replica il corpo di newTestServer (myself_test.go:18) ma
// restituisce anche il *gorm.DB sottostante, così i test possono impostare
// direttamente flag/ruoli (es. promuovere un utente ad admin globale) senza
// passare per un endpoint amministrativo che al momento non esiste.
func newTestServerDB(t *testing.T) (*httptest.Server, *auth.Service, *gorm.DB) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	cfg := &config.Config{
		Port: 0, Env: "test", Secret: "contract-test-secret",
		BaseURL: "http://localhost:8080",
		DB:      config.DBConfig{Driver: "sqlite", DSN: dsn},
	}
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := store.RunMigrations(cfg.DB); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.NewRouter(cfg, s.DB))
	t.Cleanup(srv.Close)
	return srv, auth.NewService(s.DB, cfg.Secret), s.DB
}

// registerUserAndLogin registra un utente arbitrario (non admin) e ritorna il
// suo jwt, per test multi-utente che non vogliono riusare alice.
func registerUserAndLogin(t *testing.T, authSvc *auth.Service, email, username string) string {
	t.Helper()
	if _, err := authSvc.Register(email, username, username, "password-123"); err != nil {
		t.Fatal(err)
	}
	jwt, err := authSvc.Login(email, "password-123")
	if err != nil {
		t.Fatal(err)
	}
	return jwt
}

// promoteAdmin imposta is_admin=true sull'utente con quell'email. Il Checker
// rilegge is_admin dal DB ad ogni richiesta (user.Service.GetByID), quindi non
// serve un nuovo login dopo la promozione: il jwt già emesso continua a
// funzionare, la valutazione admin avviene ogni volta lato server.
func promoteAdmin(t *testing.T, db *gorm.DB, email string) {
	t.Helper()
	if err := db.Model(&user.User{}).Where("email = ?", email).Update("is_admin", true).Error; err != nil {
		t.Fatal(err)
	}
}
