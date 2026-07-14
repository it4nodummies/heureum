package contract

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAuthz_FilterMutationsAreOwnerScoped verifica che un utente non possa
// mutare (update/delete/favourite) un filtro salvato di un altro utente
// (Round 11, Task 7): bob non deve poter toccare i filtri di alice, mentre
// alice deve poter continuare a mutare i propri.
func TestAuthz_FilterMutationsAreOwnerScoped(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")

	resC := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/filter", map[string]any{
		"name": "alice's filter", "jql": "project = AZ",
	})
	if resC.StatusCode != http.StatusOK {
		t.Fatalf("POST /filter status = %d, want 200", resC.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resC.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	resC.Body.Close()
	id := created.ID
	if id == "" {
		t.Fatal("expected non-empty filter id")
	}

	// bob (non proprietario) non può aggiornare il filtro di alice.
	resU := doJSON(t, srv, http.MethodPut, bob, "/rest/api/3/filter/"+id, map[string]any{
		"name": "hijacked", "jql": "project = AZ",
	})
	if resU.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT /filter/%s (bob) status = %d, want 403", id, resU.StatusCode)
	}

	// bob non può marcare/smarcare il filtro di alice come preferito.
	resFav := doJSON(t, srv, http.MethodPut, bob, "/rest/api/3/filter/"+id+"/favourite", nil)
	if resFav.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT /filter/%s/favourite (bob) status = %d, want 403", id, resFav.StatusCode)
	}

	// bob non può cancellare il filtro di alice.
	resD := doJSON(t, srv, http.MethodDelete, bob, "/rest/api/3/filter/"+id, nil)
	if resD.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE /filter/%s (bob) status = %d, want 403", id, resD.StatusCode)
	}

	// alice, proprietaria, può ancora mutare e cancellare il proprio filtro.
	resUOK := doJSON(t, srv, http.MethodPut, alice, "/rest/api/3/filter/"+id, map[string]any{
		"name": "renamed", "jql": "project = AZ",
	})
	if resUOK.StatusCode != http.StatusOK {
		t.Fatalf("PUT /filter/%s (alice) status = %d, want 200", id, resUOK.StatusCode)
	}
	resDOK := doJSON(t, srv, http.MethodDelete, alice, "/rest/api/3/filter/"+id, nil)
	if resDOK.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /filter/%s (alice) status = %d, want 204", id, resDOK.StatusCode)
	}
}

// TestAuthz_DashboardMutationsAreOwnerScoped verifica che un utente non possa
// mutare (update/delete/widget add-remove) un dashboard di un altro utente
// (Round 11, Task 7).
func TestAuthz_DashboardMutationsAreOwnerScoped(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")

	resC := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/dashboard", map[string]any{
		"name": "alice's dashboard",
	})
	if resC.StatusCode != http.StatusCreated {
		t.Fatalf("POST /dashboard status = %d, want 201", resC.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resC.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	resC.Body.Close()
	id := created.ID
	if id == "" {
		t.Fatal("expected non-empty dashboard id")
	}

	// bob (non proprietario) non può aggiornare il dashboard di alice.
	resU := doJSON(t, srv, http.MethodPut, bob, "/rest/api/3/dashboard/"+id, map[string]any{
		"name": "hijacked",
	})
	if resU.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT /dashboard/%s (bob) status = %d, want 403", id, resU.StatusCode)
	}

	// bob non può aggiungere un widget al dashboard di alice.
	resAdd := doJSON(t, srv, http.MethodPost, bob, "/rest/api/3/dashboards/"+id+"/widgets", map[string]any{
		"widget_type": "assigned_to_me",
	})
	if resAdd.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /dashboard/%s/widgets (bob) status = %d, want 403", id, resAdd.StatusCode)
	}

	// bob non può cancellare il dashboard di alice.
	resD := doJSON(t, srv, http.MethodDelete, bob, "/rest/api/3/dashboard/"+id, nil)
	if resD.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE /dashboard/%s (bob) status = %d, want 403", id, resD.StatusCode)
	}

	// alice, proprietaria, può ancora mutare (add widget) e cancellare il
	// proprio dashboard.
	resAddOK := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/dashboards/"+id+"/widgets", map[string]any{
		"widget_type": "assigned_to_me",
	})
	if resAddOK.StatusCode != http.StatusCreated {
		t.Fatalf("POST /dashboard/%s/widgets (alice) status = %d, want 201", id, resAddOK.StatusCode)
	}
	var widget struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resAddOK.Body).Decode(&widget); err != nil {
		t.Fatal(err)
	}
	resAddOK.Body.Close()

	// bob non può rimuovere un widget del dashboard di alice.
	resRm := doJSON(t, srv, http.MethodDelete, bob, "/rest/api/3/dashboards/"+id+"/widgets/"+widget.ID, nil)
	if resRm.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE /dashboards/%s/widgets/%s (bob) status = %d, want 403", id, widget.ID, resRm.StatusCode)
	}

	resRmOK := doJSON(t, srv, http.MethodDelete, alice, "/rest/api/3/dashboards/"+id+"/widgets/"+widget.ID, nil)
	if resRmOK.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /dashboard/%s/widgets/%s (alice) status = %d, want 204", id, widget.ID, resRmOK.StatusCode)
	}

	resDOK := doJSON(t, srv, http.MethodDelete, alice, "/rest/api/3/dashboard/"+id, nil)
	if resDOK.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /dashboard/%s (alice) status = %d, want 204", id, resDOK.StatusCode)
	}
}

// TestAuthz_NonMemberForbiddenOnMutations verifica che un utente che non è
// membro di un progetto non possa mutarlo (Round 11, Task 8): bob non è
// membro del progetto "AZ" di alice, quindi sia la creazione di un'issue nel
// progetto sia l'amministrazione del progetto stesso devono rispondere 403.
func TestAuthz_NonMemberForbiddenOnMutations(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")
	createProjectViaAPI(t, srv, alice, "AZ", "Alice Proj")

	// bob non è membro del progetto di alice → creare un'issue in "AZ" = 403.
	resIssue := doJSON(t, srv, http.MethodPost, bob, "/rest/api/3/issue", map[string]any{
		"fields": map[string]any{
			"project":   map[string]any{"key": "AZ"},
			"summary":   "hack",
			"issuetype": map[string]any{"name": "Task"},
		},
	})
	if resIssue.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /issue (bob, non-member) status = %d, want 403", resIssue.StatusCode)
	}

	// bob non può amministrare (rinominare) il progetto di alice.
	resProj := doJSON(t, srv, http.MethodPut, bob, "/rest/api/3/project/AZ", map[string]any{"name": "Hijacked"})
	if resProj.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT /project/AZ (bob, non-member) status = %d, want 403", resProj.StatusCode)
	}

	// bob non può nemmeno gestire i webhook del progetto di alice
	// (richiede ADMINISTER_PROJECTS).
	resHook := doJSON(t, srv, http.MethodPost, bob, "/rest/api/3/project/AZ/webhooks", map[string]any{
		"name": "hijacked-hook", "url": "https://example.com/hook",
	})
	if resHook.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /project/AZ/webhooks (bob, non-member) status = %d, want 403", resHook.StatusCode)
	}
}

// TestAuthz_CreatorCanMutateOwnProject è il controllo positivo di
// TestAuthz_NonMemberForbiddenOnMutations: il creatore di un progetto è
// admin del progetto stesso (Round 11, Task 1) e deve poter continuare a
// creare issue al suo interno.
func TestAuthz_CreatorCanMutateOwnProject(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, alice, "AZ", "Alice Proj")

	key := createIssueViaAPI(t, srv, alice, "AZ", "legit")
	if key == "" {
		t.Fatal("il creatore del progetto deve poter creare issue al suo interno")
	}
}

// TestAuthz_NonAdminForbiddenOnGroups verifica che un utente non-admin
// globale non possa creare gruppi (Round 11, Task 8): la creazione di
// gruppi è un'operazione di amministrazione globale, non legata a un
// progetto specifico.
func TestAuthz_NonAdminForbiddenOnGroups(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)

	resp := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/group", map[string]any{"name": "x"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /group (non-admin) status = %d, want 403", resp.StatusCode)
	}
}
