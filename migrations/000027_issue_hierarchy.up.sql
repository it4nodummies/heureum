-- Issue hierarchy (Round 28): the level of an issue type in Jira's strict
-- hierarchy — 1 = Epic, 0 = Story/Task/Bug, -1 = Subtask. A child's parent must
-- be exactly one level above it and in the same project (enforced in
-- internal/domain/issue, validateParent). Existing rows are classified by
-- name / is_subtask, the same rule SeedStandardTypes and TypeIDByName apply.
ALTER TABLE issue_types ADD COLUMN hierarchy_level INTEGER NOT NULL DEFAULT 0;
UPDATE issue_types SET hierarchy_level = 1 WHERE lower(name) = 'epic';
UPDATE issue_types SET hierarchy_level = -1 WHERE is_subtask = true;

-- Epic colour, one of Jira's keys color_1..color_14, assigned round-robin per
-- project when a level-1 issue is created. NULL for every other issue and for
-- epics created before this migration: those are backfilled in Go at startup
-- (issue.Service.BackfillEpicColors, called from cmd/server after migrations;
-- idempotent — only NULL colours of level-1 issues are touched).
ALTER TABLE issues ADD COLUMN epic_color TEXT;
