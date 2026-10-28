package auth

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/user"
)

// newSharedTestDB opens a named shared-cache in-memory DB so that every pooled
// connection (and therefore every goroutine) sees the same users table —
// ":memory:" would hand each connection its own empty database.
func newSharedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&user.User{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func countUsers(t *testing.T, db *gorm.DB, admin *bool) int64 {
	t.Helper()
	q := db.Model(&user.User{})
	if admin != nil {
		q = q.Where("is_admin = ?", *admin)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRegisterWithPolicy_FirstUserIsAdminWhenClosed(t *testing.T) {
	svc := NewService(newSharedTestDB(t), "s")
	u, err := svc.RegisterWithPolicy("a@x.io", "a", "A", "pw", false)
	if err != nil {
		t.Fatalf("first registration on an empty instance must succeed: %v", err)
	}
	if !u.IsAdmin {
		t.Error("first user must be admin")
	}
}

func TestRegisterWithPolicy_SecondUserClosedIsRefused_OpenIsRegular(t *testing.T) {
	svc := NewService(newSharedTestDB(t), "s")
	if _, err := svc.RegisterWithPolicy("a@x.io", "a", "A", "pw", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterWithPolicy("b@x.io", "b", "B", "pw", false); !errors.Is(err, ErrSignupClosed) {
		t.Fatalf("second user with closed policy: err = %v, want ErrSignupClosed", err)
	}
	u, err := svc.RegisterWithPolicy("b@x.io", "b", "B", "pw", true)
	if err != nil {
		t.Fatalf("second user with open policy: %v", err)
	}
	if u.IsAdmin {
		t.Error("second user must not be admin")
	}
}

// Simulates a writer committing between the count and the insert (what another
// replica could do under READ COMMITTED): the bootstrap grant must be withdrawn.
func TestRegisterWithPolicy_RacedBootstrap_ClosedRemovesLoser(t *testing.T) {
	db := newSharedTestDB(t)
	svc := NewService(db, "s")
	svc.bootstrapHook = func() {
		if err := db.Create(&user.User{ID: "racer", Email: "r@x.io", Username: "r", IsAdmin: true, IsActive: true}).Error; err != nil {
			t.Fatal(err)
		}
	}

	_, err := svc.RegisterWithPolicy("a@x.io", "a", "A", "pw", false)
	if !errors.Is(err, ErrSignupClosed) {
		t.Fatalf("err = %v, want ErrSignupClosed (a row raced in and the policy is closed)", err)
	}
	if n := countUsers(t, db, nil); n != 1 {
		t.Errorf("users = %d, want 1 (the loser's row must be removed)", n)
	}
	admin := true
	if n := countUsers(t, db, &admin); n != 1 {
		t.Errorf("admins = %d, want 1", n)
	}
}

// Two replicas both bootstrap (each sees no admin, each inserts an admin):
// the one that re-checks and finds two active admins deletes its OWN row and
// returns a retryable error, so exactly one admin remains. Deleting — not
// demoting — the loser matters when both lose: demotion would leave zero
// admins; deletion brings the active-admin count back to 0 so the retry
// bootstraps cleanly.
func TestRegisterWithPolicy_TwoBootstrappers_LoserRetriesAndTableKeepsOneAdmin(t *testing.T) {
	db := newSharedTestDB(t)
	svc := NewService(db, "s")
	svc.bootstrapHook = func() {
		if err := db.Create(&user.User{ID: "racer", Email: "r@x.io", Username: "r", IsAdmin: true, IsActive: true}).Error; err != nil {
			t.Fatal(err)
		}
	}

	u, err := svc.RegisterWithPolicy("a@x.io", "a", "A", "pw", true)
	if !errors.Is(err, ErrBootstrapRaced) {
		t.Fatalf("err = %v (user %v), want ErrBootstrapRaced", err, u)
	}
	if n := countUsers(t, db, nil); n != 1 {
		t.Errorf("users = %d, want 1 (the loser's row must be removed, not demoted)", n)
	}
	var loser int64
	db.Model(&user.User{}).Where("email = ?", "a@x.io").Count(&loser)
	if loser != 0 {
		t.Errorf("loser row still present")
	}
	admin := true
	if n := countUsers(t, db, &admin); n != 1 {
		t.Errorf("admins = %d, want 1", n)
	}

	// The retry follows the normal policy: open → regular user.
	svc.bootstrapHook = nil
	u, err = svc.RegisterWithPolicy("a@x.io", "a", "A", "pw", true)
	if err != nil {
		t.Fatalf("retry after a raced bootstrap: %v", err)
	}
	if u.IsAdmin {
		t.Error("retried registration must not be admin (another admin exists)")
	}
}

// The bcrypt hash is computed before bootstrapMu is taken: a password bcrypt
// refuses (> 72 bytes) fails before the count, so the hook (which runs under
// the lock, after the count) is never reached and nothing is written.
func TestRegisterWithPolicy_HashesBeforeBootstrapLock(t *testing.T) {
	db := newSharedTestDB(t)
	svc := NewService(db, "s")
	hookCalled := false
	svc.bootstrapHook = func() { hookCalled = true }

	tooLong := strings.Repeat("x", 73)
	if _, err := svc.RegisterWithPolicy("a@x.io", "a", "A", tooLong, true); err == nil {
		t.Fatal("bcrypt must refuse a 73-byte password")
	}
	if hookCalled {
		t.Error("count/insert section entered although hashing failed: the hash must be computed before the lock")
	}
	if n := countUsers(t, db, nil); n != 0 {
		t.Errorf("users = %d, want 0", n)
	}
}

// Concurrent first registrations within one process: exactly one succeeds and
// becomes admin, every other one is refused by the closed policy.
func TestRegisterWithPolicy_ConcurrentBootstrapIsSerialized(t *testing.T) {
	db := newSharedTestDB(t)
	svc := NewService(db, "s")

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = svc.RegisterWithPolicy(
				fmt.Sprintf("u%d@x.io", i), fmt.Sprintf("u%d", i), "U", "pw", false)
		}(i)
	}
	wg.Wait()

	ok, closed := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrSignupClosed):
			closed++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || closed != n-1 {
		t.Errorf("successes = %d, refused = %d; want 1 and %d", ok, closed, n-1)
	}
	if total := countUsers(t, db, nil); total != 1 {
		t.Errorf("users = %d, want 1", total)
	}
	admin := true
	if a := countUsers(t, db, &admin); a != 1 {
		t.Errorf("admins = %d, want 1", a)
	}
}

// A regular user committed by another replica (open policy) between the
// admin count and the bootstrap insert must NOT make the bootstrapper
// withdraw itself: that would leave the instance with users but no admin,
// and nothing to re-trigger the bootstrap. The admin keeps its grant; the
// regular user is untouched.
func TestRegisterWithPolicy_RacedBootstrap_RegularRegistrantDoesNotStrandAdmin(t *testing.T) {
	db := newSharedTestDB(t)
	svc := NewService(db, "s")
	svc.bootstrapHook = func() {
		if err := db.Create(&user.User{ID: "racer", Email: "r@x.io", Username: "r", IsAdmin: false, IsActive: true}).Error; err != nil {
			t.Fatal(err)
		}
	}

	u, err := svc.RegisterWithPolicy("a@x.io", "a", "A", "pw", true)
	if err != nil {
		t.Fatalf("bootstrap raced by a regular registrant: err = %v, want success", err)
	}
	if !u.IsAdmin {
		t.Error("bootstrapper must keep the admin grant")
	}
	if n := countUsers(t, db, nil); n != 2 {
		t.Errorf("users = %d, want 2 (nothing withdrawn)", n)
	}
	admin := true
	if n := countUsers(t, db, &admin); n != 1 {
		t.Errorf("active admins = %d, want exactly 1", n)
	}
}

// An instance with users but no ACTIVE admin (e.g. the only admin was
// deactivated, or an earlier raced bootstrap stranded it) is treated like a
// fresh one: the next registration is accepted regardless of the signup
// policy and becomes admin. Once an active admin exists the policy applies
// again.
func TestRegisterWithPolicy_StrandedInstanceSelfHeals(t *testing.T) {
	db := newSharedTestDB(t)
	svc := NewService(db, "s")
	for _, u := range []*user.User{
		{ID: "regular", Email: "reg@x.io", Username: "reg", IsAdmin: false, IsActive: true},
		{ID: "oldadmin", Email: "old@x.io", Username: "old", IsAdmin: true, IsActive: false},
	} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	// IsActive carries gorm `default:true`, so Create silently writes true for
	// the zero value: deactivate the old admin with an explicit update.
	if err := db.Model(&user.User{}).Where("id = ?", "oldadmin").Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}

	u, err := svc.RegisterWithPolicy("a@x.io", "a", "A", "pw", false)
	if err != nil {
		t.Fatalf("registration on an instance without an active admin (closed policy): err = %v, want success", err)
	}
	if !u.IsAdmin {
		t.Error("the healing registration must become admin")
	}
	if _, err := svc.RegisterWithPolicy("b@x.io", "b", "B", "pw", false); !errors.Is(err, ErrSignupClosed) {
		t.Fatalf("second registration after the heal (closed policy): err = %v, want ErrSignupClosed", err)
	}
	var active int64
	db.Model(&user.User{}).Where("is_admin = ? AND is_active = ?", true, true).Count(&active)
	if active != 1 {
		t.Errorf("active admins = %d, want 1", active)
	}
}
