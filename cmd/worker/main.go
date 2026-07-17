package main

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/it4nodummies/heureum/internal/config"
	applog "github.com/it4nodummies/heureum/internal/log"
	"github.com/it4nodummies/heureum/internal/domain/webhook"
	"github.com/it4nodummies/heureum/internal/mailer"
	"github.com/it4nodummies/heureum/internal/store"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	logger := applog.New(cfg.Env)
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		log.Fatal(err)
	}
	defer s.Close()

	logger.Info("starting worker", "env", cfg.Env)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	mail := mailer.NewFromConfig(*cfg)
	// Client condiviso per la consegna dei webhook (timeout per non bloccare il tick).
	whClient := &http.Client{Timeout: 10 * time.Second}

	for {
		select {
		case <-ticker.C:
			processNotificationQueue(logger, s.DB, mail)
			processWebhookDeliveries(logger, s.DB, whClient)
		case <-quit:
			logger.Info("worker shutting down gracefully")
			return
		}
	}
}

func processNotificationQueue(logger *slog.Logger, db *gorm.DB, mail mailer.Mailer) {
	var pending []struct {
		ID     string
		UserID string
		Title  string
		Body   string
	}
	db.Raw(`
		SELECT n.id, n.user_id, n.title, n.body
		FROM notifications n
		JOIN notification_settings ns ON ns.user_id = n.user_id AND ns.event_type = n.type
		WHERE ns.via_email = true AND n.is_read = false AND n.email_sent = false
		LIMIT 50
	`).Scan(&pending)

	for _, p := range pending {
		var userEmail string
		db.Table("users").Where("id = ?", p.UserID).Pluck("email", &userEmail)
		if userEmail == "" {
			continue
		}

		if err := mail.Send(userEmail, p.Title, p.Body); err != nil {
			// Mailer disabled: nothing was sent, so leave email_sent=false
			// (no error log) — the backlog flushes once SMTP is configured.
			if errors.Is(err, mailer.ErrMailerDisabled) {
				continue
			}
			logger.Error("failed to send email notification", "to", userEmail, "title", p.Title, "error", err)
			continue
		}
		db.Exec("UPDATE notifications SET email_sent = true WHERE id = ?", p.ID)
	}
}

// webhookMaxAttempts è il numero massimo di tentativi prima di marcare 'dead'.
const webhookMaxAttempts = 5

// webhookBackoffBase è il ritardo base del backoff esponenziale.
const webhookBackoffBase = 30 * time.Second

// webhookBackoffCap è il tetto del ritardo tra i tentativi.
const webhookBackoffCap = time.Hour

// nextDelivery calcola lo stato di retry dopo un tentativo. Puro (nessun I/O),
// così le transizioni sono testabili senza HTTP né DB.
//   - success            → ("success", attempts+1, nil)
//   - fallimento < max    → ("failed",  attempts+1, now + base<<(newAttempts-1) capped)
//   - fallimento @ max    → ("dead",    attempts+1, nil)
func nextDelivery(attempts int, success bool, now time.Time) (status string, newAttempts int, next *time.Time) {
	newAttempts = attempts + 1
	if success {
		return "success", newAttempts, nil
	}
	if newAttempts >= webhookMaxAttempts {
		return "dead", newAttempts, nil
	}
	backoff := webhookBackoffBase << (newAttempts - 1)
	if backoff > webhookBackoffCap {
		backoff = webhookBackoffCap
	}
	n := now.Add(backoff)
	return "failed", newAttempts, &n
}

// processWebhookDeliveries consegna le consegne retryable dalla coda persistente
// con backoff esponenziale. Sostituisce il vecchio stub log-only e la consegna
// fire-and-forget della dispatcher: lo stato vive nel DB e sopravvive a un crash.
func processWebhookDeliveries(logger *slog.Logger, db *gorm.DB, client *http.Client) {
	svc := webhook.NewService(db)
	deliveries, err := svc.ListRetryable(time.Now(), 50)
	if err != nil {
		logger.Error("list retryable webhook deliveries", "error", err)
		return
	}
	for _, d := range deliveries {
		hook, err := svc.GetWebhook(d.WebhookID)
		if err != nil {
			logger.Error("webhook lookup for delivery", "delivery_id", d.ID, "webhook_id", d.WebhookID, "error", err)
			continue
		}
		res := webhook.Deliver(client, *hook, d.EventType, []byte(d.Payload))
		status, attempts, next := nextDelivery(d.Attempts, res.Success, time.Now())
		if err := svc.MarkDeliveryResult(d.ID, res.StatusCode, res.Success, res.Error, status, attempts, next); err != nil {
			logger.Error("mark webhook delivery result", "delivery_id", d.ID, "error", err)
		}
	}
}
