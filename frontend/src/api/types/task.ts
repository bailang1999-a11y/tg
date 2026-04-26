import type { BotDMTask } from './bot'

export type Task = {
  id: string
  tenant_id?: string
  name: string
  type: string
  status: string
  progress: number
  terminal_group_id?: string | null
  target_group_id?: string | null
  payload?: Record<string, unknown> | null
  summary?: Record<string, unknown> | null
  created_at: string
  updated_at: string
  creator?: {
    id: string
    username: string
    role: string
    telegram_user_id?: string
    telegram_username?: string
  }
  bot_user?: {
    id: string
    nickname: string
    username: string
    telegram_user_id: string
    status: string
    plan: string
  }
  settings?: Array<{ label: string; value: string }>
  bot_dm_settings?: Array<{ label: string; value: string }>
  bot_dm_tasks?: BotDMTask[]
}

export type TaskLog = {
  id: string
  task_id: string
  level: string
  category: string
  terminal_ref: string
  target_ref: string
  action: string
  details: string
  duration_ms: number
  trace_id: string
  created_at: string
  level_text?: string
  action_text?: string
  task?: Task
}

export type DashboardData = {
  stats: {
    today_notify: number
    total_notify: number
    today_failed: number
    total_failed: number
    online_terminal: number
    total_terminal: number
    today_hits: number
    total_hits: number
  }
  resources: {
    memory_mb: number
    goroutines: number
    queue_backlog: number
    ws_connections: number
    active_task: number
    tasks_last_hour: number
  }
  trend: Array<{
    day: string
    notify: number
    failed: number
    terminals: number
  }>
  latest_tasks: Task[]
}
