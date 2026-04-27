ALTER TABLE listener_proxies ADD COLUMN IF NOT EXISTS web_status VARCHAR(30);
ALTER TABLE listener_proxies ADD COLUMN IF NOT EXISTS web_error VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_listener_proxies_web_status ON listener_proxies(web_status);
