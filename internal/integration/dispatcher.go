// Package integration collega gli eventi di dominio (issue) alle integrazioni:
// consegna dei webhook in uscita e attivazione delle regole di automation.
package integration

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/webhook"
)

// AutomationRunner è l'astrazione minima dell'automation service (per non
// accoppiare rigidamente e per testare con nil).
type AutomationRunner interface {
	ProcessRules(triggerType, issueID string)
}

// Dispatcher implementa issue.EventSink.
type Dispatcher struct {
	webhookSvc *webhook.Service
	auto       AutomationRunner
	client     *http.Client
}

func NewDispatcher(webhookSvc *webhook.Service, auto AutomationRunner, client *http.Client) *Dispatcher {
	return &Dispatcher{webhookSvc: webhookSvc, auto: auto, client: client}
}

// IssueEvent processa le regole di automation (sincrono: solo DB, veloce) e
// accoda una consegna webhook persistente per ogni hook sottoscritto. Nessun
// HTTP nel percorso della request: il worker consegna dalla coda con retry, così
// la consegna sopravvive a un crash (lo stato è nel DB, non in una goroutine).
func (d *Dispatcher) IssueEvent(eventType string, iss *issue.Issue) {
	// automation (sincrono)
	if d.auto != nil {
		d.auto.ProcessRules(eventType, iss.ID)
	}
	// webhook: enqueue persistente
	hooks, err := d.webhookSvc.ListActiveForEvent(iss.ProjectID, eventType)
	if err != nil || len(hooks) == 0 {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"event": eventType,
		"issue": map[string]any{"id": iss.ID, "key": iss.Key, "summary": iss.Title, "projectId": iss.ProjectID},
	})
	for _, h := range hooks {
		_ = d.webhookSvc.EnqueueDelivery(h.ID, eventType, h.URL, string(payload))
	}
}
