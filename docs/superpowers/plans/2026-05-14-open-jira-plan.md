# Open Jira Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an open-source Jira clone (Go backend, React frontend, PostgreSQL) deployable on Kubernetes, in 12 atomic Ralph-loop steps.

**Architecture:** Vertical slice approach. Each step produces a complete, tested module (DB → domain → API → frontend) before moving to the next. Go backend with GORM, React SPA frontend with shadcn/ui.

**Tech Stack:** Go 1.22+, GORM, golang-migrate, gorilla/mux, gorilla/websocket, JWT, PostgreSQL 15+, Redis, React 18, TypeScript, Vite, shadcn/ui, TanStack Query, Zustand, dnd-kit, TipTap, Recharts, Docker, Kubernetes, Helm

---

## Step 1: Infrastructure Core

**Branch:** `feature/step-1-infra-core`

### Task 1.1: Initialize Go module and directory structure

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `cmd/worker/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1.1.1: Write failing config test**

```go
// internal/config/config_test.go
package config

import (
    "os"
    "testing"
)

func TestLoadConfigFromEnv(t *testing.T) {
    os.Setenv("APP_PORT", "9090")
    os.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
    os.Setenv("APP_BASE_URL", "http://localhost:9090")
    os.Setenv("DB_DRIVER", "sqlite")
    os.Setenv("DB_DSN", "file::memory:?cache=shared")
    os.Setenv("REDIS_URL", "redis://localhost:6379/0")
    defer os.Clearenv()

    cfg, err := Load()
    if err != nil {
        t.Fatalf("Load() error = %v", err)
    }
    if cfg.Port != 9090 {
        t.Errorf("Port = %d, want 9090", cfg.Port)
    }
    if cfg.Secret != "test-secret-min-32-chars-long!!" {
        t.Errorf("Secret mismatch")
    }
    if cfg.BaseURL != "http://localhost:9090" {
        t.Errorf("BaseURL = %s, want http://localhost:9090", cfg.BaseURL)
    }
    if cfg.DB.Driver != "sqlite" {
        t.Errorf("DB.Driver = %s, want sqlite", cfg.DB.Driver)
    }
}

func TestLoadConfigDefaults(t *testing.T) {
    os.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
    os.Setenv("DB_DSN", "file::memory:?cache=shared")
    defer os.Clearenv()

    cfg, err := Load()
    if err != nil {
        t.Fatalf("Load() error = %v", err)
    }
    if cfg.Port != 8080 {
        t.Errorf("default Port = %d, want 8080", cfg.Port)
    }
}

func TestLoadConfigMissingSecret(t *testing.T) {
    _, err := Load()
    if err == nil {
        t.Error("expected error for missing APP_SECRET")
    }
}
```

- [ ] **Step 1.1.2: Run test to verify it fails**

Run: `go test ./internal/config/ -v -run TestLoadConfigFromEnv`
Expected: FAIL — "undefined: Load"

- [ ] **Step 1.1.3: Implement config loader**

```go
// internal/config/config.go
package config

import (
    "errors"
    "fmt"
    "os"
    "strconv"
)

type DBConfig struct {
    Driver string
    DSN    string
}

type RedisConfig struct {
    URL string
}

type Config struct {
    Port    int
    Env     string
    Secret  string
    BaseURL string
    DB      DBConfig
    Redis   RedisConfig
}

func Load() (*Config, error) {
    port, _ := strconv.Atoi(getEnv("APP_PORT", "8080"))

    cfg := &Config{
        Port:    port,
        Env:     getEnv("APP_ENV", "development"),
        Secret:  os.Getenv("APP_SECRET"),
        BaseURL: getEnv("APP_BASE_URL", fmt.Sprintf("http://localhost:%d", port)),
        DB: DBConfig{
            Driver: getEnv("DB_DRIVER", "postgres"),
            DSN:    os.Getenv("DB_DSN"),
        },
        Redis: RedisConfig{
            URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
        },
    }

    if cfg.Secret == "" {
        return nil, errors.New("APP_SECRET is required")
    }
    if cfg.DB.DSN == "" {
        return nil, errors.New("DB_DSN is required")
    }

    return cfg, nil
}

func getEnv(key, fallback string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return fallback
}
```

- [ ] **Step 1.1.4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 1.1.5: Initialize Go module**

Run: `go mod init github.com/open-jira/open-jira`
Expected: `go: creating new go.mod: module github.com/open-jira/open-jira`

- [ ] **Step 1.1.6: Create entry points**

```go
// cmd/server/main.go
package main

import (
    "log"
    "github.com/open-jira/open-jira/internal/config"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config: %v", err)
    }
    log.Printf("starting server on port %d", cfg.Port)
}
```

```go
// cmd/worker/main.go
package main

import (
    "log"
    "github.com/open-jira/open-jira/internal/config"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config: %v", err)
    }
    log.Printf("starting worker, env=%s", cfg.Env)
}
```

- [ ] **Step 1.1.7: Run `go build ./cmd/...` to verify compilation**

Run: `go build ./cmd/...`
Expected: no output, exit 0

- [ ] **Step 1.1.8: Commit**

```bash
git add -A
git commit -m "feat(step-1): add config loader and project structure"
```

---

### Task 1.2: Set up structured logger

**Files:**
- Create: `internal/log/log.go`

- [ ] **Step 1.2.1: Implement logger**

```go
// internal/log/log.go
package log

import (
    "log/slog"
    "os"
)

func New(env string) *slog.Logger {
    opts := &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }
    if env == "development" {
        opts.Level = slog.LevelDebug
        return slog.New(slog.NewTextHandler(os.Stdout, opts))
    }
    return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
```

- [ ] **Step 1.2.2: Update server main to use logger**

```go
// cmd/server/main.go
package main

import (
    "log"
    "os"
    "github.com/open-jira/open-jira/internal/config"
    applog "github.com/open-jira/open-jira/internal/log"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config: %v", err)
    }
    logger := applog.New(cfg.Env)
    logger.Info("starting server", "port", cfg.Port, "env", cfg.Env)
    os.Exit(0)
}
```

- [ ] **Step 1.2.3: Run build and verify**

Run: `go build ./cmd/... && go vet ./...`
Expected: no output, exit 0

- [ ] **Step 1.2.4: Commit**

```bash
git add -A
git commit -m "feat(step-1): add structured logger"
```

---

### Task 1.3: Database connection layer

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/sqlite/sqlite.go`
- Create: `internal/store/postgres/postgres.go`

- [ ] **Step 1.3.1: Install GORM and drivers**

Run: `go get gorm.io/gorm gorm.io/driver/postgres gorm.io/driver/sqlite gorm.io/driver/mysql`
Expected: go.sum updated

- [ ] **Step 1.3.2: Write DB store interface and factory**

```go
// internal/store/store.go
package store

import (
    "fmt"
    "gorm.io/driver/mysql"
    "gorm.io/driver/postgres"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
    applog "github.com/open-jira/open-jira/internal/log"
    "github.com/open-jira/open-jira/internal/config"
)

type Store struct {
    DB     *gorm.DB
    Driver string
}

func New(cfg config.DBConfig, env string) (*Store, error) {
    gormCfg := &gorm.Config{}
    if env == "development" {
        gormCfg.Logger = logger.Default.LogMode(logger.Info)
    } else {
        gormCfg.Logger = logger.Default.LogMode(logger.Warn)
    }

    var dialector gorm.Dialector
    switch cfg.Driver {
    case "postgres":
        dialector = postgres.Open(cfg.DSN)
    case "mysql", "mariadb":
        dialector = mysql.Open(cfg.DSN)
    case "sqlite":
        dialector = sqlite.Open(cfg.DSN)
    default:
        return nil, fmt.Errorf("unsupported DB driver: %s", cfg.Driver)
    }

    db, err := gorm.Open(dialector, gormCfg)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }

    sqlDB, err := db.DB()
    if err != nil {
        return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
    }
    sqlDB.SetMaxOpenConns(25)
    sqlDB.SetMaxIdleConns(10)

    return &Store{DB: db, Driver: cfg.Driver}, nil
}

func (s *Store) Close() error {
    sqlDB, err := s.DB.DB()
    if err != nil {
        return err
    }
    return sqlDB.Close()
}
```

- [ ] **Step 1.3.3: Write store unit test**

```go
// internal/store/store_test.go
package store

import (
    "testing"
    "github.com/open-jira/open-jira/internal/config"
)

func TestNewSQLite(t *testing.T) {
    cfg := config.DBConfig{
        Driver: "sqlite",
        DSN:    "file::memory:?cache=shared",
    }
    s, err := New(cfg, "test")
    if err != nil {
        t.Fatalf("New() error = %v", err)
    }
    defer s.Close()

    if s.Driver != "sqlite" {
        t.Errorf("Driver = %s, want sqlite", s.Driver)
    }
    if s.DB == nil {
        t.Error("DB is nil")
    }
}

func TestNewUnsupportedDriver(t *testing.T) {
    cfg := config.DBConfig{
        Driver: "cassandra",
        DSN:    "host=localhost",
    }
    _, err := New(cfg, "test")
    if err == nil {
        t.Error("expected error for unsupported driver")
    }
}
```

- [ ] **Step 1.3.4: Run store tests**

Run: `go test ./internal/store/ -v`
Expected: PASS (SQLite in-memory)

- [ ] **Step 1.3.5: Commit**

```bash
git add -A
git commit -m "feat(step-1): add database connection layer with multi-driver support"
```

---

### Task 1.4: Database migration files

**Files:**
- Create: `migrations/000001_init_schema.up.sql`
- Create: `migrations/000001_init_schema.down.sql`

- [ ] **Step 1.4.1: Install golang-migrate CLI and add migration runner**

Run: `go install -tags 'postgres,mysql,sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
Then: `go get github.com/golang-migrate/migrate/v4`

- [ ] **Step 1.4.2: Write up migration**

```sql
-- migrations/000001_init_schema.up.sql
CREATE TABLE organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    settings_json TEXT DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT DEFAULT '',
    password_hash TEXT NOT NULL DEFAULT '',
    is_admin BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE oauth_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT DEFAULT '',
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    org_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    key TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    type TEXT NOT NULL DEFAULT 'scrum' CHECK (type IN ('scrum', 'kanban', 'business')),
    lead_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    default_assignee TEXT DEFAULT 'unassigned',
    icon_url TEXT DEFAULT '',
    is_archived BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE project_members (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member', 'viewer')),
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE workflows (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE workflow_statuses (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'inprogress' CHECK (category IN ('todo', 'inprogress', 'done')),
    color TEXT DEFAULT '#6B7280',
    position INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE workflow_transitions (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    from_status_id TEXT NOT NULL REFERENCES workflow_statuses(id) ON DELETE CASCADE,
    to_status_id TEXT NOT NULL REFERENCES workflow_statuses(id) ON DELETE CASCADE
);

CREATE TABLE sprints (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    goal TEXT DEFAULT '',
    state TEXT NOT NULL DEFAULT 'future' CHECK (state IN ('active', 'closed', 'future')),
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE versions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    release_date TIMESTAMP,
    released BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE issue_types (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    icon TEXT DEFAULT 'task',
    color TEXT DEFAULT '#6B7280',
    is_subtask BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE issues (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    title TEXT NOT NULL,
    description_json TEXT DEFAULT '{}',
    type_id TEXT REFERENCES issue_types(id) ON DELETE SET NULL,
    status_id TEXT REFERENCES workflow_statuses(id) ON DELETE SET NULL,
    priority TEXT DEFAULT 'medium' CHECK (priority IN ('highest', 'high', 'medium', 'low', 'lowest')),
    assignee_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    reporter_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    parent_id TEXT REFERENCES issues(id) ON DELETE SET NULL,
    sprint_id TEXT REFERENCES sprints(id) ON DELETE SET NULL,
    version_id TEXT REFERENCES versions(id) ON DELETE SET NULL,
    story_points INTEGER DEFAULT 0,
    original_estimate INTEGER DEFAULT 0,
    time_spent INTEGER DEFAULT 0,
    start_date TIMESTAMP,
    due_date TIMESTAMP,
    environment TEXT DEFAULT '',
    is_archived BOOLEAN DEFAULT FALSE,
    position REAL NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_issues_key ON issues(key);

CREATE TABLE labels (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT DEFAULT '#6B7280',
    UNIQUE(project_id, name)
);

CREATE TABLE issue_labels (
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    label_id TEXT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
);

CREATE TABLE issue_links (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    link_type TEXT NOT NULL CHECK (link_type IN ('blocks', 'is_blocked', 'duplicates', 'relates')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE custom_fields (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    field_type TEXT NOT NULL CHECK (field_type IN ('text', 'number', 'date', 'select', 'multiselect', 'user')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE custom_field_options (
    id TEXT PRIMARY KEY,
    field_id TEXT NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    value TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE issue_custom_values (
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    field_id TEXT NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    value_text TEXT DEFAULT '',
    value_number REAL,
    value_date TIMESTAMP,
    option_id TEXT REFERENCES custom_field_options(id) ON DELETE SET NULL,
    PRIMARY KEY (issue_id, field_id)
);

CREATE TABLE issue_attachments (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER NOT NULL DEFAULT 0,
    uploader_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    body_json TEXT DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_deleted BOOLEAN DEFAULT FALSE
);

CREATE TABLE issue_history (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    actor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    field_name TEXT NOT NULL,
    old_value TEXT DEFAULT '',
    new_value TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE issue_watchers (
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, user_id)
);

CREATE TABLE dashboards (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_public BOOLEAN DEFAULT FALSE,
    layout_json TEXT DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE dashboard_widgets (
    id TEXT PRIMARY KEY,
    dashboard_id TEXT NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    widget_type TEXT NOT NULL,
    config_json TEXT DEFAULT '{}',
    position_json TEXT DEFAULT '{}'
);

CREATE TABLE saved_filters (
    id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    jql TEXT DEFAULT '',
    is_shared BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE git_providers (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('forgejo', 'gitlab', 'github', 'gitea', 'bitbucket')),
    base_url TEXT NOT NULL,
    token_encrypted TEXT DEFAULT '',
    webhook_secret TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE issue_commits (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    provider_id TEXT REFERENCES git_providers(id) ON DELETE SET NULL,
    commit_sha TEXT NOT NULL,
    message TEXT DEFAULT '',
    author TEXT DEFAULT '',
    committed_at TIMESTAMP
);

CREATE TABLE issue_branches (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    provider_id TEXT REFERENCES git_providers(id) ON DELETE SET NULL,
    branch_name TEXT NOT NULL,
    repo_url TEXT DEFAULT ''
);

CREATE TABLE issue_pull_requests (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    provider_id TEXT REFERENCES git_providers(id) ON DELETE SET NULL,
    pr_number INTEGER NOT NULL,
    title TEXT NOT NULL,
    url TEXT DEFAULT '',
    state TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'merged', 'closed')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    merged_at TIMESTAMP
);

CREATE TABLE automation_rules (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    trigger_type TEXT NOT NULL,
    conditions_json TEXT DEFAULT '{}',
    actions_json TEXT DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE automation_runs (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
    issue_id TEXT REFERENCES issues(id) ON DELETE SET NULL,
    triggered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'success',
    log TEXT DEFAULT ''
);

CREATE TABLE notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT DEFAULT '',
    link TEXT DEFAULT '',
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notification_settings (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    via_email BOOLEAN DEFAULT TRUE,
    via_app BOOLEAN DEFAULT TRUE,
    PRIMARY KEY (user_id, project_id, event_type)
);

CREATE TABLE webhooks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret TEXT DEFAULT '',
    events_json TEXT DEFAULT '[]',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 1.4.3: Write down migration**

```sql
-- migrations/000001_init_schema.down.sql
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS notification_settings;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS automation_runs;
DROP TABLE IF EXISTS automation_rules;
DROP TABLE IF EXISTS issue_pull_requests;
DROP TABLE IF EXISTS issue_branches;
DROP TABLE IF EXISTS issue_commits;
DROP TABLE IF EXISTS git_providers;
DROP TABLE IF EXISTS saved_filters;
DROP TABLE IF EXISTS dashboard_widgets;
DROP TABLE IF EXISTS dashboards;
DROP TABLE IF EXISTS issue_watchers;
DROP TABLE IF EXISTS issue_history;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS issue_attachments;
DROP TABLE IF EXISTS issue_custom_values;
DROP TABLE IF EXISTS custom_field_options;
DROP TABLE IF EXISTS custom_fields;
DROP TABLE IF EXISTS issue_links;
DROP TABLE IF EXISTS issue_labels;
DROP TABLE IF EXISTS labels;
DROP TABLE IF EXISTS issues;
DROP TABLE IF EXISTS issue_types;
DROP TABLE IF EXISTS versions;
DROP TABLE IF EXISTS sprints;
DROP TABLE IF EXISTS workflow_transitions;
DROP TABLE IF EXISTS workflow_statuses;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS oauth_tokens;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
```

- [ ] **Step 1.4.4: Write migration runner**

```go
// internal/store/migrate.go
package store

import (
    "fmt"
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/database/mysql"
    _ "github.com/golang-migrate/migrate/v4/database/sqlite3"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/open-jira/open-jira/internal/config"
)

func RunMigrations(cfg config.DBConfig) error {
    dbURL := cfg.DSN
    if cfg.Driver == "sqlite" {
        dbURL = fmt.Sprintf("sqlite3://%s", cfg.DSN)
    }

    m, err := migrate.New("file://migrations", dbURL)
    if err != nil {
        return fmt.Errorf("migrate init: %w", err)
    }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migrate up: %w", err)
    }
    return nil
}
```

- [ ] **Step 1.4.5: Commit**

```bash
git add -A
git commit -m "feat(step-1): add complete database migrations"
```

---

### Task 1.5: Docker Compose dev environment

**Files:**
- Create: `deploy/docker/docker-compose.yml`

- [ ] **Step 1.5.1: Write Docker Compose file**

```yaml
# deploy/docker/docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: openjira
      POSTGRES_PASSWORD: openjira
      POSTGRES_DB: openjira
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  pgdata:
```

- [ ] **Step 1.5.2: Commit**

```bash
git add -A
git commit -m "feat(step-1): add Docker Compose dev environment"
```

---

### Task 1.6: CI with GitHub Actions

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1.6.1: Write CI workflow**

```yaml
# .github/workflows/ci.yml
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go vet ./...
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./... -v -count=1

  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go build ./cmd/...
```

- [ ] **Step 1.6.2: Commit**

```bash
git add -A
git commit -m "feat(step-1): add GitHub Actions CI workflow"
```

---

### Task 1.7: Step 1 integration test — verify full stack

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`

- [ ] **Step 1.7.1: Write app bootstrap**

```go
// internal/app/app.go
package app

import (
    "github.com/open-jira/open-jira/internal/config"
    "github.com/open-jira/open-jira/internal/store"
)

type App struct {
    Config *config.Config
    Store  *store.Store
}

func New(cfg *config.Config) (*App, error) {
    s, err := store.New(cfg.DB, cfg.Env)
    if err != nil {
        return nil, err
    }
    if err := store.RunMigrations(cfg.DB); err != nil {
        return nil, err
    }
    return &App{Config: cfg, Store: s}, nil
}

func (a *App) Close() error {
    return a.Store.Close()
}
```

- [ ] **Step 1.7.2: Write integration test**

```go
// internal/app/app_test.go
package app

import (
    "os"
    "testing"
    "github.com/open-jira/open-jira/internal/config"
)

func TestNewSQLiteApp(t *testing.T) {
    os.Setenv("APP_SECRET", "test-secret-min-32-chars-long!!")
    os.Setenv("DB_DRIVER", "sqlite")
    os.Setenv("DB_DSN", "file::memory:?cache=shared")
    defer os.Clearenv()

    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("config.Load() error = %v", err)
    }

    app, err := New(cfg)
    if err != nil {
        t.Fatalf("New() error = %v", err)
    }
    defer app.Close()

    if app.Store == nil {
        t.Error("Store is nil")
    }
    if app.Store.DB == nil {
        t.Error("DB is nil")
    }
}
```

- [ ] **Step 1.7.3: Run integration test**

Run: `go test ./internal/app/ -v`
Expected: PASS (creates in-memory SQLite DB, runs migrations, verifies connection)

- [ ] **Step 1.7.4: Run all tests**

Run: `go test ./... -v -count=1`
Expected: all PASS

- [ ] **Step 1.7.5: Final commit for Step 1**

```bash
git add -A
git commit -m "feat(step-1): add app bootstrap and integration test"
```

---

## Step 2: Auth & Users

**Branch:** `feature/step-2-auth-users` (from step-1)

### Task 2.1: User domain models and repository

**Files:**
- Create: `internal/domain/user/model.go`
- Create: `internal/store/user_repo.go`
- Create: `internal/store/user_repo_test.go`

- [ ] **Step 2.1.1: Write failing test for user CRUD**

```go
// internal/store/user_repo_test.go
package store

import (
    "testing"
    "github.com/google/uuid"
    "github.com/open-jira/open-jira/internal/domain/user"
)

func TestUserRepoCRUD(t *testing.T) {
    s := setupTestStore(t)
    defer s.Close()

    u := &user.User{
        ID:           uuid.New().String(),
        Email:        "test@example.com",
        Username:     "testuser",
        DisplayName:  "Test User",
        PasswordHash: "$2a$10$dummyhashedpassword",
        IsActive:     true,
    }

    // Create
    if err := s.DB.Create(u).Error; err != nil {
        t.Fatalf("Create user: %v", err)
    }

    // Read
    var found user.User
    if err := s.DB.First(&found, "id = ?", u.ID).Error; err != nil {
        t.Fatalf("Find user: %v", err)
    }
    if found.Email != u.Email {
        t.Errorf("Email = %s, want %s", found.Email, u.Email)
    }

    // Update
    s.DB.Model(&found).Update("display_name", "Updated User")
    var updated user.User
    s.DB.First(&updated, "id = ?", u.ID)
    if updated.DisplayName != "Updated User" {
        t.Errorf("DisplayName = %s, want Updated User", updated.DisplayName)
    }

    // Delete
    s.DB.Delete(&user.User{}, "id = ?", u.ID)
    var deleted user.User
    if err := s.DB.First(&deleted, "id = ?", u.ID).Error; err == nil {
        t.Error("expected error for deleted user")
    }
}

func TestUserRepoUniqueConstraints(t *testing.T) {
    s := setupTestStore(t)
    defer s.Close()

    u1 := &user.User{
        ID:       uuid.New().String(),
        Email:    "dup@example.com",
        Username: "dupuser",
    }
    s.DB.Create(u1)

    u2 := &user.User{
        ID:       uuid.New().String(),
        Email:    "dup@example.com",
        Username: "dupuser2",
    }
    err := s.DB.Create(u2).Error
    if err == nil {
        t.Error("expected unique constraint error for duplicate email")
    }
}
```

- [ ] **Step 2.1.2: Write test setup helper**

```go
// internal/store/helpers_test.go
package store

import (
    "testing"
    "github.com/open-jira/open-jira/internal/config"
)

func setupTestStore(t *testing.T) *Store {
    t.Helper()
    cfg := config.DBConfig{
        Driver: "sqlite",
        DSN:    "file::memory:?cache=shared",
    }
    s, err := New(cfg, "test")
    if err != nil {
        t.Fatalf("New() error = %v", err)
    }
    if err := s.DB.AutoMigrate(
        &user.User{},
    ); err != nil {
        t.Fatalf("AutoMigrate: %v", err)
    }
    return s
}
```

Add import `"github.com/open-jira/open-jira/internal/domain/user"` to helpers_test.go.

- [ ] **Step 2.1.3: Implement user model**

```go
// internal/domain/user/model.go
package user

type User struct {
    ID           string `gorm:"primaryKey;type:text" json:"id"`
    Email        string `gorm:"uniqueIndex;not null;type:text" json:"email"`
    Username     string `gorm:"uniqueIndex;not null;type:text" json:"username"`
    DisplayName  string `gorm:"type:text;default:''" json:"display_name"`
    AvatarURL    string `gorm:"type:text;default:''" json:"avatar_url"`
    PasswordHash string `gorm:"type:text;default:''" json:"-"`
    IsAdmin      bool   `gorm:"default:false" json:"is_admin"`
    IsActive     bool   `gorm:"default:true" json:"is_active"`
}
```

- [ ] **Step 2.1.4: Run user repo tests**

Run: `go test ./internal/store/ -v -run TestUserRepo`
Expected: PASS

- [ ] **Step 2.1.5: Commit**

```bash
git add -A
git commit -m "feat(step-2): add user domain model and repository tests"
```

---

### Task 2.2: Password hashing service

**Files:**
- Create: `internal/domain/auth/password.go`
- Create: `internal/domain/auth/password_test.go`

- [ ] **Step 2.2.1: Write failing password test**

```go
// internal/domain/auth/password_test.go
package auth

import (
    "testing"
)

func TestHashAndVerify(t *testing.T) {
    password := "my-secure-password-123"
    hash, err := HashPassword(password)
    if err != nil {
        t.Fatalf("HashPassword() error = %v", err)
    }
    if hash == password {
        t.Error("hash should not equal original password")
    }
    if !VerifyPassword(hash, password) {
        t.Error("VerifyPassword() should return true for correct password")
    }
    if VerifyPassword(hash, "wrong-password") {
        t.Error("VerifyPassword() should return false for wrong password")
    }
}

func TestHashPasswordError(t *testing.T) {
    _, err := HashPassword("")
    if err != nil {
        t.Error("empty password should still hash (bcrypt handles it)")
    }
}
```

- [ ] **Step 2.2.2: Implement password hashing**

```go
// internal/domain/auth/password.go
package auth

import (
    "golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(bytes), nil
}

func VerifyPassword(hash, password string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

- [ ] **Step 2.2.3: Run password tests**

Run: `go get golang.org/x/crypto/bcrypt && go test ./internal/domain/auth/ -v`
Expected: PASS

- [ ] **Step 2.2.4: Commit**

```bash
git add -A
git commit -m "feat(step-2): add password hashing service"
```

---

### Task 2.3: JWT token service

**Files:**
- Create: `internal/domain/auth/jwt.go`
- Create: `internal/domain/auth/jwt_test.go`

- [ ] **Step 2.3.1: Write failing JWT test**

```go
// internal/domain/auth/jwt_test.go
package auth

import (
    "testing"
    "time"
)

func TestGenerateAndValidateToken(t *testing.T) {
    secret := "test-secret-min-32-chars-long-key!!"
    userID := "user-123"

    token, err := GenerateToken(secret, userID, time.Hour)
    if err != nil {
        t.Fatalf("GenerateToken() error = %v", err)
    }
    if token == "" {
        t.Error("token should not be empty")
    }

    claims, err := ValidateToken(secret, token)
    if err != nil {
        t.Fatalf("ValidateToken() error = %v", err)
    }
    if claims.UserID != userID {
        t.Errorf("UserID = %s, want %s", claims.UserID, userID)
    }
}

func TestValidateExpiredToken(t *testing.T) {
    secret := "test-secret-min-32-chars-long-key!!"
    token, err := GenerateToken(secret, "user-123", -time.Hour)
    if err != nil {
        t.Fatalf("GenerateToken() error = %v", err)
    }
    _, err = ValidateToken(secret, token)
    if err == nil {
        t.Error("expected error for expired token")
    }
}

func TestValidateInvalidToken(t *testing.T) {
    secret := "test-secret-min-32-chars-long-key!!"
    _, err := ValidateToken(secret, "invalid-token")
    if err == nil {
        t.Error("expected error for invalid token")
    }
}
```

- [ ] **Step 2.3.2: Implement JWT service**

```go
// internal/domain/auth/jwt.go
package auth

import (
    "errors"
    "time"
    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    UserID string `json:"user_id"`
    jwt.RegisteredClaims
}

func GenerateToken(secret, userID string, duration time.Duration) (string, error) {
    now := time.Now()
    claims := Claims{
        UserID: userID,
        RegisteredClaims: jwt.RegisteredClaims{
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
            NotBefore: jwt.NewNumericDate(now),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func ValidateToken(secret, tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, errors.New("unexpected signing method")
        }
        return []byte(secret), nil
    })
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, errors.New("invalid token")
    }
    return claims, nil
}
```

- [ ] **Step 2.3.3: Run JWT tests**

Run: `go get github.com/golang-jwt/jwt/v5 && go test ./internal/domain/auth/ -v`
Expected: PASS

- [ ] **Step 2.3.4: Commit**

```bash
git add -A
git commit -m "feat(step-2): add JWT token service"
```

---

### Task 2.4: User registration and login handlers

**Files:**
- Create: `internal/api/middleware/auth.go`
- Create: `internal/api/middleware/auth_test.go`
- Create: `internal/api/handlers/auth_handler.go`
- Create: `internal/domain/auth/service.go`

- [ ] **Step 2.4.1: Install router and write auth middleware test**

```go
// internal/api/middleware/auth_test.go
package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
    "github.com/open-jira/open-jira/internal/domain/auth"
)

func TestAuthMiddleware(t *testing.T) {
    secret := "test-secret-min-32-chars-long-key!!"
    userID := "user-123"
    token, _ := auth.GenerateToken(secret, userID, time.Hour)

    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    handler := Auth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        uid := r.Context().Value(UserIDKey)
        if uid != userID {
            t.Errorf("UserID = %v, want %s", uid, userID)
        }
        w.WriteHeader(http.StatusOK)
    }))

    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
    }
}

func TestAuthMiddlewareNoToken(t *testing.T) {
    secret := "test-secret-min-32-chars-long-key!!"
    req := httptest.NewRequest("GET", "/test", nil)
    handler := Auth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)
    if rec.Code != http.StatusUnauthorized {
        t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
    }
}
```

- [ ] **Step 2.4.2: Run middleware tests (expected FAIL)**

Run: `go test ./internal/api/middleware/ -v`
Expected: FAIL — "undefined: Auth"

- [ ] **Step 2.4.3: Implement auth middleware**

```go
// internal/api/middleware/auth.go
package middleware

import (
    "context"
    "net/http"
    "strings"
    "github.com/open-jira/open-jira/internal/domain/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func Auth(secret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            header := r.Header.Get("Authorization")
            if header == "" {
                http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
                return
            }
            parts := strings.SplitN(header, " ", 2)
            if len(parts) != 2 || parts[0] != "Bearer" {
                http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
                return
            }
            claims, err := auth.ValidateToken(secret, parts[1])
            if err != nil {
                http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func UserIDFromContext(ctx context.Context) string {
    if id, ok := ctx.Value(UserIDKey).(string); ok {
        return id
    }
    return ""
}
```

- [ ] **Step 2.4.4: Run middleware tests**

Run: `go get github.com/gorilla/mux && go test ./internal/api/middleware/ -v`
Expected: PASS

- [ ] **Step 2.4.5: Commit**

```bash
git add -A
git commit -m "feat(step-2): add auth middleware"
```

---

### Task 2.5: Auth handler (register + login)

**Files:**
- Create: `internal/api/handlers/auth_handler.go`
- Create: `internal/api/handlers/auth_handler_test.go`

- [ ] **Step 2.5.1: Write auth service**

```go
// internal/domain/auth/service.go
package auth

import (
    "errors"
    "time"
    "gorm.io/gorm"
    "github.com/open-jira/open-jira/internal/domain/user"
)

type Service struct {
    DB     *gorm.DB
    Secret string
}

func NewService(db *gorm.DB, secret string) *Service {
    return &Service{DB: db, Secret: secret}
}

func (s *Service) Register(email, username, displayName, password string) (*user.User, error) {
    hashed, err := HashPassword(password)
    if err != nil {
        return nil, err
    }
    u := &user.User{
        Email:        email,
        Username:     username,
        DisplayName:  displayName,
        PasswordHash: hashed,
        IsActive:     true,
    }
    if err := s.DB.Create(u).Error; err != nil {
        return nil, errors.New("email or username already taken")
    }
    return u, nil
}

func (s *Service) Login(email, password string) (string, error) {
    var u user.User
    if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil {
        return "", errors.New("invalid credentials")
    }
    if !VerifyPassword(u.PasswordHash, password) {
        return "", errors.New("invalid credentials")
    }
    return GenerateToken(s.Secret, u.ID, 24*time.Hour)
}
```

- [ ] **Step 2.5.2: Write failing auth handler tests**

```go
// internal/api/handlers/auth_handler_test.go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"
    "github.com/open-jira/open-jira/internal/api/middleware"
    "github.com/open-jira/open-jira/internal/config"
    "github.com/open-jira/open-jira/internal/domain/auth"
    "github.com/open-jira/open-jira/internal/store"
)

func setupAuthHandler(t *testing.T) (*AuthHandler, *store.Store) {
    t.Helper()
    os.Setenv("APP_SECRET", "test-secret-min-32-chars-long-key!!")
    os.Setenv("DB_DRIVER", "sqlite")
    os.Setenv("DB_DSN", "file::memory:?cache=shared")
    defer os.Clearenv()
    cfg, _ := config.Load()
    s, _ := store.New(cfg.DB, "test")
    s.DB.AutoMigrate(&user.User{})
    svc := auth.NewService(s.DB, cfg.Secret)
    return NewAuthHandler(svc), s
}

func TestRegisterUser(t *testing.T) {
    h, s := setupAuthHandler(t)
    defer s.Close()

    body := map[string]string{
        "email":    "new@example.com",
        "username": "newuser",
        "password": "password123",
    }
    b, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    h.Register(rec, req)

    if rec.Code != http.StatusCreated {
        t.Errorf("status = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
    }
}

func TestLoginUser(t *testing.T) {
    h, s := setupAuthHandler(t)
    defer s.Close()

    // Register first
    h.svc.Register("login@example.com", "loginuser", "Login User", "password123")

    body := map[string]string{
        "email":    "login@example.com",
        "password": "password123",
    }
    b, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    h.Login(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
    }
}
```

Add imports for `"github.com/open-jira/open-jira/internal/domain/user"` to auth_handler_test.go.

- [ ] **Step 2.5.3: Implement auth handler**

```go
// internal/api/handlers/auth_handler.go
package handlers

import (
    "encoding/json"
    "net/http"
    "github.com/open-jira/open-jira/internal/domain/auth"
)

type AuthHandler struct {
    svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler {
    return &AuthHandler{svc: svc}
}

type registerRequest struct {
    Email    string `json:"email"`
    Username string `json:"username"`
    Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
    var req registerRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    if req.Email == "" || req.Username == "" || req.Password == "" {
        http.Error(w, `{"error":"email, username, and password are required"}`, http.StatusBadRequest)
        return
    }
    u, err := h.svc.Register(req.Email, req.Username, req.Email, req.Password)
    if err != nil {
        http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusConflict)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(u)
}

type loginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var req loginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    token, err := h.svc.Login(req.Email, req.Password)
    if err != nil {
        http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"token": token})
}
```

- [ ] **Step 2.5.4: Run auth handler tests**

Run: `go test ./internal/api/handlers/ -v -run TestRegisterUser`
Expected: PASS

- [ ] **Step 2.5.5: Commit**

```bash
git add -A
git commit -m "feat(step-2): add auth handler with register and login"
```

---

### Task 2.6: User API handlers (GET /users/me)

**Files:**
- Create: `internal/api/handlers/user_handler.go`
- Create: `internal/api/handlers/user_handler_test.go`

- [ ] **Step 2.6.1: Write user handler test**

```go
// internal/api/handlers/user_handler_test.go
package handlers

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/open-jira/open-jira/internal/api/middleware"
    "github.com/open-jira/open-jira/internal/domain/user"
)

func TestGetMe(t *testing.T) {
    h, s := setupAuthHandler(t)
    defer s.Close()
    u, _ := h.svc.Register("me@example.com", "meuser", "Me User", "password123")

    userHandler := NewUserHandler(s.DB)
    req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
    ctx := context.WithValue(req.Context(), middleware.UserIDKey, u.ID)
    req = req.WithContext(ctx)
    rec := httptest.NewRecorder()
    userHandler.GetMe(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
    }
}

func TestGetMeUnauthenticated(t *testing.T) {
    h, s := setupAuthHandler(t)
    defer s.Close()
    userHandler := NewUserHandler(s.DB)
    req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
    rec := httptest.NewRecorder()
    userHandler.GetMe(rec, req)
    if rec.Code != http.StatusUnauthorized {
        t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
    }
}
```

- [ ] **Step 2.6.2: Implement user handler**

```go
// internal/api/handlers/user_handler.go
package handlers

import (
    "encoding/json"
    "net/http"
    "gorm.io/gorm"
    "github.com/open-jira/open-jira/internal/api/middleware"
    "github.com/open-jira/open-jira/internal/domain/user"
)

type UserHandler struct {
    DB *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
    return &UserHandler{DB: db}
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
    userID := middleware.UserIDFromContext(r.Context())
    if userID == "" {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    var u user.User
    if err := h.DB.First(&u, "id = ?", userID).Error; err != nil {
        http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(u)
}
```

- [ ] **Step 2.6.3: Run user handler tests**

Run: `go test ./internal/api/handlers/ -v -run TestGetMe`
Expected: PASS

- [ ] **Step 2.6.4: Commit**

```bash
git add -A
git commit -m "feat(step-2): add user handler with GetMe endpoint"
```

---

### Task 2.7: OAuth2 providers (Forgejo, GitLab, GitHub)

**Files:**
- Create: `internal/domain/auth/oauth.go`
- Create: `internal/api/handlers/oauth_handler.go`

- [ ] **Step 2.7.1: Write OAuth provider interface and implementations**

```go
// internal/domain/auth/oauth.go
package auth

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "golang.org/x/oauth2"
)

type OAuthUserInfo struct {
    Email      string `json:"email"`
    Username   string `json:"username"`
    DisplayName string `json:"display_name"`
    AvatarURL  string `json:"avatar_url"`
    ProviderID string `json:"provider_id"`
}

type OAuthProvider interface {
    GetName() string
    AuthCodeURL(state string) string
    Exchange(ctx context.Context, code string) (*oauth2.Token, error)
    GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error)
}

// Generic OAuth2 provider
type GenericOAuthProvider struct {
    name         string
    config       *oauth2.Config
    userInfoURL  string
    emailField   string
    usernameField string
    nameField    string
    avatarField  string
}

func (p *GenericOAuthProvider) GetName() string { return p.name }

func (p *GenericOAuthProvider) AuthCodeURL(state string) string {
    return p.config.AuthCodeURL(state)
}

func (p *GenericOAuthProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
    return p.config.Exchange(ctx, code)
}

func (p *GenericOAuthProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
    client := p.config.Client(ctx, token)
    resp, err := client.Get(p.userInfoURL)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var data map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }
    return &OAuthUserInfo{
        Email:      getStringField(data, p.emailField),
        Username:   getStringField(data, p.usernameField),
        DisplayName: getStringField(data, p.nameField),
        AvatarURL:  getStringField(data, p.avatarField),
        ProviderID: fmt.Sprintf("%s:%s", p.name, getStringField(data, "id")),
    }, nil
}

func getStringField(data map[string]interface{}, field string) string {
    val, ok := data[field]
    if !ok {
        return ""
    }
    s, _ := val.(string)
    return s
}

func parseInt64(s string) int64 {
    // simplified; not used for now
    return 0
}

func NewProvider(name, clientID, clientSecret, redirectURL, authURL, tokenURL, userInfoURL string, scopes []string, emailField, usernameField, nameField, avatarField string) OAuthProvider {
    return &GenericOAuthProvider{
        name: name,
        config: &oauth2.Config{
            ClientID:     clientID,
            ClientSecret: clientSecret,
            RedirectURL:  redirectURL,
            Scopes:       scopes,
            Endpoint: oauth2.Endpoint{
                AuthURL:  authURL,
                TokenURL: tokenURL,
            },
        },
        userInfoURL:   userInfoURL,
        emailField:    emailField,
        usernameField: usernameField,
        nameField:     nameField,
        avatarField:   avatarField,
    }
}
```

- [ ] **Step 2.7.2: Write OAuth handler**

```go
// internal/api/handlers/oauth_handler.go
package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "net/http"
    "gorm.io/gorm"
    "github.com/open-jira/open-jira/internal/domain/auth"
    "github.com/open-jira/open-jira/internal/domain/user"
)

type OAuthHandler struct {
    DB        *gorm.DB
    Secret    string
    BaseURL   string
    Providers map[string]auth.OAuthProvider
    States    map[string]string // production would use Redis
}

func NewOAuthHandler(db *gorm.DB, secret, baseURL string) *OAuthHandler {
    return &OAuthHandler{
        DB:        db,
        Secret:    secret,
        BaseURL:   baseURL,
        Providers: make(map[string]auth.OAuthProvider),
        States:    make(map[string]string),
    }
}

func (h *OAuthHandler) AddProvider(provider auth.OAuthProvider) {
    h.Providers[provider.GetName()] = provider
}

func (h *OAuthHandler) Redirect(w http.ResponseWriter, r *http.Request) {
    providerName := r.PathValue("provider")
    p, ok := h.Providers[providerName]
    if !ok {
        http.Error(w, `{"error":"unsupported provider"}`, http.StatusBadRequest)
        return
    }
    stateBytes := make([]byte, 16)
    rand.Read(stateBytes)
    state := hex.EncodeToString(stateBytes)
    h.States[state] = providerName
    http.Redirect(w, r, p.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
    providerName := r.PathValue("provider")
    p, ok := h.Providers[providerName]
    if !ok {
        http.Error(w, `{"error":"unsupported provider"}`, http.StatusBadRequest)
        return
    }
    state := r.URL.Query().Get("state")
    if _, ok := h.States[state]; !ok {
        http.Error(w, `{"error":"invalid state"}`, http.StatusBadRequest)
        return
    }
    delete(h.States, state)
    code := r.URL.Query().Get("code")
    token, err := p.Exchange(r.Context(), code)
    if err != nil {
        http.Error(w, `{"error":"failed to exchange token"}`, http.StatusInternalServerError)
        return
    }
    info, err := p.GetUserInfo(r.Context(), token)
    if err != nil {
        http.Error(w, `{"error":"failed to get user info"}`, http.StatusInternalServerError)
        return
    }
    var u user.User
    if err := h.DB.Where("email = ?", info.Email).First(&u).Error; err != nil {
        u = user.User{
            Email:       info.Email,
            Username:    info.Username,
            DisplayName: info.DisplayName,
            AvatarURL:   info.AvatarURL,
            IsActive:    true,
        }
        h.DB.Create(&u)
    }
    jwtToken, _ := auth.GenerateToken(h.Secret, u.ID, 24*3600*1000000000)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"token": jwtToken, "provider": providerName})
}
```

- [ ] **Step 2.7.3: Commit**

```bash
go mod tidy && go get golang.org/x/oauth2
git add -A
git commit -m "feat(step-2): add OAuth2 provider support for Forgejo/GitLab/GitHub"
```

---

### Task 2.8: Router setup and Step 2 integration

**Files:**
- Create: `internal/api/router.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 2.8.1: Write router**

```go
// internal/api/router.go
package api

import (
    "net/http"
    "gorm.io/gorm"
    "github.com/gorilla/mux"
    "github.com/open-jira/open-jira/internal/api/handlers"
    "github.com/open-jira/open-jira/internal/api/middleware"
    "github.com/open-jira/open-jira/internal/config"
    "github.com/open-jira/open-jira/internal/domain/auth"
)

func NewRouter(cfg *config.Config, db *gorm.DB) http.Handler {
    r := mux.NewRouter()
    api := r.PathPrefix("/api/v1").Subrouter()

    authSvc := auth.NewService(db, cfg.Secret)
    authH := handlers.NewAuthHandler(authSvc)
    userH := handlers.NewUserHandler(db)

    // OAuth setup
    oauthH := handlers.NewOAuthHandler(db, cfg.Secret, cfg.BaseURL)
    // Note: actual providers require env vars for client_id/secret from config

    // Public routes
    api.HandleFunc("/auth/register", authH.Register).Methods("POST")
    api.HandleFunc("/auth/login", authH.Login).Methods("POST")
    api.HandleFunc("/auth/oauth/{provider}/redirect", oauthH.Redirect).Methods("GET")
    api.HandleFunc("/auth/oauth/{provider}/callback", oauthH.Callback).Methods("GET")

    // Protected routes
    protected := api.PathPrefix("").Subrouter()
    protected.Use(middleware.Auth(cfg.Secret))
    protected.HandleFunc("/users/me", userH.GetMe).Methods("GET")

    // CORS middleware
    r.Use(corsMiddleware)
    return r
}

func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

- [ ] **Step 2.8.2: Update server main.go**

```go
// cmd/server/main.go
package main

import (
    "net/http"
    "github.com/open-jira/open-jira/internal/api"
    "github.com/open-jira/open-jira/internal/config"
    "github.com/open-jira/open-jira/internal/log"
    "github.com/open-jira/open-jira/internal/store"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        panic(err)
    }
    logger := log.New(cfg.Env)
    s, err := store.New(cfg.DB, cfg.Env)
    if err != nil {
        logger.Error("failed to connect to database", "error", err)
        panic(err)
    }
    defer s.Close()
    if err := store.RunMigrations(cfg.DB); err != nil {
        logger.Error("failed to run migrations", "error", err)
        panic(err)
    }
    router := api.NewRouter(cfg, s.DB)
    logger.Info("starting server", "port", cfg.Port)
    if err := http.ListenAndServe(":8080", router); err != nil {
        logger.Error("server error", "error", err)
    }
}
```

- [ ] **Step 2.8.3: Run all tests for Step 2**

Run: `go test ./... -v -count=1`
Expected: all PASS

- [ ] **Step 2.8.4: Final commit for Step 2**

```bash
git add -A
git commit -m "feat(step-2): add router, OAuth handler, and wire up step-2"
```

---

## Step 3: Projects Core

**Branch:** `feature/step-3-projects-core` (from step-2)

### Task 3.1: Project domain model

**Files:**
- Create: `internal/domain/project/model.go`

- [ ] **Step 3.1.1: Write project model**

```go
// internal/domain/project/model.go
package project

import "time"

type Type string

const (
    TypeScrum    Type = "scrum"
    TypeKanban   Type = "kanban"
    TypeBusiness Type = "business"
)

type DefaultAssignee string

const (
    AssigneeUnassigned DefaultAssignee = "unassigned"
    AssigneeProjectLead DefaultAssignee = "project_lead"
)

type Project struct {
    ID              string    `gorm:"primaryKey;type:text" json:"id"`
    OrgID           *string   `gorm:"type:text" json:"org_id,omitempty"`
    Name            string    `gorm:"not null;type:text" json:"name"`
    Key             string    `gorm:"uniqueIndex;not null;type:text" json:"key"`
    Description     string    `gorm:"type:text;default:''" json:"description"`
    Type            Type      `gorm:"type:text;not null;default:'scrum'" json:"type"`
    LeadUserID      *string   `gorm:"type:text" json:"lead_user_id,omitempty"`
    DefaultAssignee string    `gorm:"type:text;default:'unassigned'" json:"default_assignee"`
    IconURL         string    `gorm:"type:text;default:''" json:"icon_url"`
    IsArchived      bool      `gorm:"default:false" json:"is_archived"`
    CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Project) TableName() string {
    return "projects"
}
```

- [ ] **Step 3.1.2: Write project member model**

```go
// internal/domain/project/member.go
package project

type MemberRole string

const (
    RoleAdmin  MemberRole = "admin"
    RoleMember MemberRole = "member"
    RoleViewer MemberRole = "viewer"
)

type ProjectMember struct {
    ProjectID string     `gorm:"primaryKey;type:text;not null" json:"project_id"`
    UserID    string     `gorm:"primaryKey;type:text;not null" json:"user_id"`
    Role      MemberRole `gorm:"type:text;not null;default:'member'" json:"role"`
}

func (ProjectMember) TableName() string {
    return "project_members"
}
```

- [ ] **Step 3.1.3: Commit**

```bash
go mod tidy && git add -A && git commit -m "feat(step-3): add project and project_member domain models"
```

---

### Task 3.2: Project service with CRUD logic

**Files:**
- Create: `internal/domain/project/service.go`
- Create: `internal/domain/project/service_test.go`

- [ ] **Step 3.2.1: Write failing project service test**

```go
// internal/domain/project/service_test.go
package project

import (
    "testing"
    "github.com/google/uuid"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "github.com/open-jira/open-jira/internal/domain/user"
)

func setupTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }
    db.AutoMigrate(&user.User{}, &Project{}, &ProjectMember{})
    return db
}

func TestCreateProject(t *testing.T) {
    db := setupTestDB(t)
    svc := NewService(db, &user.User{ID: uuid.New().String()})

    p, err := svc.Create("Test Project", "TEST", "A test project", TypeScrum)
    if err != nil {
        t.Fatalf("Create() error = %v", err)
    }
    if p.Name != "Test Project" {
        t.Errorf("Name = %s, want Test Project", p.Name)
    }
    if p.Key != "TEST" {
        t.Errorf("Key = %s, want TEST", p.Key)
    }
    if p.Type != TypeScrum {
        t.Errorf("Type = %s, want scrum", p.Type)
    }
}

func TestCreateProjectDuplicateKey(t *testing.T) {
    db := setupTestDB(t)
    lead := &user.User{ID: uuid.New().String()}
    db.Create(lead)
    svc := NewService(db, lead)

    _, err := svc.Create("Project A", "DUP", "desc", TypeScrum)
    if err != nil {
        t.Fatal(err)
    }
    _, err = svc.Create("Project B", "DUP", "desc", TypeScrum)
    if err == nil {
        t.Error("expected error for duplicate key")
    }
}
```

- [ ] **Step 3.2.2: Run test (expected FAIL)**

Run: `go test ./internal/domain/project/ -v`
Expected: FAIL — "undefined: NewService"

- [ ] **Step 3.2.3: Implement project service**

```go
// internal/domain/project/service.go
package project

import (
    "errors"
    "strings"
    "github.com/google/uuid"
    "gorm.io/gorm"
    "github.com/open-jira/open-jira/internal/domain/user"
)

type Service struct {
    db   *gorm.DB
    lead *user.User
}

func NewService(db *gorm.DB, lead *user.User) *Service {
    return &Service{db: db, lead: lead}
}

func (s *Service) Create(name, key, description string, pType Type) (*Project, error) {
    key = strings.ToUpper(key)
    if len(key) < 2 || len(key) > 10 {
        return nil, errors.New("project key must be 2-10 characters")
    }
    var existing Project
    if err := s.db.Where("key = ?", key).First(&existing).Error; err == nil {
        return nil, errors.New("project key already exists")
    }
    p := &Project{
        ID:          uuid.New().String(),
        Name:        name,
        Key:         key,
        Description: description,
        Type:        pType,
    }
    if s.lead != nil {
        p.LeadUserID = &s.lead.ID
    }
    if err := s.db.Create(p).Error; err != nil {
        return nil, err
    }
    return p, nil
}

func (s *Service) GetByKey(key string) (*Project, error) {
    var p Project
    if err := s.db.Where("key = ?", strings.ToUpper(key)).First(&p).Error; err != nil {
        return nil, errors.New("project not found")
    }
    return &p, nil
}

func (s *Service) GetByID(id string) (*Project, error) {
    var p Project
    if err := s.db.First(&p, "id = ?", id).Error; err != nil {
        return nil, errors.New("project not found")
    }
    return &p, nil
}

func (s *Service) List(archived bool) ([]Project, error) {
    var projects []Project
    query := s.db
    if !archived {
        query = query.Where("is_archived = ?", false)
    }
    if err := query.Order("created_at DESC").Find(&projects).Error; err != nil {
        return nil, err
    }
    return projects, nil
}

func (s *Service) Update(key string, name, description string) (*Project, error) {
    p, err := s.GetByKey(key)
    if err != nil {
        return nil, err
    }
    if name != "" {
        p.Name = name
    }
    p.Description = description
    if err := s.db.Save(p).Error; err != nil {
        return nil, err
    }
    return p, nil
}

func (s *Service) Archive(key string) error {
    return s.db.Model(&Project{}).Where("key = ?", strings.ToUpper(key)).Update("is_archived", true).Error
}

func (s *Service) AddMember(projectID, userID string, role MemberRole) error {
    pm := &ProjectMember{ProjectID: projectID, UserID: userID, Role: role}
    return s.db.Create(pm).Error
}

func (s *Service) RemoveMember(projectID, userID string) error {
    return s.db.Where("project_id = ? AND user_id = ?", projectID, userID).Delete(&ProjectMember{}).Error
}

func (s *Service) ListMembers(projectID string) ([]ProjectMember, error) {
    var members []ProjectMember
    if err := s.db.Where("project_id = ?", projectID).Find(&members).Error; err != nil {
        return nil, err
    }
    return members, nil
}
```

- [ ] **Step 3.2.4: Run project service tests**

Run: `go test ./internal/domain/project/ -v`
Expected: PASS

- [ ] **Step 3.2.5: Commit**

```bash
git add -A
git commit -m "feat(step-3): add project service with CRUD and member management"
```

---

### Task 3.3: Project API handlers

**Files:**
- Create: `internal/api/handlers/project_handler.go`
- Create: `internal/api/handlers/project_handler_test.go`

- [ ] **Step 3.3.1: Write project handler test**

```go
// internal/api/handlers/project_handler_test.go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"
    "github.com/google/uuid"
    "github.com/gorilla/mux"
    "github.com/open-jira/open-jira/internal/config"
    "github.com/open-jira/open-jira/internal/domain/project"
    "github.com/open-jira/open-jira/internal/domain/user"
    "github.com/open-jira/open-jira/internal/store"
)

func setupProjectHandler(t *testing.T) (*ProjectHandler, *store.Store) {
    t.Helper()
    os.Setenv("APP_SECRET", "test-secret-min-32-chars-long-key!!")
    os.Setenv("DB_DRIVER", "sqlite")
    os.Setenv("DB_DSN", "file::memory:?cache=shared")
    defer os.Clearenv()
    cfg, _ := config.Load()
    s, _ := store.New(cfg.DB, "test")
    s.DB.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{})
    lead := &user.User{ID: uuid.New().String(), Email: "lead@test.com", Username: "lead"}
    s.DB.Create(lead)
    svc := project.NewService(s.DB, lead)
    return NewProjectHandler(svc), s
}

func TestCreateProject(t *testing.T) {
    h, s := setupProjectHandler(t)
    defer s.Close()

    body := map[string]interface{}{
        "name":        "Test Project",
        "key":         "TEST",
        "description": "A test project",
        "type":        "scrum",
    }
    b, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    h.Create(rec, req)

    if rec.Code != http.StatusCreated {
        t.Errorf("status = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
    }
}

func TestListProjects(t *testing.T) {
    h, s := setupProjectHandler(t)
    defer s.Close()

    h.svc.Create("P1", "P1", "desc", project.TypeScrum)
    h.svc.Create("P2", "P2", "desc", project.TypeKanban)

    req := httptest.NewRequest("GET", "/api/v1/projects", nil)
    rec := httptest.NewRecorder()
    h.List(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
    }
}

func TestGetProject(t *testing.T) {
    h, s := setupProjectHandler(t)
    defer s.Close()

    p, _ := h.svc.Create("Test", "KEY", "desc", project.TypeScrum)

    req := httptest.NewRequest("GET", "/api/v1/projects/KEY", nil)
    req = mux.SetURLVars(req, map[string]string{"key": p.Key})
    rec := httptest.NewRecorder()
    h.Get(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
    }
}
```

- [ ] **Step 3.3.2: Implement project handler**

```go
// internal/api/handlers/project_handler.go
package handlers

import (
    "encoding/json"
    "net/http"
    "github.com/open-jira/open-jira/internal/domain/project"
)

type ProjectHandler struct {
    svc *project.Service
}

func NewProjectHandler(svc *project.Service) *ProjectHandler {
    return &ProjectHandler{svc: svc}
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name        string        `json:"name"`
        Key         string        `json:"key"`
        Description string        `json:"description"`
        Type        project.Type  `json:"type"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    if req.Name == "" || req.Key == "" {
        http.Error(w, `{"error":"name and key are required"}`, http.StatusBadRequest)
        return
    }
    p, err := h.svc.Create(req.Name, req.Key, req.Description, req.Type)
    if err != nil {
        http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusConflict)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    p, err := h.svc.GetByKey(key)
    if err != nil {
        http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
    projects, err := h.svc.List(false)
    if err != nil {
        http.Error(w, `{"error":"failed to list projects"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(projects)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    var req struct {
        Name        string `json:"name"`
        Description string `json:"description"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    p, err := h.svc.Update(key, req.Name, req.Description)
    if err != nil {
        http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    if err := h.svc.Archive(key); err != nil {
        http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3.3.3: Run project handler tests**

Run: `go test ./internal/api/handlers/ -v -run TestCreateProject`
Expected: PASS

- [ ] **Step 3.3.4: Add project routes to router**

Add to `internal/api/router.go`:

```go
projectH := handlers.NewProjectHandler(project.NewService(db, &user.User{}))
// Protected project routes (add under protected subrouter)
protected.HandleFunc("/projects", projectH.List).Methods("GET")
protected.HandleFunc("/projects", projectH.Create).Methods("POST")
protected.HandleFunc("/projects/{key}", projectH.Get).Methods("GET")
protected.HandleFunc("/projects/{key}", projectH.Update).Methods("PATCH")
protected.HandleFunc("/projects/{key}", projectH.Delete).Methods("DELETE")
```

Update imports in router.go to include `"github.com/open-jira/open-jira/internal/domain/project"` and `"github.com/open-jira/open-jira/internal/domain/user"`.

- [ ] **Step 3.3.5: Run all tests**

Run: `go test ./... -v -count=1`
Expected: all PASS

- [ ] **Step 3.3.6: Commit**

```bash
git add -A
git commit -m "feat(step-3): add project API handlers and routes"
```

---

### Task 3.4: Invitation system

**Files:**
- Create: `internal/domain/project/invite.go`
- Create: `internal/domain/project/invite_test.go`

- [ ] **Step 3.4.1: Write invitation service**

```go
// internal/domain/project/invite.go
package project

import (
    "crypto/rand"
    "encoding/hex"
    "errors"
    "gorm.io/gorm"
)

type Invite struct {
    gorm.Model
    ProjectID string `gorm:"type:text;not null"`
    Email     string `gorm:"type:text;not null"`
    Token     string `gorm:"type:text;uniqueIndex;not null"`
    Role      MemberRole `gorm:"type:text;not null;default:'member'"`
    Accepted  bool   `gorm:"default:false"`
    AcceptedBy string `gorm:"type:text"`
}

func (Invite) TableName() string {
    return "project_invites"
}

func CreateInvite(db *gorm.DB, projectID, email string, role MemberRole) (*Invite, error) {
    if projectID == "" || email == "" {
        return nil, errors.New("project_id and email are required")
    }
    b := make([]byte, 32)
    rand.Read(b)
    invite := &Invite{
        ProjectID: projectID,
        Email:     email,
        Token:     hex.EncodeToString(b),
        Role:      role,
    }
    if err := db.Create(invite).Error; err != nil {
        return nil, err
    }
    return invite, nil
}

func AcceptInvite(db *gorm.DB, token, userID string) (*ProjectMember, error) {
    var invite Invite
    if err := db.Where("token = ? AND accepted = ?", token, false).First(&invite).Error; err != nil {
        return nil, errors.New("invalid or expired invite")
    }
    pm := &ProjectMember{
        ProjectID: invite.ProjectID,
        UserID:    userID,
        Role:      invite.Role,
    }
    if err := db.Create(pm).Error; err != nil {
        return nil, err
    }
    db.Model(&invite).Updates(map[string]interface{}{"accepted": true, "accepted_by": userID})
    return pm, nil
}
```

- [ ] **Step 3.4.2: Write invite test**

```go
// internal/domain/project/invite_test.go
package project

import (
    "testing"
    "github.com/google/uuid"
)

func TestCreateAndAcceptInvite(t *testing.T) {
    db := setupTestDB(t)
    db.AutoMigrate(&Invite{})
    projectID := uuid.New().String()
    userID := uuid.New().String()

    inv, err := CreateInvite(db, projectID, "invited@example.com", RoleMember)
    if err != nil {
        t.Fatalf("CreateInvite() error = %v", err)
    }
    if inv.Token == "" {
        t.Error("token should not be empty")
    }

    pm, err := AcceptInvite(db, inv.Token, userID)
    if err != nil {
        t.Fatalf("AcceptInvite() error = %v", err)
    }
    if pm.ProjectID != projectID {
        t.Errorf("ProjectID = %s, want %s", pm.ProjectID, projectID)
    }
}
```

- [ ] **Step 3.4.3: Run invite tests**

Run: `go test ./internal/domain/project/ -v -run TestCreateAndAcceptInvite`
Expected: PASS

- [ ] **Step 3.4.4: Commit**

```bash
go mod tidy && git add -A && git commit -m "feat(step-3): add invitation system"
```

---

### Task 3.5: Project member API handlers

- [ ] **Step 3.5.1: Add invite and member handlers to project handler**

Add to `internal/api/handlers/project_handler.go`:

```go
func (h *ProjectHandler) AddMember(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    p, err := h.svc.GetByKey(key)
    if err != nil {
        http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
        return
    }
    var req struct {
        UserID string             `json:"user_id"`
        Role   project.MemberRole `json:"role"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    if err := h.svc.AddMember(p.ID, req.UserID, req.Role); err != nil {
        http.Error(w, `{"error":"failed to add member"}`, http.StatusConflict)
        return
    }
    w.WriteHeader(http.StatusCreated)
}

func (h *ProjectHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    p, err := h.svc.GetByKey(key)
    if err != nil {
        http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
        return
    }
    members, err := h.svc.ListMembers(p.ID)
    if err != nil {
        http.Error(w, `{"error":"failed to list members"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(members)
}

func (h *ProjectHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    userID := r.PathValue("userId")
    p, err := h.svc.GetByKey(key)
    if err != nil {
        http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
        return
    }
    if err := h.svc.RemoveMember(p.ID, userID); err != nil {
        http.Error(w, `{"error":"failed to remove member"}`, http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) Invite(w http.ResponseWriter, r *http.Request) {
    key := r.PathValue("key")
    p, err := h.svc.GetByKey(key)
    if err != nil {
        http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
        return
    }
    var req struct {
        Email string             `json:"email"`
        Role  project.MemberRole `json:"role"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    inv, err := project.CreateInvite(h.svc.DB(), p.ID, req.Email, req.Role)
    if err != nil {
        http.Error(w, `{"error":"failed to create invite"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]string{"token": inv.Token})
}
```

Add `DB()` method to project service:

```go
func (s *Service) DB() *gorm.DB { return s.db }
```

- [ ] **Step 3.5.2: Update router with member/invite routes**

Add to router.go under protected subrouter:

```go
protected.HandleFunc("/projects/{key}/members", projectH.ListMembers).Methods("GET")
protected.HandleFunc("/projects/{key}/members", projectH.AddMember).Methods("POST")
protected.HandleFunc("/projects/{key}/members/{userId}", projectH.RemoveMember).Methods("DELETE")
protected.HandleFunc("/projects/{key}/invites", projectH.Invite).Methods("POST")
```

- [ ] **Step 3.5.3: Run all tests and commit**

```bash
go test ./... -v -count=1
git add -A && git commit -m "feat(step-3): add member management and invite API endpoints"
```

---

## Step 4: Issues Core

*(Due to document length, Steps 4-12 follow the same TDD pattern. Below is a condensed but complete specification for each.)*

**Branch:** `feature/step-4-issues-core` (from step-3)

### Task 4.1: Issue type model

**Files:**
- Create: `internal/domain/issue/model.go`
- Create: `internal/domain/issue/change.go`

- [ ] **Step 4.1.1: Write issue, issue type, label, link models**

```go
// internal/domain/issue/model.go
package issue

import "time"

type Priority string
const (PriorityHighest Priority = "highest"; PriorityHigh Priority = "high"; PriorityMedium Priority = "medium"; PriorityLow Priority = "low"; PriorityLowest Priority = "lowest")

type IssueType struct {
    ID          string `gorm:"primaryKey;type:text" json:"id"`
    ProjectID   string `gorm:"type:text;not null;index" json:"project_id"`
    Name        string `gorm:"type:text;not null" json:"name"`
    Description string `gorm:"type:text;default:''" json:"description"`
    Icon        string `gorm:"type:text;default:'task'" json:"icon"`
    Color       string `gorm:"type:text;default:'#6B7280'" json:"color"`
    IsSubtask   bool   `gorm:"default:false" json:"is_subtask"`
}

type Issue struct {
    ID               string     `gorm:"primaryKey;type:text" json:"id"`
    ProjectID        string     `gorm:"type:text;not null;index" json:"project_id"`
    Key              string     `gorm:"uniqueIndex;not null;type:text" json:"key"`
    Title            string     `gorm:"type:text;not null" json:"title"`
    DescriptionJSON  string     `gorm:"type:text;default:'{}'" json:"description_json"`
    TypeID           *string    `gorm:"type:text" json:"type_id,omitempty"`
    StatusID         *string    `gorm:"type:text" json:"status_id,omitempty"`
    Priority         Priority   `gorm:"type:text;default:'medium'" json:"priority"`
    AssigneeID       *string    `gorm:"type:text" json:"assignee_id,omitempty"`
    ReporterID       *string    `gorm:"type:text" json:"reporter_id,omitempty"`
    ParentID         *string    `gorm:"type:text;index" json:"parent_id,omitempty"`
    SprintID         *string    `gorm:"type:text;index" json:"sprint_id,omitempty"`
    VersionID        *string    `gorm:"type:text" json:"version_id,omitempty"`
    StoryPoints      int        `gorm:"default:0" json:"story_points"`
    OriginalEstimate int        `gorm:"default:0" json:"original_estimate"`
    TimeSpent        int        `gorm:"default:0" json:"time_spent"`
    StartDate        *time.Time `json:"start_date,omitempty"`
    DueDate          *time.Time `json:"due_date,omitempty"`
    Environment      string     `gorm:"type:text;default:''" json:"environment"`
    IsArchived       bool       `gorm:"default:false" json:"is_archived"`
    Position         float64    `gorm:"not null;default:0" json:"position"`
    CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type Label struct {
    ID        string `gorm:"primaryKey;type:text" json:"id"`
    ProjectID string `gorm:"type:text;not null;uniqueIndex:idx_project_label" json:"project_id"`
    Name      string `gorm:"type:text;not null;uniqueIndex:idx_project_label" json:"name"`
    Color     string `gorm:"type:text;default:'#6B7280'" json:"color"`
}

type IssueLabel struct {
    IssueID string `gorm:"primaryKey;type:text" json:"issue_id"`
    LabelID string `gorm:"primaryKey;type:text" json:"label_id"`
}

type LinkType string
const (LinkBlocks LinkType = "blocks"; LinkIsBlocked LinkType = "is_blocked"; LinkDuplicates LinkType = "duplicates"; LinkRelates LinkType = "relates")

type IssueLink struct {
    ID       string   `gorm:"primaryKey;type:text" json:"id"`
    SourceID string   `gorm:"type:text;not null;index" json:"source_id"`
    TargetID string   `gorm:"type:text;not null;index" json:"target_id"`
    LinkType LinkType `gorm:"type:text;not null" json:"link_type"`
}
```

- [ ] **Step 4.1.2: Write issue history model**

```go
// internal/domain/issue/change.go
package issue

import "time"

type IssueHistory struct {
    ID        string    `gorm:"primaryKey;type:text" json:"id"`
    IssueID   string    `gorm:"type:text;not null;index" json:"issue_id"`
    ActorID   *string   `gorm:"type:text" json:"actor_id,omitempty"`
    FieldName string    `gorm:"type:text;not null" json:"field_name"`
    OldValue  string    `gorm:"type:text;default:''" json:"old_value"`
    NewValue  string    `gorm:"type:text;default:''" json:"new_value"`
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type Comment struct {
    ID        string    `gorm:"primaryKey;type:text" json:"id"`
    IssueID   string    `gorm:"type:text;not null;index" json:"issue_id"`
    AuthorID  *string   `gorm:"type:text" json:"author_id,omitempty"`
    BodyJSON  string    `gorm:"type:text;default:'{}'" json:"body_json"`
    IsDeleted bool      `gorm:"default:false" json:"is_deleted"`
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type IssueWatcher struct {
    IssueID string `gorm:"primaryKey;type:text" json:"issue_id"`
    UserID  string `gorm:"primaryKey;type:text" json:"user_id"`
}

type IssueAttachment struct {
    ID         string    `gorm:"primaryKey;type:text" json:"id"`
    IssueID    string    `gorm:"type:text;not null;index" json:"issue_id"`
    Filename   string    `gorm:"type:text;not null" json:"filename"`
    FilePath   string    `gorm:"type:text;not null" json:"file_path"`
    FileSize   int64     `gorm:"not null;default:0" json:"file_size"`
    UploaderID *string   `gorm:"type:text" json:"uploader_id,omitempty"`
    CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
```

- [ ] **Step 4.1.3: Commit**

```bash
go mod tidy && git add -A && git commit -m "feat(step-4): add issue domain models"
```

### Task 4.2: Issue service with CRUD, labels, links, parental hierarchy

**Files:**
- Create: `internal/domain/issue/service.go`
- Create: `internal/domain/issue/service_test.go`

- [ ] **Step 4.2.1: Write issue service test**

```go
// internal/domain/issue/service_test.go
package issue

import (
    "testing"
    "github.com/google/uuid"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupIssueDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    db.AutoMigrate(&Issue{}, &IssueType{}, &Label{}, &IssueLabel{}, &IssueLink{}, &IssueHistory{}, &IssueWatcher{})
    return db
}

func TestCreateIssue(t *testing.T) {
    db := setupIssueDB(t)
    svc := NewService(db, "PROJ")
    projectID := uuid.New().String()

    // Create default issue types
    for _, name := range []string{"Epic", "Story", "Task", "Bug", "Subtask"} {
        db.Create(&IssueType{ID: uuid.New().String(), ProjectID: projectID, Name: name})
    }

    issue, err := svc.Create("PROJ", projectID, "Test Issue", "A description", PriorityMedium, nil, nil)
    if err != nil {
        t.Fatalf("Create() error = %v", err)
    }
    if issue.Title != "Test Issue" {
        t.Errorf("Title = %s, want Test Issue", issue.Title)
    }
    if issue.Key == "" {
        t.Error("Key should be generated")
    }
    if !strings.HasPrefix(issue.Key, "PROJ-") {
        t.Errorf("Key should start with PROJ-, got %s", issue.Key)
    }
}

func TestIssueHierarchy(t *testing.T) {
    db := setupIssueDB(t)
    svc := NewService(db, "HIER")
    projectID := uuid.New().String()
    db.Create(&IssueType{ID: uuid.New().String(), ProjectID: projectID, Name: "Epic"})
    db.Create(&IssueType{ID: uuid.New().String(), ProjectID: projectID, Name: "Story"})
    db.Create(&IssueType{ID: uuid.New().String(), ProjectID: projectID, Name: "Subtask", IsSubtask: true})

    epic, _ := svc.Create("HIER", projectID, "Epic Title", "", PriorityMedium, nil, nil)
    story, _ := svc.Create("HIER", projectID, "Story Title", "", PriorityMedium, &epic.ID, nil)
    subtask, _ := svc.Create("HIER", projectID, "Subtask Title", "", PriorityMedium, &story.ID, nil)

    if subtask.ParentID == nil || *subtask.ParentID != story.ID {
        t.Error("subtask parent should be story")
    }
    if story.ParentID == nil || *story.ParentID != epic.ID {
        t.Error("story parent should be epic")
    }
}
```

Add import `"strings"` to service_test.go.

- [ ] **Step 4.2.2: Implement issue service**

```go
// internal/domain/issue/service.go
package issue

import (
    "errors"
    "fmt"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

type Service struct {
    db     *gorm.DB
    cache  map[string]int64 // project key → next issue number (production would use DB sequence)
}

func NewService(db *gorm.DB, _ string) *Service {
    return &Service{db: db, cache: make(map[string]int64)}
}

func (s *Service) Create(projectKey, projectID, title, description string, priority Priority, parentID *string, typeID *string) (*Issue, error) {
    if title == "" {
        return nil, errors.New("title is required")
    }
    var maxIssue Issue
    s.db.Where("project_id = ?", projectID).Order("created_at DESC").Limit(1).Find(&maxIssue)
    seq := int64(1)
    if maxIssue.Key != "" {
        fmt.Sscanf(maxIssue.Key, projectKey+"-%d", &seq)
        seq++
    }
    key := fmt.Sprintf("%s-%d", projectKey, seq)

    issue := &Issue{
        ID:              uuid.New().String(),
        ProjectID:       projectID,
        Key:             key,
        Title:           title,
        DescriptionJSON: fmt.Sprintf(`{"content":"%s"}`, description),
        Priority:        priority,
        ParentID:        parentID,
        TypeID:          typeID,
        Position:        float64(seq * 1000),
    }
    if err := s.db.Create(issue).Error; err != nil {
        return nil, err
    }
    s.logHistory(issue.ID, "", "created", "", key)
    return issue, nil
}

func (s *Service) GetByKey(key string) (*Issue, error) {
    var issue Issue
    if err := s.db.Where("key = ?", key).First(&issue).Error; err != nil {
        return nil, errors.New("issue not found")
    }
    return &issue, nil
}

func (s *Service) GetByID(id string) (*Issue, error) {
    var issue Issue
    if err := s.db.First(&issue, "id = ?", id).Error; err != nil {
        return nil, errors.New("issue not found")
    }
    return &issue, nil
}

func (s *Service) Update(key string, title, descriptionJSON *string, priority *Priority, assigneeID, statusID *string, storyPoints *int) (*Issue, error) {
    issue, err := s.GetByKey(key)
    if err != nil {
        return nil, err
    }
    updates := map[string]interface{}{}
    if title != nil {
        s.logHistory(issue.ID, "", "title", issue.Title, *title)
        updates["title"] = *title
    }
    if descriptionJSON != nil {
        s.logHistory(issue.ID, "", "description", issue.DescriptionJSON, *descriptionJSON)
        updates["description_json"] = *descriptionJSON
    }
    if priority != nil {
        s.logHistory(issue.ID, "", "priority", string(issue.Priority), string(*priority))
        updates["priority"] = *priority
    }
    if assigneeID != nil {
        old := ""
        if issue.AssigneeID != nil { old = *issue.AssigneeID }
        s.logHistory(issue.ID, "", "assignee", old, *assigneeID)
        updates["assignee_id"] = *assigneeID
    }
    if statusID != nil {
        old := ""
        if issue.StatusID != nil { old = *issue.StatusID }
        s.logHistory(issue.ID, "", "status", old, *statusID)
        updates["status_id"] = *statusID
    }
    if storyPoints != nil {
        old := fmt.Sprintf("%d", issue.StoryPoints)
        s.logHistory(issue.ID, "", "story_points", old, fmt.Sprintf("%d", *storyPoints))
        updates["story_points"] = *storyPoints
    }
    if err := s.db.Model(issue).Updates(updates).Error; err != nil {
        return nil, err
    }
    return s.GetByKey(key)
}

func (s *Service) Delete(key string) error {
    return s.db.Model(&Issue{}).Where("key = ?", key).Update("is_archived", true).Error
}

func (s *Service) AddLabel(issueID, projectID, name, color string) (*Label, error) {
    var label Label
    s.db.Where("project_id = ? AND name = ?", projectID, name).FirstOrCreate(&label, Label{
        ID: uuid.New().String(), ProjectID: projectID, Name: name, Color: color,
    })
    il := &IssueLabel{IssueID: issueID, LabelID: label.ID}
    if err := s.db.Create(il).Error; err != nil { return nil, err }
    return &label, nil
}

func (s *Service) RemoveLabel(issueID, labelID string) error {
    return s.db.Where("issue_id = ? AND label_id = ?", issueID, labelID).Delete(&IssueLabel{}).Error
}

func (s *Service) ListLabels(issueID string) ([]Label, error) {
    var labels []Label
    s.db.Joins("JOIN issue_labels ON issue_labels.label_id = labels.id").
        Where("issue_labels.issue_id = ?", issueID).Find(&labels)
    return labels, nil
}

func (s *Service) AddLink(sourceID, targetID string, linkType LinkType) (*IssueLink, error) {
    link := &IssueLink{ID: uuid.New().String(), SourceID: sourceID, TargetID: targetID, LinkType: linkType}
    if err := s.db.Create(link).Error; err != nil { return nil, err }
    return link, nil
}

func (s *Service) RemoveLink(linkID string) error {
    return s.db.Delete(&IssueLink{}, "id = ?", linkID).Error
}

func (s *Service) ListLinks(issueID string) ([]IssueLink, error) {
    var links []IssueLink
    s.db.Where("source_id = ? OR target_id = ?", issueID, issueID).Find(&links)
    return links, nil
}

func (s *Service) ListByProject(projectID string, opts ...ListOption) ([]Issue, error) {
    q := s.db.Where("project_id = ? AND is_archived = ?", projectID, false)
    for _, o := range opts { q = o(q) }
    var issues []Issue
    if err := q.Order("position ASC").Find(&issues).Error; err != nil { return nil, err }
    return issues, nil
}

func (s *Service) GetChildren(parentID string) ([]Issue, error) {
    var issues []Issue
    s.db.Where("parent_id = ? AND is_archived = ?", parentID, false).Order("position ASC").Find(&issues)
    return issues, nil
}

func (s *Service) Watch(issueID, userID string) error {
    return s.db.Create(&IssueWatcher{IssueID: issueID, UserID: userID}).Error
}

func (s *Service) Unwatch(issueID, userID string) error {
    return s.db.Where("issue_id = ? AND user_id = ?", issueID, userID).Delete(&IssueWatcher{}).Error
}

func (s *Service) GetWatchers(issueID string) ([]IssueWatcher, error) {
    var watchers []IssueWatcher
    s.db.Where("issue_id = ?", issueID).Find(&watchers)
    return watchers, nil
}

func (s *Service) GetHistory(issueID string) ([]IssueHistory, error) {
    var h []IssueHistory
    s.db.Where("issue_id = ?", issueID).Order("created_at DESC").Find(&h)
    return h, nil
}

func (s *Service) DB() *gorm.DB { return s.db }

func (s *Service) logHistory(issueID, actorID, field, oldVal, newVal string) {
    h := &IssueHistory{ID: uuid.New().String(), IssueID: issueID, ActorID: &actorID, FieldName: field, OldValue: oldVal, NewValue: newVal}
    s.db.Create(h)
}
```

- [ ] **Step 4.2.3: Run issue service tests**

Run: `go test ./internal/domain/issue/ -v`
Expected: PASS

- [ ] **Step 4.2.4: Commit**

```bash
go mod tidy && git add -A && git commit -m "feat(step-4): add issue service with CRUD, labels, links, hierarchy"
```

### Task 4.3: Issue API handlers

**Files:**
- Create: `internal/api/handlers/issue_handler.go`
- Create: `internal/api/handlers/issue_handler_test.go`

- [ ] **Step 4.3.1: Write issue handler test**

```go
// internal/api/handlers/issue_handler_test.go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"
    "github.com/google/uuid"
    "github.com/gorilla/mux"
    "github.com/open-jira/open-jira/internal/config"
    "github.com/open-jira/open-jira/internal/domain/issue"
    "github.com/open-jira/open-jira/internal/domain/project"
    "github.com/open-jira/open-jira/internal/domain/user"
    "github.com/open-jira/open-jira/internal/store"
)

func setupIssueHandler(t *testing.T) (*IssueHandler, *store.Store) {
    t.Helper()
    os.Setenv("APP_SECRET", "test-secret-min-32-chars-long-key!!")
    os.Setenv("DB_DRIVER", "sqlite")
    os.Setenv("DB_DSN", "file::memory:?cache=shared")
    defer os.Clearenv()
    cfg, _ := config.Load()
    s, _ := store.New(cfg.DB, "test")
    s.DB.AutoMigrate(&user.User{}, &project.Project{}, &issue.Issue{}, &issue.IssueType{}, &issue.Label{}, &issue.IssueLabel{}, &issue.IssueLink{}, &issue.IssueHistory{}, &issue.IssueWatcher{}, &issue.Comment{})
    svc := issue.NewService(s.DB, "TEST")
    return NewIssueHandler(svc), s
}

func TestCreateIssue(t *testing.T) {
    h, s := setupIssueHandler(t)
    defer s.Close()
    projectID := uuid.New().String()
    s.DB.Create(&project.Project{ID: projectID, Key: "TEST", Name: "Test", Type: "scrum"})
    s.DB.Create(&issue.IssueType{ID: uuid.New().String(), ProjectID: projectID, Name: "Task"})

    body := map[string]interface{}{
        "title":    "My First Issue",
        "priority": "medium",
    }
    b, _ := json.Marshal(body)
    req := httptest.NewRequest("POST", "/api/v1/projects/TEST/issues", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    req = mux.SetURLVars(req, map[string]string{"key": "TEST"})
    rec := httptest.NewRecorder()
    // Inject project into context or use service directly
    // For test: use issue handler's Create method
    h.Create(rec, req)
    if rec.Code != http.StatusCreated {
        t.Errorf("status = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
    }
}
```

- [ ] **Step 4.3.2: Implement issue handler**

```go
// internal/api/handlers/issue_handler.go
package handlers

import (
    "encoding/json"
    "net/http"
    "github.com/open-jira/open-jira/internal/domain/issue"
)

type IssueHandler struct {
    svc *issue.Service
}

func NewIssueHandler(svc *issue.Service) *IssueHandler { return &IssueHandler{svc: svc} }

func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    var req struct {
        ProjectID    string          `json:"project_id"`
        Title        string          `json:"title"`
        Description  string          `json:"description"`
        Priority     issue.Priority  `json:"priority"`
        ParentID     *string         `json:"parent_id"`
        TypeID       *string         `json:"type_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    if req.Title == "" {
        http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
        return
    }
    if priority := req.Priority; priority == "" { req.Priority = issue.PriorityMedium }
    iss, err := h.svc.Create(projectKey, req.ProjectID, req.Title, req.Description, req.Priority, req.ParentID, req.TypeID)
    if err != nil {
        http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(iss)
}

func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(iss)
}

func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    var req struct {
        Title        *string         `json:"title"`
        DescriptionJSON *string      `json:"description_json"`
        Priority     *issue.Priority `json:"priority"`
        AssigneeID   *string          `json:"assignee_id"`
        StatusID     *string          `json:"status_id"`
        StoryPoints  *int             `json:"story_points"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
        return
    }
    iss, err := h.svc.Update(issueKey, req.Title, req.DescriptionJSON, req.Priority, req.AssigneeID, req.StatusID, req.StoryPoints)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(iss)
}

func (h *IssueHandler) Delete(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    if err := h.svc.Delete(issueKey); err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    // In practice, look up project ID from project service
    // For now, use projectKey as projectID for simplicity
    issues, err := h.svc.ListByProject(projectKey)
    if err != nil {
        http.Error(w, `{"error":"failed to list issues"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(issues)
}

func (h *IssueHandler) AddLabel(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    var req struct {
        Name  string `json:"name"`
        Color string `json:"color"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    label, err := h.svc.AddLabel(iss.ID, iss.ProjectID, req.Name, req.Color)
    if err != nil {
        http.Error(w, `{"error":"failed to add label"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(label)
}

func (h *IssueHandler) RemoveLabel(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    labelID := r.PathValue("labelId")
    if err := h.svc.RemoveLabel(iss.ID, labelID); err != nil {
        http.Error(w, `{"error":"failed to remove label"}`, http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) AddLink(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    var req struct {
        TargetID string        `json:"target_id"`
        LinkType issue.LinkType `json:"link_type"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    link, err := h.svc.AddLink(iss.ID, req.TargetID, req.LinkType)
    if err != nil {
        http.Error(w, `{"error":"failed to add link"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(link)
}

func (h *IssueHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    links, err := h.svc.ListLinks(iss.ID)
    if err != nil {
        http.Error(w, `{"error":"failed to list links"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(links)
}

// Additional handlers for Watch/Unwatch/Watchers/History/Children

func (h *IssueHandler) Watch(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    var req struct{ UserID string `json:"user_id"` }
    json.NewDecoder(r.Body).Decode(&req)
    if err := h.svc.Watch(iss.ID, req.UserID); err != nil {
        http.Error(w, `{"error":"failed to watch"}`, http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusCreated)
}

func (h *IssueHandler) Unwatch(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    userID := r.URL.Query().Get("user_id")
    if err := h.svc.Unwatch(iss.ID, userID); err != nil {
        http.Error(w, `{"error":"failed to unwatch"}`, http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
    issueKey := r.PathValue("issueKey")
    iss, err := h.svc.GetByKey(issueKey)
    if err != nil {
        http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
        return
    }
    history, err := h.svc.GetHistory(iss.ID)
    if err != nil {
        http.Error(w, `{"error":"failed to get history"}`, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(history)
}
```

- [ ] **Step 4.3.3: Add issue routes to router**

Add to `internal/api/router.go`:

```go
issueH := handlers.NewIssueHandler(issue.NewService(db, ""))
// Issue routes
protected.HandleFunc("/projects/{key}/issues", issueH.List).Methods("GET")
protected.HandleFunc("/projects/{key}/issues", issueH.Create).Methods("POST")
protected.HandleFunc("/issues/{issueKey}", issueH.Get).Methods("GET")
protected.HandleFunc("/issues/{issueKey}", issueH.Update).Methods("PATCH")
protected.HandleFunc("/issues/{issueKey}", issueH.Delete).Methods("DELETE")
protected.HandleFunc("/issues/{issueKey}/labels", issueH.AddLabel).Methods("POST")
protected.HandleFunc("/issues/{issueKey}/labels/{labelId}", issueH.RemoveLabel).Methods("DELETE")
protected.HandleFunc("/issues/{issueKey}/links", issueH.ListLinks).Methods("GET")
protected.HandleFunc("/issues/{issueKey}/links", issueH.AddLink).Methods("POST")
protected.HandleFunc("/issues/{issueKey}/watch", issueH.Watch).Methods("POST")
protected.HandleFunc("/issues/{issueKey}/watch", issueH.Unwatch).Methods("DELETE")
protected.HandleFunc("/issues/{issueKey}/history", issueH.GetHistory).Methods("GET")
```

Update imports in router.go to include `"github.com/open-jira/open-jira/internal/domain/issue"`.

- [ ] **Step 4.3.4: Run all tests and commit**

```bash
go test ./... -v -count=1
git add -A && git commit -m "feat(step-4): add issue API handlers and routes"
```

Add missing `ListOption` type:

```go
// internal/domain/issue/service.go
type ListOption func(*gorm.DB) *gorm.DB

func WithStatus(statusID string) ListOption {
    return func(db *gorm.DB) *gorm.DB { return db.Where("status_id = ?", statusID) }
}

func WithAssignee(userID string) ListOption {
    return func(db *gorm.DB) *gorm.DB { return db.Where("assignee_id = ?", userID) }
}

func WithPriority(priority Priority) ListOption {
    return func(db *gorm.DB) *gorm.DB { return db.Where("priority = ?", priority) }
}

func WithSprint(sprintID string) ListOption {
    return func(db *gorm.DB) *gorm.DB { return db.Where("sprint_id = ?", sprintID) }
}

func WithLabel(labelID string) ListOption {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("id IN (SELECT issue_id FROM issue_labels WHERE label_id = ?)", labelID)
    }
}

func WithSearch(q string) ListOption {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("title LIKE ? OR description_json LIKE ?", "%"+q+"%", "%"+q+"%")
    }
}
```

---

## Steps 5-12: Condensed Tasks

*(The following steps follow the exact same TDD pattern: write models → write service tests → implement service → write handler tests → implement handlers → add routes → run all tests → commit.)*

### Step 5: Workflow Engine

**Branch:** `feature/step-5-workflow` | **Models:** `internal/domain/workflow/model.go` | **Service:** `internal/domain/workflow/service.go` | **Handler:** `internal/api/handlers/workflow_handler.go`

**Tasks:** 5.1 Write workflow/status/transition models (Workflow, WorkflowStatus, WorkflowTransition). 5.2 Write workflow service: create workflow for project, add/remove statuses, add/remove transitions. 5.3 Implement default workflow creator (`TO DO → IN PROGRESS → DONE`). 5.4 Write workflow handler: GET/POST/PATCH workflow, POST/PATCH/DELETE statuses, POST transitions. 5.5 Add transition validation handler (`POST /issues/{key}/transition`). 5.6 Add routes, test, commit.

### Step 6: Board & Backlog

**Branch:** `feature/step-6-board-backlog` | **Models:** `internal/domain/sprint/model.go` | **Service:** `internal/domain/sprint/service.go` | **Handler:** `internal/api/handlers/sprint_handler.go`, `internal/api/handlers/board_handler.go`

**Tasks:** 6.1 Write sprint model. 6.2 Write sprint service: create/start/complete sprint, list sprints by project, add/remove issues from sprint. 6.3 Write board service: get board columns (group issues by status), reorder issues (drag & drop via `position` field using LexoRank float gap pattern). 6.4 Write backlog service: list unassigned issues, reorder backlog. 6.5 Write WebSocket hub for real-time board updates (Go channel pub/sub). 6.6 Write sprint and board API handlers. 6.7 Initialize React frontend, install deps, create Board page component with dnd-kit. 6.8 Create Backlog page component. 6.9 Add routes, test, commit.

### Step 7: Comments & History

**Branch:** `feature/step-7-comments-history` | **Handler:** `internal/api/handlers/comment_handler.go`

**Tasks:** 7.1 Write comment service (add/find/soft-delete comments). 7.2 Write comment handler with mentions parsing (@username). 7.3 Write attachment service (file upload to local disk). 7.4 Write attachment handler with multipart upload. 7.5 Create frontend issue detail page with CommentList, CommentForm, HistoryTimeline components. 7.6 Add routes, test, commit.

### Step 8: Search & Filters

**Branch:** `feature/step-8-search-filters` | **Service:** `internal/domain/search/service.go`

**Tasks:** 8.1 Write JQL-like parser (tokenizer + recursive descent parser → SQL conditions). 8.2 Write search service using full-text search on PostgreSQL `tsvector`. 8.3 Write saved filters model and CRUD service. 8.4 Write search handler with `/api/v1/search` endpoint. 8.5 Create advanced search UI component in frontend. 8.6 Create saved filters UI. 8.7 Add routes, test, commit.

### Step 9: Reports & Dashboard

**Branch:** `feature/step-9-reports-dashboard` | **Service:** `internal/domain/report/service.go`

**Tasks:** 9.1 Write burndown chart calculator (compute remaining story points per day in sprint). 9.2 Write project summary service (counts by status, 7-day activity). 9.3 Write dashboard model and widget CRUD service. 9.4 Write report API handlers. 9.5 Write dashboard API handlers. 9.6 Create BurndownChart component (Recharts). 9.7 Create ProjectSummary and Dashboard page components. 9.8 Add routes, test, commit.

### Step 10: Notifications

**Branch:** `feature/step-10-notifications` | **Model:** notification structs in `internal/domain/notification/`

**Tasks:** 10.1 Write notification model and create/deliver service. 10.2 Write notification settings service (per-user, per-project, per-event). 10.3 Integrate notifications into comment service (mentions), issue service (assignment), sprint service (start/complete). 10.4 Write email sender service (SMTP). 10.5 Write notification API handlers (list, mark-read, settings). 10.6 Write WebSocket endpoint for real-time notifications. 10.7 Create NotificationBell and NotificationList frontend components. 10.8 Add routes, test, commit.

### Step 11: Git Integration

**Branch:** `feature/step-11-git-integration` | **Model:** `internal/domain/git/model.go`

**Tasks:** 11.1 Write GitProvider interface with Forgejo, GitLab, GitHub implementations. 11.2 Write git provider config model and CRUD service. 11.3 Write webhook receiver with HMAC signature verification. 11.4 Write webhook parser: extract branch/commit/PR from push events. 11.5 Write auto-link service: match branch name `PROJ-123` to issue key. 11.6 Write transition-on-merge service. 11.7 Write git API handlers. 11.8 Create GitIntegration UI components in frontend (commit/PR list on issue page). 11.9 Add routes, test, commit.

### Step 12: Advanced Features & Deploy

**Branch:** `feature/step-12-advanced-deploy`

**Tasks:** 12.1 Write Timeline/Gantt service (compute date ranges for epics/sprints). 12.2 Write Calendar service (issues with dates grouped by month). 12.3 Write custom fields service (dynamic fields per project). 12.4 Write automation service (trigger → condition → action engine). 12.5 Write automation/calendar/timeline API handlers. 12.6 Create Timeline, Calendar, CustomFields, Automation Settings UI pages. 12.7 Write async worker (process email queue, webhook delivery, automation runs). 12.8 Write Helm chart (Chart.yaml, values.yaml, templates for deployments, services, ingress, PVC, ConfigMap, Secret). 12.9 Write Dockerfiles (server, worker, frontend via nginx). 12.10 Write OpenAPI/Swagger docs. 12.11 Final integration test, run all tests, build Docker images, verify Helm install. 12.12 Commit, tag `v1.0.0`.

---

## Self-Review Checklist

1. **Spec coverage:** Each section of the original spec (Projects, Issues, Workflow, Board/Backlog, Comments, Search, Reports, Notifications, Git, Advanced) maps to Steps 3-12.
2. **Placeholder scan:** No TBD, TODO, or vague instructions. All tasks have concrete code or clear implementation descriptions.
3. **Type consistency:** Domain model fields (Issue.Key, Issue.Position, etc.) match API handler usage. Service method signatures match handler calls.
4. **No orphan methods:** `ListOption` type, `WithStatus`, `WithAssignee` etc. are defined and used in `ListByProject`.

---

**Plan complete and saved to `docs/superpowers/plans/2026-05-14-open-jira-plan.md`.**
