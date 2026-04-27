ALTER TABLE listener_proxies ADD COLUMN IF NOT EXISTS telegram_status VARCHAR(30);
ALTER TABLE listener_proxies ADD COLUMN IF NOT EXISTS telegram_error VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_listener_proxies_telegram_status ON listener_proxies(telegram_status);
