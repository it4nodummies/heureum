package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// withUID inietta l'uid nel contesto della richiesta usando la stessa chiave
// esportata (middleware.UserIDKey) letta da middleware.UserIDFromContext:
// non serve alcun nuovo seam di test, la costante è già esportata anche se
// il suo tipo (contextKey) non lo è.
func withUID(r *http.Request, uid string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, uid))
}

func nextOK() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func newEnforceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createEnforceUser(t *testing.T, db *gorm.DB, isAdmin bool) string {
	t.Helper()
	id := uuid.New().String()
	u := &user.User{
		ID:          id,
		Email:       id + "@example.com",
		Username:    id,
		DisplayName: id,
		IsAdmin:     isAdmin,
		IsActive:    true,
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func setupEnforce(t *testing.T) (chk *Checker, adminID, memberID, outsiderID, projectKey string) {
	t.Helper()
	db := newEnforceDB(t)
	userSvc := user.NewService(db)
	projSvc := project.NewService(db, nil)

	adminID = createEnforceUser(t, db, true)
	memberID = createEnforceUser(t, db, false)
	outsiderID = createEnforceUser(t, db, false)

	p, err := projSvc.Create("Project One", "P1", "", project.TypeScrum)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectKey = p.Key

	if err := projSvc.AddMember(p.ID, memberID, project.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}

	chk = New(userSvc, projSvc, nil, nil, nil, nil, nil)
	return
}

func TestEnforce_AllowsWithPermission(t *testing.T) {
	chk, _, memberID, _, key := setupEnforce(t)

	h := chk.Enforce(permission.CreateIssues, chk.ByKey, nextOK())

	r := httptest.NewRequest(http.MethodPost, "/rest/api/3/project/"+key+"/issues", nil)
	r.SetPathValue("key", key)
	r = withUID(r, memberID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEnforce_ForbidsWithoutPermission(t *testing.T) {
	chk, _, _, outsiderID, key := setupEnforce(t)

	h := chk.Enforce(permission.AdministerProjects, chk.ByKey, nextOK())

	r := httptest.NewRequest(http.MethodPut, "/rest/api/3/project/"+key, nil)
	r.SetPathValue("key", key)
	r = withUID(r, outsiderID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEnforce_MemberWithoutAdminPermissionForbidden(t *testing.T) {
	chk, _, memberID, _, key := setupEnforce(t)

	h := chk.Enforce(permission.AdministerProjects, chk.ByKey, nextOK())

	r := httptest.NewRequest(http.MethodPut, "/rest/api/3/project/"+key, nil)
	r.SetPathValue("key", key)
	r = withUID(r, memberID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member without AdministerProjects, got %d", w.Code)
	}
}

func TestEnforce_UnresolvedTargetPassesThrough(t *testing.T) {
	chk, _, _, outsiderID, _ := setupEnforce(t)

	h := chk.Enforce(permission.AdministerProjects, chk.ByKey, nextOK())

	r := httptest.NewRequest(http.MethodPut, "/rest/api/3/project/DOES-NOT-EXIST", nil)
	r.SetPathValue("key", "DOES-NOT-EXIST")
	r = withUID(r, outsiderID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	// resolver ok=false ⇒ decorator lascia decidere al handler (qui: 200
	// perché nextOK ignora il body/risoluzione — nella realtà sarebbe un 404).
	if w.Code != http.StatusOK {
		t.Fatalf("expected pass-through to next (200), got %d", w.Code)
	}
}

func TestEnforce_GlobalAdminBypassesProjectRole(t *testing.T) {
	chk, adminID, _, _, key := setupEnforce(t)

	h := chk.Enforce(permission.AdministerProjects, chk.ByKey, nextOK())

	r := httptest.NewRequest(http.MethodPut, "/rest/api/3/project/"+key, nil)
	r.SetPathValue("key", key)
	r = withUID(r, adminID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for global admin, got %d", w.Code)
	}
}

func TestEnforceGlobalAdmin_Allows(t *testing.T) {
	chk, adminID, _, _, _ := setupEnforce(t)

	h := chk.EnforceGlobalAdmin(nextOK())

	r := httptest.NewRequest(http.MethodGet, "/rest/api/3/group", nil)
	r = withUID(r, adminID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for global admin, got %d", w.Code)
	}
}

func TestEnforceGlobalAdmin_ForbidsNonAdmin(t *testing.T) {
	chk, _, memberID, _, _ := setupEnforce(t)

	h := chk.EnforceGlobalAdmin(nextOK())

	r := httptest.NewRequest(http.MethodGet, "/rest/api/3/group", nil)
	r = withUID(r, memberID)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", w.Code)
	}
}
