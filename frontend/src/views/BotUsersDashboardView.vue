<template>
  <div class="page-shell bot-users-shell">
    <div class="page-header">
      <div>
        <div class="eyebrow">BOT USERS</div>
        <h1 class="page-title">Bot 用户看板</h1>
        <p class="page-subtitle">管理员查看机器人用户、绑定 Web 用户、账号池、任务和个人设置。普通用户不可见。</p>
      </div>
      <div class="page-actions">
        <span class="status-pill" data-tone="info">共 {{ users.length }} 个 Bot 用户</span>
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
      </div>
    </div>

    <div class="bot-user-admin-grid">
      <GlassCard class="bot-admin-panel">
        <div class="panel-title compact-title">
          <span>🔐</span>
          <div>
            <h2>卡密管理</h2>
            <p>生成、禁用和删除 Bot 用户会员卡密。</p>
          </div>
        </div>

        <div class="license-form">
          <label>
            <span>数量</span>
            <input v-model.number="licenseForm.count" min="1" max="200" type="number" />
          </label>
          <label>
            <span>使用时间/天</span>
            <input v-model.number="licenseForm.duration_days" min="1" type="number" />
          </label>
          <label>
            <span>绑定数</span>
            <input v-model.number="licenseForm.max_bind" min="1" type="number" />
          </label>
          <GlassButton variant="primary" :loading="licenseBusy" @click="createLicenses">生成卡密</GlassButton>
        </div>

        <div class="license-list scrollbar-thin">
          <div v-for="license in licenses" :key="license.id" class="license-card">
            <div>
              <strong>{{ license.code }}</strong>
              <span>{{ licenseDurationDays(license) }} 天 · 已绑 {{ license.bound_count }}/{{ license.max_bind }}</span>
              <span v-if="license.bound_user_id">绑定：{{ license.bound_username || '未记录昵称' }} · ID {{ license.bound_user_id }} · 到期 {{ formatDateTime(license.expires_at) }}</span>
              <span v-else>绑定：未绑定</span>
            </div>
            <div class="license-actions">
              <span class="status-pill" :data-tone="licenseTone(license.status)">{{ licenseStatusText(license.status) }}</span>
              <button type="button" @click="toggleLicense(license)">{{ license.status === 'disabled' ? '启用' : '禁用' }}</button>
              <button type="button" @click="deleteLicense(license.id)">删除</button>
            </div>
          </div>
          <div v-if="!licenses.length" class="empty-line">还没有卡密</div>
        </div>
      </GlassCard>

      <GlassCard class="bot-admin-panel authorized-panel">
        <div class="panel-title compact-title">
          <span>👥</span>
          <div>
            <h2>授权用户</h2>
            <p>统一查看试用用户、卡密用户和最近使用状态。</p>
          </div>
        </div>

        <div class="subscriber-table scrollbar-thin">
          <table>
            <thead>
              <tr>
                <th>用户</th>
                <th>权限</th>
                <th>状态</th>
                <th>试用到期</th>
                <th>授权到期</th>
                <th>最近使用</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="subscriber in subscribers" :key="subscriber.id">
                <td>
                  <strong>{{ subscriber.nickname || subscriber.username || subscriber.telegram_user_id }}</strong>
                  <span>{{ subscriber.telegram_user_id }}</span>
                </td>
                <td>{{ planText(subscriber.plan) }}</td>
                <td><span class="status-pill" :data-tone="subscriber.status === 'active' ? 'success' : subscriber.status === 'expired' ? 'warning' : 'danger'">{{ statusText(subscriber.status) }}</span></td>
                <td>{{ formatDateTime(subscriber.trial_ends_at) }}</td>
                <td>{{ formatDateTime(subscriber.expires_at) }}</td>
                <td>{{ formatDateTime(subscriber.last_seen_at) }}</td>
              </tr>
            </tbody>
          </table>
          <div v-if="!subscribers.length" class="empty-line">还没有授权用户</div>
        </div>
      </GlassCard>
    </div>

    <div class="bot-users-layout">
      <GlassCard class="bot-user-list">
        <div v-for="item in users" :key="item.id" :class="['bot-user-row', { active: selected?.id === item.id }]" @click="select(item)">
          <div>
            <strong>{{ item.nickname || item.username || item.telegram_user_id }}</strong>
            <span>{{ item.telegram_user_id }} · @{{ item.username || '未记录' }}</span>
          </div>
          <span class="status-pill" :data-tone="item.status === 'active' ? 'success' : item.status === 'expired' ? 'warning' : 'danger'">{{ permissionText(item) }}</span>
          <small>开通 {{ formatDateTime(item.authorized_at || item.trial_started_at) }} · 到期 {{ formatDateTime(item.expires_at || item.trial_ends_at) }}</small>
          <small>账号 {{ item.account_count }} · 任务 {{ item.task_count }} · 私信 {{ item.dm_quota_used || 0 }}/{{ item.dm_quota_total || 0 }} · 邀请 {{ item.invite_count }}</small>
        </div>
        <div v-if="!users.length" class="empty-line">暂无 Bot 用户</div>
      </GlassCard>

      <GlassCard class="bot-user-detail">
        <template v-if="detail">
          <div class="detail-head">
            <div>
              <h2>{{ detail.subscriber.nickname || detail.subscriber.username || detail.subscriber.telegram_user_id }}</h2>
              <p>{{ detail.subscriber.telegram_user_id }} · 最近使用 {{ formatDateTime(detail.subscriber.last_seen_at) }}</p>
            </div>
            <span class="status-pill" :data-tone="detail.subscriber.force_joined ? 'success' : 'warning'">
              {{ detail.subscriber.force_joined ? '已完成加群' : '未确认加群' }}
            </span>
          </div>

          <div class="detail-summary-grid">
            <div>
              <span>Bot 开通时间</span>
              <strong>{{ formatDateTime(detail.subscriber.authorized_at || detail.subscriber.trial_started_at || detail.subscriber.created_at) }}</strong>
            </div>
            <div>
              <span>到期时间</span>
              <strong>{{ formatDateTime(detail.subscriber.expires_at || detail.subscriber.trial_ends_at) }}</strong>
            </div>
            <div>
              <span>运行中的任务</span>
              <strong>{{ runningTaskCount(detail) }} 个</strong>
            </div>
            <div>
              <span>私信额度</span>
              <strong>{{ detail.subscriber.dm_quota_used || 0 }} / {{ detail.subscriber.dm_quota_total || 0 }}</strong>
            </div>
            <div>
              <span>监听关键词</span>
              <strong>{{ keywordSummary(detail.subscriber) }}</strong>
            </div>
            <div>
              <span>账号权限</span>
              <strong>{{ permissionText(detail.subscriber) }}</strong>
            </div>
          </div>

          <div class="admin-edit-grid">
            <label>
              <span>绑定 Web 用户 ID</span>
              <input v-model="edit.user_id" placeholder="选择用户管理中的 Web 用户 ID" />
            </label>
            <label>
              <span>增加使用时间（天）</span>
              <input v-model.number="edit.trial_days" min="0" type="number" />
            </label>
            <label>
              <span>匹配模式</span>
              <select v-model="edit.match_mode">
                <option value="fuzzy">模糊匹配</option>
                <option value="exact">精准匹配</option>
              </select>
            </label>
            <label>
              <span>关键词上限</span>
              <input v-model.number="edit.keyword_limit" min="0" type="number" />
            </label>
            <label>
              <span>推送位置</span>
              <input v-model="edit.push_chat_id" placeholder="留空=推送到用户私聊，或填 @频道 / -100..." />
            </label>
            <label>
              <span>消息去重分钟</span>
              <input v-model.number="edit.message_dedup_minutes" min="0" type="number" />
            </label>
            <label>
              <span>私信额度总数</span>
              <input v-model.number="edit.dm_quota_total" min="0" type="number" />
            </label>
            <label>
              <span>私信已使用</span>
              <input v-model.number="edit.dm_quota_used" min="0" type="number" />
            </label>
          </div>

          <div class="flag-grid">
            <label v-for="flag in flags" :key="flag.key">
              <input v-model="edit[flag.key]" type="checkbox" />
              <span>{{ flag.label }}</span>
            </label>
          </div>

          <div class="detail-section">
            <h3>可用私信账号池分组</h3>
            <div class="multi-select-field">
              <span>下拉可多选账号池分组</span>
              <button type="button" class="group-dropdown-trigger" @click="groupDropdownOpen = !groupDropdownOpen">
                {{ selectedTerminalGroupText(detail) }}
              </button>
              <div v-if="groupDropdownOpen" class="group-dropdown-panel">
                <label v-for="group in detail.terminal_groups" :key="group.id" class="group-dropdown-item">
                  <input :checked="isTerminalGroupSelected(group.id)" type="checkbox" @change="toggleTerminalGroup(group.id)" />
                  <span>{{ group.name }}</span>
                </label>
              </div>
            </div>
            <div v-if="!detail.terminal_groups.length" class="empty-line">暂无终端分组</div>
          </div>

          <div class="bot-actions">
            <GlassButton variant="primary" :loading="saving" @click="save">保存用户设置</GlassButton>
          </div>

          <div class="stats-grid">
            <div><span>试用到期</span><strong>{{ formatDateTime(detail.subscriber.trial_ends_at) }}</strong></div>
            <div><span>授权到期</span><strong>{{ formatDateTime(detail.subscriber.expires_at) }}</strong></div>
            <div><span>私信剩余额度</span><strong>{{ remainingQuota(detail.subscriber) }}</strong></div>
            <div><span>账号分组</span><strong>{{ detail.groups.length }}</strong></div>
            <div><span>账号池</span><strong>{{ detail.accounts.length }}</strong></div>
            <div><span>监听任务</span><strong>{{ listenerTasks(detail).length }}</strong></div>
            <div><span>私信任务</span><strong>{{ detail.tasks.length }}</strong></div>
            <div><span>邀请记录</span><strong>{{ detail.referrals.length }}</strong></div>
          </div>

          <div class="detail-section">
            <h3>监听关键词</h3>
            <div class="keyword-chip-row">
              <span v-for="keyword in subscriberKeywords(detail.subscriber)" :key="keyword">{{ keyword }}</span>
              <div v-if="!subscriberKeywords(detail.subscriber).length" class="empty-line">暂未设置关键词</div>
            </div>
          </div>

          <div class="detail-section">
            <h3>账号分组与账号池</h3>
            <div class="mini-table">
              <div v-for="group in detail.groups" :key="group.id">
                <strong>{{ group.is_default ? '1. ' : '' }}{{ group.name }}</strong>
                <span>{{ detail.accounts.filter((account) => account.group_id === group.id).length }} 个账号</span>
              </div>
            </div>
          </div>

          <div class="detail-section">
            <h3>监听任务</h3>
            <div class="mini-table">
              <div v-for="task in listenerTasks(detail)" :key="`listener-${task.id}`">
                <strong>{{ taskDisplayName(task) }}</strong>
                <span>
                  {{ taskTypeText(task.type) }} · {{ taskStatusText(task.status) }} · 进度 {{ task.progress || 0 }}% · 更新 {{ formatDateTime(task.updated_at) }}
                  <template v-if="task.created_at"> · 创建 {{ formatDateTime(task.created_at) }}</template>
                  <template v-if="taskReason(task)"> · 原因：{{ taskReason(task) }}</template>
                </span>
              </div>
              <div v-if="!listenerTasks(detail).length" class="empty-line">暂无监听任务</div>
            </div>
          </div>

          <div class="detail-section">
            <h3>私信任务</h3>
            <div class="mini-table">
              <div v-for="task in detail.tasks" :key="`dm-${task.id}`">
                <strong>{{ botDMDisplayName() }}</strong>
                <span>
                  私信任务 · {{ dmTaskStatusText(task.status) }} · {{ task.account_group_name }} · {{ task.messages?.length || 0 }} 条编排 ·
                  {{ task.min_delay_seconds || 4 }}-{{ task.max_delay_seconds || 8 }} 秒 · 已发 {{ task.sent_count }}
                  <template v-if="task.keywords?.length"> · 关键词 {{ task.keywords.join('、') }}</template>
                  <template v-if="task.started_at"> · 开始 {{ formatDateTime(task.started_at) }}</template>
                  <template v-if="task.ended_at"> · 结束 {{ formatDateTime(task.ended_at) }}</template>
                  <template v-if="dmTaskReason(task)"> · 原因：{{ dmTaskReason(task) }}</template>
                </span>
              </div>
              <div v-if="!detail.tasks.length" class="empty-line">暂无私信任务</div>
            </div>
          </div>
        </template>
        <div v-else class="empty-line">请选择左侧 Bot 用户查看详情</div>
      </GlassCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, type BotLicense, type BotSubscriber, type BotUserDashboardDetail, type BotUserDashboardItem } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUiStore } from '../stores/ui'
import { taskDisplayName, taskStatusDisplay, taskTypeDisplay } from '../utils/taskDisplay'

const ui = useUiStore()
const loading = ref(false)
const saving = ref(false)
const licenseBusy = ref(false)
const users = ref<BotUserDashboardItem[]>([])
const licenses = ref<BotLicense[]>([])
const subscribers = ref<BotSubscriber[]>([])
const selected = ref<BotUserDashboardItem | null>(null)
const detail = ref<BotUserDashboardDetail | null>(null)
const edit = reactive<Record<string, any>>({})
const groupDropdownOpen = ref(false)
const licenseForm = reactive({ count: 10, duration_days: 30, max_bind: 1 })
const flags = [
  { key: 'push_enabled', label: '推送开启' },
  { key: 'user_blacklist_enabled', label: '用户黑名单' },
  { key: 'risk_control_enabled', label: 'AI 过滤' }
]

async function load() {
  loading.value = true
  try {
    const [userData, licenseData, subscriberData] = await Promise.all([
      api.botUsers(),
      api.botLicenses(),
      api.botSubscribers()
    ])
    users.value = userData
    licenses.value = licenseData
    subscribers.value = subscriberData
    if (selected.value) {
      const fresh = users.value.find((item) => item.id === selected.value?.id)
      if (fresh) await select(fresh)
    }
  } finally {
    loading.value = false
  }
}

async function createLicenses() {
  licenseBusy.value = true
  try {
    licenses.value = [...(await api.createBotLicenses({
      count: licenseForm.count,
      duration_hour: Math.max(1, Number(licenseForm.duration_days || 1)) * 24,
      max_bind: licenseForm.max_bind
    })), ...licenses.value]
    ui.toast({ title: '卡密已生成', message: `已生成 ${licenseForm.count} 个卡密。`, tone: 'success' })
  } catch (err) {
    ui.toast({ title: '生成失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    licenseBusy.value = false
  }
}

async function toggleLicense(license: BotLicense) {
  try {
    const nextStatus = license.status === 'disabled' ? 'unused' : 'disabled'
    const updated = await api.updateBotLicenseStatus(license.id, nextStatus)
    licenses.value = licenses.value.map((item) => (item.id === updated.id ? updated : item))
  } catch (err) {
    ui.toast({ title: '卡密状态更新失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  }
}

async function deleteLicense(id: string) {
  try {
    await api.deleteBotLicense(id)
    licenses.value = licenses.value.filter((item) => item.id !== id)
    ui.toast({ title: '卡密已删除', message: '已从 Bot 用户看板移除。', tone: 'success' })
  } catch (err) {
    ui.toast({ title: '删除失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  }
}

async function select(item: BotUserDashboardItem) {
  selected.value = item
  detail.value = await api.botUserDetail(item.id)
  groupDropdownOpen.value = false
  Object.assign(edit, {
    user_id: detail.value.subscriber.user_id || '',
    trial_days: 0,
    match_mode: detail.value.subscriber.match_mode || 'fuzzy',
    keyword_limit: detail.value.subscriber.keyword_limit || 0,
    push_chat_id: detail.value.subscriber.push_chat_id || '',
    message_dedup_minutes: detail.value.subscriber.message_dedup_minutes || 0,
    dm_quota_total: detail.value.subscriber.dm_quota_total || 0,
    dm_quota_used: detail.value.subscriber.dm_quota_used || 0,
    push_enabled: Boolean(detail.value.subscriber.push_enabled),
    user_blacklist_enabled: Boolean(detail.value.subscriber.user_blacklist_enabled),
    risk_control_enabled: Boolean(detail.value.subscriber.risk_control_enabled),
    private_terminal_group_ids: Array.isArray(detail.value.subscriber.private_terminal_group_ids) ? [...detail.value.subscriber.private_terminal_group_ids] : []
  })
}

async function save() {
  if (!selected.value) return
  saving.value = true
  try {
    await api.updateBotUser(selected.value.id, edit)
    ui.toast({ title: 'Bot 用户已更新', message: '设置已同步，机器人下次刷新会读取最新配置。', tone: 'success' })
    await select(selected.value)
    await load()
  } catch (err) {
    ui.toast({ title: '保存失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    saving.value = false
  }
}

function formatDateTime(value?: string | null) {
  if (!value) return '--'
  return new Date(value).toLocaleString()
}

function remainingQuota(subscriber: BotUserDashboardDetail['subscriber']) {
  const total = Number(subscriber.dm_quota_total || 0)
  const used = Number(subscriber.dm_quota_used || 0)
  return `${Math.max(total - used, 0)} / ${total}`
}

function licenseDurationDays(license: BotLicense) {
  return Math.max(1, Math.ceil(Number(license.duration_hour || 0) / 24))
}

function licenseTone(status: string) {
  if (status === 'unused') return 'success'
  if (status === 'used') return 'info'
  if (status === 'disabled') return 'danger'
  return 'warning'
}

function licenseStatusText(status: string) {
  return ({ unused: '未使用', used: '已绑定', disabled: '已禁用', expired: '已过期' } as Record<string, string>)[status] || status
}

function planText(plan: string) {
  return ({ trial: '试用用户', license: '会员用户', member: '会员用户', paid: '会员用户', vip: '会员用户' } as Record<string, string>)[plan] || plan || '未设置'
}

function statusText(status: string) {
  return ({ active: '正常', expired: '已过期', disabled: '已禁用' } as Record<string, string>)[status] || status || '未知'
}

function permissionText(subscriber: BotSubscriber) {
  if (subscriber.status === 'expired') return '会员已过期'
  if (['license', 'member', 'paid', 'vip'].includes(String(subscriber.plan || '').toLowerCase())) return '会员用户'
  if (subscriber.plan === 'trial') return '试用用户'
  return statusText(subscriber.status)
}

function subscriberKeywords(subscriber: BotSubscriber) {
  return Array.isArray(subscriber.keywords) ? subscriber.keywords.filter(Boolean) : []
}

function keywordSummary(subscriber: BotSubscriber) {
  const list = subscriberKeywords(subscriber)
  if (!list.length) return '未设置'
  if (list.length <= 3) return list.join('、')
  return `${list.slice(0, 3).join('、')} 等 ${list.length} 个`
}

function allUserTasks(current: BotUserDashboardDetail) {
  return Array.isArray(current.all_tasks) ? current.all_tasks : []
}

function listenerTasks(current: BotUserDashboardDetail) {
  return allUserTasks(current).filter((task) => task.type === 'scrm_listener')
}

function runningTaskCount(current: BotUserDashboardDetail) {
  return allUserTasks(current).filter((task) => /running|queued|pending/i.test(task.status)).length +
    current.tasks.filter((task) => /active|running|queued/i.test(task.status)).length
}

function taskTypeText(value: string) {
  return taskTypeDisplay(value)
}

function taskStatusText(status: string) {
  return taskStatusDisplay(status)
}

function botDMDisplayName() {
  const subscriber = detail.value?.subscriber
  const owner = subscriber?.nickname || subscriber?.username || subscriber?.telegram_user_id || 'Bot用户'
  return `BOT--${owner}--私信任务`
}

function dmTaskStatusText(status: string) {
  return ({ active: '进行中', running: '进行中', queued: '排队中', paused: '已暂停', completed: '已完成', failed: '失败' } as Record<string, string>)[status] || status
}

function taskReason(task: Record<string, any>) {
  const summary = (task.summary || {}) as Record<string, unknown>
  const payload = (task.payload || {}) as Record<string, unknown>
  const keys = ['reason', 'stop_reason', 'pause_reason', 'error', 'last_error']
  for (const key of keys) {
    const value = summary[key] ?? payload[key]
    if (typeof value === 'string' && value.trim() !== '') {
      return value.trim()
    }
  }
  return ''
}

function dmTaskReason(task: Record<string, any>) {
  const keys = ['stop_reason', 'pause_reason', 'error', 'last_error', 'reason']
  for (const key of keys) {
    const value = task?.[key]
    if (typeof value === 'string' && value.trim() !== '') {
      return value.trim()
    }
  }
  return ''
}

function isTerminalGroupSelected(groupID: string) {
  return Array.isArray(edit.private_terminal_group_ids) && edit.private_terminal_group_ids.includes(groupID)
}

function selectedTerminalGroupText(current: BotUserDashboardDetail) {
  const ids = Array.isArray(edit.private_terminal_group_ids) ? edit.private_terminal_group_ids : []
  if (!ids.length) return '请选择私信账号池分组（可多选）'
  const names = current.terminal_groups
    .filter((group) => ids.includes(group.id))
    .map((group) => group.name)
  if (!names.length) return '已选分组'
  if (names.length <= 2) return names.join('、')
  return `${names.slice(0, 2).join('、')} 等 ${names.length} 个分组`
}

function toggleTerminalGroup(groupID: string) {
  const current = Array.isArray(edit.private_terminal_group_ids) ? [...edit.private_terminal_group_ids] : []
  edit.private_terminal_group_ids = current.includes(groupID) ? current.filter((item) => item !== groupID) : [...current, groupID]
}

onMounted(load)
</script>

<style scoped>
.bot-users-layout {
  display: grid;
  grid-template-columns: minmax(18rem, 0.8fr) minmax(0, 1.8fr);
  gap: 1rem;
  min-height: 0;
}

.bot-user-admin-grid {
  display: grid;
  grid-template-columns: minmax(26rem, 0.75fr) minmax(0, 1.25fr);
  gap: 1rem;
}

.bot-admin-panel {
  min-height: 18rem;
}

.compact-title {
  margin-bottom: 1rem;
}

.license-form {
  display: grid;
  grid-template-columns: 0.9fr 1fr 0.9fr auto;
  gap: 0.65rem;
  align-items: end;
}

.license-form label {
  display: grid;
  gap: 0.35rem;
}

.license-form label span {
  color: var(--app-text-muted);
  font-size: 0.76rem;
  font-weight: 800;
}

.license-form input {
  min-width: 0;
  min-height: 2.65rem;
  border-radius: 8px;
  padding: 0.7rem 0.8rem;
}

.license-list {
  display: grid;
  gap: 0.55rem;
  max-height: 15rem;
  margin-top: 0.9rem;
  overflow: auto;
}

.license-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  padding: 0.8rem;
}

.license-card strong {
  color: white;
  font-weight: 900;
}

.license-card span:not(.status-pill) {
  display: block;
  margin-top: 0.25rem;
  color: var(--app-text-muted);
  font-size: 0.78rem;
}

.license-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}

.license-actions button {
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.05);
  color: white;
  padding: 0.5rem 0.65rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.license-actions button:hover {
  border-color: rgba(34, 197, 94, 0.42);
  background: rgba(34, 197, 94, 0.1);
}

.subscriber-table {
  max-height: 17rem;
  overflow: auto;
}

.subscriber-table table {
  width: 100%;
  min-width: 780px;
}

.subscriber-table th,
.subscriber-table td {
  padding: 0.8rem;
  text-align: left;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.subscriber-table th {
  color: var(--app-text-muted);
  font-size: 0.78rem;
}

.subscriber-table td strong {
  display: block;
  color: white;
}

.subscriber-table td span:not(.status-pill) {
  display: block;
  color: var(--app-text-muted);
  font-size: 0.76rem;
}

.bot-user-list,
.bot-user-detail {
  min-height: 42rem;
}

.bot-user-row {
  display: grid;
  gap: 0.45rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.85rem;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.bot-user-row + .bot-user-row {
  margin-top: 0.65rem;
}

.bot-user-row.active,
.bot-user-row:hover {
  border-color: rgba(34, 197, 94, 0.45);
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.13), rgba(79, 172, 254, 0.08));
}

.bot-user-row strong,
.detail-head h2,
.stats-grid strong {
  color: white;
}

.bot-user-row span:not(.status-pill),
.bot-user-row small,
.detail-head p,
.stats-grid span,
.mini-table span {
  color: var(--app-text-muted);
}

.detail-head {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.admin-edit-grid,
.stats-grid,
.detail-summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
  gap: 0.75rem;
}

.admin-edit-grid label {
  display: grid;
  gap: 0.4rem;
}

.admin-edit-grid span {
  color: var(--app-text-muted);
  font-size: 0.76rem;
  font-weight: 800;
}

.admin-edit-grid input,
.admin-edit-grid select {
  min-height: 2.65rem;
  border-radius: 8px;
  padding: 0.75rem 0.85rem;
}

.flag-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.65rem;
}

.flag-grid label,
.stats-grid div,
.detail-summary-grid div,
.mini-table div {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  padding: 0.8rem;
}

.detail-summary-grid div {
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.12), rgba(79, 172, 254, 0.055));
}

.detail-summary-grid span {
  display: block;
  color: var(--app-text-muted);
  font-size: 0.74rem;
  font-weight: 800;
}

.detail-summary-grid strong {
  display: block;
  margin-top: 0.28rem;
  color: white;
}

.flag-grid label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.terminal-group-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.65rem;
}

.terminal-group-item {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  padding: 0.8rem;
}

.terminal-group-item input {
  width: 1rem;
  height: 1rem;
}

.multi-select-field {
  display: grid;
  gap: 0.45rem;
  position: relative;
}

.multi-select-field span {
  color: var(--app-text-muted);
  font-size: 0.76rem;
  font-weight: 800;
}

.group-dropdown-trigger {
  min-height: 2.7rem;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  color: white;
  padding: 0.65rem 0.8rem;
  text-align: left;
  font-weight: 800;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.group-dropdown-trigger:hover {
  border-color: rgba(0, 242, 254, 0.42);
  box-shadow: 0 4px 12px rgba(0, 242, 254, 0.22);
}

.group-dropdown-panel {
  position: absolute;
  z-index: 20;
  top: calc(100% + 0.35rem);
  left: 0;
  right: 0;
  max-height: 14rem;
  overflow: auto;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-top-color: rgba(255, 255, 255, 0.2);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.92);
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.35);
  padding: 0.45rem;
}

.group-dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  border-radius: 8px;
  padding: 0.55rem 0.6rem;
  color: #cfe6ff;
  font-weight: 700;
}

.group-dropdown-item:hover {
  background: rgba(255, 255, 255, 0.06);
}

.group-dropdown-item input {
  width: 1rem;
  height: 1rem;
}

.keyword-chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.keyword-chip-row > span {
  border: 1px solid rgba(0, 242, 254, 0.2);
  border-radius: 999px;
  background: rgba(0, 242, 254, 0.08);
  color: #67e8f9;
  padding: 0.42rem 0.7rem;
  font-size: 0.82rem;
  font-weight: 900;
}

.detail-section h3 {
  margin: 0 0 0.7rem;
  color: white;
}

.mini-table {
  display: grid;
  gap: 0.55rem;
}

.mini-table div {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

@media (max-width: 1280px) {
  .bot-user-admin-grid,
  .bot-users-layout,
  .admin-edit-grid,
  .stats-grid,
  .detail-summary-grid,
  .flag-grid,
  .license-form {
    grid-template-columns: 1fr;
  }
}
</style>
