DROP TABLE IF EXISTS issue_versions;
ALTER TABLE versions DROP COLUMN archived;
ALTER TABLE versions DROP COLUMN start_date;
