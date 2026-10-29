package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/auth"
	"github.com/it4nodummies/heureum/internal/storetest"
)

// TestAPIToken_ExpiryRoundTripsOnRealSchema is the real-driver counterpart of
// TestCreateAPIToken_ExpiryIsUTC (R27). That test pins the UTC *intent* on an
// AutoMigrate SQLite DB; this one stores the expiry through the real schema
// (migration 000025: api_tokens.expires_at TIMESTAMP, no time zone) and reads
// it back, so the Postgres leg of the CI matrix exercises the actual codec.
//
// Why only the Postgres leg proves anything about the codec: pgx's timestamp
// codec (pgtype.Timestamp, discardTimeZone in pgtype/timestamp.go) writes the
// wall-clock DIGITS of the Go value in the value's own Location and reads the
// digits back tagged as UTC — it never converts the instant. The round-trip is
// therefore exact only while the value handed to GORM is UTC-located (which is
// what auth.Service.Clock guarantees). If that ever regressed — Clock() losing
// its .UTC(), or the handler adding days to a local time.Now() — the stored
// digits would be the local wall-clock and the instant read back would be off
// by the zone offset. gorm.io/driver/sqlite, by contrast, serializes a
// time.Time as text WITH its offset and parses the same instant back whatever
// the Location, so SQLite passes this test even with the regression. The
// `Equal` assertions below are driver-neutral so the test runs (and must pass)
// on both legs, but it is the Postgres leg (CI runs it with TZ=Europe/Rome so
// that a drift towards local time has a non-zero offset to expose) that turns
// them into a real guard. The to_char assertion is Postgres-only because it
// inspects the stored digits directly, bypassing the decoder entirely.
func TestAPIToken_ExpiryRoundTripsOnRealSchema(t *testing.T) {
	db := storetest.New(t)
	svc := auth.NewService(db, "test-secret-min-32-chars-long-key!!")
	h := NewAuthHandler(svc, true)

	u, err := svc.Register("roundtrip@test.com", "roundtrip", "Round Trip", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Pin the clock to a non-UTC instant with a fixed zone, independent of the
	// process TZ: a zone-digit skew is then visible on any host, not just one
	// where TZ happens to be set.
	cest := time.FixedZone("CEST", 2*3600)
	now := time.Date(2026, 10, 28, 14, 30, 0, 0, cest) // 12:30:00Z
	svc.Now = func() time.Time { return now }
	want := now.UTC().Add(24 * time.Hour) // 2026-10-29T12:30:00Z

	rec := httptest.NewRecorder()
	h.CreateAPIToken(rec, apiTokenRequest(u.ID, "POST", "/rest/api/3/auth/api-tokens", []byte(`{"label":"roundtrip","expiresInDays":1}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID        string     `json:"id"`
		Token     string     `json:"token"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body: %s", err, rec.Body.String())
	}
	if resp.Token == "" {
		t.Fatal("plaintext token missing from the create response")
	}
	if resp.ExpiresAt == nil || !resp.ExpiresAt.Equal(want) {
		t.Errorf("response expires_at = %v, want %v", resp.ExpiresAt, want)
	}

	// Read back through the real schema. Compare instants (Equal), never
	// strings or Locations: the decoded Location legitimately differs per
	// driver, the instant must not.
	var stored auth.APIToken
	if err := db.First(&stored, "id = ?", resp.ID).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.ExpiresAt == nil {
		t.Fatal("stored expires_at is NULL, want the requested expiry")
	}
	if !stored.ExpiresAt.Equal(want) {
		t.Errorf("stored expires_at = %v, want %v (skew %v — on Postgres a skew equal to a zone offset means the value reached pgx in a non-UTC Location, see the comment on this test)",
			stored.ExpiresAt, want, stored.ExpiresAt.Sub(want))
	}

	if storetest.Driver() == "postgres" {
		// Bypass the decoder: the digits Postgres holds must be the UTC digits
		// of the instant. This is the one check that cannot be fooled by a
		// symmetric encode/decode bug, and it has no portable spelling on
		// SQLite (which stores the driver's own text form).
		var digits string
		if err := db.Raw("SELECT to_char(expires_at, 'YYYY-MM-DD HH24:MI:SS') FROM api_tokens WHERE id = ?", resp.ID).Scan(&digits).Error; err != nil {
			t.Fatalf("to_char: %v", err)
		}
		if wantDigits := want.Format("2006-01-02 15:04:05"); digits != wantDigits {
			t.Errorf("stored digits = %q, want %q (the TIMESTAMP column must hold UTC wall-clock digits)", digits, wantDigits)
		}
		if _, offset := time.Now().Zone(); offset == 0 {
			// Not a failure — the fixed-zone clock above still catches a
			// Clock() regression — but a drift towards time.Now() would be
			// invisible on a UTC host. CI sets TZ=Europe/Rome for this reason.
			t.Log("process TZ is UTC: a regression towards local time.Now() would not skew here; run the postgres leg with a non-UTC TZ")
		}
	}

	// Behavioural check with the clock still zoned: the expiry that was stored
	// must be honoured as the instant requested. One hour before it the token
	// is accepted; one hour after it is rejected. A stored value skewed by a
	// (>= 1h) zone offset would flip one of the two.
	svc.Now = func() time.Time { return want.Add(-time.Hour).In(cest) }
	if uid, err := svc.VerifyAPIToken("roundtrip@test.com", resp.Token); err != nil || uid != u.ID {
		t.Errorf("VerifyAPIToken 1h before expiry: uid=%q err=%v, want uid=%q and no error", uid, err, u.ID)
	}
	svc.Now = func() time.Time { return want.Add(time.Hour).In(cest) }
	if _, err := svc.VerifyAPIToken("roundtrip@test.com", resp.Token); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("VerifyAPIToken 1h after expiry: err=%v, want ErrInvalidToken", err)
	}
}
