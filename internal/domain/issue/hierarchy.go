package issue

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Issue hierarchy (Round 28). Heureum enforces Jira's strict three-level
// hierarchy: Epic (level 1) → Story/Task/Bug (level 0) → Subtask (level -1).
// The level lives on the issue type (IssueType.HierarchyLevel); an issue
// without a type counts as level 0. Every write path that sets parent_id —
// Create/CreateWithOptions and SetParent — goes through validateParent, so
// REST create/update, the agile epic moves and automation all obey the same
// rules.

// ErrParentNotFound: the requested parent does not exist or is archived.
// Handlers map it to 400 errors.parent (an unknown key is a client error,
// not a 404 of the issue being written).
var ErrParentNotFound = errors.New("parent issue not found")

// ErrParentInvalid: the parent exists but the link breaks the hierarchy. The
// returned error wraps this sentinel with the reason ("parent must be an
// Epic", "an Epic cannot have a parent", "parent must be in the same project",
// "parent would create a cycle"), so errors.Is works and the message says why.
var ErrParentInvalid = errors.New("invalid parent")

// EpicColorKeys are Jira's epic colour keys, in round-robin order. Later
// tasks validate client-supplied colours against this list.
var EpicColorKeys = []string{
	"color_1", "color_2", "color_3", "color_4", "color_5", "color_6", "color_7",
	"color_8", "color_9", "color_10", "color_11", "color_12", "color_13", "color_14",
}

// IsEpicColorKey reports whether key is one of EpicColorKeys.
func IsEpicColorKey(key string) bool {
	for _, k := range EpicColorKeys {
		if k == key {
			return true
		}
	}
	return false
}

// parentWalkLimit bounds the walk up parent_id in the cycle check. The
// hierarchy is three levels deep, so a chain longer than this is already
// corrupt; the bound only guarantees termination on a pre-existing loop.
const parentWalkLimit = 32

// levelForTypeName maps a standard type name to its hierarchy level: "epic"
// → 1, subtask names → -1, anything else → 0. It is the single rule used by
// TypeIDByName/SeedStandardTypes and mirrored by migration 000027's backfill.
func levelForTypeName(name string) int {
	switch strings.ToLower(name) {
	case "epic":
		return 1
	case "subtask", "sub-task":
		return -1
	}
	return 0
}

// typeLevel returns the hierarchy level of the issue type typeID, 0 when the
// issue has no type (nil) or the type row is gone (type_id is ON DELETE SET
// NULL, but be lenient anyway).
func typeLevel(tx *gorm.DB, typeID *string) (int, error) {
	if typeID == nil {
		return 0, nil
	}
	var it IssueType
	err := tx.Select("hierarchy_level").Where("id = ?", *typeID).First(&it).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return it.HierarchyLevel, nil
}

// validateParent checks that parentID is an acceptable parent for an issue of
// level childLevel in project projectID. childID is the issue being
// re-parented (SetParent) and may be "" on create, when no cycle is possible.
// Rules, in order:
//   - a level-1 issue (epic) never has a parent;
//   - the parent exists and is not archived (ErrParentNotFound);
//   - the parent is in the same project;
//   - the parent's level is exactly childLevel + 1;
//   - walking parent_id upwards from the parent never reaches childID.
//
// It runs on the caller's transaction so create and SetParent validate against
// the same snapshot they write in.
func validateParent(tx *gorm.DB, projectID string, childLevel int, childID, parentID string) error {
	if childLevel >= 1 {
		return fmt.Errorf("%w: an Epic cannot have a parent", ErrParentInvalid)
	}
	if childID != "" && parentID == childID {
		return fmt.Errorf("%w: an issue cannot be its own parent", ErrParentInvalid)
	}
	var parent Issue
	err := tx.Where("id = ?", parentID).First(&parent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrParentNotFound
	}
	if err != nil {
		return err
	}
	if parent.IsArchived {
		return ErrParentNotFound
	}
	if parent.ProjectID != projectID {
		return fmt.Errorf("%w: parent must be in the same project", ErrParentInvalid)
	}
	parentLevel, err := typeLevel(tx, parent.TypeID)
	if err != nil {
		return err
	}
	if parentLevel != childLevel+1 {
		switch childLevel {
		case 0:
			return fmt.Errorf("%w: parent must be an Epic", ErrParentInvalid)
		default:
			return fmt.Errorf("%w: parent of a subtask must be a Story, Task or Bug", ErrParentInvalid)
		}
	}
	if childID != "" {
		cur := parent.ParentID
		for hops := 0; cur != nil && hops < parentWalkLimit; hops++ {
			if *cur == childID {
				return fmt.Errorf("%w: parent would create a cycle", ErrParentInvalid)
			}
			var next struct{ ParentID *string }
			if err := tx.Model(&Issue{}).Select("parent_id").Where("id = ?", *cur).Scan(&next).Error; err != nil {
				return err
			}
			cur = next.ParentID
		}
	}
	return nil
}

// SetParent moves the issue identified by key under parentID, or clears its
// parent when parentID is nil, after validateParent. The new parent_id is
// written as SQL NULL when cleared. A change is logged in issue_history as
// field "parent" with the old and new parent KEYS (what the changelog shows;
// "" when none) and emits issue_updated; setting the parent it already has is
// a no-op. Handlers resolve the parent's key/id to its internal id before
// calling (GetByKey/GetBySeqID), as they do for every other reference.
func (s *Service) SetParent(key string, parentID *string) error {
	iss, err := s.GetByKey(key)
	if err != nil {
		return err
	}
	if parentID != nil && *parentID == "" {
		parentID = nil
	}
	same := (iss.ParentID == nil && parentID == nil) ||
		(iss.ParentID != nil && parentID != nil && *iss.ParentID == *parentID)
	if same {
		return nil
	}
	oldKey, newKey := "", ""
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if parentID != nil {
			childLevel, err := typeLevel(tx, iss.TypeID)
			if err != nil {
				return err
			}
			if err := validateParent(tx, iss.ProjectID, childLevel, iss.ID, *parentID); err != nil {
				return err
			}
			if err := tx.Model(&Issue{}).Select("key").Where("id = ?", *parentID).Scan(&newKey).Error; err != nil {
				return err
			}
		}
		if iss.ParentID != nil {
			// The old parent may be gone (ON DELETE SET NULL only runs on
			// hard deletes, which don't exist); an empty key is fine either way.
			if err := tx.Model(&Issue{}).Select("key").Where("id = ?", *iss.ParentID).Scan(&oldKey).Error; err != nil {
				return err
			}
		}
		return tx.Model(&Issue{}).Where("id = ?", iss.ID).Update("parent_id", parentID).Error
	})
	if err != nil {
		return err
	}
	s.logHistory(iss.ID, "", "parent", oldKey, newKey)
	if updated, err := s.GetByKey(key); err == nil {
		s.emit("issue_updated", updated)
	}
	return nil
}

// countEpics returns how many level-1 issues the project has (archived
// included: the index only feeds the round-robin and must not shift when an
// epic is archived).
func countEpics(tx *gorm.DB, projectID string) (int64, error) {
	var n int64
	err := tx.Model(&Issue{}).
		Joins("JOIN issue_types ON issue_types.id = issues.type_id").
		Where("issues.project_id = ? AND issue_types.hierarchy_level = 1", projectID).
		Count(&n).Error
	return n, err
}

// nextEpicColor picks the colour for the project's next epic: the (count of
// existing epics mod 14)+1-th key, inside the create transaction so two epics
// created back to back get different colours.
func nextEpicColor(tx *gorm.DB, projectID string) (string, error) {
	n, err := countEpics(tx, projectID)
	if err != nil {
		return "", err
	}
	return EpicColorKeys[int(n%int64(len(EpicColorKeys)))], nil
}

// BackfillEpicColors gives a colour to every level-1 issue still without one
// (epics created before migration 000027), round-robin per project continuing
// after the project's already-coloured epics, in seq_id order. Called from
// cmd/server after migrations. Idempotent: a coloured epic is never touched
// and a second run finds nothing to do. Returns the number of issues updated.
func (s *Service) BackfillEpicColors() (int, error) {
	type row struct {
		ID        string
		ProjectID string
	}
	var pending []row
	if err := s.db.Model(&Issue{}).
		Select("issues.id AS id", "issues.project_id AS project_id").
		Joins("JOIN issue_types ON issue_types.id = issues.type_id").
		Where("issue_types.hierarchy_level = 1 AND issues.epic_color IS NULL").
		Order("issues.project_id ASC, issues.seq_id ASC").
		Scan(&pending).Error; err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}
	updated := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		offsets := map[string]int64{}
		for _, r := range pending {
			next, seen := offsets[r.ProjectID]
			if !seen {
				var n int64
				if err := tx.Model(&Issue{}).
					Joins("JOIN issue_types ON issue_types.id = issues.type_id").
					Where("issues.project_id = ? AND issue_types.hierarchy_level = 1 AND issues.epic_color IS NOT NULL", r.ProjectID).
					Count(&n).Error; err != nil {
					return err
				}
				next = n
			}
			color := EpicColorKeys[int(next%int64(len(EpicColorKeys)))]
			res := tx.Model(&Issue{}).Where("id = ? AND epic_color IS NULL", r.ID).Update("epic_color", color)
			if res.Error != nil {
				return res.Error
			}
			updated += int(res.RowsAffected)
			offsets[r.ProjectID] = next + 1
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("backfill epic colours: %w", err)
	}
	return updated, nil
}

// EpicProgress counts the epic's direct, non-archived children (total) and
// how many of them sit in a done-category status (done). Children without a
// status count in total only. Grandchildren (subtasks of the stories) are not
// counted: progress is over the level below the epic, as in Jira.
func (s *Service) EpicProgress(epicID string) (done, total int, err error) {
	var row struct {
		Total int64
		Done  int64
	}
	err = s.db.Model(&Issue{}).
		Select("COUNT(*) AS total", "COALESCE(SUM(CASE WHEN workflow_statuses.category = 'done' THEN 1 ELSE 0 END), 0) AS done").
		Joins("LEFT JOIN workflow_statuses ON workflow_statuses.id = issues.status_id").
		Where("issues.parent_id = ? AND issues.is_archived = ?", epicID, false).
		Scan(&row).Error
	if err != nil {
		return 0, 0, err
	}
	return int(row.Done), int(row.Total), nil
}

// ErrInvalidEpicColor: the colour key is not one of EpicColorKeys.
var ErrInvalidEpicColor = errors.New("invalid epic colour")

// SetEpicColor sets the epic colour of the issue identified by key to one of
// EpicColorKeys (ErrInvalidEpicColor otherwise) and logs the change in
// issue_history as field "epic_color". The caller has already checked that
// the issue is an epic (level 1): the colour column exists on every issue,
// so nothing here depends on the type.
func (s *Service) SetEpicColor(key, colorKey string) error {
	if !IsEpicColorKey(colorKey) {
		return fmt.Errorf("%w: %q", ErrInvalidEpicColor, colorKey)
	}
	iss, err := s.GetByKey(key)
	if err != nil {
		return err
	}
	old := ""
	if iss.EpicColor != nil {
		old = *iss.EpicColor
	}
	if old == colorKey {
		return nil
	}
	if err := s.db.Model(&Issue{}).Where("id = ?", iss.ID).Update("epic_color", colorKey).Error; err != nil {
		return err
	}
	s.logHistory(iss.ID, "", "epic_color", old, colorKey)
	return nil
}
