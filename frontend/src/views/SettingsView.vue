<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">系统设置</h1>
        <p class="page-subtitle">安全、频控、日志和适配器全部改成真实可保存配置，页面保持满幅工作台布局，不再只是说明卡片。</p>
      </div>
      <div class="page-actions">
        <span class="status-pill" :data-tone="dirty ? 'warning' : 'success'">{{ dirty ? '有未保存改动' : '已同步' }}</span>
        <span class="status-pill" data-tone="info">最近更新 {{ formatDate(updatedAt) }}</span>
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
        <GlassButton variant="primary" :loading="saving" :disabled="!dirty" @click="save">保存设置</GlassButton>
      </div>
    </div>

    <div class="grid flex-1 gap-4 xl:grid-cols-[minmax(0,1.15fr)_360px]">
      <div class="space-y-4">
        <GlassCard class="h-full">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">Security</div>
              <h2 class="mt-2 text-xl font-black">安全与隔离</h2>
            </div>
            <span class="status-pill" :data-tone="form.security.enforce_tenant_isolation ? 'success' : 'warning'">
              {{ form.security.enforce_tenant_isolation ? '强隔离' : '已放宽' }}
            </span>
          </div>
          <div class="space-y-3">
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">租户强隔离</div>
                <div class="mt-1 text-sm text-steel">普通用户只读写当前租户数据</div>
              </div>
              <input v-model="form.security.enforce_tenant_isolation" type="checkbox" class="h-5 w-5" />
            </label>
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">主动触达需管理员复核</div>
                <div class="mt-1 text-sm text-steel">新建主动触达任务时强制进入复核流程</div>
              </div>
              <input v-model="form.security.require_admin_approval" type="checkbox" class="h-5 w-5" />
            </label>
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">敏感日志脱敏</div>
                <div class="mt-1 text-sm text-steel">手机号、会话标识与代理字段按审计规则脱敏</div>
              </div>
              <input v-model="form.security.mask_sensitive_logs" type="checkbox" class="h-5 w-5" />
            </label>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="text-xs uppercase tracking-[0.16em] text-steel">Throughput</div>
          <h2 class="mt-2 text-xl font-black">频控与刷新</h2>
          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">全局最大并发任务</div>
              <input v-model.number="form.frequency.max_concurrent_tasks" type="number" min="1" max="64" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
            </label>
            <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">主动触达并发槽位</div>
              <input v-model.number="form.frequency.max_concurrent_outreach" type="number" min="1" max="32" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
            </label>
            <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">实时日志单批条数</div>
              <input v-model.number="form.frequency.ws_log_batch_size" type="number" min="20" max="200" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
            </label>
            <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">Dashboard 刷新周期（秒）</div>
              <input v-model.number="form.frequency.dashboard_refresh_second" type="number" min="10" max="300" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
            </label>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">Listener Health</div>
              <h2 class="mt-2 text-xl font-black">监听健康检测</h2>
            </div>
            <span class="status-pill" :data-tone="form.listener_health.auto_account_check_enabled ? 'success' : 'warning'">
              {{ form.listener_health.auto_account_check_enabled ? '定时检测开启' : '手动检测' }}
            </span>
          </div>
          <div class="space-y-3">
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">自动刷新监听账号状态</div>
                <div class="mt-1 text-sm text-steel">后台按设定周期创建“一键检测监听账号”任务并写入任务日志</div>
              </div>
              <input v-model="form.listener_health.auto_account_check_enabled" type="checkbox" class="h-5 w-5" />
            </label>
            <div class="grid gap-4 md:grid-cols-2">
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">账号状态检测周期（分钟）</div>
                <input v-model.number="form.listener_health.account_check_interval_minutes" type="number" min="5" max="1440" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">无消息提醒阈值（分钟）</div>
                <input v-model.number="form.listener_health.silence_alert_minutes" type="number" min="1" max="1440" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
            </div>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">Risk Control</div>
              <h2 class="mt-2 text-xl font-black">风控避让阈值</h2>
            </div>
            <span class="status-pill" :data-tone="form.risk_control.auto_bypass_high_risk ? 'warning' : 'info'">
              {{ form.risk_control.auto_bypass_high_risk ? '自动避让已开启' : '自动避让已关闭' }}
            </span>
          </div>
          <div class="space-y-3">
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">高风险账号自动避让</div>
                <div class="mt-1 text-sm text-steel">发送、私信、加群选号时，达到阈值的账号会自动让出调度机会。</div>
              </div>
              <input v-model="form.risk_control.auto_bypass_high_risk" type="checkbox" class="h-5 w-5" />
            </label>
            <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">策略预设</div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button
                  v-for="item in riskPresetOptions"
                  :key="item.value"
                  type="button"
                  class="terminal-filter-chip"
                  :class="{ 'terminal-filter-chip-active': activeRiskPreset === item.value }"
                  @click="applyRiskPreset(item.value)"
                >
                  {{ item.label }}
                </button>
              </div>
              <div class="mt-3 text-sm text-steel">{{ riskPresetHelpText }}</div>
            </div>
            <div class="grid gap-4 md:grid-cols-2">
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">生效中限制达到多少条就避让</div>
                <input v-model.number="form.risk_control.auto_bypass_active_restrictions" type="number" min="1" max="20" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">24 小时命中多少次就避让</div>
                <input v-model.number="form.risk_control.auto_bypass_failures_24h" type="number" min="1" max="200" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
            </div>
            <div class="grid gap-4 md:grid-cols-2">
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">每个账号发信冷却（分钟）</div>
                <input v-model.number="form.risk_control.message_cooldown_minutes" type="number" min="1" max="1440" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">发信时间随机浮动（±分钟）</div>
                <input v-model.number="form.risk_control.message_jitter_minutes" type="number" min="0" max="120" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">每账号每日最多加群</div>
                <input v-model.number="form.risk_control.join_daily_limit" type="number" min="1" max="1000" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
                <div class="text-sm text-steel">加群间隔（分钟）</div>
                <input v-model.number="form.risk_control.join_interval_minutes" type="number" min="1" max="1440" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
              <label class="rounded-2xl border border-white/10 bg-white/5 p-4 md:col-span-2">
                <div class="text-sm text-steel">加群时间随机浮动（±分钟）</div>
                <input v-model.number="form.risk_control.join_jitter_minutes" type="number" min="0" max="240" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
              </label>
            </div>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="text-xs uppercase tracking-[0.16em] text-steel">Audit</div>
          <h2 class="mt-2 text-xl font-black">日志与审计</h2>
          <div class="mt-5 grid gap-4 md:grid-cols-[220px_minmax(0,1fr)]">
            <label class="rounded-2xl border border-white/10 bg-white/5 p-4">
              <div class="text-sm text-steel">日志保留天数</div>
              <input v-model.number="form.audit.log_retention_days" type="number" min="7" max="365" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm" />
            </label>
            <div class="space-y-3">
              <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
                <div>
                  <div class="font-semibold">开启实时日志流</div>
                  <div class="mt-1 text-sm text-steel">任务中心和主动触达详情页实时拉取最新日志</div>
                </div>
                <input v-model="form.audit.realtime_log_stream" type="checkbox" class="h-5 w-5" />
              </label>
              <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
                <div>
                  <div class="font-semibold">失败时提醒</div>
                  <div class="mt-1 text-sm text-steel">任务失败后在控制台保留高优先级提醒</div>
                </div>
                <input v-model="form.audit.notify_on_failure" type="checkbox" class="h-5 w-5" />
              </label>
            </div>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">Adapter</div>
              <h2 class="mt-2 text-xl font-black">适配器与执行策略</h2>
            </div>
            <span class="status-pill" :data-tone="form.adapter.telegram_apply_enabled ? 'success' : 'warning'">
              {{ form.adapter.telegram_apply_enabled ? '可执行' : '仅建任务' }}
            </span>
          </div>
          <div class="grid gap-3 md:grid-cols-2">
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">资料同步适配器</div>
                <div class="mt-1 text-sm text-steel">账号检测时同步手机号、昵称、签名和主页</div>
              </div>
              <input v-model="form.adapter.telegram_sync_enabled" type="checkbox" class="h-5 w-5" />
            </label>
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">执行适配器</div>
                <div class="mt-1 text-sm text-steel">允许进入真实执行队列，不再只停留在计划层</div>
              </div>
              <input v-model="form.adapter.telegram_apply_enabled" type="checkbox" class="h-5 w-5" />
            </label>
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">主动触达默认 dry-run</div>
                <div class="mt-1 text-sm text-steel">新建主动触达任务时保留 dry-run 保护层</div>
              </div>
              <input v-model="form.adapter.outreach_dry_run" type="checkbox" class="h-5 w-5" />
            </label>
            <label class="flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div>
                <div class="font-semibold">工作流默认 dry-run</div>
                <div class="mt-1 text-sm text-steel">执行通知工作流时按设置决定是否只排队</div>
              </div>
              <input v-model="form.adapter.workflow_dry_run" type="checkbox" class="h-5 w-5" />
            </label>
          </div>
        </GlassCard>
      </div>

      <div class="space-y-4">
        <GlassCard tone="cyan" class="h-full">
          <div class="text-xs uppercase tracking-[0.16em] text-steel">Live Policy</div>
          <h2 class="mt-2 text-xl font-black">当前策略摘要</h2>
          <div class="mt-5 grid gap-3">
            <div v-for="item in summaryItems" :key="item.label" class="metric-card app-card p-4" :data-tone="item.tone">
              <div class="text-sm text-steel">{{ item.label }}</div>
              <div class="mt-2 text-lg font-black">{{ item.value }}</div>
              <div class="mt-2 text-sm text-steel">{{ item.help }}</div>
            </div>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="text-xs uppercase tracking-[0.16em] text-steel">Execution</div>
          <h2 class="mt-2 text-xl font-black">配置回流说明</h2>
          <div class="mt-5 space-y-3 text-sm leading-7 text-steel">
            <p>主动触达页会读取这里的 dry-run、执行适配器与复核开关，创建任务时直接带入当前策略。</p>
            <p>工作流执行日志会同步展示当前是否按 dry-run 排队，方便你核对设置是否已经生效。</p>
            <p>实时日志批量条数已经接入日志 WebSocket，任务与日志页会按这里的数值拉取。</p>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">Version</div>
              <h2 class="mt-2 text-xl font-black">版本更新</h2>
            </div>
            <span class="status-pill" :data-tone="versionInfo?.update_available ? 'warning' : 'success'">
              {{ versionInfo?.update_available ? '有新版本' : '已是最新' }}
            </span>
          </div>
          <div class="mt-5 grid gap-3 text-sm">
            <div class="metric-card app-card p-4" data-tone="info">
              <div class="text-sm text-steel">当前版本</div>
              <div class="mt-2 text-lg font-black">v{{ versionInfo?.current_version || '读取中' }}</div>
            </div>
            <div class="metric-card app-card p-4" :data-tone="versionInfo?.update_available ? 'warning' : 'success'">
              <div class="text-sm text-steel">最新版本</div>
              <div class="mt-2 text-lg font-black">v{{ versionInfo?.latest_version || '读取中' }}</div>
              <a v-if="versionInfo?.latest_url" class="mt-2 inline-block text-sm text-neon hover:underline" :href="versionInfo.latest_url" target="_blank" rel="noreferrer">查看更新说明</a>
            </div>
            <GlassButton variant="secondary" :loading="versionLoading" @click="loadVersion">检查更新</GlassButton>
            <GlassButton variant="primary" :loading="updating" :disabled="!versionInfo?.update_enabled" @click="startUpdate">一键更新</GlassButton>
            <div class="rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm leading-6 text-steel">
              {{ versionInfo?.update_enabled ? '点击后会在后台重新拉取 GitHub 最新代码并重建 Docker 编排，更新过程会短暂重启服务。' : '当前编排未启用一键更新，请先使用 v1.0.2 之后的 docker-compose.yml 重建一次。' }}
            </div>
          </div>
        </GlassCard>

        <GlassCard class="h-full">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-steel">History</div>
              <h2 class="mt-2 text-xl font-black">最近策略变更</h2>
            </div>
            <span class="status-pill" :data-tone="history.length ? 'info' : 'neutral'">{{ history.length }} 条</span>
          </div>
          <div class="mt-4 flex flex-wrap gap-2">
            <button
              v-for="item in historyFilterOptions"
              :key="item.value"
              type="button"
              class="terminal-filter-chip"
              :class="{ 'terminal-filter-chip-active': historyFilter === item.value }"
              @click="historyFilter = item.value"
            >
              {{ item.label }}
            </button>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button
              v-for="item in historyRangeOptions"
              :key="item.value"
              type="button"
              class="terminal-filter-chip"
              :class="{ 'terminal-filter-chip-active': historyRange === item.value }"
              @click="historyRange = item.value"
            >
              {{ item.label }}
            </button>
          </div>
          <div class="mt-3">
            <label class="block rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
              <div class="text-xs uppercase tracking-[0.12em] text-steel">操作人筛选</div>
              <select v-model="historyActor" class="mt-3 min-h-11 w-full rounded-lg px-3 text-sm">
                <option value="all">全部操作人</option>
                <option v-for="item in historyActorOptions" :key="item" :value="item">{{ item }}</option>
              </select>
            </label>
          </div>
          <div v-if="filteredHistory.length" class="mt-5 space-y-3">
            <div
              v-for="item in filteredHistory"
              :key="item.id"
              class="rounded-2xl border border-white/10 bg-white/5 px-4 py-3"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="font-semibold">{{ item.summary }}</div>
                  <div class="mt-1 text-xs uppercase tracking-[0.14em] text-steel">{{ item.section }}</div>
                </div>
                <div class="text-right text-xs text-steel">
                  <div>{{ formatDate(item.created_at) }}</div>
                  <div class="mt-1">{{ item.changed_by || '系统' }}</div>
                </div>
              </div>
              <div
                v-if="historyDiffRows(item).length"
                class="mt-4 space-y-2 rounded-2xl border border-white/8 bg-black/10 px-3 py-3"
              >
                <div
                  v-for="row in historyDiffRows(item)"
                  :key="`${item.id}-${row.key}`"
                  class="grid gap-2 text-sm md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] md:items-center"
                >
                  <div>
                    <div class="text-xs uppercase tracking-[0.12em] text-steel">{{ row.label }}</div>
                    <div class="mt-1 text-steel line-through">{{ row.before }}</div>
                  </div>
                  <div class="text-center text-steel">→</div>
                  <div>
                    <div class="text-xs uppercase tracking-[0.12em] text-steel">变更后</div>
                    <div class="mt-1 font-semibold">{{ row.after }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="mt-5 rounded-2xl border border-dashed border-white/10 bg-white/5 px-4 py-6 text-sm text-steel">
            当前筛选下还没有记录到系统策略变更。
          </div>
        </GlassCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { api, type SystemSettings, type SystemSettingsHistoryItem, type SystemVersion } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUiStore } from '../stores/ui'

const ui = useUiStore()
const loading = ref(false)
const saving = ref(false)
const versionLoading = ref(false)
const updating = ref(false)
const updatedAt = ref('')
const snapshot = ref('')
const history = ref<SystemSettingsHistoryItem[]>([])
const versionInfo = ref<SystemVersion | null>(null)
const historyFilter = ref<'all' | 'risk_control' | 'listener_health'>('all')
const historyRange = ref<'all' | '24h' | '7d'>('all')
const historyActor = ref('all')
const form = reactive(createDefaultSettings())

const dirty = computed(() => JSON.stringify(serializeSettings(form)) !== snapshot.value)

const summaryItems = computed(() => [
  {
    label: '主动触达',
    value: form.adapter.outreach_dry_run ? 'dry-run 保护中' : '允许执行排队',
    help: form.security.require_admin_approval ? '创建后仍需管理员复核' : '创建后可直接进入任务队列',
    tone: form.adapter.outreach_dry_run ? 'warning' : 'success'
  },
  {
    label: '工作流',
    value: form.adapter.workflow_dry_run ? '默认 dry-run' : '允许执行排队',
    help: `最大任务并发 ${form.frequency.max_concurrent_tasks}`,
    tone: form.adapter.workflow_dry_run ? 'info' : 'success'
  },
  {
    label: '实时日志',
    value: form.audit.realtime_log_stream ? `${form.frequency.ws_log_batch_size} 条 / 批` : '已关闭实时流',
    help: `日志保留 ${form.audit.log_retention_days} 天`,
    tone: form.audit.realtime_log_stream ? 'success' : 'danger'
  },
  {
    label: '执行适配器',
    value: form.adapter.telegram_apply_enabled ? '已开启' : '未开启',
    help: form.adapter.telegram_sync_enabled ? '资料同步适配器已可用' : '资料同步适配器已关闭',
    tone: form.adapter.telegram_apply_enabled ? 'success' : 'warning'
  },
  {
    label: '监听健康',
    value: form.listener_health.auto_account_check_enabled ? `${form.listener_health.account_check_interval_minutes} 分钟检测` : '仅手动检测',
    help: `无消息 ${form.listener_health.silence_alert_minutes} 分钟后提示`,
    tone: form.listener_health.auto_account_check_enabled ? 'success' : 'warning'
  },
  {
    label: '风控避让',
    value: form.risk_control.auto_bypass_high_risk ? `高风险自动跳过 · ${riskPresetText.value}` : '仅记录不避让',
    help: `发信冷却 ${form.risk_control.message_cooldown_minutes}±${form.risk_control.message_jitter_minutes} 分钟，加群 ${form.risk_control.join_interval_minutes}±${form.risk_control.join_jitter_minutes} 分钟`,
    tone: form.risk_control.auto_bypass_high_risk ? 'warning' : 'info'
  }
])

const riskPresetOptions = [
  { label: '保守', value: 'conservative' as const },
  { label: '平衡', value: 'balanced' as const },
  { label: '激进', value: 'aggressive' as const },
  { label: '自定义', value: 'custom' as const }
]

const historyFilterOptions = [
  { label: '全部', value: 'all' as const },
  { label: '只看风控策略', value: 'risk_control' as const },
  { label: '只看监听健康', value: 'listener_health' as const }
]

const historyRangeOptions = [
  { label: '全部时间', value: 'all' as const },
  { label: '最近 24h', value: '24h' as const },
  { label: '最近 7 天', value: '7d' as const }
]

const historyFieldLabels: Record<string, string> = {
  auto_bypass_high_risk: '高风险自动避让',
  auto_bypass_active_restrictions: '生效中限制阈值',
  auto_bypass_failures_24h: '24h 命中阈值',
  message_cooldown_minutes: '发信冷却分钟',
  message_jitter_minutes: '发信随机浮动',
  join_daily_limit: '每日加群上限',
  join_interval_minutes: '加群间隔分钟',
  join_jitter_minutes: '加群随机浮动',
  auto_account_check_enabled: '自动检测监听账号',
  account_check_interval_minutes: '账号检测周期分钟',
  silence_alert_minutes: '无消息提醒分钟'
}

const filteredHistory = computed(() => {
  let items = history.value
  if (historyFilter.value !== 'all') {
    items = items.filter((item) => item.section === historyFilter.value)
  }
  if (historyRange.value === 'all') {
    return items
  }
  const now = Date.now()
  const rangeMs = historyRange.value === '24h' ? 24 * 60 * 60 * 1000 : 7 * 24 * 60 * 60 * 1000
  items = items.filter((item) => {
    const createdAt = new Date(item.created_at).getTime()
    return !Number.isNaN(createdAt) && now-createdAt <= rangeMs
  })
  if (historyActor.value === 'all') {
    return items
  }
  return items.filter((item) => (item.changed_by || '系统') === historyActor.value)
})

const historyActorOptions = computed(() => {
  return Array.from(
    new Set(
      history.value
        .map((item) => item.changed_by || '系统')
        .filter((item) => item.trim().length > 0)
    )
  ).sort((a, b) => a.localeCompare(b, 'zh-CN'))
})

const riskPresetText = computed(() => {
  switch (activeRiskPreset.value) {
    case 'conservative':
      return '保守'
    case 'balanced':
      return '平衡'
    case 'aggressive':
      return '激进'
    default:
      return '自定义'
  }
})

const riskPresetHelpText = computed(() => {
  switch (activeRiskPreset.value) {
    case 'conservative':
      return '更早避让高风险账号，适合风控压力大或账号池偏紧张的阶段。'
    case 'balanced':
      return '在安全和利用率之间取平衡，适合作为日常默认策略。'
    case 'aggressive':
      return '尽量让更多账号继续参与调度，适合短期冲量但需要更密切观察。'
    default:
      return '你已经手动改过阈值，这套策略当前是自定义值。'
  }
})

const activeRiskPreset = computed(() => {
  if (!form.risk_control.auto_bypass_high_risk) {
    return 'custom'
  }
  if (
    form.risk_control.auto_bypass_active_restrictions === 2 &&
    form.risk_control.auto_bypass_failures_24h === 6 &&
    form.risk_control.message_cooldown_minutes === 90 &&
    form.risk_control.message_jitter_minutes === 10 &&
    form.risk_control.join_daily_limit === 12 &&
    form.risk_control.join_interval_minutes === 180 &&
    form.risk_control.join_jitter_minutes === 30
  ) {
    return 'conservative'
  }
  if (
    form.risk_control.auto_bypass_active_restrictions === 3 &&
    form.risk_control.auto_bypass_failures_24h === 10 &&
    form.risk_control.message_cooldown_minutes === 60 &&
    form.risk_control.message_jitter_minutes === 10 &&
    form.risk_control.join_daily_limit === 20 &&
    form.risk_control.join_interval_minutes === 120 &&
    form.risk_control.join_jitter_minutes === 30
  ) {
    return 'balanced'
  }
  if (
    form.risk_control.auto_bypass_active_restrictions === 5 &&
    form.risk_control.auto_bypass_failures_24h === 16 &&
    form.risk_control.message_cooldown_minutes === 30 &&
    form.risk_control.message_jitter_minutes === 10 &&
    form.risk_control.join_daily_limit === 35 &&
    form.risk_control.join_interval_minutes === 60 &&
    form.risk_control.join_jitter_minutes === 30
  ) {
    return 'aggressive'
  }
  return 'custom'
})

function createDefaultSettings(): Omit<SystemSettings, 'updated_at'> {
  return {
    security: {
      enforce_tenant_isolation: true,
      require_admin_approval: true,
      mask_sensitive_logs: true
    },
    frequency: {
      max_concurrent_tasks: 12,
      max_concurrent_outreach: 4,
      ws_log_batch_size: 80,
      dashboard_refresh_second: 30
    },
    listener_health: {
      auto_account_check_enabled: true,
      account_check_interval_minutes: 60,
      silence_alert_minutes: 15
    },
    audit: {
      log_retention_days: 30,
      realtime_log_stream: true,
      notify_on_failure: true
    },
    adapter: {
      telegram_sync_enabled: true,
      telegram_apply_enabled: false,
      outreach_dry_run: true,
      workflow_dry_run: true
    },
    risk_control: {
      auto_bypass_high_risk: true,
      auto_bypass_active_restrictions: 3,
      auto_bypass_failures_24h: 10,
      message_cooldown_minutes: 60,
      message_jitter_minutes: 10,
      join_daily_limit: 20,
      join_interval_minutes: 120,
      join_jitter_minutes: 30
    }
  }
}

function serializeSettings(source: Omit<SystemSettings, 'updated_at'>) {
  return {
    security: { ...source.security },
    frequency: { ...source.frequency },
    listener_health: { ...source.listener_health },
    audit: { ...source.audit },
    adapter: { ...source.adapter },
    risk_control: { ...source.risk_control }
  }
}

function applySettings(settings: SystemSettings) {
  form.security.enforce_tenant_isolation = settings.security.enforce_tenant_isolation
  form.security.require_admin_approval = settings.security.require_admin_approval
  form.security.mask_sensitive_logs = settings.security.mask_sensitive_logs
  form.frequency.max_concurrent_tasks = settings.frequency.max_concurrent_tasks
  form.frequency.max_concurrent_outreach = settings.frequency.max_concurrent_outreach
  form.frequency.ws_log_batch_size = settings.frequency.ws_log_batch_size
  form.frequency.dashboard_refresh_second = settings.frequency.dashboard_refresh_second
  form.listener_health.auto_account_check_enabled = settings.listener_health?.auto_account_check_enabled ?? true
  form.listener_health.account_check_interval_minutes = settings.listener_health?.account_check_interval_minutes ?? 60
  form.listener_health.silence_alert_minutes = settings.listener_health?.silence_alert_minutes ?? 15
  form.audit.log_retention_days = settings.audit.log_retention_days
  form.audit.realtime_log_stream = settings.audit.realtime_log_stream
  form.audit.notify_on_failure = settings.audit.notify_on_failure
  form.adapter.telegram_sync_enabled = settings.adapter.telegram_sync_enabled
  form.adapter.telegram_apply_enabled = settings.adapter.telegram_apply_enabled
  form.adapter.outreach_dry_run = settings.adapter.outreach_dry_run
  form.adapter.workflow_dry_run = settings.adapter.workflow_dry_run
  form.risk_control.auto_bypass_high_risk = settings.risk_control.auto_bypass_high_risk
  form.risk_control.auto_bypass_active_restrictions = settings.risk_control.auto_bypass_active_restrictions
  form.risk_control.auto_bypass_failures_24h = settings.risk_control.auto_bypass_failures_24h
  form.risk_control.message_cooldown_minutes = settings.risk_control.message_cooldown_minutes
  form.risk_control.message_jitter_minutes = settings.risk_control.message_jitter_minutes
  form.risk_control.join_daily_limit = settings.risk_control.join_daily_limit
  form.risk_control.join_interval_minutes = settings.risk_control.join_interval_minutes
  form.risk_control.join_jitter_minutes = settings.risk_control.join_jitter_minutes
}

function applyRiskPreset(preset: 'conservative' | 'balanced' | 'aggressive' | 'custom') {
  if (preset === 'custom') {
    return
  }
  form.risk_control.auto_bypass_high_risk = true
  if (preset === 'conservative') {
    form.risk_control.auto_bypass_active_restrictions = 2
    form.risk_control.auto_bypass_failures_24h = 6
    form.risk_control.message_cooldown_minutes = 90
    form.risk_control.message_jitter_minutes = 10
    form.risk_control.join_daily_limit = 12
    form.risk_control.join_interval_minutes = 180
    form.risk_control.join_jitter_minutes = 30
    return
  }
  if (preset === 'balanced') {
    form.risk_control.auto_bypass_active_restrictions = 3
    form.risk_control.auto_bypass_failures_24h = 10
    form.risk_control.message_cooldown_minutes = 60
    form.risk_control.message_jitter_minutes = 10
    form.risk_control.join_daily_limit = 20
    form.risk_control.join_interval_minutes = 120
    form.risk_control.join_jitter_minutes = 30
    return
  }
  form.risk_control.auto_bypass_active_restrictions = 5
  form.risk_control.auto_bypass_failures_24h = 16
  form.risk_control.message_cooldown_minutes = 30
  form.risk_control.message_jitter_minutes = 10
  form.risk_control.join_daily_limit = 35
  form.risk_control.join_interval_minutes = 60
  form.risk_control.join_jitter_minutes = 30
}

function historyDiffRows(item: SystemSettingsHistoryItem) {
  const before = item.before || {}
  const after = item.after || {}
  const keys = Array.from(new Set([...Object.keys(before), ...Object.keys(after)]))
  return keys
    .filter((key) => !isEqualHistoryValue(before[key], after[key]))
    .map((key) => ({
      key,
      label: historyFieldLabels[key] || key,
      before: formatHistoryValue(before[key]),
      after: formatHistoryValue(after[key])
    }))
}

function isEqualHistoryValue(left: unknown, right: unknown) {
  return JSON.stringify(left ?? null) === JSON.stringify(right ?? null)
}

function formatHistoryValue(value: unknown) {
  if (typeof value === 'boolean') {
    return value ? '开启' : '关闭'
  }
  if (typeof value === 'number') {
    return `${value}`
  }
  if (typeof value === 'string') {
    return value || '-'
  }
  if (value === null || value === undefined) {
    return '-'
  }
  return JSON.stringify(value)
}

async function load() {
  loading.value = true
  try {
    const [settings, settingsHistory, version] = await Promise.all([
      api.systemSettings(),
      api.systemSettingsHistory(),
      api.systemVersion()
    ])
    applySettings(settings)
    history.value = settingsHistory
    versionInfo.value = version
    updatedAt.value = settings.updated_at || ''
    snapshot.value = JSON.stringify(serializeSettings(form))
  } catch (err) {
    ui.toast({
      title: '系统设置读取失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    loading.value = false
  }
}

async function loadVersion() {
  versionLoading.value = true
  try {
    versionInfo.value = await api.systemVersion()
    ui.toast({
      title: versionInfo.value.update_available ? '发现新版本' : '当前已是最新版本',
      message: `当前 v${versionInfo.value.current_version}，最新 v${versionInfo.value.latest_version}`,
      tone: versionInfo.value.update_available ? 'warning' : 'success'
    })
  } catch (err) {
    ui.toast({
      title: '版本检查失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    versionLoading.value = false
  }
}

async function startUpdate() {
  updating.value = true
  try {
    const result = await api.startSystemUpdate()
    ui.toast({
      title: '更新已启动',
      message: result.message || '后台正在重建服务，请稍后刷新页面。',
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '启动更新失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    updating.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const saved = await api.updateSystemSettings(serializeSettings(form))
    applySettings(saved)
    history.value = await api.systemSettingsHistory()
    updatedAt.value = saved.updated_at || ''
    snapshot.value = JSON.stringify(serializeSettings(form))
    ui.toast({
      title: '系统设置已保存',
      message: '新的频控与执行策略已经写入后台',
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '系统设置保存失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    saving.value = false
  }
}

function formatDate(value: string) {
  if (!value) return '刚刚初始化'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

onMounted(load)
</script>
