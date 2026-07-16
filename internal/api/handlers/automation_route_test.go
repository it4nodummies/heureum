package handlers_test

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/api/handlers"
	"github.com/it4nodummies/heureum/internal/domain/automation"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

// Compile-time guard: the automation handler must take a project service so it
// can resolve {key} -> UUID (the domain service keys on the project UUID).
func TestAutomationHandlerTakesProjectSvc(t *testing.T) {
	var as *automation.Service
	var ps *project.Service
	_ = handlers.NewAutomationHandler(as, ps) // must accept project svc
}
