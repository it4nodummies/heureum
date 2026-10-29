package notification

import (
	"errors"
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/storetest"
	"gorm.io/gorm"
)

// createTestUser inserts a genuine users row via GORM's own model — the real
// schema's notifications.user_id has a NOT NULL FK to users(id) ON DELETE
// CASCADE (migrations/000001_init_schema.up.sql:311), so every notification in
// these tests needs a real parent row. username is NOT NULL UNIQUE, so each
// caller must pass a distinct one.
func createTestUser(t *testing.T, db *gorm.DB, id, username string) user.User {
	t.Helper()
	u := user.User{
		ID:          id,
		Email:       id + "@example.test",
		Username:    username,
		DisplayName: username,
		IsActive:    true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create test user %q: %v", id, err)
	}
	return u
}

// TestCreateStoresCreatedAtAsATimestamp is the regression guard for the July
// 2026 defect: Notification.CreatedAt was declared int64 with
// gorm:"autoCreateTime" while notifications.created_at is a TIMESTAMP column
// (migrations/000001_init_schema.up.sql:317). SQLite is loosely typed and
// accepted the integer; Postgres rejected it, so every notification insert
// failed there (welcome, mention, comment/watch, sprint, status-change — all
// of them). A test that only checked err == nil on SQLite would not have
// caught this: it must actually read the value back and assert it is a real
// timestamp, and it must run on Postgres.
func TestCreateStoresCreatedAtAsATimestamp(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-created-at", "created-at-user")
	svc := NewService(db)

	// notifications.created_at is TIMESTAMP WITHOUT TIME ZONE. pgx's timestamp
	// codec (pgtype.Timestamp — see discardTimeZone in pgtype/timestamp.go)
	// writes the wall-clock digits of time.Now() in the process's *local*
	// zone, and Postgres round-trips those same digits back tagged as UTC.
	// So on a non-UTC host the read-back value is offset from the true
	// instant by exactly the host's local UTC offset: ~0 on SQLite (no such
	// codec involved), ~= the host offset on Postgres. This is a real,
	// systemic driver-level quirk (independently confirmed against pgx's own
	// source, see the follow-up recommendation in task-4-report.md) — it
	// affects every TIMESTAMP column in the schema via internal/store, not
	// something local to this package, and it is reported rather than fixed
	// here since a fix needs either a migration change or a store-level
	// NowFunc change, both outside this task's scope.
	//
	// Assert the skew directly against that mechanism instead of tolerating
	// a generic wide window: pinning the expectation to "zero on SQLite (no
	// such codec involved), the host's own UTC offset on Postgres (the
	// discardTimeZone round-trip), give or take a few seconds of test
	// execution time" still passes everywhere a wide window would, but
	// additionally fails if the skew is ever something other than the
	// documented quirk — e.g. an unrelated multi-hour timestamp bug under
	// the old 24h band would have slipped through undetected.
	//
	// NOTE for whoever eventually fixes the underlying quirk (migrating these
	// columns to TIMESTAMPTZ, or setting GORM's NowFunc to UTC in
	// internal/store, see docs/superpowers/STATE.md): that fix will make
	// wantSkew wrong on Postgres and this assertion will start failing BY
	// DESIGN. That failure is the fix working, not a regression — update
	// wantSkew (it should become ~0 on both drivers) rather than reverting it.
	driver := storetest.Driver()
	var wantSkew time.Duration
	if driver == "postgres" {
		_, offsetSeconds := time.Now().Zone()
		wantSkew = time.Duration(offsetSeconds) * time.Second
	}
	const skewSlack = 5 * time.Second

	if err := svc.Create("u-created-at", "welcome", "Welcome", "Welcome aboard", "/home"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	notifs, err := svc.ListByUser("u-created-at", false)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(notifs) != 1 {
		t.Fatalf("len(notifs) = %d, want 1", len(notifs))
	}

	got := notifs[0]
	if got.Type != "welcome" || got.Title != "Welcome" || got.Body != "Welcome aboard" || got.Link != "/home" {
		t.Errorf("unexpected notification fields: %+v", got)
	}
	if got.IsRead {
		t.Errorf("IsRead = true, want false for a freshly created notification")
	}

	// The actual regression guard: CreatedAt must round-trip as a real point in
	// time near "now", not as a raw epoch integer smuggled through as seconds
	// (which would land in 1970) nor as a zero value (which a failed/ignored
	// write could also produce).
	if got.CreatedAt.IsZero() {
		t.Fatalf("CreatedAt is zero, want a real timestamp")
	}
	skew := got.CreatedAt.Sub(time.Now())
	if diff := skew - wantSkew; diff < -skewSlack || diff > skewSlack {
		t.Errorf("CreatedAt skew from now = %v, want %v +/- %v (the host's own UTC offset, tolerated only because of the pgx discardTimeZone round-trip quirk on TIMESTAMP-without-timezone columns — a skew of any other size means something else is wrong); got.CreatedAt=%v now=%v",
			skew, wantSkew, skewSlack, got.CreatedAt, time.Now())
	}
	if got.CreatedAt.Year() < 2000 {
		t.Errorf("CreatedAt = %v, looks like an epoch/garbage value (e.g. an int64 written as Unix seconds), not a real timestamp", got.CreatedAt)
	}
}

func TestListByUserScopesToUser(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-a", "user-a")
	createTestUser(t, db, "u-b", "user-b")
	svc := NewService(db)

	if err := svc.Create("u-a", "assignment", "For A", "body", "/a"); err != nil {
		t.Fatalf("Create for A: %v", err)
	}
	if err := svc.Create("u-b", "assignment", "For B", "body", "/b"); err != nil {
		t.Fatalf("Create for B: %v", err)
	}

	notifsA, err := svc.ListByUser("u-a", false)
	if err != nil {
		t.Fatalf("ListByUser(A): %v", err)
	}
	if len(notifsA) != 1 {
		t.Fatalf("len(notifsA) = %d, want 1", len(notifsA))
	}
	if notifsA[0].UserID != "u-a" || notifsA[0].Title != "For A" {
		t.Errorf("A got %+v, want the notification addressed to u-a", notifsA[0])
	}

	notifsB, err := svc.ListByUser("u-b", false)
	if err != nil {
		t.Fatalf("ListByUser(B): %v", err)
	}
	if len(notifsB) != 1 {
		t.Fatalf("len(notifsB) = %d, want 1", len(notifsB))
	}
	if notifsB[0].UserID != "u-b" || notifsB[0].Title != "For B" {
		t.Errorf("B got %+v, want the notification addressed to u-b", notifsB[0])
	}
}

func TestListByUserUnreadOnly(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-unread", "unread-user")
	svc := NewService(db)

	if err := svc.Create("u-unread", "comment", "First", "body", "/1"); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	if err := svc.Create("u-unread", "comment", "Second", "body", "/2"); err != nil {
		t.Fatalf("Create second: %v", err)
	}

	all, err := svc.ListByUser("u-unread", false)
	if err != nil {
		t.Fatalf("ListByUser(all): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(all) = %d, want 2", len(all))
	}

	// Mark just one as read directly, then confirm unreadOnly excludes it.
	if err := svc.MarkRead("u-unread", all[0].ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	unread, err := svc.ListByUser("u-unread", true)
	if err != nil {
		t.Fatalf("ListByUser(unreadOnly): %v", err)
	}
	if len(unread) != 1 {
		t.Fatalf("len(unread) = %d, want 1", len(unread))
	}
	if unread[0].ID == all[0].ID {
		t.Errorf("unreadOnly still returned the notification that was marked read")
	}
}

func TestGetUnreadCountMatchesUnreadRows(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-count", "count-user")
	svc := NewService(db)

	if err := svc.Create("u-count", "comment", "One", "body", "/1"); err != nil {
		t.Fatalf("Create one: %v", err)
	}
	if err := svc.Create("u-count", "comment", "Two", "body", "/2"); err != nil {
		t.Fatalf("Create two: %v", err)
	}
	if err := svc.Create("u-count", "comment", "Three", "body", "/3"); err != nil {
		t.Fatalf("Create three: %v", err)
	}

	count, err := svc.GetUnreadCount("u-count")
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}
	if count != 3 {
		t.Fatalf("GetUnreadCount = %d, want 3", count)
	}

	all, err := svc.ListByUser("u-count", false)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if err := svc.MarkRead("u-count", all[0].ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	count, err = svc.GetUnreadCount("u-count")
	if err != nil {
		t.Fatalf("GetUnreadCount after MarkRead: %v", err)
	}
	if count != 2 {
		t.Fatalf("GetUnreadCount after MarkRead = %d, want 2", count)
	}
}

func TestMarkReadFlipsExactlyOne(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-mark", "mark-user")
	svc := NewService(db)

	if err := svc.Create("u-mark", "comment", "One", "body", "/1"); err != nil {
		t.Fatalf("Create one: %v", err)
	}
	if err := svc.Create("u-mark", "comment", "Two", "body", "/2"); err != nil {
		t.Fatalf("Create two: %v", err)
	}

	all, err := svc.ListByUser("u-mark", false)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(all) = %d, want 2", len(all))
	}

	target := all[0]
	other := all[1]

	if err := svc.MarkRead("u-mark", target.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	var targetRow, otherRow Notification
	if err := db.First(&targetRow, "id = ?", target.ID).Error; err != nil {
		t.Fatalf("reload target: %v", err)
	}
	if err := db.First(&otherRow, "id = ?", other.ID).Error; err != nil {
		t.Fatalf("reload other: %v", err)
	}
	if !targetRow.IsRead {
		t.Errorf("target notification IsRead = false, want true after MarkRead")
	}
	if otherRow.IsRead {
		t.Errorf("other notification IsRead = true, want it untouched by MarkRead")
	}
}

// TestMarkReadRefusesAnotherUsersNotification pins the ownership rule R27
// added to MarkRead: the update is scoped to (id, user_id), so a caller that
// knows (or guesses) someone else's notification id gets ErrNotFound — the same
// answer as for a nonexistent id, so the call leaks nothing — and the row stays
// unread. Both users are real users rows because notifications.user_id is a
// NOT NULL FK in the migrated schema.
func TestMarkReadRefusesAnotherUsersNotification(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-owner", "owner-user")
	createTestUser(t, db, "u-intruder", "intruder-user")
	svc := NewService(db)

	if err := svc.Create("u-owner", "comment", "Mine", "body", "/1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	owned, err := svc.ListByUser("u-owner", false)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(owned) != 1 {
		t.Fatalf("len(owned) = %d, want 1", len(owned))
	}

	if err := svc.MarkRead("u-intruder", owned[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkRead by another user: err = %v, want ErrNotFound", err)
	}
	if err := svc.MarkRead("u-owner", "does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkRead of a nonexistent id: err = %v, want ErrNotFound", err)
	}

	var row Notification
	if err := db.First(&row, "id = ?", owned[0].ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.IsRead {
		t.Errorf("IsRead = true after a rejected MarkRead, want the row untouched")
	}

	// The owner can still mark it.
	if err := svc.MarkRead("u-owner", owned[0].ID); err != nil {
		t.Fatalf("MarkRead by owner: %v", err)
	}
}

func TestMarkAllReadOnlyAffectsThatUser(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-mark-a", "mark-all-a")
	createTestUser(t, db, "u-mark-b", "mark-all-b")
	svc := NewService(db)

	if err := svc.Create("u-mark-a", "comment", "A1", "body", "/1"); err != nil {
		t.Fatalf("Create A1: %v", err)
	}
	if err := svc.Create("u-mark-a", "comment", "A2", "body", "/2"); err != nil {
		t.Fatalf("Create A2: %v", err)
	}
	if err := svc.Create("u-mark-b", "comment", "B1", "body", "/3"); err != nil {
		t.Fatalf("Create B1: %v", err)
	}

	if err := svc.MarkAllRead("u-mark-a"); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}

	countA, err := svc.GetUnreadCount("u-mark-a")
	if err != nil {
		t.Fatalf("GetUnreadCount(A): %v", err)
	}
	if countA != 0 {
		t.Errorf("GetUnreadCount(A) = %d, want 0 after MarkAllRead", countA)
	}

	countB, err := svc.GetUnreadCount("u-mark-b")
	if err != nil {
		t.Fatalf("GetUnreadCount(B): %v", err)
	}
	if countB != 1 {
		t.Errorf("GetUnreadCount(B) = %d, want 1 — MarkAllRead(A) must not touch B's notifications", countB)
	}
}

func TestCreateBatchCreatesOneRowPerUser(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-batch-1", "batch-user-1")
	createTestUser(t, db, "u-batch-2", "batch-user-2")
	createTestUser(t, db, "u-batch-3", "batch-user-3")
	svc := NewService(db)

	ids := []string{"u-batch-1", "u-batch-2", "u-batch-3"}
	if err := svc.CreateBatch(ids, "sprint_started", "Sprint started", `Sprint "Alpha" has started`, "/sprints/s1"); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	for _, id := range ids {
		notifs, err := svc.ListByUser(id, false)
		if err != nil {
			t.Fatalf("ListByUser(%s): %v", id, err)
		}
		if len(notifs) != 1 {
			t.Errorf("user %s got %d notifications, want exactly 1", id, len(notifs))
			continue
		}
		if notifs[0].Type != "sprint_started" {
			t.Errorf("user %s notification type = %q, want %q", id, notifs[0].Type, "sprint_started")
		}
	}

	var total int64
	db.Model(&Notification{}).Where("type = ?", "sprint_started").Count(&total)
	if total != int64(len(ids)) {
		t.Errorf("total sprint_started notifications = %d, want %d", total, len(ids))
	}
}
