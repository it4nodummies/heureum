package v3

// IssueTransition è lo shape del contratto per una transizione disponibile.
type IssueTransition struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	To            StatusRef `json:"to"`
	HasScreen     bool      `json:"hasScreen"`
	IsGlobal      bool      `json:"isGlobal"`
	IsInitial     bool      `json:"isInitial"`
	IsAvailable   bool      `json:"isAvailable"`
	IsConditional bool      `json:"isConditional"`
	Looped        bool      `json:"looped"`
}

// Transitions è la risposta di GET /issue/{id}/transitions.
type Transitions struct {
	Transitions []IssueTransition `json:"transitions"`
}

// TransitionInput porta i dati per costruire una IssueTransition.
type TransitionInput struct {
	ID, Name     string
	ToID, ToName string
	ToCategory   string // categoria interna: todo/inprogress/done
	Available    bool
	BaseURL      string
}

// MakeTransition costruisce la IssueTransition conforme (to via JiraStatus).
func MakeTransition(in TransitionInput) IssueTransition {
	return IssueTransition{
		ID:          in.ID,
		Name:        in.Name,
		To:          JiraStatus(in.ToID, in.ToName, in.ToCategory, in.BaseURL),
		IsAvailable: in.Available,
	}
}
