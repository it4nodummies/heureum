ALTER TABLE webhook_deliveries DROP COLUMN payload;
ALTER TABLE webhook_deliveries DROP COLUMN next_attempt_at;
ALTER TABLE webhook_deliveries DROP COLUMN attempts;
ALTER TABLE webhook_deliveries DROP COLUMN status;
