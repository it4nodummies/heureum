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
	if s.db.Where("key = ?", key).First(&existing).Error == nil {
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

// CreateInput holds the parameters accepted by CreateProject.
type CreateInput struct {
	Key          string
	Name         string
	Description  string
	Type         Type
	LeadUserID   *string
	CategoryID   *string
	AssigneeType string
	URL          string
}

// CreateProject creates a project from a structured input, allowing callers
// to set fields (category, assignee type, URL, ...) not covered by Create.
func (s *Service) CreateProject(in CreateInput) (*Project, error) {
	if in.Key == "" || in.Name == "" {
		return nil, errors.New("key and name are required")
	}
	if in.Type == "" {
		in.Type = TypeScrum
	}
	if in.AssigneeType == "" {
		in.AssigneeType = "UNASSIGNED"
	}
	p := &Project{
		ID:           uuid.New().String(),
		Key:          strings.ToUpper(in.Key),
		Name:         in.Name,
		Description:  in.Description,
		Type:         in.Type,
		LeadUserID:   in.LeadUserID,
		CategoryID:   in.CategoryID,
		AssigneeType: in.AssigneeType,
		Style:        "classic",
		URL:          in.URL,
	}
	if err := s.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// Restore un-archives a project identified by key.
func (s *Service) Restore(key string) error {
	return s.db.Model(&Project{}).Where("key = ?", strings.ToUpper(key)).Update("is_archived", false).Error
}

// GetCategory fetches a ProjectCategory by ID, returning (nil, nil) if id is empty or not found.
func (s *Service) GetCategory(id string) (*ProjectCategory, error) {
	if id == "" {
		return nil, nil
	}
	var c ProjectCategory
	if err := s.db.First(&c, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
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

// ListWithFilters returns paginated, filtered, sorted projects enriched with lead info and starred status.
func (s *Service) ListWithFilters(f ListFilter, userID string) ([]ProjectWithLead, int64, error) {
	if f.MaxResults <= 0 {
		f.MaxResults = 20
	}

	query := s.db.Model(&Project{}).Where("is_archived = ?", false)

	if f.Search != "" {
		like := "%" + strings.ToLower(f.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(key) LIKE ?", like, like)
	}
	if len(f.Types) > 0 {
		query = query.Where("type IN ?", f.Types)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sorting
	sortCol := "name"
	switch f.SortKey {
	case "key":
		sortCol = "key"
	case "type":
		sortCol = "type"
	case "created_at", "created":
		sortCol = "created_at"
	}
	dir := "ASC"
	if strings.EqualFold(f.SortDir, "desc") {
		dir = "DESC"
	}
	query = query.Order("projects." + sortCol + " " + dir)

	// Pagination
	query = query.Offset(f.StartAt).Limit(f.MaxResults)

	var projects []Project
	if err := query.Find(&projects).Error; err != nil {
		return nil, 0, err
	}

	// Collect project IDs and lead IDs
	projectIDs := make([]string, 0, len(projects))
	leadIDs := make([]string, 0, len(projects))
	for _, p := range projects {
		projectIDs = append(projectIDs, p.ID)
		if p.LeadUserID != nil && *p.LeadUserID != "" {
			leadIDs = append(leadIDs, *p.LeadUserID)
		}
	}

	// Fetch lead users in one query
	leadMap := make(map[string]*LeadInfo)
	if len(leadIDs) > 0 {
		var users []user.User
		s.db.Where("id IN ?", leadIDs).Find(&users)
		for i := range users {
			u := users[i]
			leadMap[u.ID] = &LeadInfo{
				ID:          u.ID,
				DisplayName: u.DisplayName,
				AvatarURL:   u.AvatarURL,
				Email:       u.Email,
			}
		}
	}

	// Fetch starred status in one query
	starredSet := make(map[string]bool)
	if userID != "" && len(projectIDs) > 0 {
		var favs []ProjectFavorite
		s.db.Where("user_id = ? AND project_id IN ?", userID, projectIDs).Find(&favs)
		for _, fav := range favs {
			starredSet[fav.ProjectID] = true
		}
	}

	result := make([]ProjectWithLead, 0, len(projects))
	for _, p := range projects {
		pwl := ProjectWithLead{
			Project:   p,
			IsStarred: starredSet[p.ID],
		}
		if p.LeadUserID != nil {
			pwl.Lead = leadMap[*p.LeadUserID]
		}
		result = append(result, pwl)
	}
	return result, total, nil
}

// Star marks a project as favourite for userID.
func (s *Service) Star(projectID, userID string) error {
	fav := ProjectFavorite{UserID: userID, ProjectID: projectID}
	return s.db.Where(fav).FirstOrCreate(&fav).Error
}

// Unstar removes a project from a user's favourites.
func (s *Service) Unstar(projectID, userID string) error {
	return s.db.Where("user_id = ? AND project_id = ?", userID, projectID).Delete(&ProjectFavorite{}).Error
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
	return s.db.Model(&Project{}).Where("key = ?", strings.ToUpper(key)).Updates(map[string]interface{}{
		"is_archived": true,
	}).Error
}

func (s *Service) AddMember(projectID, userID string, role MemberRole) error {
	return s.db.Create(&ProjectMember{ProjectID: projectID, UserID: userID, Role: role}).Error
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

func (s *Service) DB() *gorm.DB { return s.db }
