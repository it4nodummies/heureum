package handlers_test

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/api/handlers"
	"github.com/it4nodummies/heureum/internal/domain/calendar"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/timeline"
)

// Compile-time guard: both handlers must accept a project service so they can
// resolve {key} -> UUID (the domain services key on the internal UUID).
func TestTimelineCalendarHandlersTakeProjectSvc(t *testing.T) {
	var ts *timeline.Service
	var cs *calendar.Service
	var ps *project.Service
	_ = handlers.NewTimelineHandler(ts, ps)
	_ = handlers.NewCalendarHandler(cs, ps)
}
