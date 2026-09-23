// Package authz applica l'autorizzazione lato server: dato l'utente autenticato,
// il progetto risolto dalla richiesta e il permesso richiesto, concede o nega.
package authz

import (
	"errors"

	"github.com/it4nodummies/heureum/internal/domain/automation"
	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/customfield"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/version"
)

// ErrForbidden è restituito quando l'utente non ha il permesso richiesto.
var ErrForbidden = errors.New("forbidden")

// Checker valuta i permessi dell'utente autenticato su progetti/risorse.
// Gli altri servizi (issue/board/sprint/automation/customfield) servono ai
// resolver di autorizzazione (round successivo) che devono risolvere il
// progetto a partire da una risorsa figlia (es. issue -> project).
type Checker struct {
	users    *user.Service
	projects *project.Service
	issues   *issue.Service
	boards   *board.Service
	sprints  *sprint.Service
	autos    *automation.Service
	cfs      *customfield.Service
	versions *version.Service
}

// New costruisce un Checker con tutti i servizi necessari ai resolver di
// autorizzazione (issues/boards/sprints/autos/cfs/versions sono usati solo dai
// resolver, non dai metodi RequireProject/RequireGlobalAdmin in questo file).
func New(users *user.Service, projects *project.Service, issues *issue.Service, boards *board.Service, sprints *sprint.Service, autos *automation.Service, cfs *customfield.Service, versions *version.Service) *Checker {
	return &Checker{users: users, projects: projects, issues: issues, boards: boards, sprints: sprints, autos: autos, cfs: cfs, versions: versions}
}

// RequireProject verifica che userID abbia permKey sul progetto projectID
// (o sia admin globale, che bypassa qualunque controllo di ruolo).
func (c *Checker) RequireProject(userID, projectID, permKey string) error {
	if c.isGlobalAdmin(userID) {
		return nil
	}
	// Ruolo effettivo = più permissivo tra individuale (project_members) e quelli
	// ereditati dai team (project_teams ⋈ group_members). ok=false → nessun accesso.
	role, ok := c.projects.EffectiveRole(userID, projectID)
	if !ok {
		return ErrForbidden
	}
	if permission.ForRole(string(role), false)[permKey] {
		return nil
	}
	return ErrForbidden
}

// RequireGlobalAdmin verifica che userID abbia il flag is_admin globale.
func (c *Checker) RequireGlobalAdmin(userID string) error {
	if c.isGlobalAdmin(userID) {
		return nil
	}
	return ErrForbidden
}

func (c *Checker) isGlobalAdmin(userID string) bool {
	u, err := c.users.GetByID(userID)
	return err == nil && u.IsAdmin
}

// IsGlobalAdmin espone isGlobalAdmin agli handler che devono applicare un
// bypass admin-globale su controlli di ownership eseguiti in-handler (es.
// filtri/dashboard: solo il proprietario, o un admin globale, può mutarli).
func (c *Checker) IsGlobalAdmin(userID string) bool {
	return c.isGlobalAdmin(userID)
}
