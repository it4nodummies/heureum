CREATE TABLE boards (
    id TEXT PRIMARY KEY,
    seq_id INTEGER UNIQUE,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'scrum',
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    filter_id TEXT REFERENCES saved_filters(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE sprints ADD COLUMN seq_id INTEGER;
ALTER TABLE sprints ADD COLUMN origin_board_id INTEGER;
ALTER TABLE sprints ADD COLUMN complete_date TIMESTAMP;
