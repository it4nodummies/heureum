package v3

import (
	"fmt"

	"github.com/open-jira/open-jira/internal/domain/user"
)

// User è la rappresentazione Jira v3 di un utente (schema "User" nel contratto).
type User struct {
	Self         string            `json:"self"`
	AccountID    string            `json:"accountId"`
	AccountType  string            `json:"accountType"`
	EmailAddress string            `json:"emailAddress,omitempty"`
	DisplayName  string            `json:"displayName"`
	Active       bool              `json:"active"`
	TimeZone     string            `json:"timeZone,omitempty"`
	Locale       string            `json:"locale,omitempty"`
	AvatarUrls   map[string]string `json:"avatarUrls"`
}

// DefaultAvatarPath è servito da router.go (SVG inline, non autenticato):
// è il fallback quando l'utente non ha un AvatarURL proprio.
const DefaultAvatarPath = "/static/default-avatar.svg"

// JiraUser mappa il modello interno user.User nella forma Jira v3.
//
// AccountType è fisso a "atlassian". TimeZone/Locale sono popolati da
// u.TimeZone/u.Locale (colonne time_zone/locale, migrazione 000014).
func JiraUser(u user.User, baseURL string) User {
	avatar := u.AvatarURL
	if avatar == "" {
		avatar = baseURL + DefaultAvatarPath
	}
	return User{
		Self:         fmt.Sprintf("%s/rest/api/3/user?accountId=%s", baseURL, u.ID),
		AccountID:    u.ID,
		AccountType:  "atlassian",
		EmailAddress: u.Email,
		DisplayName:  u.DisplayName,
		Active:       u.IsActive,
		TimeZone:     u.TimeZone,
		Locale:       u.Locale,
		AvatarUrls: map[string]string{
			"16x16": avatar, "24x24": avatar, "32x32": avatar, "48x48": avatar,
		},
	}
}
