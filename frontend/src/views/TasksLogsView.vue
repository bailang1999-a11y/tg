<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">任务中心 & 日志中心</h1>
        <p class="page-subtitle">长耗时任务、状态控制和实时日志集中在同一块主面板里，表格卡片自动向下撑满。</p>
      </div>
      <div class="page-actions">
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
      </div>
    </div>

    <GlassCard class="flex-1">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[900px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="py-2">任务 ID</th>
              <th>名称</th>
              <th>类型</th>
              <th>状态</th>
              <th>进度</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="task in tasks"
              :key="task.id"
              class="border-t border-white/8 transition"
              :class="selectedTaskID === task.id ? 'bg-white/6' : ''"
              @click="selectTask(task.id)"
            >
              <td class="max-w-[140px] truncate py-3 text-steel">{{ task.id }}</td>
              <td>{{ taskDisplayName(task) }}</td>
              <td>{{ taskTypeDisplay(task.type) }}</td>
              <td><span class="status-pill" :data-tone="statusTone(task.status)">{{ statusText(task.status) }}</span></td>
              <td>
                <div class="progress-track w-32">
                  <div class="progress-fill" :style="{ width: `${task.progress}%` }"></div>
                </div>
              </td>
              <td class="text-steel">{{ task.updated_at }}</td>
              <td>
                <div class="flex flex-wrap gap-2">
                  <button class="action-link text-ice" @click="action(task.id, 'start')">开始</button>
                  <button class="action-link text-amber" @click="action(task.id, 'pause')">暂停</button>
                  <button class="action-link text-neon" @click="action(task.id, 'resume')">恢复</button>
                  <button class="action-link text-danger" @click="action(task.id, 'stop')">停止</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!tasks.length" class="py-8 text-center text-sm text-steel">暂无任务</div>
      </div>
    </GlassCard>

    <GlassCard class="h-full">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 class="font-bold">实时日志</h2>
        <span class="status-pill" :data-tone="socketConnected ? 'success' : 'warning'">
          {{ socketConnected ? '实时连接中' : '等待连接' }}
        </span>
      </div>
      <div class="max-h-[420px] overflow-auto rounded-2xl border border-white/10 scrollbar-thin">
        <div v-for="log in logs" :key="log.id" class="grid grid-cols-[160px_80px_1fr] gap-3 border-b border-white/8 px-3 py-2 text-sm">
          <span class="text-steel">{{ log.created_at }}</span>
          <span :class="log.level === 'ERROR' ? 'text-danger' : log.level === 'WARN' ? 'text-amber' : 'text-neon'">{{ log.level }}</span>
          <span class="min-w-0 truncate">{{ log.action }}：{{ log.details }}</span>
        </div>
        <div v-if="!logs.length" class="py-8 text-center text-sm text-steel">暂无日志</div>
      </div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { api, type Task, type TaskLog } from '../api/client'
import { useUiStore } from '../stores/ui'
import { taskDisplayName, taskStatusDisplay, taskTypeDisplay } from '../utils/taskDisplay'

const tasks = ref<Task[]>([])
const logs = ref<TaskLog[]>([])
const loading = ref(false)
const selectedTaskID = ref('')
const socketConnected = ref(false)
const ui = useUiStore()
let logSocket: WebSocket | null = null

const wsURL = computed(() => {
  const token = localStorage.getItem('codex3_token')
  if (!token || !selectedTaskID.value) return ''

  const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''
  const base =
    apiBase !== ''
      ? apiBase.replace(/^http/i, 'ws')
      : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`

  return `${base}/api/v1/ws/logs?access_token=${encodeURIComponent(token)}&task_id=${encodeURIComponent(selectedTaskID.value)}`
})

async function load() {
  loading.value = true
  try {
    tasks.value = (await api.tasks({ status: 'running', limit: 500 })).filter(isRunningTask)
    if (tasks.value.length === 0) {
      selectedTaskID.value = ''
      logs.value = []
      return
    }

    const stillExists = tasks.value.some((task) => task.id === selectedTaskID.value)
    selectedTaskID.value = stillExists ? selectedTaskID.value : tasks.value[0].id
    logs.value = await api.taskLogs(selectedTaskID.value, { limit: 1000 })
  } catch (err) {
    ui.toast({
      title: '任务中心加载失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    loading.value = false
  }
}

function selectTask(id: string) {
  const task = tasks.value.find((item) => item.id === id)
  if (!task || !isRunningTask(task)) return
  if (selectedTaskID.value === id) return
  selectedTaskID.value = id
}

async function action(id: string, next: string) {
  try {
    await api.taskAction(id, next)
    ui.toast({
      title: '任务指令已发送',
      message: `任务状态切换动作「${next}」已提交。`,
      tone: 'success'
    })
    await load()
  } catch (err) {
    ui.toast({
      title: '任务操作失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  }
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
      if (payload.error) {
        return
      }
      if (payload.type === 'logs' && Array.isArray(payload.data)) {
        logs.value = payload.data
      }
    } catch {
      socketConnected.value = false
    }
  }
}

function statusTone(status: string) {
  if (/dry_run/i.test(status)) return 'info'
  if (/success|done|completed|finished|resume|running/i.test(status)) return 'success'
  if (/fail|error|stopped/i.test(status)) return 'danger'
  if (/queue|pending|wait|pause/i.test(status)) return 'warning'
  return 'info'
}

function statusText(status: string) {
  return taskStatusDisplay(status)
}

function isRunningTask(task: Task) {
  return ['running', 'active'].includes(String(task.status || '').trim().toLowerCase())
}

watch(selectedTaskID, async (taskID) => {
  if (!taskID) {
    logs.value = []
    connectLogStream()
    return
  }
  try {
    logs.value = await api.taskLogs(taskID, { limit: 1000 })
  } catch {
    logs.value = []
  }
  connectLogStream()
})

onMounted(load)

onBeforeUnmount(() => {
  if (logSocket) {
    logSocket.close()
    logSocket = null
  }
})
</script>
