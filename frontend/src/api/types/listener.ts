export type ListenerAdminOverview = {
  account_count: number
  target_count: number
  proxy_count: number
  assigned_count: number
}

export type ListenerProxyAssignment = {
  accounts: number
  proxies: number
  assigned: number
  skipped: number
}

export type ListenerAccount = {
  id: string
  phone: string
  phone_display?: string
  nickname: string
  avatar_url?: string
  status: string
  status_text?: string
  risk_status: string
  access_type: string
  group_id: string | null
  proxy_id?: string | null
  exit_ip: string
  exit_country: string
  exit_flag: string
  joined_targets?: number
  joined_target_count?: number
  target_total_count?: number
  created_at: string
}

export type ListenerTarget = {
  id: string
  identifier: string
  name: string
  type: string
  type_text?: string
  size: number
  status: string
  group_id: string | null
  group_name?: string
  created_at: string
}

export type ListenerProxy = {
  id: string
  code: string
  ip: string
  port: number
  protocol: string
  protocol_display?: string
  username: string
  exit_ip?: string
  latency_ms?: number
  country: string
  flag: string
  bound_accounts: number
  bound_display?: string
  location_display?: string
  assignment_limit?: number
  assignment_percent?: number
  status: string
  group_id: string | null
  created_at: string
}

export type ListenerCheckSummary = {
  total: number
  normal: number
  abnormal: number
  offline: number
}

export type ListenerProxyCheckSummary = {
  total: number
  normal: number
  failed: number
  timeout: number
}

export type ListenerTargetRefreshSummary = {
  total: number
  success: number
  failed: number
}

export type ListenerJoinTargetsRequest = {
  account_scope: 'all' | 'group'
  account_group_id?: string
  target_scope: 'all' | 'group'
  target_group_id?: string
  daily_limit: number
  interval_minutes: number
  max_joins?: number
  prefer_uncovered?: boolean
}
