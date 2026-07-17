package v3

import (
	"time"

	"github.com/it4nodummies/heureum/internal/domain/version"
)

// JiraVersion è la rappresentazione v3 di una project version (schema Version).
// I campi data sono date-only (YYYY-MM-DD), NON il formato RFC3339 di JiraTime.
// projectId è il seq_id numerico del progetto (int64), non l'UUID interno.
type JiraVersion struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Released    bool   `json:"released"`
	Archived    bool   `json:"archived"`
	Overdue     bool   `json:"overdue"`
	StartDate   string `json:"startDate,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	ProjectID   int64  `json:"projectId"`
}

// VersionRef è la forma ridotta usata quando una version è referenziata da un
// altro oggetto (es. IssueFields.fixVersions).
type VersionRef struct {
	Self string `json:"self"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

const versionDateLayout = "2006-01-02"

// VersionFrom mappa una version di dominio nel wire-model v3. projectSeqID è il
// seq_id del progetto proprietario; baseURL è l'origine per il link self.
func VersionFrom(v version.Version, projectSeqID int64, baseURL string) JiraVersion {
	jv := JiraVersion{
		Self:        baseURL + "/rest/api/3/version/" + v.ID,
		ID:          v.ID,
		Name:        v.Name,
		Description: v.Description,
		Released:    v.Released,
		Archived:    v.Archived,
		ProjectID:   projectSeqID,
	}
	if v.StartDate != nil {
		jv.StartDate = v.StartDate.Format(versionDateLayout)
	}
	if v.ReleaseDate != nil {
		jv.ReleaseDate = v.ReleaseDate.Format(versionDateLayout)
		// overdue: release date passata e version non ancora rilasciata.
		if !v.Released && v.ReleaseDate.Before(time.Now()) {
			jv.Overdue = true
		}
	}
	return jv
}

// VersionRefFrom mappa una version di dominio nella sua forma ridotta.
func VersionRefFrom(v version.Version, baseURL string) VersionRef {
	return VersionRef{
		Self: baseURL + "/rest/api/3/version/" + v.ID,
		ID:   v.ID,
		Name: v.Name,
	}
}
