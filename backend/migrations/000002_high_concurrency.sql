ALTER TABLE tasks ADD COLUMN IF NOT EXISTS run_id VARCHAR(80);
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS run_locked_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_tasks_run_id ON tasks(run_id);
CREATE INDEX IF NOT EXISTS idx_tasks_run_locked_at ON tasks(run_locked_at);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_created ON tasks(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_status_created ON tasks(tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_type_created ON tasks(tenant_id, type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_updated ON tasks(tenant_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_created ON task_logs(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_task_created ON task_logs(tenant_id, task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_level_created ON task_logs(tenant_id, level, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_category_created ON task_logs(tenant_id, category, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bot_subscribers_tenant_user ON bot_subscribers(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_bot_subscribers_tenant_status_updated ON bot_subscribers(tenant_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_bot_dm_tasks_tenant_subscriber_created ON bot_dm_tasks(tenant_id, subscriber_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bot_dm_tasks_tenant_status_created ON bot_dm_tasks(tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bot_private_account_groups_tenant_subscriber ON bot_private_account_groups(tenant_id, subscriber_id);
CREATE INDEX IF NOT EXISTS idx_bot_private_accounts_tenant_subscriber ON bot_private_accounts(tenant_id, subscriber_id);
CREATE INDEX IF NOT EXISTS idx_bot_private_uploads_tenant_subscriber ON bot_private_uploads(tenant_id, subscriber_id);
CREATE INDEX IF NOT EXISTS idx_bot_referrals_tenant_inviter ON bot_referrals(tenant_id, inviter_id);

ALTER TABLE terminals ADD COLUMN IF NOT EXISTS last_message_at TIMESTAMPTZ;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS dm_hourly_limit INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS dm_daily_limit INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS join_hourly_limit INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS join_daily_limit INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS dm_hourly_count INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS dm_daily_count INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS join_hourly_count INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS join_daily_count INT DEFAULT 0;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS dm_hourly_reset_at TIMESTAMPTZ;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS dm_daily_reset_at TIMESTAMPTZ;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS join_hourly_reset_at TIMESTAMPTZ;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS join_daily_reset_at TIMESTAMPTZ;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS dm_cooldown_until TIMESTAMPTZ;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS last_join_at TIMESTAMPTZ;
ALTER TABLE terminals ADD COLUMN IF NOT EXISTS join_cooldown_until TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS terminal_target_restrictions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    terminal_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL,
    target_type VARCHAR(30),
    target_value VARCHAR(255),
    target_key VARCHAR(320) NOT NULL,
    reason VARCHAR(500),
    fail_count INT DEFAULT 0,
    cooldown_until TIMESTAMPTZ,
    last_failed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_terminal_target_restrictions_lookup ON terminal_target_restrictions(tenant_id, terminal_id, action, target_key);
CREATE INDEX IF NOT EXISTS idx_terminal_target_restrictions_cooldown ON terminal_target_restrictions(cooldown_until);
CREATE INDEX IF NOT EXISTS idx_terminal_target_restrictions_global_lookup ON terminal_target_restrictions(terminal_id, action, target_key);

ALTER TABLE listener_accounts ADD COLUMN IF NOT EXISTS last_join_at TIMESTAMPTZ;
ALTER TABLE listener_accounts ADD COLUMN IF NOT EXISTS join_daily_limit INT DEFAULT 0;
ALTER TABLE listener_accounts ADD COLUMN IF NOT EXISTS join_daily_count INT DEFAULT 0;
ALTER TABLE listener_accounts ADD COLUMN IF NOT EXISTS join_daily_reset_at TIMESTAMPTZ;
ALTER TABLE listener_accounts ADD COLUMN IF NOT EXISTS join_cooldown_until TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS account_target_joins (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    account_kind VARCHAR(30) NOT NULL,
    account_id UUID NOT NULL,
    target_id UUID,
    target_type VARCHAR(30),
    target_value VARCHAR(255),
    target_key VARCHAR(320) NOT NULL,
    source_task_id UUID,
    status VARCHAR(30) NOT NULL DEFAULT 'active',
    status_reason VARCHAR(500),
    active BOOLEAN NOT NULL DEFAULT true,
    joined_at TIMESTAMPTZ NOT NULL,
    last_checked_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE account_target_joins ADD COLUMN IF NOT EXISTS status VARCHAR(30) NOT NULL DEFAULT 'active';
ALTER TABLE account_target_joins ADD COLUMN IF NOT EXISTS status_reason VARCHAR(500);
ALTER TABLE account_target_joins ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE account_target_joins ADD COLUMN IF NOT EXISTS last_checked_at TIMESTAMPTZ;
ALTER TABLE account_target_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
ALTER TABLE account_target_joins ADD COLUMN IF NOT EXISTS removed_at TIMESTAMPTZ;
UPDATE account_target_joins
SET status = COALESCE(NULLIF(status, ''), 'active'),
    active = COALESCE(active, true),
    last_seen_at = COALESCE(last_seen_at, joined_at)
WHERE status IS NULL OR status = '' OR active IS NULL OR last_seen_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_account_target_joins_lookup ON account_target_joins(tenant_id, account_kind, account_id, target_key);
CREATE INDEX IF NOT EXISTS idx_account_target_joins_target ON account_target_joins(tenant_id, account_kind, target_key);
CREATE INDEX IF NOT EXISTS idx_account_target_joins_joined ON account_target_joins(joined_at DESC);
CREATE INDEX IF NOT EXISTS idx_account_target_joins_active_target ON account_target_joins(tenant_id, account_kind, active, target_key);
CREATE INDEX IF NOT EXISTS idx_account_target_joins_checked ON account_target_joins(last_checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_account_target_joins_refresh ON account_target_joins(account_kind, active, updated_at);
CREATE INDEX IF NOT EXISTS idx_account_target_joins_account_active ON account_target_joins(account_kind, account_id, active);
CREATE INDEX IF NOT EXISTS idx_account_target_joins_global_target ON account_target_joins(account_kind, active, target_key);
