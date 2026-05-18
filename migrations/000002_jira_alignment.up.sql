CREATE TABLE IF NOT EXISTS project_categories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS resolutions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS components (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    lead_user_id TEXT REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE issues ADD COLUMN resolution_id TEXT REFERENCES resolutions(id) ON DELETE SET NULL;
