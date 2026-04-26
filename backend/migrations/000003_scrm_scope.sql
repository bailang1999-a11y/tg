ALTER TABLE scrm_keyword_rules ADD COLUMN IF NOT EXISTS owner_user_id UUID;
ALTER TABLE scrm_keyword_rules ADD COLUMN IF NOT EXISTS monitor_group_id UUID;
CREATE INDEX IF NOT EXISTS idx_scrm_keyword_rules_owner_user_id ON scrm_keyword_rules(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_scrm_keyword_rules_monitor_group_id ON scrm_keyword_rules(monitor_group_id);

ALTER TABLE scrm_leads ADD COLUMN IF NOT EXISTS owner_user_id UUID;
ALTER TABLE scrm_leads ADD COLUMN IF NOT EXISTS source_task_id UUID;
CREATE INDEX IF NOT EXISTS idx_scrm_leads_owner_user_id ON scrm_leads(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_scrm_leads_source_task_id ON scrm_leads(source_task_id);
