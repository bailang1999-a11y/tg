import type { Task } from '../api/types/task'

export type TaskDisplaySource = Pick<Task, 'name' | 'type' | 'status'> & {
  creator?: Task['creator'] | null
  bot_user?: Task['bot_user'] | null
  payload?: Record<string, unknown> | null
}

const taskTypeLabels: Record<string, string> = {
  import: '导入任务',
  terminal_check: '账号检测',
  network_test: '代理检测',
  profile_modify: '资料修改任务',
  mass_messaging: '通知工作流',
  scrm_listener: '监听任务',
  direct_messages: '监听私信任务',
  bot_dm: 'Bot 私信任务',
  join_targets: '加群任务',
  listener_join_targets: '监听号自动加群',
  listener_proxy_check: '监听代理检测',
  target_membership_refresh: '目标群状态刷新',
  workflow_execution: '工作流执行任务',
  event_outreach: '主动触达任务'
}

const taskStatusLabels: Record<string, string> = {
  dry_run: '演练完成',
  success: '执行成功',
  partial_success: '部分成功',
  failed: '执行失败',
  queued: '排队中',
  running: '执行中',
  active: '进行中',
  pending: '待执行',
  paused: '已暂停',
  stopped: '已停止',
  completed: '已完成',
  cancelled: '已取消'
}

export function taskTypeDisplay(type?: string | null, fallbackName?: string | null) {
  const key = clean(type).toLowerCase()
  if (taskTypeLabels[key]) return taskTypeLabels[key]
  const name = clean(fallbackName)
  if (name && !/^bot\s*radar$/i.test(name) && !/^scrm\s+/i.test(name)) return name
  return key || '任务'
}

export function taskStatusDisplay(status?: string | null) {
  const key = clean(status).toLowerCase()
  return taskStatusLabels[key] || clean(status) || '未知状态'
}

export function taskDisplayName(task: TaskDisplaySource) {
  const kind = taskOwnerKind(task)
  const owner = taskOwnerName(task)
  return `${kind}--${owner}--${taskTypeDisplay(task.type, task.name)}`
}

export function taskOptionLabel(task: TaskDisplaySource) {
  return `${taskDisplayName(task)} · ${taskStatusDisplay(task.status)}`
}

export function taskOwnerKind(task: TaskDisplaySource) {
  if (task.bot_user || payloadString(task.payload, 'bot_subscriber_id')) return 'BOT'
  return 'WEB'
}

export function taskOwnerName(task: TaskDisplaySource) {
  if (taskOwnerKind(task) === 'BOT') {
    return (
      clean(task.bot_user?.nickname) ||
      clean(task.bot_user?.username) ||
      clean(task.bot_user?.telegram_user_id) ||
      clean(payloadString(task.payload, 'bot_subscriber_name')) ||
      clean(payloadString(task.payload, 'bot_subscriber_id')) ||
      'Bot用户'
    )
  }
  return (
    clean(task.creator?.username) ||
    clean(task.creator?.telegram_username) ||
    clean(task.creator?.telegram_user_id) ||
    '系统'
  )
}

function payloadString(payload: TaskDisplaySource['payload'], key: string) {
  const value = payload && typeof payload === 'object' ? payload[key] : undefined
  return typeof value === 'string' ? value : ''
}

function clean(value: unknown) {
  return String(value ?? '').trim()
}
