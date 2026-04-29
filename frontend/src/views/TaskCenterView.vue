<template>
  <div class="page-shell">
    <PageToolbar
      title="运行中任务"
      subtitle="展示正在运行、排队和等待执行的任务，长耗时操作会在这里持续更新进度。"
    >
      <template #actions>
        <span class="status-pill" :data-tone="riskPolicyTone">{{ riskPolicyPresetText }}</span>
        <GlassButton variant="secondary" :disabled="!selectedTaskIDs.length" @click="bulkAction('start')">批量开始</GlassButton>
        <GlassButton variant="secondary" :disabled="!selectedTaskIDs.length" @click="bulkAction('stop')">批量停止</GlassButton>
        <GlassButton variant="danger" :disabled="!selectedTaskIDs.length" @click="bulkDelete">批量删除</GlassButton>
        <GlassButton variant="secondary" :loading="refreshing" @click="refreshTaskStatus">一键刷新任务状态</GlassButton>
        <GlassButton variant="primary" :loading="loading" @click="load">刷新列表</GlassButton>
      </template>
    </PageToolbar>

    <FilterCard grid-class="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        <label class="filter-field">
          <span>任务类型</span>
          <select v-model="filters.type" @change="load">
            <option value="">全部任务</option>
            <option v-for="option in taskTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>任务状态</span>
          <select v-model="filters.status" disabled>
            <option value="">运行中 / 排队中</option>
          </select>
        </label>
        <label class="filter-field">
          <span>Web 用户</span>
          <select v-model="filters.user_id" @change="load">
            <option value="">全部 Web 用户</option>
            <option v-for="user in webUsers" :key="user.id" :value="user.id">{{ user.username }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>Bot 用户</span>
          <select v-model="filters.bot_user_id" @change="load">
            <option value="">全部 Bot 用户</option>
            <option v-for="user in botUsers" :key="user.id" :value="user.id">{{ botName(user) }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>搜索</span>
          <input v-model="taskKeyword" placeholder="任务名 / ID / 用户 / 原因" />
        </label>
    </FilterCard>

    <div class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,0.55fr)]">
      <TableCard :empty="!filteredTasks.length" empty-text="暂无任务">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2 text-sm text-steel">
            <span>显示 {{ pagedTasks.length }} / {{ filteredTasks.length }}，已选 {{ selectedTaskIDs.length }}</span>
            <span v-if="tasks.length !== filteredTasks.length">总任务 {{ tasks.length }}</span>
          </div>
          <table class="w-full min-w-[1120px] text-left text-sm">
            <thead class="text-steel">
              <tr>
                <th class="py-2"><input :checked="allVisibleSelected" :disabled="!selectablePagedTasks.length" type="checkbox" @change="toggleAllVisible" /></th>
                <th class="py-2">任务</th>
                <th>类型</th>
                <th>账户</th>
                <th>Bot 用户</th>
                <th>状态</th>
                <th>进度</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="task in pagedTasks"
                :key="task.id"
                class="border-t border-white/8 transition hover:bg-white/5"
                :class="selectedTask?.id === task.id ? 'bg-white/6' : ''"
                @click="selectTask(task)"
              >
                <td class="py-3" @click.stop>
                  <input :checked="selectedTaskIDs.includes(task.id)" :disabled="!isRunningTask(task)" type="checkbox" @change="toggleTaskSelection(task.id)" />
                </td>
                <td class="py-3">
                  <div class="font-bold text-white">{{ taskDisplayName(task) }}</div>
                  <div class="mt-1 max-w-[220px] truncate text-xs text-steel">{{ task.id }}</div>
                </td>
                <td>{{ taskTypeText(task.type) }}</td>
                <td>
                  <div>{{ task.creator?.username || '系统任务' }}</div>
                  <div class="text-xs text-steel">{{ task.creator?.role === 'admin' ? '管理员' : task.creator?.role === 'user' ? '普通用户' : '后台' }}</div>
                </td>
                <td>
                  <div>{{ task.bot_user?.nickname || '-' }}</div>
                  <div class="text-xs text-steel">{{ task.bot_user?.telegram_user_id || '' }}</div>
                </td>
                <td>
                  <span class="status-pill" :data-tone="statusTone(task.status)">{{ statusText(task.status) }}</span>
                  <div v-if="taskReason(task)" class="mt-1 max-w-[180px] truncate text-xs text-steel">原因：{{ taskReason(task) }}</div>
                </td>
                <td>
                  <div class="progress-track w-36">
                    <div class="progress-fill" :style="{ width: `${task.progress || 0}%` }"></div>
                  </div>
                  <div class="mt-1 text-xs text-steel">{{ task.progress || 0 }}%</div>
                </td>
                <td class="text-steel">{{ formatTime(task.updated_at) }}</td>
                <td>
                  <div class="flex flex-wrap gap-2" @click.stop>
                    <button class="action-link text-ice" @click="action(task.id, 'start')">开始</button>
                    <button class="action-link text-amber" @click="action(task.id, 'pause')">暂停</button>
                    <button class="action-link text-neon" @click="action(task.id, 'resume')">恢复</button>
                    <button class="action-link text-danger" @click="action(task.id, 'stop')">停止</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="taskPageCount > 1" class="mt-4 flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
            <span>第 {{ taskPage }} / {{ taskPageCount }} 页</span>
            <GlassButton variant="secondary" size="sm" :disabled="taskPage <= 1" @click="taskPage--">上一页</GlassButton>
            <GlassButton variant="secondary" size="sm" :disabled="taskPage >= taskPageCount" @click="taskPage++">下一页</GlassButton>
          </div>
      </TableCard>

      <GlassCard class="min-h-0">
        <div v-if="selectedTask" class="space-y-5">
          <div>
            <div class="text-xs uppercase text-steel">Task Detail</div>
            <h2 class="mt-2 text-xl font-black text-white">{{ taskDisplayName(selectedTask) }}</h2>
            <p class="mt-1 break-all text-xs text-steel">{{ selectedTask.id }}</p>
          </div>

          <div class="detail-grid">
            <div v-for="item in selectedTask.settings || []" :key="`${item.label}-${item.value}`" class="detail-cell">
              <span>{{ item.label }}</span>
              <strong>{{ settingText(item) }}</strong>
            </div>
          </div>

          <div v-if="selectedTask.bot_user" class="detail-section">
            <h3>Bot 用户</h3>
            <p>{{ selectedTask.bot_user.nickname }} · ID {{ selectedTask.bot_user.telegram_user_id }}</p>
            <p>{{ selectedTask.bot_user.plan }} · {{ selectedTask.bot_user.status }}</p>
          </div>

          <div v-if="selectedTask.bot_dm_settings?.length" class="detail-section">
            <h3>私信设置</h3>
            <div v-for="item in selectedTask.bot_dm_settings" :key="`${item.label}-${item.value}`" class="detail-row">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>

          <div class="detail-section">
            <h3>原始设置</h3>
            <pre>{{ prettyJSON(selectedTask.payload) }}</pre>
          </div>
        </div>
        <div v-else class="py-12 text-center text-sm text-steel">选择左侧任务查看完整设置</div>
      </GlassCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import FilterCard from '../components/FilterCard.vue'
import PageToolbar from '../components/PageToolbar.vue'
import TableCard from '../components/TableCard.vue'
import { api, type BotUserDashboardItem, type SystemSettings, type Task, type User } from '../api/client'
import { useUiStore } from '../stores/ui'
import { taskDisplayName, taskStatusDisplay, taskTypeDisplay } from '../utils/taskDisplay'

const ui = useUiStore()
const tasks = ref<Task[]>([])
const selectedTask = ref<Task | null>(null)
const webUsers = ref<User[]>([])
const botUsers = ref<BotUserDashboardItem[]>([])
const systemSettings = ref<SystemSettings | null>(null)
const loading = ref(false)
const refreshing = ref(false)
const filters = reactive({ type: '', status: '', user_id: '', bot_user_id: '' })
const selectedTaskIDs = ref<string[]>([])
const taskKeyword = ref('')
const taskPage = ref(1)
const taskPageSize = 100
const filteredTasks = computed(() => {
  const keyword = normalizeKeyword(taskKeyword.value)
  if (!keyword) return tasks.value
  return tasks.value.filter((task) => {
    return [
      task.id,
      task.name,
      taskDisplayName(task),
      task.type,
      taskTypeText(task.type),
      task.status,
      statusText(task.status),
      task.creator?.username,
      task.creator?.role,
      task.bot_user?.nickname,
      task.bot_user?.username,
      task.bot_user?.telegram_user_id,
      taskReason(task)
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const taskPageCount = computed(() => Math.max(1, Math.ceil(filteredTasks.value.length / taskPageSize)))
const pagedTasks = computed(() => {
  const start = (taskPage.value - 1) * taskPageSize
  return filteredTasks.value.slice(start, start + taskPageSize)
})
const selectablePagedTasks = computed(() => pagedTasks.value.filter(isRunningTask))
const allVisibleSelected = computed(() => selectablePagedTasks.value.length > 0 && selectablePagedTasks.value.every((task) => selectedTaskIDs.value.includes(task.id)))
const riskPolicyPresetText = computed(() => {
  const risk = systemSettings.value?.risk_control
  if (!risk?.auto_bypass_high_risk) return '风控避让关闭'
  if (risk.auto_bypass_active_restrictions === 2 && risk.auto_bypass_failures_24h === 6) return '保守模式'
  if (risk.auto_bypass_active_restrictions === 3 && risk.auto_bypass_failures_24h === 10) return '平衡模式'
  if (risk.auto_bypass_active_restrictions === 5 && risk.auto_bypass_failures_24h === 16) return '激进模式'
  return '自定义风控'
})
const riskPolicyTone = computed(() => {
  if (!systemSettings.value?.risk_control.auto_bypass_high_risk) return 'info'
  if (riskPolicyPresetText.value.includes('保守')) return 'warning'
  if (riskPolicyPresetText.value.includes('激进')) return 'success'
  return 'cyan'
})

const taskTypeOptions = [
  { value: 'import', label: '导入任务' },
  { value: 'terminal_check', label: '账号检测' },
  { value: 'network_test', label: '代理检测' },
  { value: 'profile_modify', label: '资料修改' },
  { value: 'mass_messaging', label: '通知工作流' },
  { value: 'scrm_listener', label: '监听任务' },
  { value: 'bot_dm', label: 'Bot 私信任务' },
  { value: 'join_targets', label: '终端加入目标池' },
  { value: 'listener_join_targets', label: '监听号自动加群' },
  { value: 'listener_proxy_check', label: '监听代理检测' },
  { value: 'target_membership_refresh', label: '目标群状态刷新' }
]
const activeTaskStatuses = ['running', 'active', 'queued', 'pending', 'retrying']

async function load() {
  loading.value = true
  try {
    filters.status = ''
    const results = await Promise.all(
      activeTaskStatuses.map((status) => api.tasks({ ...filters, status, limit: 500 }))
    )
    tasks.value = dedupeTasks(results.flat())
      .filter(isRunningTask)
      .sort((left, right) => new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime())
    selectedTask.value = tasks.value.find((task) => task.id === selectedTask.value?.id) || tasks.value[0] || null
    selectedTaskIDs.value = selectedTaskIDs.value.filter((id) => tasks.value.some((task) => task.id === id && isRunningTask(task)))
  } catch (err) {
    ui.toast({ title: '任务加载失败', message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  } finally {
    loading.value = false
  }
}

async function loadFilterOptions() {
  const results = await Promise.allSettled([api.users(), api.botUsers(), api.systemSettings()])
  if (results[0].status === 'fulfilled') webUsers.value = results[0].value
  if (results[1].status === 'fulfilled') botUsers.value = results[1].value
  if (results[2].status === 'fulfilled') systemSettings.value = results[2].value
}

async function refreshTaskStatus() {
  refreshing.value = true
  try {
    const result = await api.refreshTasks()
    ui.toast({ title: '任务状态已刷新', message: result.message, tone: 'success' })
    await load()
    await new Promise((resolve) => setTimeout(resolve, 700))
    await load()
  } catch (err) {
    ui.toast({ title: '刷新失败', message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  } finally {
    refreshing.value = false
  }
}

async function action(id: string, next: string) {
  try {
    await api.taskAction(id, next)
    ui.toast({ title: '任务指令已发送', message: `任务状态切换为 ${actionText(next)}`, tone: 'success' })
    await load()
  } catch (err) {
    ui.toast({ title: '任务操作失败', message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  }
}

function selectTask(task: Task) {
  if (!isRunningTask(task)) return
  selectedTask.value = task
}

function toggleTaskSelection(id: string) {
  const task = tasks.value.find((item) => item.id === id)
  if (!task || !isRunningTask(task)) return
  selectedTaskIDs.value = selectedTaskIDs.value.includes(id)
    ? selectedTaskIDs.value.filter((item) => item !== id)
    : [...selectedTaskIDs.value, id]
}

function toggleAllVisible() {
  const ids = selectablePagedTasks.value.map((task) => task.id)
  if (!ids.length) return
  if (allVisibleSelected.value) {
    selectedTaskIDs.value = selectedTaskIDs.value.filter((id) => !ids.includes(id))
    return
  }
  selectedTaskIDs.value = [...new Set([...selectedTaskIDs.value, ...ids])]
}

async function bulkAction(next: string) {
  if (!selectedTaskIDs.value.length) return
  try {
    const ids = [...selectedTaskIDs.value]
    const result = await api.batchTaskAction(ids, next)
    const successCount = result.results.filter((item) => item.ok).length
    const failed = result.results.filter((item) => !item.ok)
    const message = failed.length
      ? `成功 ${successCount} 个，失败 ${failed.length} 个。`
      : `${successCount} 个任务已切换为 ${actionText(next)}。`
    ui.toast({ title: failed.length ? '批量任务部分完成' : '批量任务指令已发送', message, tone: failed.length ? 'warning' : 'success' })
    if (failed.length) {
      console.warn('batchAction failed items', failed)
    }
    await refreshTaskStatus()
  } catch (err) {
    ui.toast({ title: '批量操作失败', message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  }
}

async function bulkDelete() {
  if (!selectedTaskIDs.value.length) return
  try {
    const ids = [...selectedTaskIDs.value]
    const result = await api.batchDeleteTasks(ids)
    const successCount = result.results.filter((item) => item.ok).length
    const failed = result.results.filter((item) => !item.ok)
    const message = failed.length
      ? `已删除 ${successCount} 个，失败 ${failed.length} 个（执行中任务请先停止）。`
      : `已删除 ${successCount} 个任务及日志。`
    ui.toast({ title: failed.length ? '批量删除部分完成' : '批量删除完成', message, tone: failed.length ? 'warning' : 'success' })
    if (failed.length) {
      console.warn('batchDelete failed items', failed)
    }
    selectedTaskIDs.value = []
    await refreshTaskStatus()
  } catch (err) {
    ui.toast({ title: '批量删除失败', message: err instanceof Error ? err.message : '执行中任务请先停止，或稍后再删。', tone: 'error' })
  }
}

function botName(user: BotUserDashboardItem) {
  return user.nickname || user.username || user.telegram_user_id
}

function taskTypeText(value: string) {
  return taskTypeDisplay(value)
}

function statusText(status: string) {
  return taskStatusDisplay(status)
}

function actionText(action: string) {
  return ({ start: '执行中', pause: '已暂停', resume: '执行中', stop: '已停止' } as Record<string, string>)[action] || action
}

function statusTone(status: string) {
  if (/success|done|completed|finished|resume|running/i.test(status)) return 'success'
  if (/fail|error|stopped/i.test(status)) return 'danger'
  if (/queue|pending|wait|pause/i.test(status)) return 'warning'
  return 'info'
}

function isRunningTask(task: Task) {
  return ['running', 'active', 'queued', 'pending', 'retrying'].includes(normalizeKeyword(task.status))
}

function dedupeTasks(items: Task[]) {
  return Array.from(new Map(items.map((task) => [task.id, task])).values())
}

function settingText(item: { label: string; value: string }) {
  if (item.label === '匹配模式') return item.value === 'exact' ? '精准匹配' : item.value === 'fuzzy' ? '模糊匹配' : item.value
  if (item.label === '自动私信') return String(item.value) === 'true' ? '开启' : '关闭'
  return item.value
}

function taskReason(task: Task) {
  const summary = (task.summary || {}) as Record<string, unknown>
  const payload = (task.payload || {}) as Record<string, unknown>
  const keys = ['waiting_reason', 'reason', 'stop_reason', 'pause_reason', 'error', 'last_error']
  for (const key of keys) {
    const value = summary[key] ?? payload[key]
    if (typeof value === 'string' && value.trim() !== '') return value.trim()
  }
  return ''
}

function prettyJSON(value: unknown) {
  if (!value || (typeof value === 'object' && !Object.keys(value as Record<string, unknown>).length)) return '无'
  return JSON.stringify(value, null, 2)
}

function normalizeKeyword(value: unknown) {
  return String(value ?? '').trim().toLowerCase()
}

function formatTime(value: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

watch(taskKeyword, () => {
  taskPage.value = 1
})
watch(taskPageCount, (count) => {
  if (taskPage.value > count) taskPage.value = count
})

onMounted(async () => {
  await Promise.all([loadFilterOptions(), load()])
})
</script>
