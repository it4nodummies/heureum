// Package permission definisce le chiavi di permesso supportate e la loro
// derivazione dal ruolo di progetto (project_members.role) e dal flag globale
// is_admin. NB: è un modello PRAGMATICO per il gating UI — non un permission
// scheme configurabile (rinviato a un round successivo).
package permission

// Chiavi di permesso esportate — single source of truth per defs e ForRole
// (e per i consumer come internal/api/authz).
const (
	Administer         = "ADMINISTER"
	AdministerProjects = "ADMINISTER_PROJECTS"
	BrowseProjects     = "BROWSE_PROJECTS"
	CreateIssues       = "CREATE_ISSUES"
	EditIssues         = "EDIT_ISSUES"
	TransitionIssues   = "TRANSITION_ISSUES"
	DeleteIssues       = "DELETE_ISSUES"
	ManageSprints      = "MANAGE_SPRINTS"
)

// Def è la definizione di una chiave di permesso stile Jira.
type Def struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

var defs = []Def{
	{Administer, "Administer Jira", "Global administration", "GLOBAL"},
	{AdministerProjects, "Administer Projects", "Manage project settings/workflow", "PROJECT"},
	{BrowseProjects, "Browse Projects", "View project and issues", "PROJECT"},
	{CreateIssues, "Create Issues", "Create issues in the project", "PROJECT"},
	{EditIssues, "Edit Issues", "Edit issues in the project", "PROJECT"},
	{TransitionIssues, "Transition Issues", "Move issues through workflow", "PROJECT"},
	{DeleteIssues, "Delete Issues", "Delete issues", "PROJECT"},
	{ManageSprints, "Manage Sprints", "Create/start/complete sprints", "PROJECT"},
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
		set(AdministerProjects, BrowseProjects, CreateIssues, EditIssues, TransitionIssues, DeleteIssues, ManageSprints)
	case "member":
		set(BrowseProjects, CreateIssues, EditIssues, TransitionIssues, ManageSprints)
	case "viewer":
		set(BrowseProjects)
	}
	return out
}
