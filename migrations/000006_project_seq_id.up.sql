ALTER TABLE projects ADD COLUMN seq_id INTEGER;
CREATE UNIQUE INDEX idx_projects_seq_id ON projects(seq_id);
