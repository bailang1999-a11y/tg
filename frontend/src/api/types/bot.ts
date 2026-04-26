import type { Group, User } from './common'
import type { Task } from './task'

export type BotConfig = {
  id: string
  name: string
  token: string
  username: string
  push_chat_id: string
  admin_chat_id: string
  admin_contact?: string
  enabled: boolean
  running: boolean
  force_join_enabled?: boolean
  force_join_url?: string
  force_join_handle?: string
  trial_enabled?: boolean
  trial_hours: number
  trial_features?: string[]
  enabled_commands?: string[]
  command_labels?: Record<string, string>
  default_keywords?: string[]
  default_keyword_limit?: number
  default_match_mode: 'fuzzy' | 'exact'
  private_terminal_ids?: string[]
  welcome_title?: string
  service_overview?: string
  quick_start_text?: string
  faq_text?: string
  support_text?: string
  menu_info_label?: string
  menu_settings_label?: string
  menu_faq_label?: string
  menu_support_label?: string
  menu_placeholder?: string
  button_labels?: Record<string, string>
  reply_templates?: Record<string, string>
  default_dm_messages?: string[]
  dm_min_delay_seconds?: number
  dm_max_delay_seconds?: number
  dm_max_messages?: number
  webhook_url: string
  webhook_secret: string
  last_webhook_status: string
  last_webhook_message: string
  last_webhook_at?: string | null
  last_test_status: string
  last_test_message: string
  last_test_at?: string | null
  updated_at: string
}

export type BotLicense = {
  id: string
  code: string
  status: string
  duration_hour: number
  max_bind: number
  bound_count: number
  bound_user_id: string
  bound_username: string
  used_at?: string | null
  expires_at?: string | null
  created_at: string
  updated_at: string
}

export type BotSubscriber = {
  id: string
  user_id?: string | null
  telegram_user_id: string
  username: string
  nickname: string
  invite_code?: string
  force_joined?: boolean
  push_enabled?: boolean
  push_chat_id?: string
  keywords?: string[]
  keyword_limit?: number
  match_mode?: string
  user_blacklist_enabled?: boolean
  risk_control_enabled?: boolean
  push_interval_minutes?: number
  message_dedup_minutes?: number
  dm_quota_total?: number
  dm_quota_used?: number
  private_terminal_group_ids?: string[]
  status: string
  plan: string
  trial_started_at?: string | null
  trial_ends_at?: string | null
  authorized_at?: string | null
  expires_at?: string | null
  last_seen_at?: string | null
  created_at?: string
  updated_at: string
}

export type BotUserDashboardItem = BotSubscriber & {
  web_user?: User | null
  account_count: number
  account_group_count: number
  task_count: number
  invite_count: number
}

export type BotPrivateAccountGroup = {
  id: string
  name: string
  is_default: boolean
  created_at: string
}

export type BotPrivateAccount = {
  id: string
  group_id?: string | null
  phone: string
  nickname: string
  status: string
  risk_status: string
  access_type: string
  created_at: string
}

export type BotDMTask = {
  id: string
  name: string
  account_group_name: string
  cooldown_minutes: number
  keywords?: string[]
  messages?: string[]
  min_delay_seconds?: number
  max_delay_seconds?: number
  sent_count: number
  status: string
  started_at?: string | null
  ended_at?: string | null
  created_at: string
}

export type BotUserDashboardDetail = {
  subscriber: BotSubscriber
  web_user?: User | null
  groups: BotPrivateAccountGroup[]
  accounts: BotPrivateAccount[]
  uploads: Array<Record<string, unknown>>
  tasks: BotDMTask[]
  all_tasks?: Task[]
  referrals: Array<Record<string, unknown>>
  terminal_groups: Group[]
}

export type BotPollingStatus = {
  running: boolean
  started_at?: string
  last_update_id?: number
  handled_count?: number
  last_error?: string
  last_message_at?: string
}
