CREATE TABLE issue_worklogs (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id TEXT,
    comment_json TEXT NOT NULL DEFAULT '{}',
    time_spent_seconds INTEGER NOT NULL DEFAULT 0,
    started TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_issue_worklogs_issue_id ON issue_worklogs(issue_id);
