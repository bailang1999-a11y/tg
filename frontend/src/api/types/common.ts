import type { Task } from './task'

export type User = {
  id: string
  tenant_id: string
  username: string
  email: string
  role: 'admin' | 'user'
  status: 'active' | 'disabled'
  telegram_user_id?: string
  telegram_username?: string
  trial_ends_at?: string | null
  created_at: string
}

export type Group = {
  id: string
  resource_type: string
  name: string
  description: string
  asset_count?: number
}

export type Terminal = {
  id: string
  phone: string
  phone_display?: string
  nickname: string
  avatar_url: string
  bio: string
  homepage: string
  channel_name?: string
  status: string
  status_text?: string
  account_status?: string
  account_status_text?: string
  online_status?: string
  online_status_text?: string
  last_online_at: string | null
  last_message_at?: string | null
  sleep_until?: string | null
  dm_cooldown_until?: string | null
  last_join_at?: string | null
  join_cooldown_until?: string | null
  access_type: string
  origin_country: string
  origin_flag: string
  exit_ip: string
  exit_country: string
  exit_flag: string
  group_id: string | null
  today_success: number
  total_success: number
  today_failed: number
  total_failed: number
  risk_status: string
  ban_status: string
  dm_hourly_limit: number
  dm_daily_limit: number
  join_hourly_limit: number
  join_daily_limit: number
  dm_hourly_count: number
  dm_daily_count: number
  join_hourly_count: number
  join_daily_count: number
  dm_hourly_reset_at?: string | null
  dm_daily_reset_at?: string | null
  join_hourly_reset_at?: string | null
  join_daily_reset_at?: string | null
}

export type TerminalRestriction = {
  id: string
  action: string
  action_text: string
  target_type: string
  target_value: string
  reason: string
  fail_count: number
  cooldown_until?: string | null
  last_failed_at?: string | null
  active: boolean
}

export type TerminalRiskStats = {
  active_restriction_count: number
  expired_restriction_count: number
  restriction_24h_dm: number
  restriction_24h_join: number
  failure_24h_total: number
  cooldown_active: boolean
  cooldown_until?: string | null
  dm_hourly_usage: number
  dm_daily_usage: number
  join_hourly_usage: number
  join_daily_usage: number
  dm_hourly_limit: number
  dm_daily_limit: number
  join_hourly_limit: number
  join_daily_limit: number
  risk_score: string
}

export type TerminalRiskBoardItem = {
  terminal_id: string
  active_restriction_count: number
  expired_restriction_count: number
  restriction_24h_dm: number
  restriction_24h_join: number
  failure_24h_total: number
  cooldown_active: boolean
  cooldown_until?: string | null
  dm_hourly_usage: number
  dm_daily_usage: number
  join_hourly_usage: number
  join_daily_usage: number
  risk_score: string
}

export type NetworkNode = {
  id: string
  code: string
  ip: string
  port: number
  protocol: string
  username: string
  latency_ms: number
  country: string
  flag: string
  bound_terminals: number
  status: string
  group_id: string | null
  created_at: string
}

export type Target = {
  id: string
  avatar_url: string
  identifier: string
  name: string
  type: string
  size: number
  group_id: string | null
  group_ids?: string[]
  notification_count: number
  linked_terminals: number
  active_member_count?: number
  invalid_member_count?: number
  last_membership_check_at?: string | null
  has_verification: boolean
  created_at: string
}

export type TargetMembership = {
  id: string
  account_kind: string
  account_id: string
  account_label: string
  phone: string
  nickname: string
  account_status: string
  risk_status: string
  status: string
  status_text: string
  status_reason: string
  active: boolean
  joined_at?: string | null
  last_checked_at?: string | null
  last_seen_at?: string | null
  removed_at?: string | null
}

export type Asset = {
  id: string
  group_id: string | null
  name: string
  mime_type: string
  md5: string
  url: string
  file_path: string
  created_at: string
}

export type WorkflowDefinition = {
  nodes: Array<{
    id: string
    position: { x: number; y: number }
    data?: Record<string, unknown>
  }>
  edges: Array<{
    id: string
    source: string
    target: string
  }>
}

export type Workflow = {
  id: string
  name: string
  description: string
  definition: WorkflowDefinition | null
  status: string
  created_at: string
  updated_at: string
}

export type SystemSettings = {
  security: {
    enforce_tenant_isolation: boolean
    require_admin_approval: boolean
    mask_sensitive_logs: boolean
  }
  frequency: {
    max_concurrent_tasks: number
    max_concurrent_outreach: number
    ws_log_batch_size: number
    dashboard_refresh_second: number
  }
  audit: {
    log_retention_days: number
    realtime_log_stream: boolean
    notify_on_failure: boolean
  }
  adapter: {
    telegram_sync_enabled: boolean
    telegram_apply_enabled: boolean
    outreach_dry_run: boolean
    workflow_dry_run: boolean
  }
  risk_control: {
    auto_bypass_high_risk: boolean
    auto_bypass_active_restrictions: number
    auto_bypass_failures_24h: number
    message_cooldown_minutes: number
    message_jitter_minutes: number
    join_daily_limit: number
    join_interval_minutes: number
    join_jitter_minutes: number
  }
  updated_at?: string
}

export type SystemSettingsHistoryItem = {
  id: string
  section: string
  summary: string
  before: Record<string, unknown>
  after: Record<string, unknown>
  changed_by: string
  created_at: string
}

export type SystemVersion = {
  current_version: string
  latest_version: string
  latest_url: string
  update_available: boolean
  update_enabled: boolean
}

export type SystemUpdateResult = {
  status: string
  exec_id: string
  message: string
  command: string
  container: string
}

export type OutreachJobRequest = {
  name: string
  job_type: 'keyword_reply' | 'member_invite' | 'bulk_message' | 'identity_sync' | 'content_cleanup'
  terminal_group_id?: string
  target_group_id?: string
  keyword?: string
  message?: string
  sync_profile: boolean
  content_cleanup: boolean
  compliance_review: boolean
}

export type MassMessageStep = {
  type: string
  content?: string
  media_asset_id?: string
  source_chat_id?: string
  message_id?: string
  delay_seconds: number
}

export type MassMessagingRequest = {
  terminal_group_ids: string[]
  target_group_ids: string[]
  steps: MassMessageStep[]
  send_count?: number
  send_interval_seconds?: number
}

export type DirectMessageStep = {
  type: 'text' | 'image' | 'voice' | 'gif' | 'forward'
  content: string
  media_asset_id?: string
  source_chat_id?: string
  message_id?: string
  delay_seconds: number
}

export type DirectMessageJobRequest = {
  name?: string
  lead_ids: string[]
  terminal_scope: 'all' | 'group' | 'terminal'
  terminal_group_id?: string
  terminal_id?: string
  terminal_ids?: string[]
  steps: DirectMessageStep[]
  min_delay_seconds?: number
  max_delay_seconds?: number
  cooldown_minutes?: number
  cooldown_jitter_minutes?: number
  dedupe_days?: number
  skip_no_account?: boolean
  stop_on_reply?: boolean
  dry_run?: boolean
}

export type ProxyImportSummary = {
  success: number
  failed: number
  duplicate: number
  skipped: number
  group_id?: string
  group_name?: string
  items: Array<{
    line: string
    protocol?: string
    address?: string
    status: string
    reason?: string
  }>
}

export type TargetImportSummary = {
  success: number
  failed: number
  duplicate: number
  skipped: number
  group_id?: string
  group_name?: string
  items: Array<{
    line: string
    identifier?: string
    type?: string
    status: string
    reason?: string
  }>
}

export type AssetUploadSummary = {
  success: number
  failed: number
  duplicate: number
  skipped: number
  group_id?: string
  group_name?: string
  items: Array<{
    id?: string
    name: string
    status: string
    reason?: string
    url?: string
  }>
}

export type ProfileModifySummary = {
  task_id: string
  terminal_count: number
  fields: string[]
  counts: Record<string, number>
  status: string
  applied_count: number
  partial_count: number
  failed_count: number
  requested_field_count: number
  applied_field_count: number
  failed_field_count: number
  field_applied_count: Record<string, number>
  field_failed_count: Record<string, number>
  failure_categories?: Record<string, number>
  pending_refresh: boolean
  assignments: Array<{
    terminal_id: string
    phone?: string
    nickname?: string
    bio?: string
    homepage?: string
    avatar_url?: string
  }>
  results: Array<{
    terminal_id: string
    phone?: string
    status: string
    message?: string
    applied_fields?: string[]
    failed_fields?: Record<string, string>
    failure_category?: string
  }>
}

export type TerminalCheckSummary = {
  task_id: string
  total: number
  online: number
  offline: number
  abnormal: number
  items: Array<{
    terminal_id: string
    phone?: string
    nickname?: string
    bio?: string
    homepage?: string
    status: string
    last_online_at?: string | null
    reason?: string
  }>
}

export type TerminalCheckRequest = {
  groupID?: string
  terminalID?: string
}

export type ImportSummary = {
  task_id: string
  success: number
  failed: number
  duplicate: number
  skipped: number
  terminals: number
  assets: number
  failures: Record<string, number>
  stages?: Array<{
    key: string
    label: string
    status: string
    current: number
    total: number
    percent: number
    detail: string
    metrics?: Record<string, number>
  }>
  items: Array<{
    name: string
    type: string
    status: string
    reason?: string
  }>
}
