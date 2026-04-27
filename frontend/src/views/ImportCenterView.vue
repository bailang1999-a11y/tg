<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">导入账号</h1>
        <p class="page-subtitle">选择账号包文件夹、.session 或 zip 后先确认，再开始导入；系统自动识别 Session / TData，并按手机号文件夹聚合多个账号，同账号优先按 TData 导入。</p>
      </div>
    </div>

    <GlassCard padding="sm">
      <GroupSelect v-model="groupID" :groups="groups" :loading="groupLoading" @create="createGroup" />
    </GlassCard>

    <GlassCard>
      <div
        class="import-dropzone grid place-items-center px-5 py-10 text-center"
        :class="{ 'is-dragging': dragging }"
        @dragenter.prevent="dragging = true"
        @dragover.prevent="dragging = true"
        @dragleave.prevent="dragging = false"
        @drop.prevent="drop"
      >
        <div>
          <div class="text-xl font-bold text-ice">选择文件 / 文件夹 / Zip 路径</div>
          <p class="mt-2 text-sm text-steel">点击选择账号包文件夹；.session 或 zip 也可以直接拖进来。选择完成后会先确认是否导入，同账号同时存在时优先使用 TData。</p>
          <p class="mt-2 text-xs text-steel">文件夹结构支持：手机号文件夹 / tdata，或手机号文件夹直接包含 key_data 等 tdata 数据。</p>
          <div class="mt-5 flex flex-wrap justify-center gap-3">
            <GlassButton variant="primary" :loading="uploading" :disabled="uploading" @click="chooseImportPath">选择文件 / 文件夹 / Zip 路径</GlassButton>
            <GlassButton variant="ghost" :disabled="files.length === 0 || uploading" @click="clearFiles">清空</GlassButton>
          </div>
          <input ref="fileInput" class="hidden" type="file" multiple accept=".session,.zip" @change="pickFiles" />
          <input ref="folderInput" class="hidden" type="file" multiple webkitdirectory @change="pickFiles" />
        </div>
      </div>
    </GlassCard>

    <div class="grid flex-1 gap-4 xl:grid-cols-[0.8fr_1.2fr]">
      <GlassCard class="h-full">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-bold">导入阶段</h2>
          <span class="status-pill" data-tone="info">{{ progress }}%</span>
        </div>
        <div class="progress-track mb-5">
          <div class="progress-fill" :style="{ width: `${progress}%` }"></div>
        </div>
        <div class="space-y-3">
          <div v-for="(stage, index) in stageRows" :key="stage.key" class="import-stage-card">
            <div class="flex items-start gap-3">
              <span class="import-stage-index" :data-status="stage.status">{{ index + 1 }}</span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div class="font-bold text-white">{{ stage.label }}</div>
                  <div class="text-xs font-bold text-steel">{{ stage.current }} / {{ stage.total || stage.current }}</div>
                </div>
                <div class="mt-2 progress-track progress-track-sm">
                  <div class="progress-fill" :style="{ width: `${stage.percent}%` }"></div>
                </div>
                <p class="mt-2 text-xs leading-5 text-steel">{{ stage.detail }}</p>
                <div v-if="stage.metrics && Object.keys(stage.metrics).length" class="mt-3 flex flex-wrap gap-2">
                  <span v-for="(value, key) in stage.metrics" :key="key" class="import-stage-metric">
                    {{ key }}：{{ value }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </GlassCard>

      <GlassCard class="h-full">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-bold">最近选择</h2>
          <span class="status-pill" data-tone="warning">{{ files.length }} 个文件</span>
        </div>
        <div v-if="sourceLabel" class="mb-3 rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-steel">
          当前选择：{{ sourceLabel }}
        </div>
        <div class="max-h-80 overflow-auto rounded-2xl border border-white/10 scrollbar-thin">
          <div v-for="file in files" :key="fileKey(file)" class="grid grid-cols-[1fr_100px] gap-3 border-b border-white/8 px-3 py-2 text-sm">
            <span class="min-w-0 truncate">{{ displayName(file) }}</span>
            <span class="text-right text-steel">{{ formatSize(file.size) }}</span>
          </div>
          <div v-if="!files.length" class="py-10 text-center text-sm text-steel">还没有选择文件、zip 或拖拽账号文件夹</div>
        </div>
      </GlassCard>
    </div>

    <GlassCard v-if="summary" class="flex-1">
      <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">完成汇总</h2>
          <p class="mt-1 text-sm text-steel">任务 ID：{{ summary.task_id }}</p>
        </div>
        <RouterLink class="action-link" to="/tasks-logs">查看任务日志</RouterLink>
      </div>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
        <div v-for="card in resultCards" :key="card.label" class="metric-card app-card p-4" :data-tone="card.accent">
          <div class="text-sm text-steel">{{ card.label }}</div>
          <div class="mt-2 text-2xl font-black" :class="card.tone">{{ card.value }}</div>
        </div>
      </div>
      <div class="mt-5 overflow-x-auto">
        <table class="w-full min-w-[760px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="py-2">文件</th>
              <th>类型</th>
              <th>结果</th>
              <th>原因</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in summary.items" :key="`${item.name}-${item.status}`" class="border-t border-white/8">
              <td class="max-w-[360px] truncate py-3">{{ item.name }}</td>
              <td>{{ item.type }}</td>
              <td>
                <span class="rounded px-2 py-1" :class="statusClass(item.status)">{{ item.status }}</span>
              </td>
              <td class="text-steel">{{ item.reason || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </GlassCard>

    <p v-if="error" class="rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import GroupSelect from '../components/GroupSelect.vue'
import { api, type Group, type ImportSummary } from '../api/client'
import { useUiStore } from '../stores/ui'

const steps = ['文件扫描中', '格式检查中', '重复检测中', '入库中', '状态校验中', '完成汇总']
type ImportStageRow = {
  key: string
  label: string
  status: string
  current: number
  total: number
  percent: number
  detail: string
  metrics?: Record<string, number>
}
type ImportFile = File & {
  relativePath?: string
  webkitRelativePath?: string
}
type FileSystemHandleLike = {
  kind: 'file' | 'directory'
  name: string
}
type FileSystemFileHandleLike = FileSystemHandleLike & {
  kind: 'file'
  getFile: () => Promise<File>
}
type FileSystemDirectoryHandleLike = FileSystemHandleLike & {
  kind: 'directory'
  entries: () => AsyncIterable<[string, FileSystemHandleLike]>
}
type DroppedEntry = {
  name: string
  fullPath?: string
  isFile: boolean
  isDirectory: boolean
  file?: (callback: (file: File) => void, error?: (err: DOMException) => void) => void
  createReader?: () => {
    readEntries: (callback: (entries: DroppedEntry[]) => void, error?: (err: DOMException) => void) => void
  }
}

const files = ref<File[]>([])
const groups = ref<Group[]>([])
const groupID = ref('')
const uploading = ref(false)
const groupLoading = ref(false)
const dragging = ref(false)
const summary = ref<ImportSummary | null>(null)
const error = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const folderInput = ref<HTMLInputElement | null>(null)
const sourceLabel = ref('')
const ui = useUiStore()

const progress = computed(() => {
  if (!stageRows.value.length) return 0
  return Math.round(stageRows.value.reduce((total, stage) => total + stage.percent, 0) / stageRows.value.length)
})

const stageRows = computed<ImportStageRow[]>(() => {
  if (summary.value?.stages?.length) {
    return summary.value.stages.map((stage) => ({
      key: stage.key,
      label: stage.label,
      status: stage.status,
      current: stage.current,
      total: stage.total,
      percent: stage.percent,
      detail: stage.detail,
      metrics: stage.metrics
    }))
  }

  const selected = files.value.length
  const waitingText = uploading.value ? '服务器正在处理，完成后显示真实统计。' : '等待选择路径并确认导入。'
  return [
    {
      key: 'scan',
      label: '文件扫描',
      status: selected ? 'success' : 'pending',
      current: selected,
      total: selected,
      percent: selected ? 100 : 0,
      detail: selected ? `已选择 ${selected} 个本地文件，确认后提交后端扫描。` : waitingText,
      metrics: selected ? { 本地文件: selected } : {}
    },
    ...steps.slice(1).map((label, index) => ({
      key: `pending-${index}`,
      label: label.replace('中', ''),
      status: uploading.value ? 'running' : 'pending',
      current: 0,
      total: 0,
      percent: 0,
      detail: waitingText,
      metrics: {}
    }))
  ]
})

const resultCards = computed(() => {
  const data = summary.value
  if (!data) return []
  return [
    { label: '成功', value: data.success, tone: 'text-neon', accent: 'success' },
    { label: '失败', value: data.failed, tone: 'text-danger', accent: 'danger' },
    { label: '重复', value: data.duplicate, tone: 'text-amber', accent: 'warning' },
    { label: '跳过', value: data.skipped, tone: 'text-steel', accent: 'info' },
    { label: '终端', value: data.terminals, tone: 'text-ice', accent: 'cyan' },
    { label: '素材', value: data.assets, tone: 'text-white', accent: 'info' }
  ]
})

async function loadGroups() {
  groups.value = await api.groups('terminal')
}

async function createGroup(name: string) {
  groupLoading.value = true
  try {
    await api.createGroup('terminal', name)
    await loadGroups()
    ui.toast({ title: '分组已创建', message: `终端分组「${name}」已可用于导入。`, tone: 'success' })
  } finally {
    groupLoading.value = false
  }
}

async function chooseImportPath() {
  if (uploading.value) return
  const picker = window as Window & { showDirectoryPicker?: () => Promise<FileSystemDirectoryHandleLike> }
  if (picker.showDirectoryPicker) {
    try {
      const directory = await picker.showDirectoryPicker()
      const selected = await readDirectoryHandle(directory, directory.name)
      await queueImport(selected, directory.name)
      return
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      ui.toast({
        title: '读取文件夹失败',
        message: err instanceof Error ? err.message : '请选择文件夹或拖拽 zip / session 文件',
        tone: 'error'
      })
      return
    }
  }

  folderInput.value?.click()
}

function pickFiles(event: Event) {
  const input = event.target as HTMLInputElement
  void queueImport(Array.from(input.files || []), '本地选择')
  input.value = ''
}

function drop(event: DragEvent) {
  dragging.value = false
  void collectDroppedFiles(event).then((items) => queueImport(items, '拖拽内容'))
}

async function queueImport(next: File[], label = '本地选择') {
  if (uploading.value) {
    ui.toast({ title: '正在导入', message: '请等待当前导入完成后再选择新的文件。', tone: 'warning' })
    return
  }

  const unique: File[] = []
  const seen = new Set<string>()
  for (const file of next) {
    const key = fileKey(file)
    if (!seen.has(key)) {
      unique.push(file)
      seen.add(key)
    }
  }

  if (!unique.length) {
    error.value = '没有识别到可导入的文件、zip 或文件夹内容'
    return
  }

  files.value = unique
  sourceLabel.value = label
  summary.value = null
  error.value = ''

  const accepted = await ui.confirm({
    title: '确认导入账号',
    message: buildConfirmMessage(unique, label),
    confirmText: '确认导入',
    cancelText: '取消',
    tone: 'info'
  })
  if (!accepted) {
    ui.toast({ title: '已取消导入', message: '已保留最近选择，确认后可重新选择路径。', tone: 'info' })
    return
  }

  await upload()
}

async function readDirectoryHandle(directory: FileSystemDirectoryHandleLike, rootPath: string): Promise<File[]> {
  const collected: File[] = []
  for await (const [, handle] of directory.entries()) {
    const path = `${rootPath}/${handle.name}`
    if (handle.kind === 'file') {
      const file = await (handle as FileSystemFileHandleLike).getFile()
      collected.push(attachRelativePath(file, path))
    } else {
      collected.push(...(await readDirectoryHandle(handle as FileSystemDirectoryHandleLike, path)))
    }
  }
  return collected
}

async function collectDroppedFiles(event: DragEvent) {
  const items = Array.from(event.dataTransfer?.items || [])
  const entryItems: DroppedEntry[] = []
  for (const item of items) {
    const maybeItem = item as unknown as { webkitGetAsEntry?: () => DroppedEntry | null }
    const entry = maybeItem.webkitGetAsEntry?.()
    if (entry) {
      entryItems.push(entry)
    }
  }

  if (!entryItems.length) {
    return Array.from(event.dataTransfer?.files || [])
  }

  const collected: File[] = []
  for (const entry of entryItems) {
    collected.push(...(await readDroppedEntry(entry, entry.name)))
  }
  return collected
}

async function readDroppedEntry(entry: DroppedEntry, path: string): Promise<File[]> {
  if (entry.isFile && entry.file) {
    return new Promise((resolve, reject) => {
      entry.file?.(
        (file) => resolve([attachRelativePath(file, path)]),
        (err) => reject(err)
      )
    })
  }

  if (!entry.isDirectory || !entry.createReader) {
    return []
  }

  const reader = entry.createReader()
  const children = await readAllDirectoryEntries(reader)
  const files: File[] = []
  for (const child of children) {
    files.push(...(await readDroppedEntry(child, `${path}/${child.name}`)))
  }
  return files
}

function readAllDirectoryEntries(reader: ReturnType<NonNullable<DroppedEntry['createReader']>>) {
  const entries: DroppedEntry[] = []

  return new Promise<DroppedEntry[]>((resolve, reject) => {
    const readBatch = () => {
      reader.readEntries(
        (batch) => {
          if (!batch.length) {
            resolve(entries)
            return
          }
          entries.push(...batch)
          readBatch()
        },
        (err) => reject(err)
      )
    }

    readBatch()
  })
}

function attachRelativePath(file: File, path: string) {
  ;(file as ImportFile).relativePath = path
  return file
}

function buildConfirmMessage(selectedFiles: File[], label: string) {
  const sessionCount = selectedFiles.filter((file) => displayName(file).toLowerCase().endsWith('.session')).length
  const zipCount = selectedFiles.filter((file) => displayName(file).toLowerCase().endsWith('.zip')).length
  const tdataLikeCount = selectedFiles.filter((file) => /\/tdata\/|key_data|key_datas|\.data$/i.test(displayName(file))).length
  const groupName = groups.value.find((group) => group.id === groupID.value)?.name || '全部分组'
  const preview = selectedFiles.slice(0, 5).map(displayName).join('，')
  const more = selectedFiles.length > 5 ? ` 等 ${selectedFiles.length} 个文件` : ''

  return `已选择：${label}
导入分组：${groupName}
文件数量：${selectedFiles.length}
Session：${sessionCount}，Zip：${zipCount}，TData 相关文件：${tdataLikeCount}
同账号优先：TData
预览：${preview}${more}

是否开始导入？`
}

function clearFiles() {
  files.value = []
  summary.value = null
  error.value = ''
  sourceLabel.value = ''
}

async function upload() {
  if (!files.value.length) return
  uploading.value = true
  summary.value = null
  error.value = ''
  try {
    const result = await api.importFiles(files.value, groupID.value, 'mixed')
    summary.value = result.summary
    ui.toast({
      title: '智能导入完成',
      message: `成功 ${result.summary.success} 条，失败 ${result.summary.failed} 条。`,
      tone: 'success'
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导入失败'
    ui.toast({
      title: '导入失败',
      message: error.value,
      tone: 'error'
    })
  } finally {
    uploading.value = false
  }
}

function displayName(file: File) {
  const item = file as ImportFile
  return item.relativePath || item.webkitRelativePath || file.name
}

function fileKey(file: File) {
  return `${displayName(file)}-${file.size}-${file.lastModified}`
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function statusClass(status: string) {
  if (status === 'success') return 'bg-neon/10 text-neon'
  if (status === 'failed') return 'bg-danger/10 text-danger'
  if (status === 'duplicate') return 'bg-amber/10 text-amber'
  return 'bg-white/8 text-steel'
}

onMounted(loadGroups)
</script>
