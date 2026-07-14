package v3

import (
	"fmt"
	"time"
)

// --- Board ---

type BoardLocation struct {
	ProjectID      int64  `json:"projectId"`
	ProjectKey     string `json:"projectKey"`
	ProjectName    string `json:"projectName"`
	ProjectTypeKey string `json:"projectTypeKey"`
	DisplayName    string `json:"displayName"`
	Name           string `json:"name"`
}

type Board struct {
	ID       int64          `json:"id"`
	Self     string         `json:"self"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Location *BoardLocation `json:"location,omitempty"`
}

type BoardInput struct {
	SeqID          int64
	Name           string
	Type           string
	ProjectID      int64
	ProjectKey     string
	ProjectName    string
	ProjectTypeKey string
	BaseURL        string
}

func AgileBoard(in BoardInput) Board {
	return Board{
		ID:   in.SeqID,
		Self: fmt.Sprintf("%s/rest/agile/1.0/board/%d", in.BaseURL, in.SeqID),
		Name: in.Name,
		Type: in.Type,
		Location: &BoardLocation{
			ProjectID:      in.ProjectID,
			ProjectKey:     in.ProjectKey,
			ProjectName:    in.ProjectName,
			ProjectTypeKey: in.ProjectTypeKey,
			DisplayName:    fmt.Sprintf("%s (%s)", in.ProjectName, in.ProjectKey),
			Name:           in.ProjectName,
		},
	}
}

// --- BoardConfiguration ---

type BoardColumnStatus struct {
	ID string `json:"id"`
}

type BoardColumnConfig struct {
	Name     string              `json:"name"`
	Statuses []BoardColumnStatus `json:"statuses"`
}

type BoardConfig struct {
	ID           int64  `json:"id"`
	Self         string `json:"self"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	ColumnConfig struct {
		Columns        []BoardColumnConfig `json:"columns"`
		ConstraintType string              `json:"constraintType"`
	} `json:"columnConfig"`
}

// --- Sprint ---

type Sprint struct {
	ID            int64  `json:"id"`
	Self          string `json:"self"`
	State         string `json:"state"`
	Name          string `json:"name"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
	CompleteDate  string `json:"completeDate,omitempty"`
	OriginBoardID int64  `json:"originBoardId,omitempty"`
	Goal          string `json:"goal,omitempty"`
}

type SprintInput struct {
	SeqID         int64
	Name          string
	State         string
	Goal          string
	OriginBoardID *int64
	StartDate     *time.Time
	EndDate       *time.Time
	CompleteDate  *time.Time
	BaseURL       string
}

func AgileSprint(in SprintInput) Sprint {
	sp := Sprint{
		ID:    in.SeqID,
		Self:  fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", in.BaseURL, in.SeqID),
		State: in.State,
		Name:  in.Name,
		Goal:  in.Goal,
	}
	if in.OriginBoardID != nil {
		sp.OriginBoardID = *in.OriginBoardID
	}
	if in.StartDate != nil {
		sp.StartDate = JiraTime(*in.StartDate)
	}
	if in.EndDate != nil {
		sp.EndDate = JiraTime(*in.EndDate)
	}
	if in.CompleteDate != nil {
		sp.CompleteDate = JiraTime(*in.CompleteDate)
	}
	return sp
}
