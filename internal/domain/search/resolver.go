package search

import (
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
	"gorm.io/gorm"
)

// DBResolver implementa jql.Resolver risolvendo i nomi Jira su DB.
type DBResolver struct {
	db            *gorm.DB
	currentUserID string
}

func NewDBResolver(db *gorm.DB, currentUserID string) *DBResolver {
	return &DBResolver{db: db, currentUserID: currentUserID}
}

func (r *DBResolver) ProjectID(keyOrID string) (string, bool) {
	var p project.Project
	if err := r.db.Where("key = ? OR id = ?", keyOrID, keyOrID).First(&p).Error; err != nil {
		return "", false
	}
	return p.ID, true
}

func (r *DBResolver) UserID(login string) (string, bool) {
	var u user.User
	if err := r.db.Where("username = ? OR email = ? OR id = ?", login, login, login).First(&u).Error; err != nil {
		return "", false
	}
	return u.ID, true
}

func (r *DBResolver) CurrentUserID() string { return r.currentUserID }
