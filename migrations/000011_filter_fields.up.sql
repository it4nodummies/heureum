ALTER TABLE saved_filters ADD COLUMN description TEXT DEFAULT '';
ALTER TABLE saved_filters ADD COLUMN is_favourite BOOLEAN DEFAULT FALSE;
