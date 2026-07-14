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
}

// New costruisce un Checker con tutti i servizi necessari ai resolver di
// autorizzazione (issues/boards/sprints/autos/cfs sono usati solo dai
// resolver, non dai metodi RequireProject/RequireGlobalAdmin in questo file).
func New(users *user.Service, projects *project.Service, issues *issue.Service, boards *board.Service, sprints *sprint.Service, autos *automation.Service, cfs *customfield.Service) *Checker {
	return &Checker{users: users, projects: projects, issues: issues, boards: boards, sprints: sprints, autos: autos, cfs: cfs}
}

// RequireProject verifica che userID abbia permKey sul progetto projectID
// (o sia admin globale, che bypassa qualunque controllo di ruolo).
func (c *Checker) RequireProject(userID, projectID, permKey string) error {
	if c.isGlobalAdmin(userID) {
		return nil
	}
	role, err := c.projects.GetRole(projectID, userID)
	if err != nil {
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
