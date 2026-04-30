import { request } from '../request'
import type {
  ListenerAccount,
  ListenerAdminOverview,
  ListenerCheckTaskResponse,
  ListenerJoinTargetsRequest,
  ListenerProxy,
  ListenerProxyAssignment,
  ListenerProxyCheckSummary,
  ListenerTarget,
  ListenerTargetRefreshSummary,
  ProxyImportSummary,
  Task,
  TargetImportSummary
} from '../types'

export const listenerApi = {
  listenerAdminOverview: () => request<ListenerAdminOverview>('/api/v1/listener-admin/overview'),
  listenerAccounts: (groupID = '') => request<ListenerAccount[]>(`/api/v1/listener-admin/accounts${groupID ? `?group_id=${groupID}` : ''}`),
  importListenerAccounts: (payload: { content: string; group_id?: string; new_group_name?: string }) =>
    request<TargetImportSummary>('/api/v1/listener-admin/accounts/import', { method: 'POST', body: JSON.stringify(payload) }),
  importListenerAccountFiles: (payload: { files: File[]; group_id?: string; new_group_name?: string }) => {
    const form = new FormData()
    payload.files.forEach((file) => {
      const path = (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name
      form.append('files', file, file.name)
      form.append('paths', path)
    })
    if (payload.group_id) form.append('group_id', payload.group_id)
    if (payload.new_group_name) form.append('new_group_name', payload.new_group_name)
    return request<TargetImportSummary>('/api/v1/listener-admin/accounts/import-files', { method: 'POST', body: form })
  },
  checkListenerAccounts: (payload: { group_id?: string }) =>
    request<ListenerCheckTaskResponse>('/api/v1/listener-admin/accounts/check', { method: 'POST', body: JSON.stringify(payload) }),
  deleteAbnormalListenerAccounts: () =>
    request<{ deleted: number }>('/api/v1/listener-admin/accounts/abnormal', { method: 'DELETE' }),
  deleteListenerAccount: (id: string) =>
    request<{ deleted: number }>(`/api/v1/listener-admin/accounts/${id}`, { method: 'DELETE' }),
  listenerTargets: (groupID = '') => request<ListenerTarget[]>(`/api/v1/listener-admin/targets${groupID ? `?group_id=${groupID}` : ''}`),
  refreshListenerTargets: (payload: { group_id?: string }) =>
    request<{ task: Task; summary: ListenerTargetRefreshSummary }>('/api/v1/listener-admin/targets/refresh', { method: 'POST', body: JSON.stringify(payload) }),
  importListenerTargets: (payload: { content: string; group_id?: string; new_group_name?: string }) =>
    request<TargetImportSummary>('/api/v1/listener-admin/targets/import', { method: 'POST', body: JSON.stringify(payload) }),
  deleteListenerTarget: (id: string) =>
    request<{ deleted: number }>(`/api/v1/listener-admin/targets/${id}`, { method: 'DELETE' }),
  createListenerJoinTargetsTask: (payload: ListenerJoinTargetsRequest) =>
    request<{ task: Task }>('/api/v1/listener-admin/join-targets', { method: 'POST', body: JSON.stringify(payload) }),
  listenerProxies: (groupID = '') => request<ListenerProxy[]>(`/api/v1/listener-admin/proxies${groupID ? `?group_id=${groupID}` : ''}`),
  checkListenerProxies: (payload: { group_id?: string }) =>
    request<{ task: Task; summary: ListenerProxyCheckSummary }>('/api/v1/listener-admin/proxies/check', { method: 'POST', body: JSON.stringify(payload) }),
  importListenerProxies: (payload: {
    content: string
    default_protocol: string
    group_id?: string
    new_group_name?: string
    account_group_id?: string
    assign_to_accounts?: boolean
  }) => request<{ import: ProxyImportSummary; assignment: ListenerProxyAssignment; assignment_error?: string }>('/api/v1/listener-admin/proxies/import', { method: 'POST', body: JSON.stringify(payload) }),
  assignListenerProxies: (payload: { proxy_group_id: string; account_group_id?: string }) =>
    request<ListenerProxyAssignment>('/api/v1/listener-admin/proxies/assign', { method: 'POST', body: JSON.stringify(payload) }),
  deleteListenerProxy: (id: string) =>
    request<{ deleted: number }>(`/api/v1/listener-admin/proxies/${id}`, { method: 'DELETE' })
}
