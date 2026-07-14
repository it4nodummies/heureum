package permission

import "testing"

func TestForRole(t *testing.T) {
	admin := ForRole("admin", false)
	if !admin["ADMINISTER_PROJECTS"] || !admin["CREATE_ISSUES"] || !admin["BROWSE_PROJECTS"] {
		t.Errorf("admin deve avere tutti i permessi progetto: %v", admin)
	}
	member := ForRole("member", false)
	if !member["CREATE_ISSUES"] || member["ADMINISTER_PROJECTS"] {
		t.Errorf("member: create sì, administer no: %v", member)
	}
	viewer := ForRole("viewer", false)
	if !viewer["BROWSE_PROJECTS"] || viewer["CREATE_ISSUES"] {
		t.Errorf("viewer: solo browse: %v", viewer)
	}
	// admin globale (is_admin) → tutto, anche senza ruolo progetto
	glob := ForRole("", true)
	if !glob["ADMINISTER"] || !glob["ADMINISTER_PROJECTS"] {
		t.Errorf("global admin deve avere tutto: %v", glob)
	}
}

func TestAllKeys(t *testing.T) {
	if len(All()) < 5 {
		t.Error("attese >=5 chiavi permesso")
	}
	for _, p := range All() {
		if p.Key == "" || p.Name == "" {
			t.Errorf("permesso senza key/name: %+v", p)
		}
	}
}
