CREATE TABLE board_columns (
    id TEXT PRIMARY KEY,
    board_id TEXT NOT NULL,
    name TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE board_column_statuses (
    column_id TEXT NOT NULL,
    status_id TEXT NOT NULL,
    PRIMARY KEY (column_id, status_id)
);

CREATE TABLE board_quick_filters (
    id TEXT PRIMARY KEY,
    board_id TEXT NOT NULL,
    name TEXT NOT NULL,
    jql TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0
);

ALTER TABLE boards ADD COLUMN swimlane_mode TEXT NOT NULL DEFAULT 'none';
