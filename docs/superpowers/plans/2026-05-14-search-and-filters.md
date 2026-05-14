# Search & Filters Implementation Plan

> **For agentic workers:** TDD not used - implementing directly per spec. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add JQL-like search, search API, saved filters, and search UI.

**Architecture:** Parser tokenizes query → query struct → GORM conditions. Search service uses parser + issue service filters. Saved filters CRUD via REST.

**Tech Stack:** Go 1.26, GORM, React/TypeScript/Tailwind

---

### Task 1: JQL Parser

**Files:**
- Create: `internal/domain/search/parser.go`
- Create: `internal/domain/search/parser_test.go`

- [ ] **Step 1: Create parser with tokenization and Query struct**

```go
package search

import (
    "strings"
    "gorm.io/gorm"
)

type Query struct {
    ProjectKey string
    TypeName   string
    Status     string
    Assignee   string
    Priority   string
    Sprint     string
    Label      string
    Text       string
}

func Parse(query string) *Query {
    q := &Query{}
    parts := strings.Fields(query)
    for _, part := range parts {
        if strings.HasPrefix(part, "project=") { q.ProjectKey = strings.TrimPrefix(part, "project=") }
        if strings.HasPrefix(part, "status=") { q.Status = strings.TrimPrefix(part, "status=") }
        if strings.HasPrefix(part, "assignee=") { q.Assignee = strings.TrimPrefix(part, "assignee=") }
        if strings.HasPrefix(part, "priority=") { q.Priority = strings.TrimPrefix(part, "priority=") }
        if strings.HasPrefix(part, "type=") { q.TypeName = strings.TrimPrefix(part, "type=") }
    }
    if q.Text == "" && len(parts) > 0 && !strings.Contains(query, "=") {
        q.Text = query
    }
    return q
}

func (q *Query) Apply(db *gorm.DB) *gorm.DB {
    if q.ProjectKey != "" {
        db = db.Where("project_id IN (SELECT id FROM projects WHERE key = ?)", q.ProjectKey)
    }
    if q.Text != "" {
        db = db.Where("title LIKE ? OR description_json LIKE ?", "%"+q.Text+"%", "%"+q.Text+"%")
    }
    if q.Priority != "" {
        db = db.Where("priority = ?", q.Priority)
    }
    if q.Assignee != "" {
        db = db.Where("assignee_id IN (SELECT id FROM users WHERE username = ?)", q.Assignee)
    }
    if q.Status != "" {
        db = db.Where("status_id IN (SELECT id FROM workflow_statuses WHERE name = ?)", q.Status)
    }
    if q.TypeName != "" {
        db = db.Where("type_id IN (SELECT id FROM issue_types WHERE name = ?)", q.TypeName)
    }
    return db
}
```

- [ ] **Step 2: Run go test and commit**

### Task 2: Search Service

**Files:**
- Create: `internal/domain/search/service.go`

The service wraps the parser and issue service, applies the query to GORM, returns results.

- [ ] **Step 1: Create search service**
- [ ] **Step 2: Run go test and commit**

### Task 3: Search Handler

**Files:**
- Create: `internal/api/handlers/search_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create search handler with GET /api/v1/search?q=...**
- [ ] **Step 2: Register route in router.go**
- [ ] **Step 3: Run go test and commit**

### Task 4: Saved Filters

**Files:**
- Create: `internal/domain/search/saved_filter.go`
- Modify: `internal/api/handlers/search_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create saved filter model and CRUD service**
- [ ] **Step 2: Add filter handler endpoints (list, create, get, delete)**
- [ ] **Step 3: Register routes in router.go**
- [ ] **Step 4: Run go test and commit**

### Task 5: Frontend Search UI

**Files:**
- Create: `frontend/src/pages/Search.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Create Search page with bar, results, filter chips, saved filters dropdown**
- [ ] **Step 2: Wire into App.tsx navigation**
- [ ] **Step 3: Commit**
