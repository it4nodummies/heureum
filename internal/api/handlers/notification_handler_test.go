package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/notification"
)

func setupNotificationHandler(t *testing.T) (*NotificationHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&notification.Notification{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewNotificationHandler(notification.NewService(db)), db
}

func patchRead(h *NotificationHandler, uid, id string) *httptest.ResponseRecorder {
	req := withUID(httptest.NewRequest("PATCH", "/rest/api/3/notifications/"+id+"/read", nil), uid)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.MarkRead(rec, req)
	return rec
}

// TestMarkRead_OtherUsersNotificationIs404: the notification id is not
// scoped by the path, so the handler must bind it to the caller; another
// user's notification is a 404 (not leaked, not modified). The owner can
// still mark it read.
func TestMarkRead_OtherUsersNotificationIs404(t *testing.T) {
	h, db := setupNotificationHandler(t)
	owner, other := uuid.New().String(), uuid.New().String()
	svc := notification.NewService(db)
	if err := svc.Create(owner, "mention", "hi", "", "/issues/X-1"); err != nil {
		t.Fatal(err)
	}
	var n notification.Notification
	if err := db.First(&n).Error; err != nil {
		t.Fatal(err)
	}

	if rec := patchRead(h, other, n.ID); rec.Code != http.StatusNotFound {
		t.Errorf("other user's MarkRead: status = %d, want 404, body: %s", rec.Code, rec.Body.String())
	}
	var reloaded notification.Notification
	if err := db.First(&reloaded, "id = ?", n.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.IsRead {
		t.Errorf("notification marked read by a non-owner")
	}

	if rec := patchRead(h, owner, n.ID); rec.Code != http.StatusNoContent {
		t.Errorf("owner MarkRead: status = %d, want 204, body: %s", rec.Code, rec.Body.String())
	}
	if err := db.First(&reloaded, "id = ?", n.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reloaded.IsRead {
		t.Errorf("owner's MarkRead did not persist")
	}
	if rec := patchRead(h, owner, uuid.New().String()); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: status = %d, want 404", rec.Code)
	}
}
