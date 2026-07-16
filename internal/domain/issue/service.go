package issue

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	seqID, err := s.nextSeqID()
	if err != nil {
		return nil, err
	}
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
		SeqID:           seqID,
	}
	if err := s.db.Create(issue).Error; err != nil {
		return nil, err
	}
	s.logHistory(issue.ID, "", "created", "", key)
	s.emit("issue_created", issue)
	return issue, nil
}

func (s *Service) GetByKey(key string) (*Issue, error) {
	var issue Issue
	if err := s.db.Where("key = ?", key).First(&issue).Error; err != nil {
		return nil, errors.New("issue not found")
	}
	return &issue, nil
}

func (s *Service) nextSeqID() (int64, error) {
	var max sql.NullInt64
	// nota: MAX+1 ha una race teorica sotto create concorrenti; accettabile a questa scala.
	if err := s.db.Model(&Issue{}).Select("COALESCE(MAX(seq_id), 9999)").Scan(&max).Error; err != nil {
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
		updates["assignee_id"] = *assigneeID
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

func (s *Service) Delete(key string) error {
	return s.db.Model(&Issue{}).Where("key = ?", key).Update("is_archived", true).Error
}

func (s *Service) AddLabel(issueID, projectID, name, color string) (*Label, error) {
	label := Label{ID: uuid.New().String(), ProjectID: projectID, Name: name, Color: color}
	s.db.Where("project_id = ? AND name = ?", projectID, name).FirstOrCreate(&label)
	il := &IssueLabel{IssueID: issueID, LabelID: label.ID}
	if err := s.db.Create(il).Error; err != nil {
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
	h := &IssueHistory{ID: uuid.New().String(), IssueID: issueID, ActorID: &actorID, FieldName: field, OldValue: oldVal, NewValue: newVal}
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
