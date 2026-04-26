<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">控制台概览</h1>
        <p class="page-subtitle">系统状态、业务统计与最新任务全部换成真实数据回流，图表、资源卡和任务表统一铺满这张工作台。</p>
      </div>
      <div class="page-actions">
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
      </div>
    </div>

    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <GlassCard v-for="card in statCards" :key="card.label" class="metric-card" :data-tone="card.accent">
        <div class="text-sm text-steel">{{ card.label }}</div>
        <div class="mt-2 text-3xl font-black" :class="card.tone">{{ card.value }}</div>
      </GlassCard>
    </div>

    <div class="grid flex-1 gap-4 xl:grid-cols-[1.35fr_0.65fr]">
      <GlassCard class="h-full">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-bold">最近 7 天趋势</h2>
          <span class="status-pill" data-tone="info">通知 / 失败 / 活跃终端</span>
        </div>
        <div class="grid h-64 grid-cols-7 items-end gap-3">
          <div v-for="row in trendRows" :key="row.day" class="flex h-full flex-col justify-end gap-2">
            <div class="flex flex-1 items-end gap-1">
              <span class="w-full rounded-t bg-ice/70" :style="{ height: row.notifyHeight }"></span>
              <span class="w-full rounded-t bg-danger/70" :style="{ height: row.failedHeight }"></span>
              <span class="w-full rounded-t bg-neon/70" :style="{ height: row.terminalsHeight }"></span>
            </div>
            <div class="truncate text-center text-xs text-steel">{{ row.day }}</div>
          </div>
        </div>
      </GlassCard>

      <GlassCard class="h-full">
        <h2 class="mb-4 font-bold">资源监控</h2>
        <div class="space-y-3">
          <div v-for="item in resources" :key="item.label">
            <div class="mb-1 flex justify-between text-sm">
              <span class="text-steel">{{ item.label }}</span>
              <span>{{ item.value }}</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill" :style="{ width: item.width }"></div>
            </div>
          </div>
        </div>
      </GlassCard>
    </div>

    <GlassCard class="flex-1">
      <h2 class="mb-4 font-bold">最新任务</h2>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[720px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="py-2">任务名</th>
              <th>类型</th>
              <th>状态</th>
              <th>进度</th>
              <th>更新时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="task in dashboard?.latest_tasks || []" :key="task.id" class="border-t border-white/8">
              <td class="py-3">{{ taskDisplayName(task) }}</td>
              <td>{{ taskTypeDisplay(task.type) }}</td>
              <td><span class="status-pill" :data-tone="taskTone(task.status)">{{ taskStatusDisplay(task.status) }}</span></td>
              <td>{{ task.progress }}%</td>
              <td class="text-steel">{{ formatDate(task.updated_at) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="!dashboard?.latest_tasks?.length" class="py-8 text-center text-sm text-steel">暂无任务</div>
      </div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type DashboardData } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUiStore } from '../stores/ui'
import { taskDisplayName, taskStatusDisplay, taskTypeDisplay } from '../utils/taskDisplay'

const loading = ref(false)
const dashboard = ref<DashboardData | null>(null)
const ui = useUiStore()
const emptyStats: DashboardData['stats'] = {
  today_notify: 0,
  total_notify: 0,
  today_failed: 0,
  total_failed: 0,
  online_terminal: 0,
  total_terminal: 0,
  today_hits: 0,
  total_hits: 0
}
const emptyResources: DashboardData['resources'] = {
  memory_mb: 0,
  goroutines: 0,
  queue_backlog: 0,
  ws_connections: 0,
  active_task: 0,
  tasks_last_hour: 0
}

const statCards = computed(() => {
  const stats = dashboard.value?.stats ?? emptyStats
  return [
    { label: '今日通知数', value: stats.today_notify ?? 0, tone: 'text-ice', accent: 'cyan' },
    { label: '历史通知数', value: stats.total_notify ?? 0, tone: 'text-white', accent: 'info' },
    { label: '今日失败数', value: stats.today_failed ?? 0, tone: 'text-danger', accent: 'danger' },
    { label: '终端在线/总数', value: `${stats.online_terminal ?? 0}/${stats.total_terminal ?? 0}`, tone: 'text-neon', accent: 'success' },
    { label: '今日命中数', value: stats.today_hits ?? 0, tone: 'text-amber', accent: 'success' },
    { label: '历史命中数', value: stats.total_hits ?? 0, tone: 'text-white', accent: 'info' },
    { label: '历史失败数', value: stats.total_failed ?? 0, tone: 'text-danger', accent: 'danger' },
    { label: '任务活跃度', value: dashboard.value?.resources?.active_task ?? 0, tone: 'text-neon', accent: 'cyan' }
  ]
})

const trendRows = computed(() => {
  const rows = dashboard.value?.trend || []
  const maxNotify = Math.max(...rows.map((row) => row.notify || 0), 1)
  const maxFailed = Math.max(...rows.map((row) => row.failed || 0), 1)
  const maxTerminals = Math.max(...rows.map((row) => row.terminals || 0), 1)

  return rows.map((row) => ({
    ...row,
    notifyHeight: `${Math.max(10, ((row.notify || 0) / maxNotify) * 100)}%`,
    failedHeight: `${Math.max(10, ((row.failed || 0) / maxFailed) * 100)}%`,
    terminalsHeight: `${Math.max(10, ((row.terminals || 0) / maxTerminals) * 100)}%`
  }))
})

const resources = computed(() => {
  const data = dashboard.value?.resources ?? emptyResources
  return [
    { label: '进程内存', value: `${data.memory_mb ?? 0} MB`, width: `${Math.min(100, ((data.memory_mb ?? 0) / 512) * 100)}%` },
    { label: '协程数', value: data.goroutines ?? 0, width: `${Math.min(100, ((data.goroutines ?? 0) / 200) * 100)}%` },
    { label: '队列积压', value: data.queue_backlog ?? 0, width: `${Math.min(100, (data.queue_backlog ?? 0) * 10)}%` },
    { label: 'WebSocket 连接', value: data.ws_connections ?? 0, width: `${Math.min(100, (data.ws_connections ?? 0) * 10)}%` },
    { label: '近 1 小时任务', value: data.tasks_last_hour ?? 0, width: `${Math.min(100, (data.tasks_last_hour ?? 0) * 8)}%` }
  ]
})

async function load() {
  loading.value = true
  try {
    dashboard.value = await api.dashboard()
  } catch (err) {
    ui.toast({
      title: 'Dashboard 加载失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    loading.value = false
  }
}

function taskTone(status: string) {
  if (/success|done|completed|finished/i.test(status)) return 'success'
  if (/fail|error|stopped/i.test(status)) return 'danger'
  if (/queue|pending|wait|pause/i.test(status)) return 'warning'
  return 'info'
}

function formatDate(value: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

onMounted(load)
</script>
