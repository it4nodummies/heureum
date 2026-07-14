ALTER TABLE sprints DROP COLUMN complete_date;
ALTER TABLE sprints DROP COLUMN origin_board_id;
ALTER TABLE sprints DROP COLUMN seq_id;
DROP TABLE IF EXISTS boards;
