// Package integration collega gli eventi di dominio (issue) alle integrazioni:
// consegna dei webhook in uscita e attivazione delle regole di automation.
package integration

import (
	"encoding/json"
	"net/http"
	"sync"

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
	wg         sync.WaitGroup // per attendere le consegne nei test se serve
}

func NewDispatcher(webhookSvc *webhook.Service, auto AutomationRunner, client *http.Client) *Dispatcher {
	return &Dispatcher{webhookSvc: webhookSvc, auto: auto, client: client}
}

// IssueEvent consegna l'evento ai webhook sottoscritti (async) e alle regole di
// automation (sincrono: solo DB, veloce).
func (d *Dispatcher) IssueEvent(eventType string, iss *issue.Issue) {
	// automation (sincrono)
	if d.auto != nil {
		d.auto.ProcessRules(eventType, iss.ID)
	}
	// webhook (async fire-and-forget)
	hooks, err := d.webhookSvc.ListActiveForEvent(iss.ProjectID, eventType)
	if err != nil || len(hooks) == 0 {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"event": eventType,
		"issue": map[string]any{"id": iss.ID, "key": iss.Key, "summary": iss.Title, "projectId": iss.ProjectID},
	})
	for _, h := range hooks {
		hook := h
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			del := webhook.Deliver(d.client, hook, eventType, payload)
			_ = d.webhookSvc.RecordDelivery(hook.ID, eventType, hook.URL, del.StatusCode, del.Success, del.Error)
		}()
	}
}

// Wait attende le consegne in volo (usato dai test).
func (d *Dispatcher) Wait() { d.wg.Wait() }
