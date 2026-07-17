package main

import (
	"errors"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/domain/automation"
	applog "github.com/it4nodummies/heureum/internal/log"
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

	autoSvc := automation.NewService(s.DB)
	mail := mailer.NewFromConfig(*cfg)

	for {
		select {
		case <-ticker.C:
			processNotificationQueue(logger, s.DB, mail)
			processWebhookDeliveries(logger, s.DB)
			processAutomationRules(logger, autoSvc)
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

func processWebhookDeliveries(logger *slog.Logger, db *gorm.DB) {
	var webhooks []struct {
		ID        string
		URL       string
		Secret    string
		EventsJSON string
	}
	db.Raw("SELECT id, url, secret, events_json FROM webhooks WHERE is_active = true").Scan(&webhooks)
	for _, wh := range webhooks {
		logger.Info("webhook delivery", "url", wh.URL)
	}
}

func processAutomationRules(logger *slog.Logger, svc *automation.Service) {
	var issues []string
	db := svc.DB()
	db.Table("issues").Where("updated_at > ?", time.Now().Add(-5*time.Minute)).Order("updated_at DESC").Limit(100).Pluck("id", &issues)
	for _, issueID := range issues {
		svc.ProcessRules("issue_updated", issueID)
	}
}
