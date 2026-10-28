ALTER TABLE git_providers ADD COLUMN webhook_url_token TEXT;
CREATE UNIQUE INDEX idx_git_providers_webhook_url_token ON git_providers(webhook_url_token);
