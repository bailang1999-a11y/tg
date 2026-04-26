import { request } from '../request'
import type {
  Asset,
  AssetUploadSummary,
  DashboardData,
  DirectMessageJobRequest,
  Group,
  ImportSummary,
  MassMessagingRequest,
  NetworkNode,
  OutreachJobRequest,
  ProfileModifySummary,
  ProxyImportSummary,
  SystemSettings,
  SystemSettingsHistoryItem,
  Target,
  TargetMembership,
  TargetImportSummary,
  Terminal,
  TerminalRiskBoardItem,
  TerminalRiskStats,
  TerminalRestriction,
  TerminalCheckRequest,
  TerminalCheckSummary,
  Task,
  User,
  Workflow,
  WorkflowDefinition
} from '../types'

export const coreApi = {
  login: (username: string, password: string) =>
    request<{ token: string; user: User }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password })
    }),
  me: () => request<User>('/api/v1/me'),
  dashboard: () => request<DashboardData>('/api/v1/dashboard'),
  users: () => request<User[]>('/api/v1/users'),
  createUser: (payload: { username: string; password: string; email: string; role: string; telegram_user_id?: string; trial_days?: number }) =>
    request<User>('/api/v1/users', { method: 'POST', body: JSON.stringify(payload) }),
  updateUserStatus: (id: string, status: string) =>
    request(`/api/v1/users/${id}/status`, { method: 'PUT', body: JSON.stringify({ status }) }),
  bindUserTelegram: (id: string, payload: { telegram_user_id: string; trial_days?: number }) =>
    request<User>(`/api/v1/users/${id}/telegram`, { method: 'PUT', body: JSON.stringify(payload) }),
  groups: (resource: string) => request<Group[]>(`/api/v1/groups/${resource}`),
  createGroup: (resource: string, name: string) =>
    request<Group>(`/api/v1/groups/${resource}`, { method: 'POST', body: JSON.stringify({ name }) }),
  deleteGroup: (resource: string, id: string) => request(`/api/v1/groups/${resource}/${id}`, { method: 'DELETE' }),
  terminals: (groupID = '') => request<Terminal[]>(`/api/v1/terminals${groupID ? `?group_id=${groupID}` : ''}`),
  terminalRiskBoard: (groupID = '') => request<TerminalRiskBoardItem[]>(`/api/v1/terminals/risk-board${groupID ? `?group_id=${groupID}` : ''}`),
  terminalRiskStats: (id: string) => request<TerminalRiskStats>(`/api/v1/terminals/${id}/risk-stats`),
  batchTerminals: (payload: { ids: string[]; action: 'reduce_limits' | 'clear_cooldown' | 'clear_expired_restrictions'; multiplier?: number }) =>
    request<{ action: string; results: Array<{ id: string; ok: boolean; message?: string }> }>('/api/v1/terminals/batch', {
      method: 'POST',
      body: JSON.stringify(payload)
    }),
  terminalRestrictions: (id: string, filters?: { action?: 'all' | 'dm' | 'join'; state?: 'all' | 'active' | 'expired' }) => {
    const params = new URLSearchParams()
    if (filters?.action && filters.action !== 'all') {
      params.set('action', filters.action)
    }
    if (filters?.state && filters.state !== 'all') {
      params.set('state', filters.state)
    }
    const query = params.toString()
    return request<TerminalRestriction[]>(`/api/v1/terminals/${id}/restrictions${query ? `?${query}` : ''}`)
  },
  clearTerminalCooldown: (id: string) => request<{ cleared: string }>(`/api/v1/terminals/${id}/cooldown/clear`, { method: 'PUT', body: JSON.stringify({}) }),
  clearTerminalRestrictions: (id: string, payload: { mode?: 'expired' | 'filtered' | 'all'; action?: 'all' | 'dm' | 'join'; state?: 'all' | 'active' | 'expired' }) =>
    request<{ deleted_count: number }>(`/api/v1/terminals/${id}/restrictions/clear`, { method: 'POST', body: JSON.stringify(payload) }),
  deleteTerminalRestriction: (id: string, restrictionID: string) =>
    request<{ deleted: string }>(`/api/v1/terminals/${id}/restrictions/${restrictionID}`, { method: 'DELETE' }),
  updateTerminalLimits: (id: string, payload: {
    dm_hourly_limit?: number
    dm_daily_limit?: number
    join_hourly_limit?: number
    join_daily_limit?: number
  }) => request<Terminal>(`/api/v1/terminals/${id}/limits`, { method: 'PUT', body: JSON.stringify(payload) }),
  networkNodes: (groupID = '') => request<NetworkNode[]>(`/api/v1/network-nodes${groupID ? `?group_id=${groupID}` : ''}`),
  importNetworkNodes: (payload: { content: string; default_protocol: string; group_id?: string; new_group_name?: string }) =>
    request<ProxyImportSummary>('/api/v1/network-nodes/import', { method: 'POST', body: JSON.stringify(payload) }),
  targets: (groupID = '') => request<Target[]>(`/api/v1/targets${groupID ? `?group_id=${groupID}` : ''}`),
  targetMemberships: (id: string, accountKind = 'terminal') =>
    request<TargetMembership[]>(`/api/v1/targets/${id}/memberships?account_kind=${encodeURIComponent(accountKind)}`),
  refreshTargetMemberships: (payload: {
    account_kind?: 'terminal' | 'listener' | 'all'
    target_scope?: 'target' | 'group' | 'all'
    target_id?: string
    target_group_id?: string
  }) => request<{ task: Task }>('/api/v1/targets/memberships/refresh', { method: 'POST', body: JSON.stringify(payload) }),
  importTargets: (payload: { content: string; group_id?: string; new_group_name?: string }) =>
    request<TargetImportSummary>('/api/v1/targets/import', { method: 'POST', body: JSON.stringify(payload) }),
  importTerminalTargets: (payload: {
    scope: 'terminal' | 'group' | 'all'
    terminal_id?: string
    terminal_group_id?: string
    group_id?: string
    new_group_name?: string
  }) => request<TargetImportSummary>('/api/v1/targets/import-terminals', { method: 'POST', body: JSON.stringify(payload) }),
  joinTargets: (payload: {
    terminal_scope: 'terminal' | 'group' | 'all'
    terminal_group_id?: string
    terminal_id?: string
    target_scope: 'group' | 'all'
    target_group_id?: string
  }) => request<{ task: Task }>('/api/v1/targets/join', { method: 'POST', body: JSON.stringify(payload) }),
  assets: (groupID = '') => request<Asset[]>(`/api/v1/assets${groupID ? `?group_id=${groupID}` : ''}`),
  uploadAssets: (files: File[], groupID: string, newGroupName: string) => {
    const form = new FormData()
    for (const file of files) {
      form.append('files', file, (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name)
    }
    if (groupID) {
      form.append('group_id', groupID)
    }
    if (newGroupName) {
      form.append('new_group_name', newGroupName)
    }
    return request<AssetUploadSummary>('/api/v1/assets/upload', { method: 'POST', body: form })
  },
  uploadWorkflowMedia: (files: File[]) => {
    const form = new FormData()
    for (const file of files) {
      form.append('files', file, file.name)
    }
    return request<AssetUploadSummary>('/api/v1/assets/workflow-media', { method: 'POST', body: form })
  },
  deleteAsset: (id: string) => request(`/api/v1/assets/${id}`, { method: 'DELETE' }),
  modifyProfiles: (payload: {
    scope: 'terminal' | 'group' | 'all'
    terminal_id?: string
    terminal_group_id?: string
    nicknames: string[]
    bios: string[]
    homepages: string[]
    avatar_asset_ids: string[]
  }) => request<{ task: Task; summary: ProfileModifySummary }>('/api/v1/profiles/modify', { method: 'POST', body: JSON.stringify(payload) }),
  workflows: () => request<Workflow[]>('/api/v1/workflows'),
  createWorkflow: (payload: { name: string; description?: string; definition: WorkflowDefinition }) =>
    request<Workflow>('/api/v1/workflows', { method: 'POST', body: JSON.stringify(payload) }),
  runWorkflow: (id: string) => request<Task>(`/api/v1/workflows/${id}/run`, { method: 'POST', body: JSON.stringify({}) }),
  systemSettings: () => request<SystemSettings>('/api/v1/settings'),
  systemSettingsHistory: () => request<SystemSettingsHistoryItem[]>('/api/v1/settings/history'),
  updateSystemSettings: (payload: Omit<SystemSettings, 'updated_at'>) =>
    request<SystemSettings>('/api/v1/settings', { method: 'PUT', body: JSON.stringify(payload) }),
  createOutreachJob: (payload: OutreachJobRequest) =>
    request<Task>('/api/v1/outreach/jobs', { method: 'POST', body: JSON.stringify(payload) }),
  createMassMessagingJob: (payload: MassMessagingRequest) =>
    request<Task>('/api/v1/mass-messaging/jobs', { method: 'POST', body: JSON.stringify(payload) }),
  createDirectMessageJob: (payload: DirectMessageJobRequest) =>
    request<Task>('/api/v1/direct-messages/jobs', { method: 'POST', body: JSON.stringify(payload) }),
  importFiles: (files: File[], groupID: string, mode: 'session' | 'tdata' | 'mixed' = 'mixed') => {
    const form = new FormData()
    for (const file of files) {
      const uploadName = (file as File & { relativePath?: string; webkitRelativePath?: string }).relativePath || (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name
      form.append('files', file, uploadName)
    }
    if (groupID) {
      form.append('group_id', groupID)
    }
    const path = mode === 'mixed' ? '/api/v1/imports' : `/api/v1/imports/${mode}`
    return request<{ task: Task; summary: ImportSummary }>(path, {
      method: 'POST',
      body: form
    })
  },
  createTask: (path: string) => request<Task>(path, { method: 'POST', body: JSON.stringify({}) }),
  checkTerminals: ({ groupID = '', terminalID = '' }: TerminalCheckRequest = {}) =>
    request<{ task: Task; summary: TerminalCheckSummary }>('/api/v1/terminals/check', {
      method: 'POST',
      body: JSON.stringify({
        ...(groupID ? { group_id: groupID } : {}),
        ...(terminalID ? { terminal_id: terminalID } : {})
      })
    }),
  deleteTerminal: (id: string) => request<{ deleted: string }>(`/api/v1/terminals/${id}`, { method: 'DELETE' })
}
