ALTER TABLE versions ADD COLUMN start_date TIMESTAMP;
ALTER TABLE versions ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE issue_versions (
    issue_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    PRIMARY KEY (issue_id, version_id)
);
