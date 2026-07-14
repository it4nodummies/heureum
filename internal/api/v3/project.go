package v3

import (
	"fmt"

	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// ProjectCategory è la rappresentazione Jira v3 di una categoria progetto
// (schema "ProjectCategory" nel contratto).
type ProjectCategory struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Project è la rappresentazione Jira v3 di un progetto (schema "Project" nel
// contratto).
type Project struct {
	Self            string            `json:"self"`
	ID              string            `json:"id"`
	Key             string            `json:"key"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	ProjectTypeKey  string            `json:"projectTypeKey"`
	Simplified      bool              `json:"simplified"`
	Style           string            `json:"style"`
	IsPrivate       bool              `json:"isPrivate"`
	Archived        bool              `json:"archived"`
	AssigneeType    string            `json:"assigneeType,omitempty"`
	URL             string            `json:"url,omitempty"`
	AvatarUrls      map[string]string `json:"avatarUrls"`
	Lead            *User             `json:"lead,omitempty"`
	ProjectCategory *ProjectCategory  `json:"projectCategory,omitempty"`
}

// DefaultProjectAvatarPath è servito da router.go (SVG inline, non
// autenticato): è il fallback quando il progetto non ha un IconURL proprio.
const DefaultProjectAvatarPath = "/static/default-project-avatar.svg"

// JiraProject mappa il modello interno project.Project (più lead e categoria
// opzionali, già risolti dal chiamante) nella forma Jira v3.
func JiraProject(p project.Project, lead *user.User, cat *project.ProjectCategory, baseURL string) Project {
	avatar := p.IconURL
	if avatar == "" {
		avatar = baseURL + DefaultProjectAvatarPath
	}
	jp := Project{
		Self:           fmt.Sprintf("%s/rest/api/3/project/%d", baseURL, p.SeqID),
		ID:             fmt.Sprint(p.SeqID),
		Key:            p.Key,
		Name:           p.Name,
		Description:    p.Description,
		ProjectTypeKey: project.ProjectTypeKeyForType(p.Type),
		Simplified:     p.Simplified,
		Style:          p.Style,
		IsPrivate:      p.IsPrivate,
		Archived:       p.IsArchived,
		AssigneeType:   p.AssigneeType,
		URL:            p.URL,
		AvatarUrls: map[string]string{
			"16x16": avatar, "24x24": avatar, "32x32": avatar, "48x48": avatar,
		},
	}
	if jp.Style == "" {
		jp.Style = "classic"
	}
	if lead != nil {
		lu := JiraUser(*lead, baseURL)
		jp.Lead = &lu
	}
	if cat != nil {
		jp.ProjectCategory = &ProjectCategory{
			Self:        fmt.Sprintf("%s/rest/api/3/projectCategory/%s", baseURL, cat.ID),
			ID:          cat.ID,
			Name:        cat.Name,
			Description: cat.Description,
		}
	}
	return jp
}

// ProjectType è la rappresentazione Jira v3 di un tipo di progetto (schema
// "ProjectType" nel contratto).
type ProjectType struct {
	Key                string `json:"key"`
	FormattedKey       string `json:"formattedKey"`
	DescriptionI18nKey string `json:"descriptionI18nKey"`
	Icon               string `json:"icon"`
	Color              string `json:"color"`
}

// projectTypeLabels mappa la chiave del tipo di progetto (come restituita da
// project.ProjectTypeKeyForType) all'etichetta leggibile usata da Jira.
var projectTypeLabels = map[string]string{
	"software":     "Software",
	"business":     "Business",
	"service_desk": "Service Desk",
}

// JiraProjectType mappa una projectTypeKey nella forma Jira v3 "ProjectType".
//
// Il contratto (components.schemas.ProjectType, additionalProperties:false) non
// prevede un campo "self": la ProjectType non è una risorsa indirizzabile.
// baseURL resta nella firma per coerenza con gli altri mapper v3.
func JiraProjectType(key, baseURL string) ProjectType {
	_ = baseURL
	return ProjectType{
		Key:                key,
		FormattedKey:       projectTypeLabels[key],
		DescriptionI18nKey: "jira.project.type." + key + ".description",
		Color:              "#0052CC",
	}
}
