<template>
  <div class="page-shell dm-shell">
    <div class="dm-topbar">
      <div class="min-w-0">
        <div class="eyebrow">LISTENER DIRECT MESSAGE</div>
        <h1 class="page-title dm-title">监听私信</h1>
        <p class="page-subtitle dm-subtitle">使用当前 web 用户自己的账号池，对监听命中的线索进行私信编排、风控预览和任务开启。</p>
      </div>
      <div class="dm-actions">
        <span class="status-pill" data-tone="info">线索 {{ leads.length }}</span>
        <span class="status-pill" data-tone="info">任务 {{ listenerTasks.length }}</span>
        <span class="status-pill" :data-tone="availableTerminals.length ? 'success' : 'warning'">可用账号 {{ availableTerminals.length }}</span>
        <span class="status-pill" :data-tone="selectedContactableLeads.length ? 'success' : 'warning'">已选 {{ selectedContactableLeads.length }}</span>
        <label v-if="isAdmin" class="dm-member-filter">
          <span>会员视图</span>
          <select v-model="memberFilterID">
            <option value="">全部会员</option>
            <option v-for="user in memberUsers" :key="user.id" :value="user.id">{{ userLabel(user) }}</option>
          </select>
        </label>
        <GlassButton variant="secondary" size="sm" :loading="loading" @click="load">刷新</GlassButton>
      </div>
    </div>

    <div class="dm-stat-grid">
      <div v-for="item in statCards" :key="item.label" class="dm-stat">
        <span>{{ item.label }}</span>
        <strong :class="item.tone">{{ item.value }}</strong>
      </div>
    </div>

    <div v-if="error" class="dm-alert danger">{{ error }}</div>
    <div v-if="notice" class="dm-alert success">{{ notice }}</div>

    <div class="dm-workspace">
      <GlassCard padding="none" class="dm-panel lead-panel">
        <div class="panel-head">
          <div>
            <div class="eyebrow">01 LEADS</div>
            <h2>监听线索</h2>
          </div>
          <span class="status-pill" data-tone="info">{{ filteredLeads.length }} 条</span>
        </div>

        <div class="lead-toolbar">
          <select v-model="selectedTaskID" class="dm-select">
            <option value="">全部运行中监听任务</option>
            <option v-for="task in listenerTasks" :key="task.id" :value="task.id">{{ taskLabel(task) }}</option>
          </select>
          <input v-model="leadKeyword" class="dm-input" placeholder="搜索用户 / 来源群 / 关键词 / 消息" />
          <select v-model="leadStatusFilter" class="dm-select">
            <option value="all">全部线索</option>
            <option value="contactable">可私信</option>
            <option value="fresh">未触达</option>
            <option value="contacted">已触达</option>
          </select>
        </div>

        <div class="lead-bulk-row">
          <button type="button" @click="selectFilteredLeads">选中当前筛选</button>
          <button type="button" @click="clearLeadSelection">清空选择</button>
          <span>隐藏无用户名：{{ skipNoAccount ? '是' : '否' }}</span>
        </div>

        <div class="lead-grid scrollbar-thin">
          <article
            v-for="lead in visibleLeads"
            :key="lead.id"
            class="lead-card"
            :class="{ selected: selectedLeadIDs.includes(lead.id), muted: !lead.user_account }"
            role="button"
            tabindex="0"
            @click="toggleLead(lead.id)"
            @keydown.enter.prevent="toggleLead(lead.id)"
            @keydown.space.prevent="toggleLead(lead.id)"
          >
            <div class="lead-card-head">
              <div class="lead-avatar">{{ leadInitial(lead) }}</div>
              <div class="min-w-0">
                <strong>{{ leadDisplayName(lead) }}</strong>
                <span>{{ lead.user_account || '缺少用户名' }}</span>
              </div>
              <span class="lead-time">{{ formatTime(lead.hit_time || lead.hit_at || lead.created_at) }}</span>
            </div>
            <div class="lead-meta-grid">
              <div>
                <span>来源</span>
                <strong :title="lead.source_chat_name || lead.source_chat_id || '未知来源'">{{ lead.source_chat_name || lead.source_chat_id || '未知来源' }}</strong>
              </div>
              <div>
                <span>命中词</span>
                <strong>{{ lead.trigger_word || '-' }}</strong>
              </div>
            </div>
            <p class="lead-message">{{ lead.trigger_message || '没有记录命中消息内容' }}</p>
            <div class="lead-footer">
              <div class="lead-footer-main">
                <span>{{ ownerLabel(lead) }}</span>
                <strong :class="lead.user_account ? 'ok' : 'warn'">{{ lead.user_account ? '可私信' : '需人工补账号' }}</strong>
              </div>
              <button
                type="button"
                class="blacklist-btn"
                :disabled="blacklistingLeadIDs.includes(lead.id) || !canBlacklistLead(lead)"
                @click.stop="blacklistLead(lead)"
              >
                {{ blacklistingLeadIDs.includes(lead.id) ? '加入中...' : '加入黑名单' }}
              </button>
            </div>
          </article>
          <div v-if="!visibleLeads.length" class="dm-empty">当前筛选下没有线索</div>
        </div>
      </GlassCard>

      <div class="dm-center-stack">
        <GlassCard padding="none" class="dm-panel account-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">02 ACCOUNT POOL</div>
              <h2>私信账号池</h2>
            </div>
            <span class="status-pill" :data-tone="availableTerminals.length ? 'success' : 'warning'">{{ availableTerminals.length }}/{{ scopedTerminals.length }} 可调度</span>
          </div>

          <div class="scope-tabs">
            <button type="button" :class="{ active: terminalScope === 'all' }" @click="terminalScope = 'all'">全部账号</button>
            <button type="button" :class="{ active: terminalScope === 'group' }" @click="terminalScope = 'group'">账号分组</button>
            <button type="button" :class="{ active: terminalScope === 'terminal' }" @click="terminalScope = 'terminal'">指定账号</button>
          </div>

          <div v-if="terminalScope === 'group'" class="dm-field">
            <label>选择账号分组</label>
            <select v-model="selectedTerminalGroupID">
              <option value="">请选择分组</option>
              <option v-for="group in terminalGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
          </div>

          <div class="terminal-toolbar">
            <input v-model="terminalKeyword" class="dm-input" placeholder="搜索手机号 / 昵称 / 风控状态" />
            <label class="dm-check">
              <input v-model="onlyAvailableTerminals" type="checkbox" />
              <span>只看可用</span>
            </label>
          </div>

          <div class="terminal-list scrollbar-thin">
            <label v-for="terminal in visibleTerminals" :key="terminal.id" class="terminal-row" :class="{ selected: selectedTerminalIDs.includes(terminal.id), disabled: !terminalReady(terminal) }">
              <input
                v-if="terminalScope === 'terminal'"
                v-model="selectedTerminalIDs"
                type="checkbox"
                :value="terminal.id"
              />
              <span class="terminal-dot" :data-tone="terminalReady(terminal) ? 'success' : 'warning'"></span>
              <span class="min-w-0">
                <strong>{{ terminalLabel(terminal) }}</strong>
                <small>{{ terminalMeta(terminal) }}</small>
              </span>
              <em>{{ terminalQuotaText(terminal) }}</em>
            </label>
            <div v-if="!visibleTerminals.length" class="dm-empty">当前账号范围内没有可展示账号</div>
          </div>
        </GlassCard>

        <GlassCard padding="none" class="dm-panel message-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">03 SEQUENCE</div>
              <h2>消息编排</h2>
            </div>
            <span class="status-pill" data-tone="info">{{ messageSteps.length }} 阶段</span>
          </div>

          <div class="variable-row">
            <button v-for="token in templateTokens" :key="token.value" type="button" @click="insertToken(token.value)">
              {{ token.label }}
            </button>
          </div>

          <div class="message-step-list">
            <div
              v-for="(step, index) in messageSteps"
              :key="step.id"
              class="message-step"
              :class="{ active: focusedStepID === step.id }"
              @click="focusedStepID = step.id"
            >
              <div class="step-head">
                <strong>第 {{ index + 1 }} 阶段 · {{ stepTypeLabel(step.type) }}</strong>
                <button v-if="messageSteps.length > 1" type="button" @click="removeStep(step.id)">删除</button>
              </div>
              <div class="step-type-tabs">
                <button v-for="type in stepTypes" :key="type.value" type="button" :class="{ active: step.type === type.value }" @click="setStepType(step, type.value)">
                  {{ type.label }}
                </button>
              </div>
              <textarea
                v-if="step.type === 'text' || step.type === 'image' || step.type === 'gif'"
                v-model="step.content"
                class="dm-textarea"
                rows="4"
                :placeholder="step.type === 'text' ? '例如：你好 {昵称}，刚看到你在 {来源群} 提到“{命中词}”，方便聊一下吗？' : '可选图片/GIF配文，支持变量。'"
                @focus="focusedStepID = step.id"
              ></textarea>
              <div v-else-if="step.type === 'voice'" class="media-step-note">语音阶段会发送已上传语音素材，文本内容不会作为语音发送。</div>
              <div v-if="step.type === 'image' || step.type === 'voice' || step.type === 'gif'" class="dm-media-box" :class="{ uploading: step.uploading }" @dragover.prevent @drop.prevent="handleMediaDrop(index, $event)">
                <div>
                  <strong>{{ stepTypeLabel(step.type) }}素材</strong>
                  <span>{{ step.mediaName || '拖拽或选择本地文件，上传后自动绑定素材 ID' }}</span>
                </div>
                <input :id="`dm-media-${step.id}`" class="hidden" type="file" :accept="mediaAccept(step.type)" @change="handleMediaSelect(index, $event)" />
                <label :for="`dm-media-${step.id}`">{{ step.uploading ? '上传中...' : '选择文件' }}</label>
                <input v-model="step.mediaAssetID" placeholder="素材 ID" />
              </div>
              <div v-if="step.type === 'forward'" class="forward-grid">
                <label>
                  <span>来源 Chat ID</span>
                  <input v-model="step.sourceChatID" placeholder="-100123456789 或 @channel" @focus="focusedStepID = step.id" />
                </label>
                <label>
                  <span>Message ID</span>
                  <input v-model="step.messageID" placeholder="9481" @focus="focusedStepID = step.id" />
                </label>
              </div>
              <label class="step-delay">
                <span>本阶段发送后等待</span>
                <input v-model.number="step.delayMinutes" type="number" min="0" max="1440" />
                <span>分钟</span>
              </label>
            </div>
          </div>

          <button type="button" class="add-step-btn" @click="addStep">增加消息阶段</button>
        </GlassCard>
      </div>

      <GlassCard padding="none" class="dm-panel launch-panel">
        <div class="panel-head">
          <div>
            <div class="eyebrow">04 LAUNCH</div>
            <h2>开启私信任务</h2>
          </div>
          <span class="status-pill" :data-tone="canSubmit ? 'success' : 'warning'">{{ canSubmit ? '可创建' : '待补全' }}</span>
        </div>

        <div class="launch-summary">
          <div>
            <span>可私信线索</span>
            <strong>{{ selectedContactableLeads.length }}</strong>
          </div>
          <div>
            <span>可用账号</span>
            <strong>{{ availableTerminals.length }}</strong>
          </div>
          <div>
            <span>预计线索</span>
            <strong>{{ estimatedDeliveries }}</strong>
          </div>
        </div>

        <div class="launch-options">
          <label class="dm-check">
            <input v-model="stopOnReply" type="checkbox" />
            <span>线索回复后停止后续编排</span>
          </label>
          <label class="dm-check">
            <input v-model="dryRun" type="checkbox" />
            <span>只创建 dry-run 任务</span>
          </label>
          <label class="dm-check">
            <input v-model="skipNoAccount" type="checkbox" />
            <span>跳过无用户名线索</span>
          </label>
        </div>

        <div class="launch-grid">
          <label>
            <span>去重窗口</span>
            <input v-model.number="dedupeDays" type="number" min="0" max="365" />
            <small>天内已触达则跳过</small>
          </label>
          <label>
            <span>发送最小间隔</span>
            <input v-model.number="minDelayMinutes" type="number" min="0" max="1440" />
            <small>分钟</small>
          </label>
          <label>
            <span>发送最大间隔</span>
            <input v-model.number="maxDelayMinutes" type="number" min="0" max="1440" />
            <small>分钟</small>
          </label>
          <label>
            <span>账号冷却时间</span>
            <input v-model.number="cooldownMinutes" type="number" min="0" max="1440" />
            <small>发完后账号进入冷却</small>
          </label>
          <label>
            <span>冷却浮动</span>
            <input v-model.number="cooldownJitterMinutes" type="number" min="0" max="120" />
            <small>正负分钟</small>
          </label>
        </div>

        <div class="preview-list scrollbar-thin">
          <div v-for="row in previewRows" :key="row.lead.id" class="preview-row">
            <div>
              <strong>{{ leadDisplayName(row.lead) }}</strong>
              <span>{{ row.lead.user_account }}</span>
            </div>
            <em>{{ row.terminal ? terminalLabel(row.terminal) : '暂无账号' }}</em>
            <p>{{ row.message }}</p>
          </div>
          <div v-if="!previewRows.length" class="dm-empty">选择线索和账号后显示预览</div>
        </div>

        <GlassButton class="w-full" variant="primary" :loading="submitting" :disabled="!canSubmit" @click="submitJob">
          开启监听私信任务
        </GlassButton>
        <p class="launch-note">每条线索只会分配一个账号发送整套编排；任务执行时仍会二次校验账号风控、私信限额、冷却期和目标限制。</p>
      </GlassCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type Group, type SCRMLead, type Task, type Terminal, type User } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUserStore } from '../stores/user'
import { taskDisplayName, taskOptionLabel } from '../utils/taskDisplay'

type StepType = 'text' | 'image' | 'voice' | 'gif' | 'forward'

type MessageStepDraft = {
  id: string
  type: StepType
  content: string
  mediaAssetID: string
  mediaName: string
  mediaUrl: string
  sourceChatID: string
  messageID: string
  uploading: boolean
  delayMinutes: number
}

const userStore = useUserStore()
const loading = ref(false)
const submitting = ref(false)
const blacklistingLeadIDs = ref<string[]>([])
const error = ref('')
const notice = ref('')
const leads = ref<SCRMLead[]>([])
const listenerTasks = ref<Task[]>([])
const terminals = ref<Terminal[]>([])
const terminalGroups = ref<Group[]>([])
const users = ref<User[]>([])
const memberFilterID = ref('')
const selectedTaskID = ref('')
const leadKeyword = ref('')
const leadStatusFilter = ref<'all' | 'contactable' | 'fresh' | 'contacted'>('contactable')
const selectedLeadIDs = ref<string[]>([])
const terminalScope = ref<'all' | 'group' | 'terminal'>('all')
const selectedTerminalGroupID = ref('')
const selectedTerminalIDs = ref<string[]>([])
const terminalKeyword = ref('')
const onlyAvailableTerminals = ref(true)
const skipNoAccount = ref(true)
const stopOnReply = ref(true)
const dryRun = ref(false)
const dedupeDays = ref(15)
const minDelayMinutes = ref(1)
const maxDelayMinutes = ref(3)
const cooldownMinutes = ref(60)
const cooldownJitterMinutes = ref(10)
const focusedStepID = ref('')
const messageSteps = ref<MessageStepDraft[]>([
  {
    id: createDraftID(),
    type: 'text',
    content: '你好 {昵称}，刚看到你在 {来源群} 提到“{命中词}”，这块我可以帮你对接一下，方便聊聊吗？',
    mediaAssetID: '',
    mediaName: '',
    mediaUrl: '',
    sourceChatID: '',
    messageID: '',
    uploading: false,
    delayMinutes: 0
  }
])
const stepTypes: Array<{ label: string; value: StepType }> = [
  { label: '文本', value: 'text' },
  { label: '图片', value: 'image' },
  { label: '语音', value: 'voice' },
  { label: 'GIF', value: 'gif' },
  { label: '引用消息', value: 'forward' }
]

const isAdmin = computed(() => userStore.user?.role === 'admin')
const memberUsers = computed(() => users.value.filter((user) => user.role === 'user'))
const filteredLeads = computed(() => {
  const keyword = normalizeText(leadKeyword.value)
  return leads.value.filter((lead) => {
    if (normalizeText(lead.status) === 'blacklisted') return false
    if (skipNoAccount.value && leadStatusFilter.value === 'contactable' && !lead.user_account) return false
    if (leadStatusFilter.value === 'contactable' && !lead.user_account) return false
    if (leadStatusFilter.value === 'fresh' && leadContacted(lead)) return false
    if (leadStatusFilter.value === 'contacted' && !leadContacted(lead)) return false
    if (!keyword) return true
    return normalizeText([lead.user_nickname, lead.user_account, lead.source_chat_name, lead.trigger_word, lead.trigger_message].join(' ')).includes(keyword)
  })
})
const visibleLeads = computed(() => filteredLeads.value.slice(0, 160))
const selectedLeads = computed(() => leads.value.filter((lead) => selectedLeadIDs.value.includes(lead.id)))
const selectedContactableLeads = computed(() => selectedLeads.value.filter((lead) => Boolean(lead.user_account?.trim())))
const displayScopeTerminals = computed(() => {
  let items = terminals.value
  if (terminalScope.value === 'group') {
    items = selectedTerminalGroupID.value ? items.filter((terminal) => terminal.group_id === selectedTerminalGroupID.value) : []
  }
  const keyword = normalizeText(terminalKeyword.value)
  if (keyword) {
    items = items.filter((terminal) => normalizeText([terminal.phone, terminal.nickname, terminal.status, terminal.risk_status, terminal.ban_status].join(' ')).includes(keyword))
  }
  return items
})
const scopedTerminals = computed(() => {
  if (terminalScope.value === 'terminal') {
    return selectedTerminalIDs.value.length ? displayScopeTerminals.value.filter((terminal) => selectedTerminalIDs.value.includes(terminal.id)) : []
  }
  return displayScopeTerminals.value
})
const availableTerminals = computed(() => scopedTerminals.value.filter(terminalReady))
const visibleTerminals = computed(() => {
  const source = terminalScope.value === 'terminal' ? displayScopeTerminals.value : scopedTerminals.value
  const items = onlyAvailableTerminals.value ? source.filter(terminalReady) : source
  return items.slice(0, 120)
})
const validSteps = computed(() => messageSteps.value.filter(isStepReady))
const estimatedDeliveries = computed(() => selectedContactableLeads.value.length)
const canSubmit = computed(() => selectedContactableLeads.value.length > 0 && availableTerminals.value.length > 0 && validSteps.value.length > 0 && !submitting.value)
const statCards = computed(() => [
  { label: '全部线索', value: String(leads.value.length), tone: 'text-white' },
  { label: '可私信线索', value: String(leads.value.filter((lead) => lead.user_account?.trim()).length), tone: 'text-neon' },
  { label: '账号池', value: String(terminals.value.length), tone: 'text-white' },
  { label: '冷却/限额可用', value: String(availableTerminals.value.length), tone: availableTerminals.value.length ? 'text-green-300' : 'text-amber' },
  { label: '预计线索', value: String(estimatedDeliveries.value), tone: 'text-cyan-200' }
])
const previewRows = computed(() => {
  const accounts = availableTerminals.value
  const firstStep = validSteps.value[0]
  if (!firstStep) return []
  return selectedContactableLeads.value.slice(0, 10).map((lead, index) => ({
    lead,
    terminal: accounts.length ? accounts[index % accounts.length] : null,
    message: stepPreview(firstStep, lead)
  }))
})
const templateTokens = [
  { label: '昵称', value: '{昵称}' },
  { label: '账号', value: '{账号}' },
  { label: '来源群', value: '{来源群}' },
  { label: '命中词', value: '{命中词}' },
  { label: '原消息', value: '{原消息}' }
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    const leadFilters = {
      ...(isAdmin.value && memberFilterID.value ? { user_id: memberFilterID.value } : {}),
      ...(selectedTaskID.value ? { task_id: selectedTaskID.value } : {})
    }
    const taskFilters = {
      type: 'scrm_listener',
      status: 'running',
      limit: 500,
      ...(isAdmin.value && memberFilterID.value ? { user_id: memberFilterID.value } : {})
    }
    const lookups: Array<Promise<unknown>> = [
      api.scrmLeads(leadFilters),
      api.tasks(taskFilters),
      api.terminals(),
      api.groups('terminal')
    ]
    if (isAdmin.value) {
      lookups.push(api.users())
    }
    const [leadData, taskData, terminalData, groupData, userData] = await Promise.all(lookups)
    leads.value = leadData as SCRMLead[]
    listenerTasks.value = (taskData as Task[]).filter(isRunningTask)
    terminals.value = terminalData as Terminal[]
    terminalGroups.value = groupData as Group[]
    if (userData) users.value = userData as User[]
    if (selectedTaskID.value && !listenerTasks.value.some((task) => task.id === selectedTaskID.value)) {
      selectedTaskID.value = ''
    }
    selectedLeadIDs.value = selectedLeadIDs.value.filter((id) => leads.value.some((lead) => lead.id === id))
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载监听私信数据失败'
  } finally {
    loading.value = false
  }
}

function toggleLead(id: string) {
  if (selectedLeadIDs.value.includes(id)) {
    selectedLeadIDs.value = selectedLeadIDs.value.filter((item) => item !== id)
  } else {
    selectedLeadIDs.value = [...selectedLeadIDs.value, id]
  }
}

function selectFilteredLeads() {
  const ids = filteredLeads.value.filter((lead) => !skipNoAccount.value || lead.user_account?.trim()).map((lead) => lead.id)
  selectedLeadIDs.value = Array.from(new Set([...selectedLeadIDs.value, ...ids]))
}

function clearLeadSelection() {
  selectedLeadIDs.value = []
}

function canBlacklistLead(lead: SCRMLead) {
  return Boolean(lead.source_task_id && (lead.user_account?.trim() || lead.user_nickname?.trim() || lead.target_id))
}

async function blacklistLead(lead: SCRMLead) {
  if (!canBlacklistLead(lead) || blacklistingLeadIDs.value.includes(lead.id)) return
  blacklistingLeadIDs.value = [...blacklistingLeadIDs.value, lead.id]
  error.value = ''
  notice.value = ''
  try {
    await api.blacklistScrmLeadUser(lead.id)
    leads.value = leads.value.filter((item) => item.id !== lead.id)
    selectedLeadIDs.value = selectedLeadIDs.value.filter((id) => id !== lead.id)
    notice.value = `${leadDisplayName(lead)} 已加入当前任务黑名单，后续该用户命中关键词不会再推送。`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加入任务黑名单失败'
  } finally {
    blacklistingLeadIDs.value = blacklistingLeadIDs.value.filter((id) => id !== lead.id)
  }
}

function addStep() {
  const id = createDraftID()
  messageSteps.value.push({
    id,
    type: 'text',
    content: '',
    mediaAssetID: '',
    mediaName: '',
    mediaUrl: '',
    sourceChatID: '',
    messageID: '',
    uploading: false,
    delayMinutes: 0
  })
  focusedStepID.value = id
}

function createDraftID() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  return `draft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function removeStep(id: string) {
  messageSteps.value = messageSteps.value.filter((step) => step.id !== id)
}

function insertToken(token: string) {
  const target = messageSteps.value.find((step) => step.id === focusedStepID.value) || messageSteps.value[messageSteps.value.length - 1]
  if (!target) return
  target.content = `${target.content}${target.content.endsWith(' ') || !target.content ? '' : ' '}${token}`
}

function setStepType(step: MessageStepDraft, type: StepType) {
  step.type = type
  focusedStepID.value = step.id
}

function isStepReady(step: MessageStepDraft) {
  if (step.type === 'text') return Boolean(step.content.trim())
  if (step.type === 'forward') return Boolean(step.sourceChatID.trim() && step.messageID.trim())
  return Boolean(step.mediaAssetID.trim())
}

function mediaAccept(type: StepType) {
  if (type === 'image') return 'image/jpeg,image/png,image/gif'
  if (type === 'gif') return 'image/gif'
  if (type === 'voice') return 'audio/mpeg,audio/mp4,audio/aac,audio/ogg,audio/wav,audio/webm,.mp3,.m4a,.aac,.ogg,.oga,.wav,.webm'
  return ''
}

function handleMediaSelect(index: number, event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  void uploadStepMedia(index, file)
}

function handleMediaDrop(index: number, event: DragEvent) {
  const file = event.dataTransfer?.files?.[0]
  if (!file) return
  void uploadStepMedia(index, file)
}

async function uploadStepMedia(index: number, file: File) {
  const step = messageSteps.value[index]
  if (!step || !['image', 'voice', 'gif'].includes(step.type)) return
  if (!isFileAllowedForStep(step.type, file)) {
    error.value = `${stepTypeLabel(step.type)}阶段不能使用 ${file.name}`
    return
  }

  step.uploading = true
  if (step.mediaUrl?.startsWith('blob:')) URL.revokeObjectURL(step.mediaUrl)
  step.mediaUrl = URL.createObjectURL(file)
  step.mediaName = file.name
  try {
    const result = await api.uploadWorkflowMedia([file])
    const item = result.items.find((entry) => entry.status === 'success' || entry.status === 'duplicate')
    if (!item?.id) {
      const reason = result.items.find((entry) => entry.reason)?.reason || '媒体上传失败'
      throw new Error(reason)
    }
    step.mediaAssetID = item.id
    step.mediaName = item.name || file.name
    step.mediaUrl = item.url || step.mediaUrl
    notice.value = `${file.name} 已上传并绑定到第 ${index + 1} 阶段`
  } catch (err) {
    step.mediaAssetID = ''
    error.value = err instanceof Error ? err.message : '媒体上传失败'
  } finally {
    step.uploading = false
  }
}

function isFileAllowedForStep(type: StepType, file: File) {
  const name = file.name.toLowerCase()
  const mime = file.type.toLowerCase()
  if (type === 'image') return mime.startsWith('image/') || /\.(jpe?g|jepg|png|gif)$/.test(name)
  if (type === 'gif') return mime === 'image/gif' || name.endsWith('.gif')
  if (type === 'voice') return mime.startsWith('audio/') || /\.(mp3|m4a|aac|ogg|oga|wav|webm)$/.test(name)
  return false
}

async function submitJob() {
  if (!canSubmit.value) return
  submitting.value = true
  error.value = ''
  notice.value = ''
  try {
    const minDelay = Math.max(0, Number(minDelayMinutes.value) || 0)
    const maxDelay = Math.max(minDelay, Number(maxDelayMinutes.value) || minDelay)
    const task = await api.createDirectMessageJob({
      name: `监听私信 · ${new Date().toLocaleString()}`,
      lead_ids: selectedContactableLeads.value.map((lead) => lead.id),
      terminal_scope: terminalScope.value,
      terminal_group_id: terminalScope.value === 'group' ? selectedTerminalGroupID.value : undefined,
      terminal_ids: terminalScope.value === 'terminal' ? selectedTerminalIDs.value : undefined,
      steps: validSteps.value.map((step) => ({
        type: step.type,
        content: step.content.trim(),
        media_asset_id: step.mediaAssetID.trim() || undefined,
        source_chat_id: step.sourceChatID.trim() || undefined,
        message_id: step.messageID.trim() || undefined,
        delay_seconds: Math.max(0, Number(step.delayMinutes) || 0) * 60
      })),
      min_delay_seconds: minDelay * 60,
      max_delay_seconds: maxDelay * 60,
      cooldown_minutes: Math.max(0, Number(cooldownMinutes.value) || 0),
      cooldown_jitter_minutes: Math.max(0, Number(cooldownJitterMinutes.value) || 0),
      dedupe_days: Math.max(0, Number(dedupeDays.value) || 0),
      skip_no_account: skipNoAccount.value,
      stop_on_reply: stopOnReply.value,
      dry_run: dryRun.value
    })
    notice.value = `已开启监听私信任务：${taskDisplayName(task)}`
    selectedLeadIDs.value = []
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '开启监听私信任务失败'
  } finally {
    submitting.value = false
  }
}

function terminalReady(terminal: Terminal) {
  const status = normalizeText(terminal.status)
  const risk = normalizeText(`${terminal.risk_status} ${terminal.ban_status}`)
  if (['abnormal', 'disabled', 'banned', 'offline'].some((item) => status.includes(item))) return false
  if (['封', '限制', 'banned', 'restricted', 'disabled'].some((item) => risk.includes(item))) return false
  if (dateInFuture(terminal.sleep_until) || dateInFuture(terminal.dm_cooldown_until)) return false
  if (terminal.dm_hourly_limit > 0 && terminal.dm_hourly_count >= terminal.dm_hourly_limit) return false
  if (terminal.dm_daily_limit > 0 && terminal.dm_daily_count >= terminal.dm_daily_limit) return false
  return true
}

function terminalQuotaText(terminal: Terminal) {
  const hourly = terminal.dm_hourly_limit > 0 ? `${terminal.dm_hourly_count}/${terminal.dm_hourly_limit}` : `${terminal.dm_hourly_count}/不限`
  const daily = terminal.dm_daily_limit > 0 ? `${terminal.dm_daily_count}/${terminal.dm_daily_limit}` : `${terminal.dm_daily_count}/不限`
  if (dateInFuture(terminal.dm_cooldown_until)) return `冷却到 ${formatTime(terminal.dm_cooldown_until)}`
  return `私信 ${hourly} · 日 ${daily}`
}

function terminalMeta(terminal: Terminal) {
  return [terminal.nickname || '未命名', terminal.status_text || terminal.status || '未知状态', terminal.risk_status || '风控正常'].join(' · ')
}

function terminalLabel(terminal: Terminal) {
  return terminal.phone_display || terminal.phone || terminal.nickname || terminal.id
}

function taskLabel(task: Task) {
  return taskOptionLabel(task)
}

function isRunningTask(task: Task) {
  return normalizeText(task.status) === 'running'
}

function leadDisplayName(lead: SCRMLead) {
  return lead.user_nickname?.trim() || lead.user_account?.trim() || lead.target_id || '未识别用户'
}

function leadInitial(lead: SCRMLead) {
  return leadDisplayName(lead).trim().slice(0, 1).toUpperCase() || 'U'
}

function leadContacted(lead: SCRMLead) {
  return ['replied', 'dm_sent', 'contacted'].includes(normalizeText(lead.status))
}

function ownerLabel(lead: SCRMLead) {
  if (!isAdmin.value) return '当前会员'
  const ownerID = lead.owner_user_id || (lead.tenant_id && lead.tenant_id !== '00000000-0000-0000-0000-000000000000' ? lead.tenant_id : '')
  if (!ownerID) return '管理员 / 全局'
  return userLabel(users.value.find((user) => user.id === ownerID) || null)
}

function userLabel(user?: User | null) {
  if (!user) return '未知会员'
  const telegram = user.telegram_username || user.telegram_user_id
  return telegram ? `${user.username} · ${telegram}` : user.username
}

function renderTemplate(template: string, lead: SCRMLead) {
  const values: Record<string, string> = {
    昵称: lead.user_nickname || lead.user_account || '朋友',
    账号: lead.user_account || '',
    来源群: lead.source_chat_name || lead.source_chat_id || '',
    命中词: lead.trigger_word || '',
    原消息: lead.trigger_message || '',
    命中时间: formatDate(lead.hit_time || lead.hit_at || lead.created_at)
  }
  return Object.entries(values).reduce((text, [key, value]) => replaceToken(replaceToken(text, `{${key}}`, value), `{{${key}}}`, value), template)
}

function stepPreview(step: MessageStepDraft, lead: SCRMLead) {
  if (step.type === 'image') return `图片${step.content ? `：${renderTemplate(step.content, lead)}` : ''}`
  if (step.type === 'voice') return `语音：${step.mediaName || step.mediaAssetID || '未绑定'}`
  if (step.type === 'gif') return `GIF${step.content ? `：${renderTemplate(step.content, lead)}` : ''}`
  if (step.type === 'forward') return `引用消息：${step.sourceChatID || '-'} / ${step.messageID || '-'}`
  return renderTemplate(step.content, lead)
}

function stepTypeLabel(type: string) {
  const labels: Record<string, string> = {
    text: '文本',
    image: '图片',
    voice: '语音',
    gif: 'GIF',
    forward: '引用消息'
  }
  return labels[type] || type || '消息'
}

function replaceToken(text: string, token: string, value: string) {
  return text.split(token).join(value)
}

function dateInFuture(value?: string | null) {
  return Boolean(value && new Date(value).getTime() > Date.now())
}

function formatTime(value?: string | null) {
  if (!value) return '--'
  return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function formatDate(value?: string | null) {
  if (!value) return ''
  return new Date(value).toLocaleString()
}

function normalizeText(value: unknown) {
  return String(value ?? '').trim().toLowerCase()
}

watch(memberFilterID, () => {
  selectedTaskID.value = ''
  void load()
})

watch(selectedTaskID, () => {
  void load()
})

watch(terminalScope, () => {
  if (terminalScope.value !== 'terminal') {
    selectedTerminalIDs.value = []
  }
})

onMounted(load)
</script>

<style scoped>
.dm-shell {
  gap: 1rem;
}

.dm-topbar,
.dm-panel,
.dm-stat,
.dm-alert {
  border: 1px solid rgba(148, 163, 184, 0.16);
  background:
    linear-gradient(145deg, rgba(30, 41, 59, 0.88), rgba(8, 13, 28, 0.94)),
    rgba(15, 23, 42, 0.86);
  box-shadow: 0 22px 60px rgba(0, 0, 0, 0.28);
}

.dm-topbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  min-width: 0;
  border-radius: 8px;
  padding: 1.25rem;
}

.dm-title {
  font-size: 2.35rem;
  line-height: 1;
}

.dm-subtitle {
  max-width: 58rem;
}

.dm-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  min-width: 18rem;
  gap: 0.6rem;
}

.dm-member-filter,
.dm-field,
.launch-grid label {
  display: grid;
  gap: 0.45rem;
  color: rgba(197, 211, 235, 0.78);
  font-size: 0.78rem;
}

.dm-member-filter select,
.dm-field select,
.dm-input,
.dm-select,
.launch-grid input {
  min-height: 2.75rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 10px;
  background: rgba(6, 12, 27, 0.78);
  padding: 0 0.85rem;
  color: white;
  outline: none;
}

.dm-member-filter select {
  min-width: 13rem;
}

.dm-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10.5rem, 1fr));
  gap: 0.85rem;
}

.dm-stat {
  min-height: 5.5rem;
  border-radius: 8px;
  padding: 1rem;
}

.dm-stat span,
.lead-meta-grid span,
.launch-summary span,
.launch-grid span {
  display: block;
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.75rem;
}

.dm-stat strong,
.launch-summary strong {
  display: block;
  margin-top: 0.45rem;
  font-size: 1.75rem;
  font-weight: 900;
}

.dm-alert {
  border-radius: 12px;
  padding: 0.9rem 1rem;
  color: white;
}

.dm-alert.danger {
  border-color: rgba(248, 113, 113, 0.38);
  background: rgba(127, 29, 29, 0.32);
}

.dm-alert.success {
  border-color: rgba(45, 212, 191, 0.38);
  background: rgba(20, 83, 45, 0.3);
}

.dm-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1.04fr) minmax(0, 1fr) minmax(18.5rem, 0.72fr);
  gap: 1rem;
  align-items: start;
}

.dm-center-stack {
  display: grid;
  min-width: 0;
  gap: 1rem;
}

.dm-panel {
  min-width: 0;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.14);
  padding: 1rem;
}

.panel-head h2 {
  margin-top: 0.2rem;
  font-size: 1.15rem;
  font-weight: 900;
}

.lead-toolbar,
.terminal-toolbar {
  display: grid;
  grid-template-columns: minmax(11rem, 0.72fr) minmax(14rem, 1.25fr) minmax(8.5rem, 0.58fr);
  gap: 0.65rem;
  padding: 1rem;
}

.terminal-toolbar {
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  padding-top: 0;
}

.lead-bulk-row,
.variable-row,
.scope-tabs,
.step-type-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0 1rem 1rem;
}

.lead-bulk-row button,
.variable-row button,
.scope-tabs button,
.step-type-tabs button,
.add-step-btn,
.message-step button {
  min-height: 2.35rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.72);
  padding: 0 0.85rem;
  color: rgba(226, 232, 240, 0.92);
  font-size: 0.82rem;
  font-weight: 800;
}

.lead-bulk-row span {
  margin-left: auto;
  align-self: center;
  min-width: 0;
  overflow: hidden;
  color: rgba(148, 163, 184, 0.8);
  font-size: 0.78rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scope-tabs button.active,
.step-type-tabs button.active,
.variable-row button:hover,
.add-step-btn:hover {
  border-color: rgba(34, 211, 238, 0.72);
  background: rgba(14, 116, 144, 0.28);
  color: #67e8f9;
}

.lead-grid {
  display: grid;
  max-height: 68rem;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.65rem;
  overflow: auto;
  padding: 0 1rem 1rem;
}

.lead-card {
  display: grid;
  grid-template-areas:
    "person meta actions"
    "person message actions";
  grid-template-columns: minmax(8.5rem, 0.72fr) minmax(0, 1.62fr) minmax(7.4rem, 0.66fr);
  gap: 0.55rem 0.7rem;
  align-items: center;
  min-width: 0;
  min-height: 6.7rem;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 12px;
  background: rgba(8, 13, 28, 0.68);
  padding: 0.75rem;
  text-align: left;
  color: white;
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    transform 0.18s ease;
}

.lead-card:hover,
.lead-card:focus-visible {
  border-color: rgba(34, 211, 238, 0.48);
  background: rgba(14, 25, 46, 0.88);
  outline: none;
  transform: translateY(-1px);
}

.lead-card.selected {
  border-color: rgba(34, 211, 238, 0.76);
  background: linear-gradient(145deg, rgba(14, 116, 144, 0.34), rgba(15, 23, 42, 0.74));
}

.lead-card.muted {
  opacity: 0.62;
}

.lead-card-head {
  grid-area: person;
  display: grid;
  grid-template-columns: 2.15rem minmax(0, 1fr);
  gap: 0.55rem;
  align-items: center;
  min-width: 0;
}

.lead-avatar {
  display: grid;
  width: 2.15rem;
  height: 2.15rem;
  place-items: center;
  border-radius: 10px;
  background: linear-gradient(145deg, #22d3ee, #34d399);
  color: #06111f;
  font-weight: 900;
}

.lead-card strong,
.terminal-row strong,
.preview-row strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lead-card-head span,
.terminal-row small,
.preview-row span,
.launch-note {
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.76rem;
}

.lead-time {
  grid-column: 2;
  color: rgba(186, 230, 253, 0.82);
  font-size: 0.72rem;
}

.lead-meta-grid,
.launch-summary {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(4.75rem, 0.45fr);
  gap: 0.45rem;
}

.lead-meta-grid {
  grid-area: meta;
}

.lead-meta-grid div,
.launch-summary div,
.preview-row,
.recent-box {
  min-width: 0;
  border: 1px solid rgba(148, 163, 184, 0.13);
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.56);
  padding: 0.55rem;
}

.lead-meta-grid strong {
  display: block;
  overflow: hidden;
  margin-top: 0.25rem;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.9rem;
}

.lead-message {
  grid-area: message;
  display: -webkit-box;
  min-height: 0;
  overflow: hidden;
  margin: 0;
  color: rgba(226, 232, 240, 0.86);
  font-size: 0.8rem;
  line-height: 1.35;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.lead-footer {
  grid-area: actions;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  justify-content: stretch;
  gap: 0.55rem;
  min-width: 0;
  text-align: left;
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.76rem;
}

.lead-footer-main {
  display: grid;
  gap: 0.25rem;
  min-width: 0;
}

.lead-footer span,
.lead-footer strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lead-footer strong.ok {
  color: #5eead4;
}

.lead-footer strong.warn {
  color: #fbbf24;
}

.blacklist-btn {
  display: inline-grid;
  width: 100%;
  min-height: 2.1rem;
  place-items: center;
  border: 1px solid rgba(248, 113, 113, 0.28);
  border-radius: 10px;
  background: rgba(127, 29, 29, 0.18);
  padding: 0 0.45rem;
  color: #fecaca;
  font-size: 0.72rem;
  font-weight: 900;
  white-space: nowrap;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

.blacklist-btn:hover:not(:disabled),
.blacklist-btn:focus-visible {
  border-color: rgba(248, 113, 113, 0.72);
  background: rgba(127, 29, 29, 0.36);
  color: #fff1f2;
  outline: none;
}

.blacklist-btn:disabled {
  cursor: not-allowed;
  opacity: 0.46;
}

.terminal-list {
  display: grid;
  max-height: 22rem;
  gap: 0.55rem;
  overflow: auto;
  padding: 0 1rem 1rem;
}

.terminal-row {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  gap: 0.65rem;
  align-items: center;
  min-width: 0;
  min-height: 4rem;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 12px;
  background: rgba(8, 13, 28, 0.62);
  padding: 0.75rem;
}

.terminal-row.disabled {
  opacity: 0.55;
}

.terminal-row.selected {
  border-color: rgba(34, 211, 238, 0.55);
}

.terminal-dot {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 999px;
  background: #f59e0b;
}

.terminal-dot[data-tone='success'] {
  background: #2dd4bf;
  box-shadow: 0 0 14px rgba(45, 212, 191, 0.5);
}

.terminal-row em {
  max-width: 8rem;
  overflow: hidden;
  color: rgba(186, 230, 253, 0.82);
  font-size: 0.72rem;
  font-style: normal;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dm-check {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: rgba(226, 232, 240, 0.82);
  font-size: 0.82rem;
}

.message-panel {
  overflow: hidden;
}

.message-step-list {
  display: grid;
  gap: 0.75rem;
  padding: 0 1rem 1rem;
}

.message-step {
  min-width: 0;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 12px;
  background: rgba(8, 13, 28, 0.62);
  padding: 0.85rem;
}

.message-step.active {
  border-color: rgba(34, 211, 238, 0.52);
  box-shadow: 0 0 0 1px rgba(34, 211, 238, 0.16);
}

.step-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
  min-width: 0;
  margin-bottom: 0.65rem;
}

.step-head strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dm-textarea {
  width: 100%;
  resize: vertical;
  border: 1px solid rgba(148, 163, 184, 0.17);
  border-radius: 10px;
  background: rgba(3, 7, 18, 0.68);
  padding: 0.85rem;
  color: white;
  outline: none;
}

.step-type-tabs {
  padding: 0 0 0.75rem;
}

.media-step-note {
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 10px;
  background: rgba(3, 7, 18, 0.45);
  padding: 0.8rem;
  color: rgba(148, 163, 184, 0.86);
  font-size: 0.82rem;
}

.dm-media-box {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.65rem;
  align-items: stretch;
  margin-top: 0.7rem;
  border: 1px solid rgba(34, 211, 238, 0.18);
  border-radius: 12px;
  background: rgba(14, 116, 144, 0.12);
  padding: 0.8rem;
}

.dm-media-box.uploading {
  opacity: 0.7;
}

.dm-media-box strong,
.dm-media-box span {
  display: block;
}

.dm-media-box span {
  overflow: hidden;
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.75rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dm-media-box label {
  cursor: pointer;
  border: 1px solid rgba(34, 211, 238, 0.38);
  border-radius: 10px;
  background: rgba(8, 47, 73, 0.45);
  padding: 0.65rem 0.85rem;
  color: #67e8f9;
  font-size: 0.8rem;
  font-weight: 900;
  text-align: center;
}

.dm-media-box input,
.forward-grid input {
  min-height: 2.45rem;
  min-width: 0;
  border: 1px solid rgba(148, 163, 184, 0.17);
  border-radius: 9px;
  background: rgba(3, 7, 18, 0.62);
  padding: 0 0.7rem;
  color: white;
  outline: none;
}

.forward-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.forward-grid label {
  display: grid;
  gap: 0.4rem;
  color: rgba(148, 163, 184, 0.86);
  font-size: 0.78rem;
}

.step-delay {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.65rem;
  color: rgba(148, 163, 184, 0.86);
  font-size: 0.8rem;
}

.step-delay input {
  width: 5.5rem;
  min-height: 2.25rem;
  border: 1px solid rgba(148, 163, 184, 0.17);
  border-radius: 8px;
  background: rgba(3, 7, 18, 0.68);
  padding: 0 0.65rem;
  color: white;
}

.add-step-btn {
  margin: 0 1rem 1rem;
  width: calc(100% - 2rem);
}

.launch-panel {
  position: sticky;
  min-width: 0;
  top: 6rem;
}

.launch-summary {
  grid-template-columns: repeat(auto-fit, minmax(6.25rem, 1fr));
  padding: 1rem;
}

.launch-options {
  display: grid;
  gap: 0.65rem;
  padding: 0 1rem 1rem;
}

.launch-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9.25rem, 1fr));
  gap: 0.65rem;
  padding: 0 1rem 1rem;
}

.launch-grid input {
  min-width: 0;
}

.launch-grid small {
  color: rgba(148, 163, 184, 0.72);
}

.preview-list {
  display: grid;
  max-height: 24rem;
  gap: 0.65rem;
  overflow: auto;
  padding: 0 1rem 1rem;
}

.preview-row {
  display: grid;
  gap: 0.45rem;
}

.preview-row div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
  gap: 0.6rem;
}

.preview-row div span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-row em {
  color: #67e8f9;
  font-size: 0.75rem;
  font-style: normal;
}

.preview-row p {
  overflow-wrap: anywhere;
  color: rgba(226, 232, 240, 0.86);
  font-size: 0.82rem;
  line-height: 1.45;
}

.launch-note {
  padding: 0.85rem 1rem 1rem;
  text-align: center;
}

.dm-empty {
  display: grid;
  min-height: 8rem;
  place-items: center;
  border: 1px dashed rgba(148, 163, 184, 0.2);
  border-radius: 12px;
  color: rgba(148, 163, 184, 0.82);
  font-size: 0.86rem;
}

@media (max-width: 1500px) {
  .dm-workspace {
    grid-template-columns: minmax(0, 1fr);
  }

  .launch-panel {
    position: static;
  }
}

@media (max-width: 1180px) {
  .lead-card {
    grid-template-areas:
      "person actions"
      "meta meta"
      "message message";
    grid-template-columns: minmax(0, 1fr) minmax(7rem, 0.42fr);
  }

  .lead-footer {
    text-align: left;
  }
}

@media (max-width: 900px) {
  .dm-topbar,
  .dm-actions {
    display: grid;
    justify-content: stretch;
  }

  .dm-stat-grid,
  .launch-grid,
  .launch-summary {
    grid-template-columns: minmax(0, 1fr);
  }

  .dm-actions {
    min-width: 0;
  }

  .lead-toolbar,
  .terminal-toolbar {
    grid-template-columns: minmax(0, 1fr);
  }

  .dm-media-box,
  .forward-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .lead-card {
    grid-template-areas:
      "person"
      "meta"
      "message"
      "actions";
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .dm-title {
    font-size: 1.85rem;
  }

  .lead-bulk-row span {
    margin-left: 0;
    width: 100%;
  }

  .lead-card-head {
    grid-template-columns: 2.3rem minmax(0, 1fr);
  }

  .lead-time {
    grid-column: 2;
  }
}
</style>
