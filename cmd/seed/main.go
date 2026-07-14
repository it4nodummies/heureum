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
	"github.com/open-jira/open-jira/internal/domain/board"
	"github.com/open-jira/open-jira/internal/domain/dashboard"
	"github.com/open-jira/open-jira/internal/domain/group"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/notification"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/search"
	"github.com/open-jira/open-jira/internal/domain/sprint"
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
	// Workflow di default per DEMO (idempotente). project.Service.Create non
	// crea un workflow (lo fa solo l'handler HTTP di creazione progetto), quindi
	// il seed deve garantirlo esplicitamente: senza workflow/status la board
	// agile non ha colonne e le issue restano senza status.
	wfSvc := workflow.NewService(s.DB)
	wf, err := wfSvc.GetWorkflow(demo.ID)
	if err != nil {
		wf, err = wfSvc.CreateDefaultWorkflow(demo.ID)
		if err != nil {
			log.Fatalf("seed default workflow: %v", err)
		}
		fmt.Println("created default workflow for DEMO")
	}
	var todo *workflow.WorkflowStatus
	for i := range wf.Statuses {
		if wf.Statuses[i].Name == "TO DO" {
			todo = &wf.Statuses[i]
			break
		}
	}
	if todo != nil {
		if uerr := s.DB.Model(&issue.Issue{}).Where("project_id = ? AND (status_id IS NULL OR status_id = '')", demo.ID).Update("status_id", todo.ID).Error; uerr != nil {
			log.Fatalf("assign status: %v", uerr)
		}
	}

	// Resolution demo "Done" (idempotente): senza questa riga il post-function
	// set_resolution sulla transizione "Done" del workflow di default non-oppa
	// silenziosamente, perché ResolutionIDByName("Done") non trova nulla.
	var doneResCount int64
	if err := s.DB.Table("resolutions").Where("LOWER(name) = LOWER(?)", "Done").Count(&doneResCount).Error; err != nil {
		log.Fatalf("check demo resolution: %v", err)
	}
	if doneResCount == 0 {
		if err := s.DB.Table("resolutions").Create(map[string]any{
			"id":          uuid.NewString(),
			"name":        "Done",
			"description": "Work is complete",
		}).Error; err != nil {
			log.Fatalf("seed demo resolution: %v", err)
		}
		fmt.Println("created demo resolution")
	}

	var demo1 issue.Issue
	switch err := s.DB.Where("key = ?", "DEMO-1").First(&demo1).Error; {
	case err == nil:
		commentSvc := issue.NewCommentService(s.DB)
		existing, err := commentSvc.GetComments(demo1.ID)
		if err != nil {
			log.Fatalf("get comments for DEMO-1: %v", err)
		}
		if len(existing) == 0 {
			comments := []string{
				`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Grazie, ci sto lavorando."}]}]}`,
				`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Aggiornato lo stato."}]}]}`,
			}
			for _, adfDoc := range comments {
				if _, err := commentSvc.AddComment(demo1.ID, admin.ID, adfDoc); err != nil {
					log.Fatalf("add comment to DEMO-1: %v", err)
				}
			}
			fmt.Printf("created %d comments on DEMO-1\n", len(comments))
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		// DEMO-1 non esiste: skip senza errori.
	default:
		log.Fatalf("check issue DEMO-1: %v", err)
	}

	// Filtro salvato demo (idempotente)
	filterSvc := search.NewFilterService(s.DB)
	var existingF search.SavedFilter
	err = s.DB.Where("owner_id = ? AND name = ?", admin.ID, "My open issues").First(&existingF).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if _, err := filterSvc.Create(admin.ID, nil, "My open issues", "Issue assegnate a me", "assignee = currentUser() ORDER BY updated DESC", false); err != nil {
			log.Fatalf("seed filter: %v", err)
		}
		fmt.Println("created demo filter")
	} else if err != nil {
		log.Fatalf("check demo filter: %v", err)
	}

	// Board demo (idempotente)
	boardSvc := board.NewService(s.DB)
	var existingB board.Board
	err = s.DB.Where("project_id = ? AND name = ?", demo.ID, "DEMO board").First(&existingB).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if _, err := boardSvc.Create(demo.ID, "DEMO board", "scrum", nil); err != nil {
			log.Fatalf("seed board: %v", err)
		}
		fmt.Println("created demo board")
	} else if err != nil {
		log.Fatalf("check demo board: %v", err)
	}

	// Sprint demo (idempotente), legato alla board demo (seq_id 1)
	sprintSvc := sprint.NewService(s.DB)
	var existingS sprint.Sprint
	err = s.DB.Where("project_id = ? AND name = ?", demo.ID, "Sprint 1").First(&existingS).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		boardSeq := int64(1)
		if _, err := sprintSvc.CreateFull(demo.ID, "Sprint 1", "Primo sprint demo", &boardSeq, nil, nil); err != nil {
			log.Fatalf("seed sprint: %v", err)
		}
		fmt.Println("created demo sprint")
	} else if err != nil {
		log.Fatalf("check demo sprint: %v", err)
	}

	// Dashboard demo (idempotente), con widget "assigned_to_me"
	dashSvc := dashboard.NewService(s.DB)
	var existingD dashboard.Dashboard
	err = s.DB.Where("owner_id = ? AND name = ?", admin.ID, "My Dashboard").First(&existingD).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		d, cerr := dashSvc.CreateDashboard(admin.ID, "My Dashboard")
		if cerr != nil {
			log.Fatalf("seed dashboard: %v", cerr)
		}
		if _, werr := dashSvc.AddWidget(d.ID, "assigned_to_me", "{}"); werr != nil {
			log.Fatalf("seed widget: %v", werr)
		}
		fmt.Println("created demo dashboard")
	} else if err != nil {
		log.Fatalf("check demo dashboard: %v", err)
	}

	// Gruppo demo "developers" (idempotente), con l'admin come membro
	grpSvc := group.NewService(s.DB)
	var existingG group.Group
	if err := s.DB.Where("name = ?", "developers").First(&existingG).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		g, cerr := grpSvc.Create("developers")
		if cerr != nil {
			log.Fatalf("seed group: %v", cerr)
		}
		if aerr := grpSvc.AddUser(g.ID, admin.ID); aerr != nil {
			log.Fatalf("seed group member: %v", aerr)
		}
		fmt.Println("created demo group")
	} else if err != nil {
		log.Fatalf("check demo group: %v", err)
	}

	// Notifica demo per l'admin (idempotente), così la campanella mostra qualcosa
	notifSvc := notification.NewService(s.DB)
	var existingN int64
	if err := s.DB.Model(&notification.Notification{}).Where("user_id = ? AND title = ?", admin.ID, "Welcome to Open Jira").Count(&existingN).Error; err != nil {
		log.Fatalf("check demo notification: %v", err)
	}
	if existingN == 0 {
		if err := notifSvc.Create(admin.ID, "welcome", "Welcome to Open Jira", "Your demo workspace is ready", "/jira/projects"); err != nil {
			log.Fatalf("seed notification: %v", err)
		}
		fmt.Println("created demo notification")
	}

	fmt.Println("seed complete")
}
