<template>
  <div class="page-shell">
    <div class="page-header import-page-header">
      <div>
        <h1 class="page-title">导入账号</h1>
        <p class="page-subtitle">一个页面完成分组、选择、确认、进度和结果查看；系统自动识别 Session / TData，同账号优先按 TData 导入。</p>
      </div>
      <div class="page-actions">
        <GlassButton variant="secondary" :disabled="files.length === 0 || uploading" @click="clearFiles">清空选择</GlassButton>
        <GlassButton variant="primary" :loading="uploading" :disabled="uploading" @click="chooseImportPath">选择账号包</GlassButton>
      </div>
    </div>

    <div class="import-workbench">
      <GlassCard class="import-panel import-pick-panel">
        <div class="import-section-head">
          <div>
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Import Source</div>
            <h2 class="mt-1 text-lg font-black">账号来源</h2>
          </div>
          <span class="status-pill" :data-tone="files.length ? 'success' : 'warning'">{{ files.length ? '已选择' : '待选择' }}</span>
        </div>

        <div class="mt-4">
          <GroupSelect v-model="groupID" :groups="groups" :loading="groupLoading" @create="createGroup" />
        </div>

        <div
          class="import-dropzone mt-4"
          :class="{ 'is-dragging': dragging }"
          @dragenter.prevent="dragging = true"
          @dragover.prevent="dragging = true"
          @dragleave.prevent="dragging = false"
          @drop.prevent="drop"
        >
          <div class="text-base font-bold text-ice">拖入账号文件夹、.session 或 zip</div>
          <p class="mt-2 text-xs leading-5 text-steel">也可以点击选择账号包。支持手机号文件夹 / tdata，或手机号文件夹直接包含 key_data。</p>
          <div class="mt-4 flex flex-wrap gap-2">
            <GlassButton size="sm" variant="primary" :loading="uploading" :disabled="uploading" @click="chooseImportPath">选择账号包</GlassButton>
            <GlassButton size="sm" variant="ghost" :disabled="files.length === 0 || uploading" @click="clearFiles">清空</GlassButton>
          </div>
          <input ref="fileInput" class="hidden" type="file" multiple accept=".session,.zip" @change="pickFiles" />
          <input ref="folderInput" class="hidden" type="file" multiple webkitdirectory @change="pickFiles" />
        </div>

        <div class="mt-4 grid grid-cols-2 gap-2">
          <div class="import-mini-stat">
            <span>导入分组</span>
            <strong>{{ selectedGroupName }}</strong>
          </div>
          <div class="import-mini-stat">
            <span>来源</span>
            <strong>{{ sourceLabel || '未选择' }}</strong>
          </div>
          <div class="import-mini-stat">
            <span>Session</span>
            <strong>{{ fileStats.session }}</strong>
          </div>
          <div class="import-mini-stat">
            <span>Zip / TData</span>
            <strong>{{ fileStats.zip }} / {{ fileStats.tdata }}</strong>
          </div>
        </div>
      </GlassCard>

      <GlassCard class="import-panel import-stage-panel">
        <div class="import-section-head">
          <div>
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Pipeline</div>
            <h2 class="mt-1 text-lg font-black">导入阶段</h2>
          </div>
          <span class="status-pill" data-tone="info">{{ progress }}%</span>
        </div>
        <div class="progress-track mt-4">
          <div class="progress-fill" :style="{ width: `${progress}%` }"></div>
        </div>
        <div class="import-stage-list mt-4">
          <div v-for="(stage, index) in stageRows" :key="stage.key" class="import-stage-card">
            <span class="import-stage-index" :data-status="stage.status">{{ index + 1 }}</span>
            <div class="min-w-0">
              <div class="flex items-center justify-between gap-2">
                <div class="truncate font-bold text-white">{{ stage.label }}</div>
                <div class="text-xs font-bold text-steel">{{ stage.percent }}%</div>
              </div>
              <div class="mt-1 text-xs leading-5 text-steel">{{ stage.current }} / {{ stage.total || stage.current }} · {{ stage.detail }}</div>
            </div>
          </div>
        </div>
      </GlassCard>

      <GlassCard class="import-panel import-result-panel">
        <div class="import-section-head">
          <div>
            <div class="text-xs uppercase tracking-[0.16em] text-steel">Selection & Result</div>
            <h2 class="mt-1 text-lg font-black">{{ summary ? '完成汇总' : '最近选择' }}</h2>
          </div>
          <RouterLink v-if="summary" class="action-link" to="/tasks-logs">任务日志</RouterLink>
          <span v-else class="status-pill" data-tone="warning">{{ files.length }} 个文件</span>
        </div>

        <div v-if="summary" class="mt-4">
          <div class="grid grid-cols-3 gap-2">
            <div v-for="card in resultCards" :key="card.label" class="import-mini-stat" :data-tone="card.accent">
              <span>{{ card.label }}</span>
              <strong :class="card.tone">{{ card.value }}</strong>
            </div>
          </div>
          <div class="mt-4 rounded-xl border border-white/10">
            <div v-for="item in visibleSummaryItems" :key="`${item.name}-${item.status}`" class="import-file-row">
              <span class="min-w-0 truncate">{{ item.name }}</span>
              <span class="status-pill" :class="statusClass(item.status)">{{ item.status }}</span>
            </div>
            <div v-if="summary.items.length > visibleSummaryItems.length" class="px-3 py-2 text-xs text-steel">
              还有 {{ summary.items.length - visibleSummaryItems.length }} 条明细，可到任务日志查看。
            </div>
          </div>
        </div>

        <div v-else class="mt-4 rounded-xl border border-white/10">
          <div v-for="file in visibleFiles" :key="fileKey(file)" class="import-file-row">
            <span class="min-w-0 truncate">{{ displayName(file) }}</span>
            <span class="text-right text-steel">{{ formatSize(file.size) }}</span>
          </div>
          <div v-if="hiddenFileCount > 0" class="px-3 py-2 text-xs text-steel">还有 {{ hiddenFileCount }} 个文件会一起导入。</div>
          <div v-if="!files.length" class="grid min-h-32 place-items-center text-center text-sm text-steel">还没有选择文件、zip 或拖拽账号文件夹</div>
        </div>
      </GlassCard>
    </div>

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

const selectedGroupName = computed(() => groups.value.find((group) => group.id === groupID.value)?.name || '全部分组')

const fileStats = computed(() => {
  const names = files.value.map((file) => displayName(file).toLowerCase())
  return {
    session: names.filter((name) => name.endsWith('.session')).length,
    zip: names.filter((name) => name.endsWith('.zip')).length,
    tdata: names.filter((name) => /\/tdata\/|key_data|key_datas|\.data$/i.test(name)).length
  }
})

const visibleFiles = computed(() => files.value.slice(0, 8))
const hiddenFileCount = computed(() => Math.max(0, files.value.length - visibleFiles.value.length))
const visibleSummaryItems = computed(() => summary.value?.items.slice(0, 8) || [])

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
  const groupName = selectedGroupName.value
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

<style scoped>
.import-page-header {
  align-items: flex-start;
}

.import-workbench {
  display: grid;
  grid-template-columns: minmax(260px, 0.8fr) minmax(320px, 1fr) minmax(320px, 1fr);
  gap: 1rem;
  align-items: stretch;
  min-height: 0;
}

.import-panel {
  min-height: 0;
}

.import-section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.import-dropzone {
  border: 1px dashed rgb(255 255 255 / 0.16);
  border-radius: 0.875rem;
  background: rgb(255 255 255 / 0.045);
  padding: 1rem;
  min-height: 10.5rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  transition: border-color 160ms ease, background 160ms ease;
}

.import-dropzone.is-dragging {
  border-color: rgb(85 240 210 / 0.7);
  background: rgb(85 240 210 / 0.08);
}

.import-mini-stat {
  min-width: 0;
  border: 1px solid rgb(255 255 255 / 0.1);
  border-radius: 0.75rem;
  background: rgb(255 255 255 / 0.045);
  padding: 0.7rem 0.75rem;
}

.import-mini-stat span {
  display: block;
  color: rgb(148 163 184);
  font-size: 0.72rem;
}

.import-mini-stat strong {
  margin-top: 0.25rem;
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(248 250 252);
  font-size: 0.95rem;
}

.import-stage-list {
  display: grid;
  gap: 0.55rem;
}

.import-stage-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  gap: 0.75rem;
  border: 1px solid rgb(255 255 255 / 0.1);
  border-radius: 0.75rem;
  background: rgb(255 255 255 / 0.045);
  padding: 0.68rem 0.75rem;
}

.import-stage-index {
  display: grid;
  place-items: center;
  width: 1.65rem;
  height: 1.65rem;
  border-radius: 0.55rem;
  background: rgb(255 255 255 / 0.08);
  color: rgb(226 232 240);
  font-size: 0.78rem;
  font-weight: 800;
}

.import-stage-index[data-status='success'] {
  background: rgb(34 197 94 / 0.16);
  color: rgb(134 239 172);
}

.import-stage-index[data-status='running'] {
  background: rgb(59 130 246 / 0.16);
  color: rgb(147 197 253);
}

.import-stage-metric {
  border-radius: 999px;
  background: rgb(255 255 255 / 0.08);
  padding: 0.25rem 0.5rem;
  font-size: 0.72rem;
  color: rgb(148 163 184);
}

.import-file-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.75rem;
  align-items: center;
  border-bottom: 1px solid rgb(255 255 255 / 0.08);
  padding: 0.58rem 0.75rem;
  font-size: 0.84rem;
}

.import-file-row:last-child {
  border-bottom: 0;
}

@media (max-width: 1280px) {
  .import-workbench {
    grid-template-columns: 1fr;
  }
}
</style>
