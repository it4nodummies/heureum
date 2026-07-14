// Package permission definisce le chiavi di permesso supportate e la loro
// derivazione dal ruolo di progetto (project_members.role) e dal flag globale
// is_admin. NB: è un modello PRAGMATICO per il gating UI — non un permission
// scheme configurabile (rinviato a un round successivo).
package permission

// Def è la definizione di una chiave di permesso stile Jira.
type Def struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

var defs = []Def{
	{"ADMINISTER", "Administer Jira", "Global administration", "GLOBAL"},
	{"ADMINISTER_PROJECTS", "Administer Projects", "Manage project settings/workflow", "PROJECT"},
	{"BROWSE_PROJECTS", "Browse Projects", "View project and issues", "PROJECT"},
	{"CREATE_ISSUES", "Create Issues", "Create issues in the project", "PROJECT"},
	{"EDIT_ISSUES", "Edit Issues", "Edit issues in the project", "PROJECT"},
	{"TRANSITION_ISSUES", "Transition Issues", "Move issues through workflow", "PROJECT"},
	{"DELETE_ISSUES", "Delete Issues", "Delete issues", "PROJECT"},
	{"MANAGE_SPRINTS", "Manage Sprints", "Create/start/complete sprints", "PROJECT"},
}

// All restituisce tutte le definizioni di permesso.
func All() []Def { return defs }

// ForRole calcola l'insieme dei permessi per un ruolo di progetto
// (admin/member/viewer, stringa vuota = nessun ruolo) + flag is_admin globale.
func ForRole(role string, isGlobalAdmin bool) map[string]bool {
	out := map[string]bool{}
	set := func(keys ...string) {
		for _, k := range keys {
			out[k] = true
		}
	}
	if isGlobalAdmin {
		for _, d := range defs {
			out[d.Key] = true
		}
		return out
	}
	switch role {
	case "admin":
		set("ADMINISTER_PROJECTS", "BROWSE_PROJECTS", "CREATE_ISSUES", "EDIT_ISSUES", "TRANSITION_ISSUES", "DELETE_ISSUES", "MANAGE_SPRINTS")
	case "member":
		set("BROWSE_PROJECTS", "CREATE_ISSUES", "EDIT_ISSUES", "TRANSITION_ISSUES", "MANAGE_SPRINTS")
	case "viewer":
		set("BROWSE_PROJECTS")
	}
	return out
}
