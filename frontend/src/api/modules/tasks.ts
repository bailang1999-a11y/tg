import { request } from '../request'
import type { Task, TaskLog, TerminalCheckRequest, TerminalCheckSummary } from '../types'

type TaskFilters = {
  type?: string
  status?: string
  user_id?: string
  bot_user_id?: string
  limit?: number
  offset?: number
}

type LogFilters = {
  type?: string
  level?: string
  user_id?: string
  bot_user_id?: string
  task_id?: string
  limit?: number
  offset?: number
}

export const taskApi = {
  tasks: (filters: TaskFilters = {}) => {
    const query = new URLSearchParams()
    if (filters.type) query.set('type', filters.type)
    if (filters.status) query.set('status', filters.status)
    if (filters.user_id) query.set('user_id', filters.user_id)
    if (filters.bot_user_id) query.set('bot_user_id', filters.bot_user_id)
    if (filters.limit) query.set('limit', String(filters.limit))
    if (filters.offset) query.set('offset', String(filters.offset))
    const suffix = query.toString()
    return request<Task[]>(`/api/v1/tasks${suffix ? `?${suffix}` : ''}`)
  },
  refreshTasks: () => request<{ status: string; message: string }>('/api/v1/tasks/refresh', { method: 'POST', body: JSON.stringify({}) }),
  taskLogs: (id: string, filters: Pick<LogFilters, 'limit' | 'offset'> = {}) => {
    const query = new URLSearchParams()
    if (filters.limit) query.set('limit', String(filters.limit))
    if (filters.offset) query.set('offset', String(filters.offset))
    const suffix = query.toString()
    return request<TaskLog[]>(`/api/v1/tasks/${id}/logs${suffix ? `?${suffix}` : ''}`)
  },
  logs: (filters: LogFilters = {}) => {
    const query = new URLSearchParams()
    if (filters.type) query.set('type', filters.type)
    if (filters.level) query.set('level', filters.level)
    if (filters.user_id) query.set('user_id', filters.user_id)
    if (filters.bot_user_id) query.set('bot_user_id', filters.bot_user_id)
    if (filters.task_id) query.set('task_id', filters.task_id)
    if (filters.limit) query.set('limit', String(filters.limit))
    if (filters.offset) query.set('offset', String(filters.offset))
    const suffix = query.toString()
    return request<TaskLog[]>(`/api/v1/logs${suffix ? `?${suffix}` : ''}`)
  },
  clearLogs: () => request<{ status: string; message: string }>('/api/v1/logs', { method: 'DELETE' }),
  deleteTask: (id: string) => request<{ deleted: string }>(`/api/v1/tasks/${id}`, { method: 'DELETE' }),
  taskAction: (id: string, action: string) => request(`/api/v1/tasks/${id}/${action}`, { method: 'PUT' }),
  batchTaskAction: (ids: string[], action: string) =>
    request<{ action: string; results: Array<{ id: string; ok: boolean; status?: string; error?: string }> }>('/api/v1/tasks/batch', {
      method: 'PUT',
      body: JSON.stringify({ ids, action })
    }),
  batchDeleteTasks: (ids: string[]) =>
    request<{ results: Array<{ id: string; ok: boolean; error?: string }> }>('/api/v1/tasks/batch', {
      method: 'DELETE',
      body: JSON.stringify({ ids })
    }),
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
