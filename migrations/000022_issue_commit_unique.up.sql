CREATE UNIQUE INDEX idx_issue_commits_issue_sha ON issue_commits(issue_id, commit_sha);
