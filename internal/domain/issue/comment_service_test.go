package issue

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/notification"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCommentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Issue{}, &Comment{}, &IssueHistory{}, &IssueWatcher{}, &user.User{}, &notification.Notification{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestAddCommentADFMentionNotifiesByID: un nodo ADF mention (attrs.id = user id)
// deve notificare l'utente citato anche quando il testo NON contiene un
// @username risolvibile. Prima del wiring (id ADF scartati) B non riceve nulla.
func TestAddCommentADFMentionNotifiesByID(t *testing.T) {
	db := newCommentTestDB(t)
	// username di B ("userb") NON compare come token @ nel corpo: solo il nodo
	// ADF può notificarlo.
	author := user.User{ID: "author-1", Email: "a@x.test", Username: "author", DisplayName: "Author", IsActive: true}
	userB := user.User{ID: "user-b", Email: "b@x.test", Username: "userb", DisplayName: "B", IsActive: true}
	db.Create(&author)
	db.Create(&userB)
	db.Create(&Issue{ID: "iss-1", ProjectID: "p1", Key: "DEMO-1", Title: "Test issue", SeqID: 10000})

	notifSvc := notification.NewService(db)
	svc := NewCommentService(db)
	svc.SetNotifier(notifSvc)

	body := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"hey "},` +
		`{"type":"mention","attrs":{"id":"user-b","text":"@B"}}` +
		`]}]}`

	// Il chiamante (comment_handler) estrae gli accountId dai nodi mention via
	// v3.ExtractMentions e li passa a AddComment.
	if _, err := svc.AddComment("iss-1", author.ID, body, "user-b"); err != nil {
		t.Fatal(err)
	}

	var bMentions int64
	db.Model(&notification.Notification{}).Where("user_id = ? AND type = ?", "user-b", "mention").Count(&bMentions)
	if bMentions != 1 {
		t.Fatalf("user B mention notifications = %d, want 1", bMentions)
	}
	var authorMentions int64
	db.Model(&notification.Notification{}).Where("user_id = ? AND type = ?", author.ID, "mention").Count(&authorMentions)
	if authorMentions != 0 {
		t.Fatalf("author mention notifications = %d, want 0", authorMentions)
	}
}

// TestAddCommentMentionDedupe: un utente citato SIA con @username testuale SIA
// con un nodo ADF mention riceve una sola notifica "mention".
func TestAddCommentMentionDedupe(t *testing.T) {
	db := newCommentTestDB(t)
	author := user.User{ID: "author-1", Email: "a@x.test", Username: "author", DisplayName: "Author", IsActive: true}
	carol := user.User{ID: "carol-1", Email: "c@x.test", Username: "carol", DisplayName: "Carol", IsActive: true}
	db.Create(&author)
	db.Create(&carol)
	db.Create(&Issue{ID: "iss-1", ProjectID: "p1", Key: "DEMO-1", Title: "Test issue", SeqID: 10000})

	notifSvc := notification.NewService(db)
	svc := NewCommentService(db)
	svc.SetNotifier(notifSvc)

	// "@carol" nel testo risolve per username; il nodo ADF risolve per id: stesso utente.
	body := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"ping @carol "},` +
		`{"type":"mention","attrs":{"id":"carol-1","text":"@Carol"}}` +
		`]}]}`

	if _, err := svc.AddComment("iss-1", author.ID, body, "carol-1"); err != nil {
		t.Fatal(err)
	}

	var carolMentions int64
	db.Model(&notification.Notification{}).Where("user_id = ? AND type = ?", "carol-1", "mention").Count(&carolMentions)
	if carolMentions != 1 {
		t.Fatalf("carol mention notifications = %d, want exactly 1", carolMentions)
	}
}
