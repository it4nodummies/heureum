ALTER TABLE issues ADD COLUMN seq_id INTEGER;
CREATE UNIQUE INDEX idx_issues_seq_id ON issues(seq_id);
