<template>
  <div class="page-shell">
    <PageToolbar
      title="日志中心"
      subtitle="按任务类型、用户和日志级别查看详细执行记录，所有状态和动作统一用中文表达。"
    >
      <template #actions>
        <span class="status-pill" :data-tone="socketConnected ? 'success' : 'warning'">{{ socketConnected ? '实时连接中' : '等待连接' }}</span>
        <GlassButton variant="danger" :loading="clearing" @click="clearAllLogs">一键清除日志</GlassButton>
        <GlassButton variant="primary" :loading="loading" @click="load">刷新日志</GlassButton>
      </template>
    </PageToolbar>

    <FilterCard grid-class="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        <label class="filter-field">
          <span>任务类型</span>
          <select v-model="filters.type" @change="loadAndReconnect">
            <option value="">全部任务</option>
            <option v-for="option in taskTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>日志级别</span>
          <select v-model="filters.level" @change="loadAndReconnect">
            <option value="">全部级别</option>
            <option value="INFO">信息</option>
            <option value="WARN">警告</option>
            <option value="ERROR">错误</option>
          </select>
        </label>
        <label class="filter-field">
          <span>Web 用户</span>
          <select v-model="filters.user_id" @change="loadAndReconnect">
            <option value="">全部 Web 用户</option>
            <option v-for="user in webUsers" :key="user.id" :value="user.id">{{ user.username }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>Bot 用户</span>
          <select v-model="filters.bot_user_id" @change="loadAndReconnect">
            <option value="">全部 Bot 用户</option>
            <option v-for="user in botUsers" :key="user.id" :value="user.id">{{ botName(user) }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>搜索</span>
          <input v-model="logKeyword" placeholder="任务 / 动作 / 详情 / 对象" />
        </label>
    </FilterCard>

    <TableCard class="min-h-0 flex-1" :empty="!filteredLogs.length" empty-text="暂无日志">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2 text-sm text-steel">
          <span>显示 {{ pagedLogs.length }} / {{ filteredLogs.length }}</span>
          <span v-if="logs.length !== filteredLogs.length">总日志 {{ logs.length }}</span>
        </div>
        <table class="w-full min-w-[1180px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="py-2">时间</th>
              <th>级别</th>
              <th>任务</th>
              <th>类型</th>
              <th>账户</th>
              <th>Bot 用户</th>
              <th>动作</th>
              <th>详细信息</th>
              <th>对象</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in pagedLogs" :key="log.id" class="border-t border-white/8 transition hover:bg-white/5">
              <td class="py-3 text-steel">{{ formatTime(log.created_at) }}</td>
              <td><span class="status-pill" :data-tone="levelTone(log.level)">{{ log.level_text || levelText(log.level) }}</span></td>
              <td>
                <div class="max-w-[180px] truncate font-bold text-white">{{ log.task ? taskDisplayName(log.task) : '-' }}</div>
                <div class="mt-1 max-w-[180px] truncate text-xs text-steel">{{ log.task_id }}</div>
              </td>
              <td>{{ taskTypeText(log.task?.type || '') }}</td>
              <td>{{ log.task?.creator?.username || '系统任务' }}</td>
              <td>
                <div>{{ log.task?.bot_user?.nickname || '-' }}</div>
                <div class="text-xs text-steel">{{ log.task?.bot_user?.telegram_user_id || '' }}</div>
              </td>
              <td>{{ actionText(log) }}</td>
              <td class="min-w-[320px]">
                <div class="whitespace-normal leading-6 text-white">{{ detailText(log) }}</div>
                <div v-if="log.duration_ms" class="mt-1 text-xs text-steel">耗时 {{ log.duration_ms }} ms</div>
              </td>
              <td>
                <div class="max-w-[160px] truncate">{{ log.terminal_ref || '-' }}</div>
                <div class="max-w-[160px] truncate text-xs text-steel">{{ log.target_ref || '' }}</div>
              </td>
              </tr>
            </tbody>
          </table>
        <div v-if="logPageCount > 1" class="mt-4 flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
          <span>第 {{ logPage }} / {{ logPageCount }} 页</span>
          <GlassButton variant="secondary" size="sm" :disabled="logPage <= 1" @click="logPage--">上一页</GlassButton>
          <GlassButton variant="secondary" size="sm" :disabled="logPage >= logPageCount" @click="logPage++">下一页</GlassButton>
        </div>
    </TableCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import FilterCard from '../components/FilterCard.vue'
import PageToolbar from '../components/PageToolbar.vue'
import TableCard from '../components/TableCard.vue'
import { api, type BotUserDashboardItem, type TaskLog, type User } from '../api/client'
import { useUiStore } from '../stores/ui'
import { taskDisplayName, taskTypeDisplay } from '../utils/taskDisplay'

const ui = useUiStore()
const logs = ref<TaskLog[]>([])
const webUsers = ref<User[]>([])
const botUsers = ref<BotUserDashboardItem[]>([])
const loading = ref(false)
const clearing = ref(false)
const socketConnected = ref(false)
const filters = reactive({ type: '', level: '', user_id: '', bot_user_id: '' })
const logKeyword = ref('')
const logPage = ref(1)
const logPageSize = 200
let logSocket: WebSocket | null = null

const taskTypeOptions = [
  { value: 'import', label: '导入任务' },
  { value: 'terminal_check', label: '账号检测' },
  { value: 'network_test', label: '代理检测' },
  { value: 'profile_modify', label: '资料修改' },
  { value: 'mass_messaging', label: '通知工作流' },
  { value: 'scrm_listener', label: '监听任务' },
  { value: 'bot_dm', label: 'Bot 私信任务' },
  { value: 'join_targets', label: '终端加入目标池' },
  { value: 'listener_proxy_check', label: '监听代理检测' },
  { value: 'target_membership_refresh', label: '目标群状态刷新' }
]

const wsURL = computed(() => {
  const token = localStorage.getItem('codex3_token')
  if (!token) return ''
  const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''
  const base = apiBase !== '' ? apiBase.replace(/^http/i, 'ws') : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`
  const query = new URLSearchParams({ access_token: token })
  if (filters.type) query.set('type', filters.type)
  if (filters.level) query.set('level', filters.level)
  if (filters.user_id) query.set('user_id', filters.user_id)
  if (filters.bot_user_id) query.set('bot_user_id', filters.bot_user_id)
  return `${base}/api/v1/ws/logs?${query.toString()}`
})
const filteredLogs = computed(() => {
  const keyword = normalizeKeyword(logKeyword.value)
  if (!keyword) return logs.value
  return logs.value.filter((log) => {
    return [
      log.id,
      log.task_id,
      log.level,
      log.level_text,
      log.action,
      log.action_text,
      actionText(log),
      log.details,
      detailText(log),
      log.terminal_ref,
      log.target_ref,
      log.task?.name,
      log.task ? taskDisplayName(log.task) : '',
      log.task?.type,
      taskTypeText(log.task?.type || ''),
      log.task?.creator?.username,
      log.task?.bot_user?.nickname,
      log.task?.bot_user?.telegram_user_id
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const logPageCount = computed(() => Math.max(1, Math.ceil(filteredLogs.value.length / logPageSize)))
const pagedLogs = computed(() => {
  const start = (logPage.value - 1) * logPageSize
  return filteredLogs.value.slice(start, start + logPageSize)
})

async function load() {
  loading.value = true
  try {
    logs.value = await api.logs({ ...filters, limit: 1000 })
  } catch (err) {
    ui.toast({ title: '日志加载失败', message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  } finally {
    loading.value = false
  }
}

async function loadAndReconnect() {
  await load()
  connectLogStream()
}

async function clearAllLogs() {
  clearing.value = true
  try {
    const result = await api.clearLogs()
    logs.value = []
    ui.toast({ title: '日志已清除', message: result.message, tone: 'success' })
  } catch (err) {
    ui.toast({ title: '清除失败', message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  } finally {
    clearing.value = false
  }
}

async function loadFilterOptions() {
  const results = await Promise.allSettled([api.users(), api.botUsers()])
  if (results[0].status === 'fulfilled') webUsers.value = results[0].value
  if (results[1].status === 'fulfilled') botUsers.value = results[1].value
}

function connectLogStream() {
  if (logSocket) {
    logSocket.close()
    logSocket = null
  }
  socketConnected.value = false
  if (!wsURL.value) return
  logSocket = new WebSocket(wsURL.value)
  logSocket.onopen = () => {
    socketConnected.value = true
  }
  logSocket.onclose = () => {
    socketConnected.value = false
  }
  logSocket.onerror = () => {
    socketConnected.value = false
  }
  logSocket.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data) as { type?: string; data?: TaskLog[]; error?: string }
      if (payload.type === 'logs' && Array.isArray(payload.data)) logs.value = payload.data
    } catch {
      socketConnected.value = false
    }
  }
}

function botName(user: BotUserDashboardItem) {
  return user.nickname || user.username || user.telegram_user_id
}

function taskTypeText(value: string) {
  return taskTypeDisplay(value)
}

function levelText(value: string) {
  return ({ INFO: '信息', WARN: '警告', ERROR: '错误' } as Record<string, string>)[value] || value
}

function levelTone(value: string) {
  if (value === 'ERROR') return 'danger'
  if (value === 'WARN') return 'warning'
  return 'info'
}

function actionText(log: TaskLog) {
  if (log.action_text?.trim()) return log.action_text.trim()
  const key = (log.action || '').trim().toLowerCase()
  const map: Record<string, string> = {
    start: '开始执行',
    created: '任务创建',
    summary: '执行汇总',
    pause: '暂停',
    resume: '恢复',
    stop: '停止',
    restart: '重启',
    force: '强制停止',
    subscriber_start: '启动监听任务',
    subscriber_stop: '暂停监听任务',
    subscriber_resume: '恢复监听任务',
    subscriber_save: '保存监听配置',
    worker_wait: '进程状态回传',
    listener_stdout: '监听进程输出',
    listener_stderr: '监听进程告警',
    listener_parse: '监听输出解析',
    listener_error: '监听错误',
    warning: '运行告警',
    ready: '监听就绪',
    match: '关键词命中',
    match_skip: '命中跳过',
    history_skip: '历史消息跳过',
    persist_lead: '线索入库',
    dm_task_start: '启动私信任务',
    dm_task_stop: '停止私信任务',
    dm_task_complete: '私信任务完成',
    dm_task_pause: '暂停私信任务',
    dm_task_resume: '恢复私信任务',
    bot_push: 'Bot 线索推送',
    bot_dm: '自动私信处理',
    join_success: '加入目标成功',
    join_failed: '加入目标失败',
    join_skipped: '加入目标跳过',
    adapter: '执行适配器状态',
    script: '脚本检查',
    terminal_path: '会话路径检查',
    terminal_copy: '会话副本准备',
    start_worker: '启动监听进程',
    stdout: '标准输出',
    stderr: '标准错误',
    dispatch_policy: '投递策略',
    round: '发送轮次',
    step: '发送阶段',
    delay: '发送延迟',
    interval: '发送间隔',
    bot_user_bind: '绑定 Bot 用户',
    bot_user_update: '更新 Bot 用户设置',
    bot_license_create: '生成卡密',
    bot_license_bind: '卡密绑定用户',
    bot_license_toggle: '切换卡密状态',
    bot_license_delete: '删除卡密',
    update_bot_config: '更新 Bot 配置',
    update_config: '修改配置',
    check_terminal_status: '账号状态检测',
    import_accounts: '导入账号',
    import_targets: '导入目标',
    test_proxy_latency: '代理延迟检测',
    proxy_check_start: '开始检测代理',
  }
  if (map[key]) return map[key]
  if (key.includes('_')) return `日志动作：${key.split('_').filter(Boolean).join('·')}`
  return log.action || '日志'
}

function detailText(log: TaskLog) {
  const raw = (log.details || '').trim()
  if (!raw) return '-'
  const translated = raw
    .replace(/exit status/gi, '退出状态')
    .replace(/exit status\s+(\d+)/gi, '退出状态 $1')
    .replace(/listener_stderr/gi, '监听进程告警')
    .replace(/listener_stdout/gi, '监听进程输出')
    .replace(/worker_wait/gi, '进程状态回传')
    .replace(/could not open key_data/gi, '无法读取 key_data 文件')
    .replace(/file not found/gi, '文件不存在')
    .replace(/tfilenotfound/gi, '文件不存在')
    .replace(/dry[- ]run/gi, '演练模式')
    .replace(/too many requests/gi, '请求过于频繁')
    .replace(/connection refused/gi, '连接被拒绝')
    .replace(/connection reset/gi, '连接被重置')
    .replace(/network is unreachable/gi, '网络不可达')
    .replace(/context deadline exceeded/gi, '请求超时')
  return translated
}

function formatTime(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

function normalizeKeyword(value: unknown) {
  return String(value ?? '').trim().toLowerCase()
}

watch(logKeyword, () => {
  logPage.value = 1
})
watch(logPageCount, (count) => {
  if (logPage.value > count) logPage.value = count
})

onMounted(async () => {
  await Promise.all([loadFilterOptions(), load()])
  connectLogStream()
})

onBeforeUnmount(() => {
  if (logSocket) logSocket.close()
})
</script>
