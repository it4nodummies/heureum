ALTER TABLE webhook_deliveries ADD COLUMN status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE webhook_deliveries ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE webhook_deliveries ADD COLUMN next_attempt_at TIMESTAMP;
ALTER TABLE webhook_deliveries ADD COLUMN payload TEXT NOT NULL DEFAULT '';
