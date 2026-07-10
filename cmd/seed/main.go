// seed popola il database con dati demo: utenti, un progetto Scrum e issue di esempio.
// Uso: APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
	"github.com/open-jira/open-jira/internal/domain/workflow"
	"github.com/open-jira/open-jira/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer s.Close()
	if err := store.RunMigrations(cfg.DB); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	authSvc := auth.NewService(s.DB, cfg.Secret)

	demoUsers := []struct{ email, username, name, password string }{
		{"admin@example.com", "admin", "Ada Admin", "admin-demo-123"},
		{"dev@example.com", "dev", "Devi Developer", "dev-demo-123"},
		{"pm@example.com", "pm", "Paolo PM", "pm-demo-123"},
	}
	for _, du := range demoUsers {
		var existing user.User
		err := s.DB.Where("email = ?", du.email).First(&existing).Error
		if err == nil {
			continue // già presente: seed idempotente
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Fatalf("check user %s: %v", du.email, err)
		}
		if _, err := authSvc.Register(du.email, du.username, du.name, du.password); err != nil {
			log.Fatalf("register %s: %v", du.email, err)
		}
		fmt.Printf("created user %s (password: %s)\n", du.email, du.password)
	}

	var admin user.User
	if err := s.DB.Where("email = ?", "admin@example.com").First(&admin).Error; err != nil {
		log.Fatalf("load admin user: %v", err)
	}

	projSvc := project.NewService(s.DB, &admin)
	var demo *project.Project
	var existingProject project.Project
	switch err := s.DB.Where("key = ?", "DEMO").First(&existingProject).Error; {
	case err == nil:
		demo = &existingProject
	case errors.Is(err, gorm.ErrRecordNotFound):
		demo, err = projSvc.Create("Demo Project", "DEMO", "Progetto demo con dati di esempio", project.Type("scrum"))
		if err != nil {
			log.Fatalf("create project: %v", err)
		}
		fmt.Println("created project DEMO")
	default:
		log.Fatalf("check project DEMO: %v", err)
	}

	var cat project.ProjectCategory
	if err := s.DB.Where("name = ?", "Demo Apps").First(&cat).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		cat = project.ProjectCategory{ID: uuid.NewString(), Name: "Demo Apps", Description: "Progetti demo"}
		if err := s.DB.Create(&cat).Error; err != nil {
			log.Fatalf("create category: %v", err)
		}
		fmt.Println("created category Demo Apps")
	} else if err != nil {
		log.Fatalf("check category: %v", err)
	}
	if err := s.DB.Model(&project.Project{}).Where("key = ?", "DEMO").Updates(map[string]any{
		"category_id": cat.ID, "lead_user_id": admin.ID, "assignee_type": "PROJECT_LEAD",
	}).Error; err != nil {
		log.Fatalf("update DEMO project: %v", err)
	}

	issueSvc := issue.NewService(s.DB)
	var count int64
	if err := s.DB.Model(&issue.Issue{}).Where("project_id = ?", demo.ID).Count(&count).Error; err != nil {
		log.Fatalf("count issues: %v", err)
	}
	if count == 0 {
		samples := []struct {
			title, desc string
			prio        issue.Priority
		}{
			{"Set up project skeleton", "Bootstrap iniziale del progetto.", issue.PriorityHigh},
			{"Design login page", "Login con email e password.", issue.PriorityMedium},
			{"Fix flaky board test", "Il test della board fallisce a intermittenza.", issue.PriorityHighest},
			{"Write onboarding docs", "Guida per i nuovi contributor.", issue.PriorityLow},
			{"Implement dark mode", "Tema scuro per la UI.", issue.PriorityMedium},
		}
		for _, it := range samples {
			if _, err := issueSvc.Create(demo.Key, demo.ID, it.title, it.desc, it.prio, nil, nil); err != nil {
				log.Fatalf("create issue %q: %v", it.title, err)
			}
		}
		fmt.Printf("created %d issues in DEMO\n", len(samples))
	}

	var taskType issue.IssueType
	if err := s.DB.Where("project_id = ? AND name = ?", demo.ID, "Task").First(&taskType).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		taskType = issue.IssueType{ID: uuid.NewString(), ProjectID: demo.ID, Name: "Task", Icon: "task", Color: "#4BADE8"}
		if err := s.DB.Create(&taskType).Error; err != nil {
			log.Fatalf("create issue type: %v", err)
		}
		fmt.Println("created issue type Task")
	} else if err != nil {
		log.Fatalf("check issue type: %v", err)
	}
	if err := s.DB.Model(&issue.Issue{}).Where("project_id = ? AND (type_id IS NULL OR type_id = '')", demo.ID).Update("type_id", taskType.ID).Error; err != nil {
		log.Fatalf("assign issue type: %v", err)
	}
	var todo workflow.WorkflowStatus
	if err := s.DB.Where("name = ?", "TO DO").First(&todo).Error; err == nil {
		if uerr := s.DB.Model(&issue.Issue{}).Where("project_id = ? AND (status_id IS NULL OR status_id = '')", demo.ID).Update("status_id", todo.ID).Error; uerr != nil {
			log.Fatalf("assign status: %v", uerr)
		}
	}

	fmt.Println("seed complete")
}
