import { request } from '../request'
import type { SCRMKeywordRule, SCRMLead, SCRMListenerStatus, SCRMMessage, Task } from '../types'

function scrmQuery(filters?: { user_id?: string; task_id?: string }) {
  const params = new URLSearchParams()
  if (filters?.user_id) params.set('user_id', filters.user_id)
  if (filters?.task_id) params.set('task_id', filters.task_id)
  const query = params.toString()
  return query ? `?${query}` : ''
}

export const scrmApi = {
  scrmRules: (filters?: { user_id?: string }) => request<SCRMKeywordRule[]>(`/api/v1/scrm/rules${scrmQuery(filters)}`),
  createScrmRule: (payload: {
    id?: string
    name: string
    keywords: { list?: string[]; text?: string }
    listen_group_id?: string
    strike_group_id?: string
    monitor_group_id?: string
    monitor_terminal_ids?: string[]
    match_mode?: 'fuzzy' | 'exact'
    push_to_bot?: boolean
    strike_enabled?: boolean
  }) =>
    request<SCRMKeywordRule>('/api/v1/scrm/rules', { method: 'POST', body: JSON.stringify(payload) }),
  deleteScrmRule: (id: string) => request<{ deleted: string }>(`/api/v1/scrm/rules/${id}`, { method: 'DELETE' }),
  scrmLeads: (filters?: { user_id?: string; task_id?: string }) => request<SCRMLead[]>(`/api/v1/scrm/leads${scrmQuery(filters)}`),
  blacklistScrmLeadUser: (leadId: string) =>
    request<{ status: string; lead_id: string; task_id: string; user_key: string }>(`/api/v1/scrm/leads/${leadId}/blacklist`, {
      method: 'POST',
      body: JSON.stringify({})
    }),
  scrmMessages: (leadId: string) => request<SCRMMessage[]>(`/api/v1/scrm/messages/${leadId}`),
  sendScrmMessage: (leadId: string, content: string) =>
    request<{ status: string; message: SCRMMessage }>(`/api/v1/scrm/messages/${leadId}`, { method: 'POST', body: JSON.stringify({ content }) }),
  scrmListenerStatus: () => request<SCRMListenerStatus>('/api/v1/scrm/listener'),
  startScrmListener: () => request<{ task: Task; status: string }>('/api/v1/scrm/listener/start', { method: 'POST', body: JSON.stringify({}) }),
  stopScrmListener: () => request<{ status: string; task_id?: string }>('/api/v1/scrm/listener/stop', { method: 'POST', body: JSON.stringify({}) }),
  startScrmRule: (id: string) => request<{ task: Task; status: string }>(`/api/v1/scrm/rules/${id}/start`, { method: 'POST', body: JSON.stringify({}) }),
  pauseScrmRule: (id: string) => request<{ status: string; rule_id: string }>(`/api/v1/scrm/rules/${id}/pause`, { method: 'POST', body: JSON.stringify({}) })
}
