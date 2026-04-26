<template>
  <div class="page-shell profile-assets-shell">
    <div class="profile-topbar">
      <div class="min-w-0">
        <div class="eyebrow">PROFILE WORKSPACE</div>
        <h1 class="page-title profile-title">资料与素材</h1>
        <p class="page-subtitle profile-subtitle">集中管理昵称、签名、个人频道和头像素材，按终端范围批量生成资料修改任务。</p>
      </div>
      <div class="profile-top-actions">
        <span class="profile-pill">范围：{{ scopeLabel }}</span>
        <span class="profile-pill">上传：{{ uploadTargetLabel }}</span>
        <span class="profile-pill">头像：{{ selectedAvatarIDs.length }}</span>
        <GlassButton variant="secondary" size="sm" :loading="loading || assetLoading" @click="load">刷新</GlassButton>
      </div>
    </div>

    <div class="profile-stat-grid">
      <div v-for="item in statCards" :key="item.label" class="profile-stat">
        <span>{{ item.label }}</span>
        <strong :class="item.tone">{{ item.value }}</strong>
      </div>
      <div class="profile-stat">
        <span>资料条目</span>
        <strong class="text-white">{{ totalProfileEntries }}</strong>
      </div>
    </div>

    <div class="profile-command-grid">
      <GlassCard padding="none" class="profile-panel upload-panel">
        <div class="panel-head">
          <div>
            <div class="eyebrow">01 UPLOAD</div>
            <h2>头像上传</h2>
          </div>
          <span class="status-pill" :data-tone="uploading ? 'warning' : 'success'">{{ uploading ? '上传中' : '就绪' }}</span>
        </div>

        <div class="upload-layout">
          <div
            class="upload-dropzone"
            :class="{ dragging: uploadDragging }"
            @dragenter.prevent="uploadDragging = true"
            @dragover.prevent="uploadDragging = true"
            @dragleave.prevent="uploadDragging = false"
            @drop.prevent="dropUploadFiles"
          >
            <input
              ref="avatarFileInput"
              class="hidden"
              type="file"
              multiple
              accept=".jpeg,.jepg,.jpg,.png,.gif,.zip,image/jpeg,image/png,image/gif"
              :disabled="uploading"
              @change="pickUploadFiles"
            />
            <input
              ref="avatarFolderInput"
              class="hidden"
              type="file"
              multiple
              webkitdirectory
              :disabled="uploading"
              @change="pickUploadFiles"
            />

            <div class="upload-mark">AVATAR</div>
            <div>
              <h3>{{ uploading ? '正在导入头像素材' : '选择文件后自动上传' }}</h3>
              <p>{{ sourceKindText }}</p>
            </div>
            <div class="upload-actions">
              <button type="button" :disabled="uploading" @click="openAvatarPicker('file')">
                选择图片 / zip
                <span>多选文件</span>
              </button>
              <button type="button" :disabled="uploading" @click="openAvatarPicker('folder')">
                选择文件夹
                <span>批量导入</span>
              </button>
            </div>
          </div>

          <div class="upload-side">
            <div class="target-toggle">
              <button type="button" :class="{ active: upload.mode === 'existing' }" @click="upload.mode = 'existing'">现有分组</button>
              <button type="button" :class="{ active: upload.mode === 'new' }" @click="upload.mode = 'new'">新建分组</button>
            </div>

            <div v-if="upload.mode === 'existing'" class="profile-field">
              <label>上传目标</label>
              <select v-model="upload.group_id">
                <option value="">全部分组</option>
                <option v-for="group in realAvatarGroups" :key="group.id" :value="group.id">{{ groupLabel(group) }}</option>
              </select>
              <div class="field-hint">
                <span>{{ groupAssetCount(upload.group_id) }} 张</span>
                <button v-if="upload.group_id" type="button" @click="deleteAvatarGroup(upload.group_id)">删除分组</button>
              </div>
            </div>

            <div v-else class="profile-field">
              <label>新分组名称</label>
              <input v-model="upload.new_group_name" placeholder="默认头像分组" />
              <div class="field-hint"><span>{{ uploadHint }}</span></div>
            </div>

            <div class="recent-upload">
              <span>最近选择</span>
              <strong>{{ recentUploadLabel || '还没有选择本地文件' }}</strong>
              <div v-if="recentUploadFiles.length" class="recent-file-list">
                <span v-for="name in recentUploadFiles" :key="name">{{ name }}</span>
              </div>
            </div>

            <div v-if="uploadSummary" class="upload-result-grid">
              <div>
                <span>成功</span>
                <strong class="text-neon">{{ uploadSummary.success }}</strong>
              </div>
              <div>
                <span>失败</span>
                <strong class="text-danger">{{ uploadSummary.failed }}</strong>
              </div>
              <div>
                <span>重复</span>
                <strong class="text-amber">{{ uploadSummary.duplicate }}</strong>
              </div>
            </div>
          </div>
        </div>
      </GlassCard>

      <GlassCard padding="none" class="profile-panel task-panel">
        <div class="panel-head">
          <div>
            <div class="eyebrow">02 TASK</div>
            <h2>任务控制</h2>
          </div>
          <span class="status-pill" :data-tone="canSubmit ? 'success' : 'warning'">{{ canSubmit ? '可创建' : '待补全' }}</span>
        </div>

        <div class="task-body">
          <GlassButton
            class="w-full min-h-14 border-neon/40 bg-neon/15 text-base text-neon shadow-green hover:bg-neon/20"
            :loading="submitting"
            :disabled="!canSubmit"
            @click="submitProfiles"
          >
            创建资料修改任务
          </GlassButton>
          <GlassButton class="w-full min-h-12" :loading="refreshingProfiles" :disabled="!selectedTerminals.length || submitting" @click="refreshProfiles">
            真实刷新当前范围
          </GlassButton>

          <div class="task-summary">
            <div><span>当前范围</span><strong>{{ scopeLabel }}</strong></div>
            <div><span>命中终端</span><strong>{{ selectedTerminals.length }}</strong></div>
            <div><span>资料条目</span><strong>{{ totalProfileEntries }}</strong></div>
            <div><span>头像目标</span><strong>{{ selectedAvatarIDs.length }}</strong></div>
          </div>

          <div v-if="profileSummary" class="summary-box" :class="profileSummaryCardClass">
            <div class="summary-head">
              <div>
                <strong :class="profileSummaryTitleClass">{{ profileSummaryHeadline }}</strong>
                <span>任务 ID：{{ profileSummary.task_id }}</span>
              </div>
              <span class="status-pill" :data-tone="profileSummaryTone">{{ profileSummaryStatusText }}</span>
            </div>
            <div class="summary-grid">
              <div><span>成功</span><strong class="text-neon">{{ profileSummary.applied_count }}</strong></div>
              <div><span>部分</span><strong class="text-amber">{{ profileSummary.partial_count }}</strong></div>
              <div><span>失败</span><strong class="text-danger">{{ profileSummary.failed_count }}</strong></div>
              <div><span>字段</span><strong>{{ profileSummary.applied_field_count }} / {{ profileSummary.requested_field_count }}</strong></div>
            </div>
            <div v-if="profileFailureCategories.length" class="failure-tabs">
              <button
                v-for="category in profileFailureCategories"
                :key="category.key"
                type="button"
                :class="activeFailureCategory === category.key ? category.activeClass : ''"
                @click="activeFailureCategory = category.key"
              >
                {{ category.label }} · {{ category.count }}
              </button>
            </div>
            <div v-if="profileFailurePreview.length" class="failure-list">
              <div v-for="item in profileFailurePreview" :key="item.key">
                <div><strong>{{ item.phone }}</strong><span class="status-pill" :data-tone="item.tone">{{ item.categoryLabel }}</span></div>
                <p>{{ item.field }}：{{ item.reason }}</p>
              </div>
            </div>
          </div>

          <div v-if="refreshSummary" class="refresh-box">
            <strong>真实刷新完成</strong>
            <span>共 {{ refreshSummary.total }} 个，在线 {{ refreshSummary.online }}，离线 {{ refreshSummary.offline }}，异常 {{ refreshSummary.abnormal }}</span>
          </div>

          <p v-if="error" class="error-banner">{{ error }}</p>
        </div>
      </GlassCard>
    </div>

    <div class="profile-workspace">
      <div class="profile-main-stack">
        <GlassCard padding="none" class="profile-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">03 SCOPE</div>
              <h2>选择修改范围</h2>
            </div>
            <span class="profile-pill">平均分配</span>
          </div>

          <div class="scope-body">
            <div class="scope-options">
              <button
                v-for="entry in scopeOptions"
                :key="entry.value"
                type="button"
                :class="{ active: scope.mode === entry.value }"
                @click="scope.mode = entry.value"
              >
                <strong>{{ entry.label }}</strong>
                <span>{{ entry.help }}</span>
              </button>
            </div>

            <div class="scope-select-grid">
              <div class="profile-field">
                <label>终端组</label>
                <select v-model="scope.terminal_group_id" :disabled="scope.mode !== 'group'">
                  <option value="">选择终端组</option>
                  <option v-for="group in terminalGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
                </select>
              </div>
              <div class="profile-field">
                <label>终端</label>
                <select v-model="scope.terminal_id" :disabled="scope.mode !== 'terminal'">
                  <option value="">选择终端</option>
                  <option v-for="terminal in terminals" :key="terminal.id" :value="terminal.id">{{ terminalLabel(terminal) }}</option>
                </select>
              </div>
            </div>
          </div>
        </GlassCard>

        <GlassCard padding="none" class="profile-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">04 POOL</div>
              <h2>资料池</h2>
            </div>
            <span class="profile-pill">{{ activeOptionMeta.label }} {{ optionCount(activeOption) }} 条</span>
          </div>

          <div class="pool-body">
            <div class="profile-tabs">
              <button
                v-for="option in profileOptions"
                :key="option.key"
                type="button"
                :class="{ active: activeOption === option.key }"
                @click="activeOption = option.key"
              >
                <strong>{{ option.label }}</strong>
                <span>{{ option.help }}</span>
              </button>
            </div>

            <div v-if="activeOption !== 'avatars'" class="pool-editor">
              <textarea v-model="textModels[activeOption]" :placeholder="activeOptionMeta.placeholder"></textarea>
            </div>

            <div v-else class="avatar-pool-inline">
              <div class="avatar-inline-toolbar">
                <select v-model="assetGroupID">
                  <option v-for="group in avatarFilterGroups" :key="group.id" :value="group.id">{{ groupLabel(group) }}</option>
                </select>
                <GlassButton :loading="assetLoading" @click="loadAssets">刷新头像库</GlassButton>
              </div>
              <div class="avatar-inline-state">
                <span>当前分组 {{ groupAssetCount(assetGroupID) }} 张，本页 {{ pagedAssets.length }} / {{ assets.length }}</span>
                <button type="button" @click="toggleSelectVisibleAvatars">{{ allVisibleAvatarsSelected ? '取消本页全选' : '全选本页' }}</button>
                <button v-if="selectedAvatarIDs.length" type="button" @click="clearSelectedAvatars">清空已选</button>
              </div>
            </div>
          </div>
        </GlassCard>

        <GlassCard padding="none" class="profile-panel avatar-library-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">05 AVATARS</div>
              <h2>头像库</h2>
            </div>
            <div class="panel-tools">
              <span class="profile-pill">已选 {{ selectedAvatarIDs.length }}</span>
              <button v-if="!isAllAvatarGroup(assetGroupID)" type="button" class="danger-link" @click="deleteAvatarGroup(assetGroupID)">删除当前分组</button>
            </div>
          </div>

          <div class="avatar-library-toolbar">
            <select v-model="assetGroupID">
              <option v-for="group in avatarFilterGroups" :key="group.id" :value="group.id">{{ groupLabel(group) }}</option>
            </select>
            <div>
              <button type="button" @click="toggleSelectVisibleAvatars">{{ allVisibleAvatarsSelected ? '取消本页全选' : '全选本页' }}</button>
              <button v-if="selectedAvatarIDs.length" type="button" @click="clearSelectedAvatars">清空已选</button>
            </div>
          </div>

          <div class="avatar-grid">
            <div
              v-for="asset in pagedAssets"
              :key="asset.id"
              class="avatar-tile"
              :class="{ selected: selectedAvatarIDs.includes(asset.id) }"
            >
              <button type="button" @click="toggleAvatar(asset.id)">
                <img v-if="asset.url && !brokenAssetIDs.includes(asset.id)" :src="assetURL(asset.url)" :alt="asset.name || '头像'" @error="markBrokenAsset(asset.id)" />
                <div v-else class="avatar-fallback">
                  <strong>{{ assetInitial(asset) }}</strong>
                  <small>{{ asset.name || '图片失效' }}</small>
                </div>
                <span>{{ selectedAvatarIDs.includes(asset.id) ? '已选择' : '选择' }}</span>
              </button>
              <button type="button" class="delete-avatar" @click.stop="deleteAsset(asset)">删除</button>
            </div>
          </div>

          <div v-if="!assets.length" class="empty-state">暂无头像素材</div>
          <div v-if="assetPageCount > 1" class="pager-row">
            <span>第 {{ assetPage }} / {{ assetPageCount }} 页</span>
            <GlassButton size="sm" variant="secondary" :disabled="assetPage <= 1" @click="assetPage--">上一页</GlassButton>
            <GlassButton size="sm" variant="secondary" :disabled="assetPage >= assetPageCount" @click="assetPage++">下一页</GlassButton>
          </div>
        </GlassCard>
      </div>

      <GlassCard padding="none" class="profile-panel preview-panel">
        <div class="panel-head">
          <div>
            <div class="eyebrow">PREVIEW</div>
            <h2>分配预览</h2>
          </div>
          <span class="profile-pill">已命中 {{ selectedTerminals.length }}</span>
        </div>

        <div class="preview-body scrollbar-thin">
          <div class="preview-stat-grid">
            <div v-for="item in statCards" :key="item.label">
              <span>{{ item.label }}</span>
              <strong :class="item.tone">{{ item.value }}</strong>
            </div>
          </div>

          <div class="preview-list">
            <div v-for="item in previewAssignments" :key="item.terminal.id" class="preview-item">
              <strong>{{ terminalLabel(item.terminal) }}</strong>
              <div>
                <span>昵称</span><b>{{ item.nickname || '保持原样' }}</b>
              </div>
              <div>
                <span>签名</span><b>{{ item.bio || '保持原样' }}</b>
              </div>
              <div>
                <span>频道</span><b>{{ item.homepage || '保持原样' }}</b>
              </div>
              <div>
                <span>头像</span><b>{{ item.avatar ? '已分配' : '保持原样' }}</b>
              </div>
            </div>
            <div v-if="!previewAssignments.length" class="empty-state">选择终端并填写资料后显示预览</div>
          </div>
        </div>
      </GlassCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { api, type Asset, type AssetUploadSummary, type Group, type ProfileModifySummary, type Terminal, type TerminalCheckSummary } from '../api/client'
import { useUiStore } from '../stores/ui'

type ScopeMode = 'all' | 'group' | 'terminal'
type ProfileOption = 'nicknames' | 'bios' | 'homepages' | 'avatars'
type UploadMode = 'existing' | 'new'
type UploadSourceKind = 'file' | 'folder'
type FailureCategory = 'all' | 'frozen' | 'session_locked' | 'format' | 'occupied' | 'auth' | 'unknown'

const allAvatarGroupID = '00000000-0000-0000-0000-000000000000'

const scopeOptions: Array<{ value: ScopeMode; label: string; help: string }> = [
  { value: 'all', label: '全部终端', help: '当前租户下全部账号' },
  { value: 'group', label: '终端组', help: '只处理一个终端组' },
  { value: 'terminal', label: '单个终端', help: '只处理一个账号' }
]

const profileOptions: Array<{ key: ProfileOption; label: string; help: string; placeholder: string }> = [
  { key: 'nicknames', label: '昵称', help: '多昵称平均分配', placeholder: '一行一个昵称\nAlice\nBob\nCharlie' },
  { key: 'bios', label: '个性签名', help: '多签名平均分配', placeholder: '一行一条个性签名\n今天也在认真生活\n保持联系，慢慢更新' },
  { key: 'homepages', label: '个人频道', help: '多频道平均分配', placeholder: '一行一个个人频道\nhttps://t.me/AIGOGGGG\n@AIGOGGGG' },
  { key: 'avatars', label: '头像', help: '从头像库里选择多张', placeholder: '' }
]

const terminals = ref<Terminal[]>([])
const terminalGroups = ref<Group[]>([])
const avatarGroups = ref<Group[]>([])
const assets = ref<Asset[]>([])
const selectedAvatarIDs = ref<string[]>([])
const brokenAssetIDs = ref<string[]>([])
const assetPage = ref(1)
const assetPageSize = 80
const activeOption = ref<ProfileOption>('nicknames')
const assetGroupID = ref(allAvatarGroupID)
const profileSummary = ref<ProfileModifySummary | null>(null)
const activeFailureCategory = ref<FailureCategory>('all')
const uploadSummary = ref<AssetUploadSummary | null>(null)
const refreshSummary = ref<TerminalCheckSummary | null>(null)
const loading = ref(false)
const assetLoading = ref(false)
const submitting = ref(false)
const refreshingProfiles = ref(false)
const uploading = ref(false)
const uploadDragging = ref(false)
const error = ref('')
const recentUploadLabel = ref('')
const recentUploadFiles = ref<string[]>([])
const avatarFileInput = ref<HTMLInputElement | null>(null)
const avatarFolderInput = ref<HTMLInputElement | null>(null)
const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''
const ui = useUiStore()

const scope = reactive({
  mode: 'all' as ScopeMode,
  terminal_group_id: '',
  terminal_id: ''
})

const textModels = reactive<Record<ProfileOption, string>>({
  nicknames: '',
  bios: '',
  homepages: '',
  avatars: ''
})

const upload = reactive({
  mode: 'existing' as UploadMode,
  source_kind: 'file' as UploadSourceKind,
  group_id: '',
  new_group_name: '',
  files: [] as File[]
})

const activeOptionMeta = computed(() => profileOptions.find((option) => option.key === activeOption.value) || profileOptions[0])
const nicknameLines = computed(() => splitLines(textModels.nicknames))
const bioLines = computed(() => splitLines(textModels.bios))
const homepageLines = computed(() => splitLines(textModels.homepages))
const realAvatarGroups = computed(() => avatarGroups.value.filter((group) => !isAllAvatarGroup(group.id)))
const avatarFilterGroups = computed(() => {
  const allGroup = avatarGroups.value.find((group) => isAllAvatarGroup(group.id))
  if (allGroup) return [allGroup, ...realAvatarGroups.value]
  return [{ id: allAvatarGroupID, resource_type: 'avatar', name: '全部分组', description: '', asset_count: assets.value.length }, ...realAvatarGroups.value]
})

const selectedTerminals = computed(() => {
  if (scope.mode === 'all') return terminals.value
  if (scope.mode === 'group') return terminals.value.filter((terminal) => terminal.group_id === scope.terminal_group_id)
  if (scope.mode === 'terminal') return terminals.value.filter((terminal) => terminal.id === scope.terminal_id)
  return []
})

const scopeLabel = computed(() => {
  if (scope.mode === 'all') return '全部终端'
  if (scope.mode === 'group') return terminalGroups.value.find((group) => group.id === scope.terminal_group_id)?.name || '未选择终端组'
  return terminalLabel(terminals.value.find((terminal) => terminal.id === scope.terminal_id) || emptyTerminal)
})

const totalProfileEntries = computed(() => nicknameLines.value.length + bioLines.value.length + homepageLines.value.length + selectedAvatarIDs.value.length)

const statCards = computed(() => [
  { label: '命中终端', value: selectedTerminals.value.length, tone: 'text-neon' },
  { label: '昵称池', value: nicknameLines.value.length, tone: 'text-ice' },
  { label: '签名池', value: bioLines.value.length, tone: 'text-white' },
  { label: '头像池', value: selectedAvatarIDs.value.length, tone: 'text-amber' }
])

const canSubmit = computed(() => selectedTerminals.value.length > 0 && totalProfileEntries.value > 0 && !submitting.value)

const profileSummaryTone = computed(() => summaryTone(profileSummary.value?.status))

const profileSummaryStatusText = computed(() => summaryStatusText(profileSummary.value?.status))

const profileSummaryHeadline = computed(() => {
  if (!profileSummary.value) return '任务已创建'
  if (profileSummary.value.status === 'success') return '资料修改已全部提交'
  if (profileSummary.value.status === 'partial_success') return '资料修改已部分提交'
  return '资料修改提交失败'
})

const profileSummaryCardClass = computed(() => {
  if (profileSummaryTone.value === 'success') return 'border-neon/20 bg-neon/5'
  if (profileSummaryTone.value === 'warning') return 'border-amber/20 bg-amber/5'
  return 'border-danger/20 bg-danger/5'
})

const profileSummaryTitleClass = computed(() => {
  if (profileSummaryTone.value === 'success') return 'text-neon'
  if (profileSummaryTone.value === 'warning') return 'text-amber'
  return 'text-danger'
})

const uploadTargetLabel = computed(() => {
  if (upload.mode === 'existing' && upload.group_id) {
    return avatarGroups.value.find((group) => group.id === upload.group_id)?.name || '已选头像分组'
  }
  if (upload.mode === 'existing') return '全部分组'
  return upload.new_group_name.trim() || '默认头像分组'
})

const uploadHint = computed(() => {
  if (uploading.value) return `正在上传到：${uploadTargetLabel.value}`
  if (upload.mode === 'existing' && upload.group_id) return `将上传到：${uploadTargetLabel.value}`
  if (upload.mode === 'existing') return '未选择现有分组时，会直接上传到：全部分组。'
  if (upload.new_group_name.trim()) return `将新建分组：${upload.new_group_name.trim()}`
  return '未填写新分组名称时，会自动创建默认头像分组。'
})

const sourceKindText = computed(() => (upload.source_kind === 'folder' ? '点击这里选择文件夹，确认后直接上传' : '点击这里选择文件或 zip，确认后直接上传'))

const profileFailureItems = computed(() => {
  const summary = profileSummary.value
  if (!summary?.results?.length) return []
  return summary.results
    .flatMap((result) =>
      Object.entries(result.failed_fields || {}).map(([field, reason]) => {
        const category = normalizeFailureCategory(result.failure_category) || failureCategory(reason || result.message || '')
        return {
          key: `${result.terminal_id}-${field}`,
          phone: result.phone || result.terminal_id,
          field: profileFieldLabel(field),
          reason: reason || result.message || '执行失败',
          category,
          categoryLabel: failureCategoryLabel(category),
          tone: failureCategoryTone(category)
        }
      })
    )
})

const profileFailureCategories = computed(() => {
  const items = profileFailureItems.value
  if (!items.length) return []
  const counts = new Map<FailureCategory, number>()
  for (const item of items) {
    counts.set(item.category, (counts.get(item.category) || 0) + 1)
  }
  const entries: Array<{ key: FailureCategory; label: string; count: number; activeClass: string }> = [
    { key: 'all', label: '全部失败', count: items.length, activeClass: failureCategoryActiveClass('all') }
  ]
  for (const key of ['frozen', 'session_locked', 'format', 'occupied', 'auth', 'unknown'] as FailureCategory[]) {
    const count = counts.get(key) || 0
    if (count > 0) {
      entries.push({ key, label: failureCategoryLabel(key), count, activeClass: failureCategoryActiveClass(key) })
    }
  }
  return entries
})

const profileFailurePreview = computed(() => {
  const selected = activeFailureCategory.value
  const items = selected === 'all' ? profileFailureItems.value : profileFailureItems.value.filter((item) => item.category === selected)
  return items.slice(0, 8)
})

const previewAssignments = computed(() => {
  const terminalsForPreview = selectedTerminals.value.slice(0, 8)
  const total = selectedTerminals.value.length
  return terminalsForPreview.map((terminal, index) => ({
    terminal,
    nickname: pickDistributed(nicknameLines.value, index, total),
    bio: pickDistributed(bioLines.value, index, total),
    homepage: pickDistributed(homepageLines.value, index, total),
    avatar: pickDistributed(selectedAvatarIDs.value, index, total)
  }))
})

const assetPageCount = computed(() => Math.max(1, Math.ceil(assets.value.length / assetPageSize)))
const pagedAssets = computed(() => {
  const start = (assetPage.value - 1) * assetPageSize
  return assets.value.slice(start, start + assetPageSize)
})
const allVisibleAvatarsSelected = computed(() => pagedAssets.value.length > 0 && pagedAssets.value.every((asset) => selectedAvatarIDs.value.includes(asset.id)))

const emptyTerminal: Terminal = {
  id: '',
  phone: '',
  phone_display: '',
  nickname: '',
  avatar_url: '',
  bio: '',
  homepage: '',
  channel_name: '',
  status: '',
  status_text: '',
  account_status: '',
  account_status_text: '',
  online_status: '',
  online_status_text: '',
  last_online_at: null,
  last_message_at: null,
  sleep_until: null,
  access_type: '',
  origin_country: '',
  origin_flag: '',
  exit_ip: '',
  exit_country: '',
  exit_flag: '',
  group_id: null,
  today_success: 0,
  total_success: 0,
  today_failed: 0,
  total_failed: 0,
  risk_status: '',
  ban_status: '',
  dm_hourly_limit: 0,
  dm_daily_limit: 0,
  join_hourly_limit: 0,
  join_daily_limit: 0,
  dm_hourly_count: 0,
  dm_daily_count: 0,
  join_hourly_count: 0,
  join_daily_count: 0,
  dm_hourly_reset_at: null,
  dm_daily_reset_at: null,
  join_hourly_reset_at: null,
  join_daily_reset_at: null
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [terminalData, terminalGroupData, avatarGroupData] = await Promise.all([api.terminals(), api.groups('terminal'), api.groups('avatar')])
    terminals.value = terminalData
    terminalGroups.value = terminalGroupData
    avatarGroups.value = avatarGroupData
    await loadAssets()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '读取资料与素材失败'
  } finally {
    loading.value = false
  }
}

async function loadAssets() {
  assetLoading.value = true
  try {
    assets.value = await api.assets(isAllAvatarGroup(assetGroupID.value) ? '' : assetGroupID.value)
    brokenAssetIDs.value = brokenAssetIDs.value.filter((id) => assets.value.some((asset) => asset.id === id))
  } catch (err) {
    error.value = err instanceof Error ? err.message : '读取头像素材失败'
  } finally {
    assetLoading.value = false
  }
}

async function submitProfiles() {
  if (!canSubmit.value) return
  submitting.value = true
  error.value = ''
  profileSummary.value = null
  try {
    const result = await api.modifyProfiles({
      scope: scope.mode,
      terminal_id: scope.mode === 'terminal' ? scope.terminal_id : '',
      terminal_group_id: scope.mode === 'group' ? scope.terminal_group_id : '',
      nicknames: nicknameLines.value,
      bios: bioLines.value,
      homepages: homepageLines.value,
      avatar_asset_ids: selectedAvatarIDs.value
    })
    profileSummary.value = result.summary
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建资料修改任务失败'
  } finally {
    submitting.value = false
  }
}

async function refreshProfiles() {
  if (!selectedTerminals.value.length) return
  refreshingProfiles.value = true
  error.value = ''
  refreshSummary.value = null
  try {
    const result = await api.checkTerminals({
      groupID: scope.mode === 'group' ? scope.terminal_group_id : '',
      terminalID: scope.mode === 'terminal' ? scope.terminal_id : ''
    })
    refreshSummary.value = result.summary
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '真实刷新账号资料失败'
  } finally {
    refreshingProfiles.value = false
  }
}

function pickUploadFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  void queueUploadFiles(files, describeSelection(files))
}

function openAvatarPicker(kind: UploadSourceKind) {
  if (uploading.value) return
  upload.source_kind = kind
  if (kind === 'folder') {
    avatarFolderInput.value?.click()
    return
  }
  avatarFileInput.value?.click()
}

function summaryStatusText(status?: string) {
  if (status === 'success') return '全部成功'
  if (status === 'partial_success') return '部分成功'
  if (status === 'failed') return '全部失败'
  return '处理中'
}

function summaryTone(status?: string) {
  if (status === 'success') return 'success'
  if (status === 'partial_success') return 'warning'
  if (status === 'failed') return 'danger'
  return 'info'
}

function profileFieldLabel(field: string) {
  if (field === 'nickname') return '昵称'
  if (field === 'bio') return '个性签名'
  if (field === 'homepage') return '个人频道'
  if (field === 'avatar') return '头像'
  return field
}

function failureCategory(reason: string): FailureCategory {
  const text = (reason || '').toLowerCase()
  if (/冻结|frozen|not available for frozen accounts/.test(text)) return 'frozen'
  if (/会话文件正在被占用|database is locked|locked/.test(text)) return 'session_locked'
  if (/格式|format|invalid|无效/.test(text)) return 'format'
  if (/占用|occupied|taken|username/.test(text)) return 'occupied'
  if (/授权|登录|重新登录|unauthorized|auth|login/.test(text)) return 'auth'
  return 'unknown'
}

function normalizeFailureCategory(value?: string): FailureCategory | '' {
  if (value === 'frozen') return 'frozen'
  if (value === 'session_locked') return 'session_locked'
  if (value === 'format') return 'format'
  if (value === 'occupied') return 'occupied'
  if (value === 'auth') return 'auth'
  if (value === 'unknown') return 'unknown'
  return ''
}

function failureCategoryLabel(category: FailureCategory) {
  if (category === 'all') return '全部失败'
  if (category === 'frozen') return '账号冻结'
  if (category === 'session_locked') return '会话占用'
  if (category === 'format') return '格式错误'
  if (category === 'occupied') return '资源占用'
  if (category === 'auth') return '授权异常'
  return '其他原因'
}

function failureCategoryTone(category: FailureCategory) {
  if (category === 'frozen' || category === 'auth') return 'danger'
  if (category === 'session_locked' || category === 'format' || category === 'occupied') return 'warning'
  return 'info'
}

function failureCategoryActiveClass(category: FailureCategory) {
  if (category === 'frozen' || category === 'auth') return 'border-danger/35 bg-danger/15 text-danger'
  if (category === 'session_locked' || category === 'format' || category === 'occupied') return 'border-amber/35 bg-amber/15 text-amber'
  if (category === 'all') return 'border-ice/35 bg-ice/15 text-ice'
  return 'border-white/20 bg-white/10 text-white'
}

function dropUploadFiles(event: DragEvent) {
  uploadDragging.value = false
  void queueUploadFiles(Array.from(event.dataTransfer?.files || []), '拖拽内容')
}

async function queueUploadFiles(files: File[], label: string) {
  if (uploading.value) {
    error.value = '正在上传，请稍后再试'
    return
  }

  const supported = files.filter(isAvatarUploadSource)
  if (!supported.length) {
    error.value = '没有识别到支持的图片、zip 或文件夹内容'
    return
  }

  const unique: File[] = []
  const seen = new Set<string>()
  for (const file of supported) {
    const key = fileKey(file)
    if (!seen.has(key)) {
      unique.push(file)
      seen.add(key)
    }
  }

  upload.files = unique
  recentUploadLabel.value = label || describeSelection(unique)
  recentUploadFiles.value = unique.map(displayUploadName).slice(0, 10)
  error.value = ''
  await uploadAvatars()
}

async function uploadAvatars() {
  if (!upload.files.length) {
    error.value = '请先选择图片、头像文件夹或 zip 压缩包'
    return
  }

  const newGroupName = upload.mode === 'new' ? upload.new_group_name.trim() || '默认头像分组' : ''
  uploading.value = true
  error.value = ''
  uploadSummary.value = null

  try {
    const result = await api.uploadAssets(upload.files, upload.mode === 'existing' ? upload.group_id : '', newGroupName)
    uploadSummary.value = result
    upload.files = []
    upload.new_group_name = ''
    avatarGroups.value = await api.groups('avatar')

    if (result.group_id) {
      assetGroupID.value = result.group_id
      upload.group_id = result.group_id
      upload.mode = 'existing'
    } else if (result.group_name === '全部分组') {
      assetGroupID.value = allAvatarGroupID
    }

    await loadAssets()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '上传头像失败'
  } finally {
    uploading.value = false
  }
}

async function deleteAsset(asset: Asset) {
  const confirmed = await ui.confirm({
    title: '删除头像图片',
    message: `确认删除图片「${asset.name}」？删除后会同步移除缓存和列表记录。`,
    confirmText: '删除图片',
    tone: 'error'
  })
  if (!confirmed) return
  error.value = ''
  try {
    await api.deleteAsset(asset.id)
    selectedAvatarIDs.value = selectedAvatarIDs.value.filter((id) => id !== asset.id)
    await loadAssets()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除图片失败'
  }
}

async function deleteAvatarGroup(groupID: string) {
  if (isAllAvatarGroup(groupID)) {
    error.value = '全部分组是系统分组，不能删除'
    return
  }
  const count = groupAssetCount(groupID)
  const group = avatarGroups.value.find((item) => item.id === groupID)
  if (!group) return
  const confirmed = await ui.confirm({
    title: '删除头像分组',
    message: `确认删除头像分组「${group.name}」？会同时删除该分组下 ${count} 张图片和本地缓存。`,
    confirmText: '删除分组',
    tone: 'error'
  })
  if (!confirmed) return
  error.value = ''
  try {
    await api.deleteGroup('avatar', groupID)
    if (assetGroupID.value === groupID) assetGroupID.value = allAvatarGroupID
    if (upload.group_id === groupID) upload.group_id = ''
    selectedAvatarIDs.value = []
    avatarGroups.value = await api.groups('avatar')
    await loadAssets()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除头像分组失败'
  }
}

function toggleAvatar(id: string) {
  if (selectedAvatarIDs.value.includes(id)) {
    selectedAvatarIDs.value = selectedAvatarIDs.value.filter((item) => item !== id)
  } else {
    selectedAvatarIDs.value = [...selectedAvatarIDs.value, id]
  }
}

function toggleSelectVisibleAvatars() {
  if (!pagedAssets.value.length) return
  if (allVisibleAvatarsSelected.value) {
    const visibleIDs = new Set(pagedAssets.value.map((asset) => asset.id))
    selectedAvatarIDs.value = selectedAvatarIDs.value.filter((id) => !visibleIDs.has(id))
    return
  }
  const merged = new Set(selectedAvatarIDs.value)
  for (const asset of pagedAssets.value) {
    merged.add(asset.id)
  }
  selectedAvatarIDs.value = Array.from(merged)
}

function clearSelectedAvatars() {
  selectedAvatarIDs.value = []
}

function optionCount(key: ProfileOption) {
  if (key === 'nicknames') return nicknameLines.value.length
  if (key === 'bios') return bioLines.value.length
  if (key === 'homepages') return homepageLines.value.length
  return selectedAvatarIDs.value.length
}

function terminalLabel(terminal: Terminal) {
  return terminal.nickname || terminal.phone || (terminal.id ? terminal.id.slice(0, 8) : '未选择终端')
}

function groupLabel(group: Group) {
  return `${group.name}（${group.asset_count || 0}张）`
}

function groupAssetCount(groupID: string) {
  if (isAllAvatarGroup(groupID)) {
    return avatarGroups.value.find((group) => isAllAvatarGroup(group.id))?.asset_count || assets.value.length
  }
  return avatarGroups.value.find((group) => group.id === groupID)?.asset_count || 0
}

function isAllAvatarGroup(groupID: string) {
  return !groupID || groupID === allAvatarGroupID
}

function splitLines(value: string) {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

function fileKey(file: File) {
  return `${displayUploadName(file)}-${file.size}-${file.lastModified}`
}

function displayUploadName(file: File) {
  return (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name
}

function describeSelection(files: File[]) {
  if (!files.length) return ''
  const firstPath = (files[0] as File & { webkitRelativePath?: string }).webkitRelativePath || ''
  if (firstPath) {
    const topLevel = firstPath.split('/')[0] || '已选文件夹'
    return `文件夹：${topLevel}`
  }
  if (files.length === 1) return `文件：${displayUploadName(files[0])}`
  return `文件：${files.length} 个`
}

function pickDistributed(values: string[], index: number, total: number) {
  if (!values.length) return ''
  if (total <= 1) return values[0]
  const slot = Math.min(values.length - 1, Math.floor((index * values.length) / total))
  return values[slot]
}

function assetURL(url: string) {
  if (!url || url.startsWith('http://') || url.startsWith('https://')) return url || ''
  return `${apiBase}${url}`
}

function markBrokenAsset(id: string) {
  if (!brokenAssetIDs.value.includes(id)) {
    brokenAssetIDs.value = [...brokenAssetIDs.value, id]
  }
}

function assetInitial(asset: Asset) {
  return (asset.name || '头像').trim().slice(0, 1).toUpperCase()
}

function isAvatarUploadSource(file: File) {
  const name = file.name.toLowerCase()
  return name.endsWith('.jpeg') || name.endsWith('.jepg') || name.endsWith('.jpg') || name.endsWith('.png') || name.endsWith('.gif') || name.endsWith('.zip')
}

watch(assetGroupID, () => {
  assetPage.value = 1
  void loadAssets()
})
watch(assetPageCount, (count) => {
  if (assetPage.value > count) assetPage.value = count
})

watch(
  () => scope.mode,
  (mode) => {
    if (mode !== 'group') scope.terminal_group_id = ''
    if (mode !== 'terminal') scope.terminal_id = ''
  }
)

watch(profileSummary, () => {
  activeFailureCategory.value = 'all'
})

watch(
  () => upload.mode,
  (mode) => {
    if (mode === 'existing') {
      upload.new_group_name = ''
    } else {
      upload.group_id = ''
    }
  }
)

onMounted(load)
</script>

<style scoped>
.profile-assets-shell {
  gap: 1rem;
  overflow: visible;
  padding: 0.25rem 0 1.25rem;
}

.profile-topbar,
.panel-head,
.profile-top-actions,
.panel-tools,
.avatar-library-toolbar,
.summary-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.profile-topbar {
  align-items: flex-end;
}

.eyebrow {
  color: rgba(147, 164, 198, 0.9);
  font-size: 0.68rem;
  font-weight: 900;
  letter-spacing: 0;
  text-transform: uppercase;
}

.profile-title {
  margin-top: 0.15rem;
  font-size: 2.35rem;
}

.profile-subtitle {
  max-width: 52rem;
  font-size: 0.88rem;
}

.profile-top-actions {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.profile-pill {
  display: inline-flex;
  min-height: 2.25rem;
  align-items: center;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-top-color: rgba(255, 255, 255, 0.18);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.5);
  padding: 0.48rem 0.72rem;
  color: rgba(200, 210, 234, 0.95);
  font-size: 0.78rem;
  font-weight: 800;
  white-space: nowrap;
}

.profile-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.65rem;
}

.profile-stat,
.task-summary > div,
.summary-grid > div,
.preview-stat-grid > div {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.055), transparent 30%),
    rgba(15, 23, 42, 0.54);
  box-shadow: 0 8px 22px rgba(2, 6, 23, 0.22);
}

.profile-stat {
  min-height: 4.25rem;
  padding: 0.75rem 0.9rem;
}

.profile-stat span,
.task-summary span,
.summary-grid span,
.preview-stat-grid span,
.profile-field label,
.recent-upload span {
  display: block;
  color: rgba(147, 164, 198, 0.94);
  font-size: 0.7rem;
  font-weight: 800;
}

.profile-stat strong,
.task-summary strong,
.summary-grid strong,
.preview-stat-grid strong {
  display: block;
  margin-top: 0.3rem;
  overflow: hidden;
  color: white;
  font-size: 1rem;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-command-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(340px, 0.38fr);
  align-items: start;
  gap: 1rem;
}

.profile-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(360px, 0.32fr);
  align-items: start;
  gap: 1rem;
}

.profile-main-stack {
  display: grid;
  gap: 1rem;
  min-width: 0;
}

.profile-panel {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
}

.panel-head {
  flex-shrink: 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background:
    linear-gradient(90deg, rgba(0, 242, 254, 0.08), transparent 45%),
    rgba(15, 23, 42, 0.22);
  padding: 0.85rem 1rem;
}

.panel-head h2 {
  margin: 0.12rem 0 0;
  color: white;
  font-size: 1rem;
  font-weight: 900;
}

.upload-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(280px, 0.34fr);
  gap: 0.8rem;
  padding: 0.85rem;
}

.upload-dropzone {
  display: flex;
  min-height: 18.2rem;
  flex-direction: column;
  justify-content: space-between;
  gap: 1.1rem;
  border: 1px dashed rgba(79, 172, 254, 0.44);
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(0, 242, 254, 0.09), rgba(79, 172, 254, 0.035)),
    rgba(2, 6, 23, 0.22);
  padding: 1.1rem;
}

.upload-dropzone.dragging {
  border-color: rgba(52, 211, 153, 0.76);
  background:
    linear-gradient(135deg, rgba(52, 211, 153, 0.16), rgba(0, 242, 254, 0.07)),
    rgba(2, 6, 23, 0.2);
  box-shadow: 0 18px 34px rgba(52, 211, 153, 0.14);
}

.upload-mark {
  width: fit-content;
  border: 1px solid rgba(79, 172, 254, 0.24);
  border-radius: 8px;
  background: rgba(0, 242, 254, 0.08);
  padding: 0.42rem 0.62rem;
  color: #7deeff;
  font-size: 0.72rem;
  font-weight: 900;
  letter-spacing: 0;
}

.upload-dropzone h3 {
  margin: 0;
  color: white;
  font-size: 1.65rem;
  font-weight: 950;
}

.upload-dropzone p {
  margin: 0.35rem 0 0;
  color: rgba(147, 164, 198, 0.94);
  font-size: 0.86rem;
  line-height: 1.55;
}

.upload-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.upload-actions button,
.scope-options button,
.profile-tabs button {
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-top-color: rgba(255, 255, 255, 0.16);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.045);
  padding: 0.78rem 0.85rem;
  color: rgba(224, 231, 255, 0.94);
  text-align: left;
}

.upload-actions button:hover:not(:disabled),
.scope-options button:hover,
.profile-tabs button:hover,
.scope-options button.active,
.profile-tabs button.active {
  border-color: rgba(79, 172, 254, 0.46);
  background: linear-gradient(135deg, rgba(0, 242, 254, 0.12), rgba(79, 172, 254, 0.08));
  color: #7deeff;
  box-shadow: 0 12px 28px rgba(79, 172, 254, 0.14);
}

.upload-actions button {
  color: #7deeff;
  font-weight: 900;
}

.upload-actions span,
.scope-options span,
.profile-tabs span {
  display: block;
  margin-top: 0.25rem;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.72rem;
  font-weight: 700;
}

.upload-side,
.task-body,
.scope-body,
.pool-body,
.preview-body {
  display: grid;
  gap: 0.78rem;
  padding: 0.85rem;
}

.target-toggle {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
}

.target-toggle button {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  color: rgba(200, 210, 234, 0.96);
  padding: 0.72rem 0.75rem;
  font-size: 0.82rem;
  font-weight: 900;
}

.target-toggle button.active {
  border-color: rgba(52, 211, 153, 0.45);
  background: rgba(52, 211, 153, 0.12);
  color: #a7f3d0;
}

.profile-field {
  display: grid;
  gap: 0.36rem;
}

.profile-field select,
.profile-field input,
.avatar-inline-toolbar select,
.avatar-library-toolbar select,
.pool-editor textarea {
  min-height: 2.65rem;
  width: 100%;
  border-radius: 8px;
  padding-left: 0.82rem;
  color: white;
  font-size: 0.86rem;
}

.profile-field select:disabled {
  opacity: 0.46;
}

.field-hint {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.74rem;
  line-height: 1.45;
}

.field-hint button,
.danger-link {
  color: #fb7185;
  font-size: 0.76rem;
  font-weight: 900;
}

.recent-upload {
  display: grid;
  gap: 0.35rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.72rem 0.78rem;
}

.recent-upload strong {
  overflow: hidden;
  color: white;
  font-size: 0.88rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-file-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.recent-file-list span {
  max-width: 100%;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 0.25rem 0.42rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.upload-result-grid,
.summary-grid,
.preview-stat-grid,
.task-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.55rem;
}

.upload-result-grid > div,
.summary-grid > div {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.65rem 0.7rem;
}

.upload-result-grid span {
  display: block;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.68rem;
  font-weight: 800;
}

.upload-result-grid strong {
  display: block;
  margin-top: 0.25rem;
  font-size: 1.15rem;
  font-weight: 950;
}

.task-summary {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.task-summary > div,
.preview-stat-grid > div {
  padding: 0.62rem 0.72rem;
}

.summary-box {
  border-radius: 8px;
  padding: 0.82rem;
  font-size: 0.8rem;
}

.summary-head strong,
.preview-item > strong {
  display: block;
  overflow: hidden;
  color: white;
  font-weight: 950;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-head span:not(.status-pill) {
  display: block;
  margin-top: 0.25rem;
  color: rgba(147, 164, 198, 0.92);
}

.failure-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-top: 0.75rem;
}

.failure-tabs button {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(2, 6, 23, 0.24);
  padding: 0.42rem 0.56rem;
  color: rgba(147, 164, 198, 0.94);
  font-size: 0.7rem;
  font-weight: 900;
}

.failure-list {
  display: grid;
  gap: 0.5rem;
  margin-top: 0.75rem;
}

.failure-list > div {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(2, 6, 23, 0.24);
  padding: 0.58rem 0.65rem;
}

.failure-list > div > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.failure-list p {
  margin: 0.35rem 0 0;
  color: rgba(147, 164, 198, 0.94);
  line-height: 1.45;
}

.refresh-box,
.error-banner {
  border-radius: 8px;
  padding: 0.72rem 0.8rem;
  font-size: 0.82rem;
  line-height: 1.55;
}

.refresh-box {
  border: 1px solid rgba(0, 242, 254, 0.24);
  background: rgba(0, 242, 254, 0.06);
}

.refresh-box strong,
.refresh-box span {
  display: block;
}

.refresh-box strong {
  color: #7deeff;
}

.refresh-box span {
  margin-top: 0.25rem;
  color: rgba(147, 164, 198, 0.94);
}

.error-banner {
  border: 1px solid rgba(251, 113, 133, 0.3);
  background: rgba(251, 113, 133, 0.1);
  color: #fb7185;
}

.scope-options,
.profile-tabs {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.6rem;
}

.profile-tabs {
  grid-template-columns: repeat(auto-fit, minmax(7.5rem, 1fr));
}

.scope-options strong,
.profile-tabs strong {
  display: block;
  color: white;
  font-size: 0.9rem;
  font-weight: 950;
}

.scope-options button.active strong,
.profile-tabs button.active strong {
  color: #7deeff;
}

.scope-select-grid,
.avatar-inline-toolbar {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.pool-editor textarea {
  min-height: 20rem;
  resize: vertical;
  padding: 0.88rem 0.95rem;
  line-height: 1.7;
}

.avatar-pool-inline {
  display: grid;
  gap: 0.65rem;
}

.avatar-inline-toolbar {
  grid-template-columns: minmax(0, 1fr) auto;
}

.avatar-inline-state,
.avatar-library-toolbar > div,
.pager-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: rgba(147, 164, 198, 0.94);
  font-size: 0.8rem;
}

.avatar-inline-state button,
.avatar-library-toolbar button {
  color: #7deeff;
  font-weight: 900;
}

.avatar-library-toolbar {
  padding: 0.8rem 0.9rem 0;
}

.avatar-library-toolbar select {
  max-width: 28rem;
}

.avatar-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 0.7rem;
  padding: 0.85rem;
}

.avatar-tile {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(2, 6, 23, 0.28);
  aspect-ratio: 1;
}

.avatar-tile.selected {
  border-color: rgba(52, 211, 153, 0.78);
  box-shadow: 0 12px 30px rgba(52, 211, 153, 0.18);
}

.avatar-tile > button:first-child {
  width: 100%;
  height: 100%;
}

.avatar-tile img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-fallback {
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
  gap: 0.35rem;
  background:
    linear-gradient(145deg, rgba(14, 116, 144, 0.18), rgba(15, 23, 42, 0.72)),
    rgba(2, 6, 23, 0.7);
  color: rgba(224, 231, 255, 0.95);
  padding: 0.7rem;
  text-align: center;
}

.avatar-fallback strong {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  place-items: center;
  border-radius: 8px;
  background: rgba(34, 211, 238, 0.18);
  color: #7deeff;
  font-size: 1rem;
  font-weight: 950;
}

.avatar-fallback small {
  display: -webkit-box;
  overflow: hidden;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.72rem;
  line-height: 1.35;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.avatar-tile span {
  position: absolute;
  left: 0.5rem;
  top: 0.5rem;
  border-radius: 8px;
  background: rgba(2, 6, 23, 0.72);
  padding: 0.24rem 0.45rem;
  color: rgba(224, 231, 255, 0.96);
  font-size: 0.68rem;
  font-weight: 900;
}

.avatar-tile.selected span {
  background: #34d399;
  color: #02131f;
}

.delete-avatar {
  position: absolute;
  right: 0.5rem;
  bottom: 0.5rem;
  border: 1px solid rgba(251, 113, 133, 0.34);
  border-radius: 8px;
  background: rgba(2, 6, 23, 0.76);
  padding: 0.25rem 0.5rem;
  color: #fb7185;
  font-size: 0.68rem;
  font-weight: 900;
  opacity: 0;
}

.avatar-tile:hover .delete-avatar,
.delete-avatar:focus-visible {
  opacity: 1;
}

.empty-state {
  display: grid;
  min-height: 8rem;
  place-items: center;
  margin: 0.85rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  color: rgba(147, 164, 198, 0.94);
  text-align: center;
}

.pager-row {
  justify-content: flex-end;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding: 0.75rem 0.9rem;
}

.preview-panel {
  position: sticky;
  top: 1rem;
}

.preview-body {
  max-height: calc(100vh - 9rem);
  overflow: auto;
}

.preview-stat-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.preview-list {
  display: grid;
  gap: 0.65rem;
}

.preview-item {
  display: grid;
  gap: 0.5rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.04), transparent 32%),
    rgba(15, 23, 42, 0.48);
  padding: 0.75rem;
}

.preview-item div {
  display: grid;
  grid-template-columns: 3rem minmax(0, 1fr);
  gap: 0.5rem;
  color: rgba(147, 164, 198, 0.94);
  font-size: 0.78rem;
}

.preview-item b {
  overflow: hidden;
  color: rgba(248, 251, 255, 0.94);
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 1380px) {
  .profile-command-grid,
  .profile-workspace {
    grid-template-columns: 1fr;
  }

  .preview-panel {
    position: static;
  }

  .preview-body {
    max-height: none;
  }
}

@media (max-width: 980px) {
  .profile-topbar,
  .profile-top-actions,
  .panel-head,
  .summary-head,
  .avatar-library-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .profile-stat-grid,
  .upload-layout,
  .scope-select-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .upload-layout,
  .scope-options,
  .profile-tabs {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .profile-title {
    font-size: 1.85rem;
  }

  .profile-stat-grid,
  .upload-actions,
  .scope-select-grid,
  .avatar-inline-toolbar,
  .task-summary,
  .summary-grid,
  .preview-stat-grid {
    grid-template-columns: 1fr;
  }

  .avatar-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
