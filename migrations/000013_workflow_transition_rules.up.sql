ALTER TABLE workflow_transitions ADD COLUMN name TEXT DEFAULT '';
ALTER TABLE workflow_transitions ADD COLUMN require_assignee BOOLEAN DEFAULT FALSE;
ALTER TABLE workflow_transitions ADD COLUMN set_resolution BOOLEAN DEFAULT FALSE;
