package webhook

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Events deserializza EventsJSON in slice (vuoto se non valido).
func (w *Webhook) Events() []string {
	var out []string
	if w.EventsJSON == "" {
		return out
	}
	_ = json.Unmarshal([]byte(w.EventsJSON), &out)
	return out
}

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Create(projectID, url, secret string, events []string) (*Webhook, error) {
	ev, _ := json.Marshal(events)
	h := &Webhook{ID: uuid.NewString(), ProjectID: projectID, URL: url, Secret: secret, EventsJSON: string(ev), IsActive: true}
	if err := s.db.Create(h).Error; err != nil {
		return nil, err
	}
	return h, nil
}

func (s *Service) ListByProject(projectID string) ([]Webhook, error) {
	var hooks []Webhook
	if err := s.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&hooks).Error; err != nil {
		return nil, err
	}
	return hooks, nil
}

func (s *Service) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&Webhook{}).Error
}

// ListActiveForEvent restituisce i webhook attivi del progetto sottoscritti a eventType.
func (s *Service) ListActiveForEvent(projectID, eventType string) ([]Webhook, error) {
	var hooks []Webhook
	if err := s.db.Where("project_id = ? AND is_active = ?", projectID, true).Find(&hooks).Error; err != nil {
		return nil, err
	}
	out := make([]Webhook, 0, len(hooks))
	for _, h := range hooks {
		for _, e := range h.Events() {
			if e == eventType {
				out = append(out, h)
				break
			}
		}
	}
	return out, nil
}

func (s *Service) RecordDelivery(webhookID, eventType, url string, statusCode int, success bool, errMsg string) error {
	d := &Delivery{ID: uuid.NewString(), WebhookID: webhookID, EventType: eventType, URL: url, StatusCode: statusCode, Success: success, Error: errMsg}
	return s.db.Create(d).Error
}

// GetWebhook recupera un webhook per id (per la riconsegna: serve URL + secret).
func (s *Service) GetWebhook(id string) (*Webhook, error) {
	var h Webhook
	if err := s.db.Where("id = ?", id).First(&h).Error; err != nil {
		return nil, err
	}
	return &h, nil
}

// EnqueueDelivery accoda una consegna persistente in stato 'pending', dovuta
// subito (next_attempt_at=now), con il payload serializzato. Il worker la
// consegnerà entro un tick — nessun HTTP nel percorso della request.
func (s *Service) EnqueueDelivery(webhookID, eventType, url, payload string) error {
	now := time.Now()
	d := &Delivery{
		ID:            uuid.NewString(),
		WebhookID:     webhookID,
		EventType:     eventType,
		URL:           url,
		Status:        "pending",
		Attempts:      0,
		NextAttemptAt: &now,
		Payload:       payload,
	}
	return s.db.Create(d).Error
}

// ListRetryable restituisce le consegne dovute (pending o failed con
// next_attempt_at <= now), dalla più vecchia, limitate a limit.
func (s *Service) ListRetryable(now time.Time, limit int) ([]Delivery, error) {
	var out []Delivery
	err := s.db.
		Where("status IN ? AND next_attempt_at IS NOT NULL AND next_attempt_at <= ?", []string{"pending", "failed"}, now).
		Order("created_at ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

// MarkDeliveryResult persiste l'esito di un tentativo e il nuovo stato di retry.
// nextAttemptAt nil azzera la colonna (per gli stati terminali success/dead).
func (s *Service) MarkDeliveryResult(id string, statusCode int, success bool, errMsg, status string, attempts int, nextAttemptAt *time.Time) error {
	return s.db.Model(&Delivery{}).Where("id = ?", id).Updates(map[string]any{
		"status_code":     statusCode,
		"success":         success,
		"error":           errMsg,
		"status":          status,
		"attempts":        attempts,
		"next_attempt_at": nextAttemptAt,
	}).Error
}
