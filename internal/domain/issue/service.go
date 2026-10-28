package issue

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/adf"
)

type Service struct {
	db        *gorm.DB
	notifier  Notifier
	eventSink EventSink
}

type Notifier interface {
	NotifyIssueAssigned(issueID, assigneeUserID, issueKey, issueTitle string) error
	NotifyStatusChanged(issueID, newStatus, issueKey, issueTitle, actorUserID string) error
}

// EventSink riceve eventi di dominio sulle issue (per integrazioni: webhook, automation).
type EventSink interface {
	IssueEvent(eventType string, iss *Issue)
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

func (s *Service) SetEventSink(e EventSink) { s.eventSink = e }

func (s *Service) emit(eventType string, iss *Issue) {
	if s.eventSink != nil {
		s.eventSink.IssueEvent(eventType, iss)
	}
}

// ErrProjectNotFound is returned by Create when the project the issue should
// belong to does not exist (no key can be allocated for it).
var ErrProjectNotFound = errors.New("project not found")

// seqIDRetries bounds the retry loop around the global seq_id allocation in
// Create. seq_id is MAX(seq_id)+1 computed inside the create transaction:
// two creates in the SAME project are serialized by the row lock the counter
// UPDATE takes on projects, so they never collide; two creates in DIFFERENT
// projects hold different row locks and can still both read the same MAX on
// Postgres/MySQL. The loser fails on issues.seq_id's unique index and its
// transaction is rolled back (counter included), so it is simply retried.
const seqIDRetries = 5

// ErrAssigneeNotFound is returned by CreateWithOptions when
// CreateOptions.AssigneeID is not a user: a client error, not an FK
// violation surfacing as an internal failure.
var ErrAssigneeNotFound = errors.New("assignee not found")

// CreateOptions carries the create-time fields that used to be applied by
// the handler as separate UPDATEs after the insert — each with its error
// discarded, so a failure left a half-initialized issue (no status, no
// assignee, missing labels) that the client was told was created. They are
// now applied inside the create transaction: either everything is there, or
// no issue exists.
type CreateOptions struct {
	// StatusID is the initial workflow status (the project's TO DO).
	StatusID *string
	// AssigneeID is the initial assignee; must be an existing user.
	AssigneeID *string
	// Labels are attached by name (reusing/creating the project's label).
	// Empty names and duplicates are ignored.
	Labels []string
	// Extra, if set, runs inside the create transaction right after the
	// issue row and its labels are written, for writes that belong to other
	// domains (the fix-versions pivot). Returning an error rolls the whole
	// create back.
	Extra func(tx *gorm.DB, iss *Issue) error
}

// Create allocates the issue key and seq_id and inserts the row in ONE
// transaction. The key number comes from projects.issue_counter: the UPDATE
// that increments it row-locks the project for the rest of the transaction
// (Postgres/MySQL; SQLite is single-writer), so concurrent creates in one
// project are serialized and a failed insert rolls the counter back together
// with the row. Every error is propagated; nothing falls back to a guessed
// number.
func (s *Service) Create(projectKey, projectID, title, description string, priority Priority, parentID *string, typeID *string) (*Issue, error) {
	return s.CreateWithOptions(projectKey, projectID, title, description, priority, parentID, typeID, CreateOptions{})
}

// CreateWithOptions is Create plus the create-time follow-ups in
// CreateOptions, all in the same transaction (see CreateOptions). The initial
// status and assignee are part of the row itself, so they produce no
// "changed from empty" history entries, no transition event and no
// status-change notification — only the "created" entry and the
// issue_created event, as for any other create. The assignee IS notified of
// the assignment, after the commit.
func (s *Service) CreateWithOptions(projectKey, projectID, title, description string, priority Priority, parentID *string, typeID *string, opts CreateOptions) (*Issue, error) {
	if title == "" {
		return nil, errors.New("title is required")
	}
	// Nullable FKs take SQL NULL, never "" (anti-pattern #2).
	if opts.StatusID != nil && *opts.StatusID == "" {
		opts.StatusID = nil
	}
	if opts.AssigneeID != nil && *opts.AssigneeID == "" {
		opts.AssigneeID = nil
	}
	var issue *Issue
	var err error
	for attempt := 0; attempt < seqIDRetries; attempt++ {
		issue, err = s.createOnce(projectKey, projectID, title, description, priority, parentID, typeID, opts)
		if err == nil || !isDuplicateKeyErr(err) {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	s.logHistory(issue.ID, "", "created", "", issue.Key)
	if issue.AssigneeID != nil && s.notifier != nil {
		s.notifier.NotifyIssueAssigned(issue.ID, *issue.AssigneeID, issue.Key, issue.Title)
	}
	s.emit("issue_created", issue)
	return issue, nil
}

func (s *Service) createOnce(projectKey, projectID, title, description string, priority Priority, parentID *string, typeID *string, opts CreateOptions) (*Issue, error) {
	var issue *Issue
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if opts.AssigneeID != nil {
			var n int64
			if err := tx.Table("users").Where("id = ?", *opts.AssigneeID).Count(&n).Error; err != nil {
				return err
			}
			if n == 0 {
				return ErrAssigneeNotFound
			}
		}
		n, err := nextKeyNumber(tx, projectID, projectKey)
		if err != nil {
			return err
		}
		seqID, err := nextSeqID(tx)
		if err != nil {
			return err
		}
		issue = &Issue{
			ID:              uuid.New().String(),
			ProjectID:       projectID,
			Key:             fmt.Sprintf("%s-%d", projectKey, n),
			Title:           title,
			DescriptionJSON: descriptionJSONForStorage(description),
			Priority:        priority,
			ParentID:        parentID,
			TypeID:          typeID,
			StatusID:        opts.StatusID,
			AssigneeID:      opts.AssigneeID,
			Position:        float64(n * 1000),
			SeqID:           seqID,
		}
		if err := tx.Create(issue).Error; err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, name := range opts.Labels {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			if _, err := addLabel(tx, issue.ID, projectID, name, ""); err != nil {
				return err
			}
		}
		if opts.Extra != nil {
			return opts.Extra(tx, issue)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return issue, nil
}

// nextKeyNumber increments projects.issue_counter inside tx and returns the
// new value. One implementation for all drivers: UPDATE then SELECT in the
// same transaction (GORM has no portable RETURNING; the UPDATE's row lock
// makes the SELECT read exactly the value this transaction wrote).
//
// If the resulting key already exists the counter is behind the data — rows
// written without Create (fixtures, raw inserts, pre-000026 projects not yet
// backfilled) — so it is re-synced from the highest existing key number and
// the allocation repeated once. This is checked up front rather than by
// catching the insert's unique violation because Postgres aborts the whole
// transaction on the first failed statement.
func nextKeyNumber(tx *gorm.DB, projectID, projectKey string) (int64, error) {
	n, err := incrementCounter(tx, projectID)
	if err != nil {
		return 0, err
	}
	taken, err := keyExists(tx, fmt.Sprintf("%s-%d", projectKey, n))
	if err != nil {
		return 0, err
	}
	if !taken {
		return n, nil
	}
	maxN, err := maxKeyNumber(tx, projectID, projectKey)
	if err != nil {
		return 0, err
	}
	if maxN < n {
		maxN = n
	}
	if err := tx.Exec("UPDATE projects SET issue_counter = ? WHERE id = ?", maxN, projectID).Error; err != nil {
		return 0, err
	}
	return incrementCounter(tx, projectID)
}

func incrementCounter(tx *gorm.DB, projectID string) (int64, error) {
	res := tx.Exec("UPDATE projects SET issue_counter = issue_counter + 1 WHERE id = ?", projectID)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, ErrProjectNotFound
	}
	var n int64
	if err := tx.Raw("SELECT issue_counter FROM projects WHERE id = ?", projectID).Scan(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func keyExists(tx *gorm.DB, key string) (bool, error) {
	var count int64
	if err := tx.Model(&Issue{}).Where("key = ?", key).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// maxKeyNumber returns the highest numeric suffix among the project's keys of
// the form "<projectKey>-<n>". Parsed in Go: there is no numeric key column
// and no SQL spelling of "digits after the last dash" that works on all three
// drivers. Keys with a non-numeric suffix are ignored.
func maxKeyNumber(tx *gorm.DB, projectID, projectKey string) (int64, error) {
	var keys []string
	if err := tx.Model(&Issue{}).Where("project_id = ?", projectID).Pluck("key", &keys).Error; err != nil {
		return 0, err
	}
	prefix := projectKey + "-"
	var maxN int64
	for _, k := range keys {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		n, err := strconv.ParseInt(k[len(prefix):], 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	return maxN, nil
}

// BackfillCounters initializes projects.issue_counter for projects created
// before migration 000026 (counter still 0) that already have issues, from
// the highest numeric key suffix, so their key sequence continues where it
// left off. Called from cmd/server after migrations. Idempotent: a project
// with a non-zero counter is never touched, and a project with no issues keeps
// 0 (its first key is "<KEY>-1" either way). Returns the number of projects
// updated.
func (s *Service) BackfillCounters() (int, error) {
	type row struct {
		ID  string
		Key string
	}
	var projects []row
	// Select through GORM so "key" is quoted per dialect (reserved on MySQL).
	if err := s.db.Table("projects").Select("id", "key").Where("issue_counter = 0").Scan(&projects).Error; err != nil {
		return 0, err
	}
	updated := 0
	for _, p := range projects {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			maxN, err := maxKeyNumber(tx, p.ID, p.Key)
			if err != nil || maxN == 0 {
				return err
			}
			res := tx.Exec("UPDATE projects SET issue_counter = ? WHERE id = ? AND issue_counter = 0", maxN, p.ID)
			if res.Error != nil {
				return res.Error
			}
			updated += int(res.RowsAffected)
			return nil
		})
		if err != nil {
			return updated, fmt.Errorf("backfill issue counter for project %s: %w", p.Key, err)
		}
	}
	return updated, nil
}

// isDuplicateKeyErr recognizes a unique-constraint violation on the three
// supported drivers (GORM's TranslateError is not enabled in store.New).
func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || // sqlite
		strings.Contains(msg, "SQLSTATE 23505") || // postgres (pgx)
		strings.Contains(msg, "Error 1062") // mysql
}

// descriptionJSONForStorage normalizes the description Create receives into
// the format issues.description_json holds: an ADF document as valid JSON —
// the same thing Update stores. The handler marshals the request's ADF and
// passes the JSON string, which is stored as-is; plain text (cmd/seed,
// programmatic callers) is wrapped into an ADF doc via json.Marshal, never by
// formatting it into a JSON literal (which produced invalid JSON as soon as
// the text contained a quote — every ADF description created from the UI).
// An empty description keeps the schema default "{}", which the v3 mapping
// renders as "no description".
func descriptionJSONForStorage(description string) string {
	if description == "" {
		return "{}"
	}
	if strings.HasPrefix(strings.TrimSpace(description), "{") && json.Valid([]byte(description)) {
		return description
	}
	b, err := json.Marshal(adf.FromText(description))
	if err != nil {
		// adf.Node is plain data; Marshal cannot fail. Keep the invariant anyway.
		return "{}"
	}
	return string(b)
}

func (s *Service) GetByKey(key string) (*Issue, error) {
	var issue Issue
	if err := s.db.Where("key = ?", key).First(&issue).Error; err != nil {
		return nil, errors.New("issue not found")
	}
	return &issue, nil
}

// nextSeqID returns the next Jira-style numeric issue id (from 10000),
// computed inside the caller's transaction. See seqIDRetries for why MAX+1 is
// safe within a project and how cross-project collisions are handled.
func nextSeqID(tx *gorm.DB) (int64, error) {
	var max sql.NullInt64
	if err := tx.Model(&Issue{}).Select("COALESCE(MAX(seq_id), 9999)").Scan(&max).Error; err != nil {
		return 0, err
	}
	return max.Int64 + 1, nil
}

func (s *Service) GetBySeqID(id int64) (*Issue, error) {
	var i Issue
	if err := s.db.First(&i, "seq_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &i, nil
}

// GetLabels restituisce i nomi delle label associate a una issue.
func (s *Service) GetLabels(issueID string) ([]string, error) {
	var names []string
	err := s.db.Table("labels").
		Joins("JOIN issue_labels ON issue_labels.label_id = labels.id").
		Where("issue_labels.issue_id = ?", issueID).
		Pluck("labels.name", &names).Error
	return names, err
}

func (s *Service) Update(key string, title, descriptionJSON *string, priority *Priority, assigneeID, statusID *string, storyPoints *int) (*Issue, error) {
	issue, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	statusChanged := false
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
		if issue.AssigneeID != nil {
			old = *issue.AssigneeID
		}
		s.logHistory(issue.ID, "", "assignee", old, *assigneeID)
		// assignee_id is a nullable FK to users.id: the "unassigned" value is
		// SQL NULL, not "". Writing an empty string violates the FK on Postgres
		// (SQLite doesn't enforce FKs, which is why this only surfaced there).
		if *assigneeID == "" {
			updates["assignee_id"] = nil
		} else {
			updates["assignee_id"] = *assigneeID
		}
		if *assigneeID != "" && s.notifier != nil {
			s.notifier.NotifyIssueAssigned(issue.ID, *assigneeID, issue.Key, issue.Title)
		}
	}
	if statusID != nil {
		old := ""
		if issue.StatusID != nil {
			old = *issue.StatusID
		}
		s.logHistory(issue.ID, "", "status", old, *statusID)
		updates["status_id"] = *statusID
		if *statusID != old {
			statusChanged = true
			if s.notifier != nil {
				s.notifier.NotifyStatusChanged(issue.ID, *statusID, issue.Key, issue.Title, "")
			}
		}
	}
	if storyPoints != nil {
		s.logHistory(issue.ID, "", "story_points", fmt.Sprintf("%d", issue.StoryPoints), fmt.Sprintf("%d", *storyPoints))
		updates["story_points"] = *storyPoints
	}
	if err := s.db.Model(issue).Updates(updates).Error; err != nil {
		return nil, err
	}
	updated, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	s.emit("issue_updated", updated)
	if statusChanged {
		s.emit("issue_transitioned", updated)
	}
	return updated, nil
}

// SetTimeTracking aggiorna Issue.OriginalEstimate e/o Issue.RemainingEstimate
// (in secondi) per l'issue identificata da key. Ciascun parametro nil lascia
// il campo corrispondente invariato. Metodo dedicato invece di allargare la
// firma di Update, che ha già molti parametri e chiamanti.
func (s *Service) SetTimeTracking(key string, originalSeconds, remainingSeconds *int) (*Issue, error) {
	issue, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if originalSeconds != nil {
		s.logHistory(issue.ID, "", "original_estimate", fmt.Sprintf("%d", issue.OriginalEstimate), fmt.Sprintf("%d", *originalSeconds))
		updates["original_estimate"] = *originalSeconds
	}
	if remainingSeconds != nil {
		s.logHistory(issue.ID, "", "remaining_estimate", fmt.Sprintf("%d", issue.RemainingEstimate), fmt.Sprintf("%d", *remainingSeconds))
		updates["remaining_estimate"] = *remainingSeconds
	}
	if len(updates) == 0 {
		return issue, nil
	}
	if err := s.db.Model(issue).Updates(updates).Error; err != nil {
		return nil, err
	}
	updated, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	s.emit("issue_updated", updated)
	return updated, nil
}

func (s *Service) Delete(key string) error {
	return s.db.Model(&Issue{}).Where("key = ?", key).Update("is_archived", true).Error
}

func (s *Service) AddLabel(issueID, projectID, name, color string) (*Label, error) {
	return addLabel(s.db, issueID, projectID, name, color)
}

// addLabel attaches the project's label `name` (created on first use) to the
// issue, on the given handle so it can run inside the create transaction.
func addLabel(db *gorm.DB, issueID, projectID, name, color string) (*Label, error) {
	label := Label{ID: uuid.New().String(), ProjectID: projectID, Name: name, Color: color}
	if err := db.Where("project_id = ? AND name = ?", projectID, name).FirstOrCreate(&label).Error; err != nil {
		return nil, err
	}
	il := &IssueLabel{IssueID: issueID, LabelID: label.ID}
	if err := db.Create(il).Error; err != nil {
		return nil, err
	}
	return &label, nil
}

func (s *Service) RemoveLabel(issueID, labelID string) error {
	return s.db.Where("issue_id = ? AND label_id = ?", issueID, labelID).Delete(&IssueLabel{}).Error
}

// SetLabels riconcilia le label di una issue con l'elenco di nomi desiderato:
// aggiunge le nuove (riusando/creando la Label del progetto per nome) e
// rimuove quelle non più presenti. Usata da PUT /rest/api/3/issue/{key}.
func (s *Service) SetLabels(issueID, projectID string, names []string) error {
	current, err := s.GetLabels(issueID)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, n := range names {
		if n != "" {
			want[n] = true
		}
	}
	have := map[string]bool{}
	for _, n := range current {
		have[n] = true
	}
	for _, n := range current {
		if !want[n] {
			var lbl Label
			if err := s.db.Where("project_id = ? AND name = ?", projectID, n).First(&lbl).Error; err != nil {
				continue
			}
			if err := s.RemoveLabel(issueID, lbl.ID); err != nil {
				return err
			}
		}
	}
	for n := range want {
		if !have[n] {
			if _, err := s.AddLabel(issueID, projectID, n, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) AddLink(sourceID, targetID string, linkType LinkType) (*IssueLink, error) {
	link := &IssueLink{ID: uuid.New().String(), SourceID: sourceID, TargetID: targetID, LinkType: linkType}
	if err := s.db.Create(link).Error; err != nil {
		return nil, err
	}
	return link, nil
}

func (s *Service) ListLinks(issueID string) ([]IssueLink, error) {
	var links []IssueLink
	s.db.Where("source_id = ? OR target_id = ?", issueID, issueID).Find(&links)
	return links, nil
}

func (s *Service) GetLink(linkID string) (*IssueLink, error) {
	var link IssueLink
	if err := s.db.Where("id = ?", linkID).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (s *Service) DeleteLink(linkID string) error {
	return s.db.Where("id = ?", linkID).Delete(&IssueLink{}).Error
}

func (s *Service) ListByProject(projectID string, opts ...ListOption) ([]Issue, error) {
	q := s.db.Where("project_id = ? AND is_archived = ?", projectID, false)
	for _, o := range opts {
		q = o(q)
	}
	var issues []Issue
	if err := q.Order("position ASC").Find(&issues).Error; err != nil {
		return nil, err
	}
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
	err := s.db.Where("issue_id = ?", issueID).Find(&watchers).Error
	return watchers, err
}

func (s *Service) GetHistory(issueID string) ([]IssueHistory, error) {
	var h []IssueHistory
	s.db.Where("issue_id = ?", issueID).Order("created_at DESC").Find(&h)
	return h, nil
}

// Rank riordina le issue indicate posizionandole tra afterID (posizione minore)
// e beforeID (posizione maggiore), usando la colonna Position (float, midpoint).
// Se manca un vicino, inserisce in coda/testa con passo fisso. afterID/beforeID
// sono id interni delle issue di riferimento (già risolti dal chiamante).
func (s *Service) Rank(issueIDs []string, beforeID, afterID *string) error {
	if len(issueIDs) == 0 {
		return nil
	}
	var lo, hi float64
	hasLo, hasHi := false, false
	if afterID != nil {
		var a Issue
		if err := s.db.First(&a, "id = ?", *afterID).Error; err != nil {
			return err
		}
		lo, hasLo = a.Position, true
	}
	if beforeID != nil {
		var b Issue
		if err := s.db.First(&b, "id = ?", *beforeID).Error; err != nil {
			return err
		}
		hi, hasHi = b.Position, true
	}
	n := float64(len(issueIDs))
	var base, step float64
	switch {
	case hasLo && hasHi:
		base = lo
		step = (hi - lo) / (n + 1)
	case hasLo:
		base = lo
		step = 1000
	case hasHi:
		base = hi - 1000*(n+1)
		step = 1000
	default:
		var maxPos float64
		if err := s.db.Model(&Issue{}).Select("COALESCE(MAX(position), 0)").Scan(&maxPos).Error; err != nil {
			return err
		}
		base = maxPos
		step = 1000
	}
	for i, id := range issueIDs {
		p := base + step*float64(i+1)
		if err := s.db.Model(&Issue{}).Where("id = ?", id).Update("position", p).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DB() *gorm.DB { return s.db }

// SetResolution imposta (o azzera, se resolutionID è nil) la resolution di una issue.
func (s *Service) SetResolution(key string, resolutionID *string) error {
	iss, err := s.GetByKey(key)
	if err != nil {
		return err
	}
	// Update esplicito su colonna: nil → NULL, valore → id.
	return s.db.Model(&Issue{}).Where("id = ?", iss.ID).Update("resolution_id", resolutionID).Error
}

// TypeIDByName restituisce l'id del tipo issue con quel nome nel progetto
// (case-insensitive), creando la riga al volo se non esiste ancora: le
// issue_types non vengono seedate di default per un progetto nuovo (solo
// cmd/seed inserisce una riga "Task" per il progetto demo), quindi senza
// questo fallback ogni issue creata passando issuetype.name invece di
// issuetype.id (il caso comune: la UI manda sempre il nome) risolverebbe a
// nessun tipo. Mirror del pattern di auto-vivificazione già usato da AddLabel.
func (s *Service) TypeIDByName(projectID, name string) (string, error) {
	var it IssueType
	err := s.db.Where("project_id = ? AND LOWER(name) = LOWER(?)", projectID, name).First(&it).Error
	if err == nil {
		return it.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	icon, isSubtask := "task", false
	switch strings.ToLower(name) {
	case "subtask", "sub-task":
		icon, isSubtask = "subtask", true
	case "bug":
		icon = "bug"
	case "story":
		icon = "story"
	case "epic":
		icon = "epic"
	}
	it = IssueType{ID: uuid.New().String(), ProjectID: projectID, Name: name, Icon: icon, IsSubtask: isSubtask}
	if err := s.db.Create(&it).Error; err != nil {
		return "", err
	}
	return it.ID, nil
}

// SeedStandardTypes crea (idempotente) un tipo issue per ciascun nome nel
// progetto, delegando a TypeIDByName — che fa get-or-create per nome e inferisce
// icon/isSubtask. Rieseguirlo con gli stessi nomi è un no-op. Ritorna il primo
// errore incontrato.
func (s *Service) SeedStandardTypes(projectID string, names []string) error {
	for _, name := range names {
		if _, err := s.TypeIDByName(projectID, name); err != nil {
			return err
		}
	}
	return nil
}

// EnsureResolution garantisce (idempotente) l'esistenza di una resolution GLOBALE
// con quel nome: se esiste già ritorna il suo id, altrimenti inserisce una riga
// (id uuid, name, description) in `resolutions` e ritorna il nuovo id. Mirror
// dell'insert "Done" di cmd/seed. La tabella resolutions non ha un model Go: vi
// si accede via db.Table("resolutions"), come ResolutionIDByName.
func (s *Service) EnsureResolution(name, description string) (string, error) {
	if id, ok := s.ResolutionIDByName(name); ok {
		return id, nil
	}
	id := uuid.New().String()
	if err := s.db.Table("resolutions").Create(map[string]any{
		"id":          id,
		"name":        name,
		"description": description,
	}).Error; err != nil {
		return "", err
	}
	return id, nil
}

// ResolutionIDByName restituisce l'id della resolution con quel nome (case-insensitive).
func (s *Service) ResolutionIDByName(name string) (string, bool) {
	var row struct{ ID string }
	err := s.db.Table("resolutions").Select("id").Where("LOWER(name) = LOWER(?)", name).Limit(1).Scan(&row).Error
	if err != nil || row.ID == "" {
		return "", false
	}
	return row.ID, true
}

func (s *Service) logHistory(issueID, actorID, field, oldVal, newVal string) {
	// actor_id is a nullable FK to users.id: a blank actor must be stored as
	// SQL NULL, not "". Every current caller passes "" (the authenticated user
	// id isn't threaded through Update yet), so without this every issue_history
	// insert would violate the FK on Postgres and the changelog would stay empty.
	var actorPtr *string
	if actorID != "" {
		actorPtr = &actorID
	}
	h := &IssueHistory{ID: uuid.New().String(), IssueID: issueID, ActorID: actorPtr, FieldName: field, OldValue: oldVal, NewValue: newVal}
	s.db.Create(h)
}

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

func WithNotArchived() ListOption {
	return func(db *gorm.DB) *gorm.DB { return db.Where("is_archived = ?", false) }
}

// ExportRow is a flattened, human-readable issue row for CSV export: the
// status/type/assignee foreign keys are resolved to their display names here
// (via LEFT JOINs) so the handler never leaks raw UUIDs. Unassigned or
// unresolved FKs come back as empty strings.
type ExportRow struct {
	Key         string
	Title       string
	Priority    string
	Status      string
	Type        string
	Assignee    string
	StoryPoints int
	Created     time.Time
	Updated     time.Time
}

// ExportRows returns every issue in the project (ordered by seq_id) with FK
// names resolved. Assignee falls back to email when display_name is blank.
func (s *Service) ExportRows(projectID string) ([]ExportRow, error) {
	var rows []ExportRow
	err := s.db.Raw(`
		SELECT i.key AS "key", i.title AS title, i.priority AS priority,
			COALESCE(ws.name, '') AS status,
			COALESCE(it.name, '') AS type,
			COALESCE(NULLIF(u.display_name, ''), u.email, '') AS assignee,
			i.story_points AS story_points,
			i.created_at AS created, i.updated_at AS updated
		FROM issues i
		LEFT JOIN workflow_statuses ws ON i.status_id = ws.id
		LEFT JOIN issue_types it ON i.type_id = it.id
		LEFT JOIN users u ON i.assignee_id = u.id
		WHERE i.project_id = ?
		ORDER BY i.seq_id ASC
	`, projectID).Scan(&rows).Error
	return rows, err
}
