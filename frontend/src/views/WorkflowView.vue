<template>
  <div class="page-shell workflow-console">
    <div class="page-header workflow-hero">
      <div>
        <div class="workflow-kicker">Notification Workflow</div>
        <h1 class="page-title">通知工作流</h1>
        <p class="page-subtitle">选择终端组和目标组，编排多阶段消息漏斗，提交后进入任务中心追踪执行进度。</p>
      </div>
      <div class="page-actions">
        <span class="status-pill" data-tone="info">阶段 {{ form.steps.length }}/10</span>
        <span class="status-pill" :data-tone="realExecutionEnabled ? 'success' : 'warning'">
          {{ realExecutionEnabled ? '真实发送' : '演练模式' }}
        </span>
        <span class="status-pill" :data-tone="canSubmit ? 'success' : 'warning'">{{ canSubmit ? '可投递' : '待配置' }}</span>
        <GlassButton variant="secondary" :loading="loading" @click="loadResources">刷新资源</GlassButton>
      </div>
    </div>

    <div class="workflow-layout">
      <main class="workflow-main">
        <div class="workflow-source-grid">
          <GlassCard class="workflow-panel workflow-source-card">
            <div class="workflow-panel-head">
              <div>
                <div class="workflow-section-label">01 Sources</div>
                <h2>发送终端</h2>
              </div>
              <span class="status-pill" data-tone="info">{{ terminalSelectionLabel }}</span>
            </div>
            <div class="workflow-picker-list">
              <label class="workflow-pick-item" :class="{ 'is-active': allTerminalGroupsSelected }">
                <input type="checkbox" :checked="allTerminalGroupsSelected" @change="selectAllTerminalGroups" />
                <span>
                  <strong>全部终端组</strong>
                  <small>不限制发送账号来源</small>
                </span>
              </label>
              <label v-for="group in terminalGroups" :key="group.id" class="workflow-pick-item" :class="{ 'is-active': isTerminalGroupSelected(group.id) }">
                <input type="checkbox" :checked="isTerminalGroupSelected(group.id)" @change="toggleTerminalGroup(group.id, $event)" />
                <span>
                  <strong>{{ group.name }}</strong>
                  <small>{{ group.description || '终端分组' }}</small>
                </span>
              </label>
              <div v-if="!terminalGroups.length" class="workflow-empty">暂无终端组，默认使用全部终端。</div>
            </div>
          </GlassCard>

          <GlassCard class="workflow-panel workflow-source-card">
            <div class="workflow-panel-head">
              <div>
                <div class="workflow-section-label">02 Audience</div>
                <h2>目标受众</h2>
              </div>
              <span class="status-pill" data-tone="info">{{ targetSelectionLabel }}</span>
            </div>
            <div class="workflow-picker-list">
              <label class="workflow-pick-item" :class="{ 'is-active': allTargetGroupsSelected }">
                <input type="checkbox" :checked="allTargetGroupsSelected" @change="selectAllTargetGroups" />
                <span>
                  <strong>全部目标组</strong>
                  <small>使用目标池中的全部目标</small>
                </span>
              </label>
              <label v-for="group in targetGroups" :key="group.id" class="workflow-pick-item" :class="{ 'is-active': isTargetGroupSelected(group.id) }">
                <input type="checkbox" :checked="isTargetGroupSelected(group.id)" @change="toggleTargetGroup(group.id, $event)" />
                <span>
                  <strong>{{ group.name }}</strong>
                  <small>{{ group.description || '目标分组' }}</small>
                </span>
              </label>
              <div v-if="!targetGroups.length" class="workflow-empty">暂无目标分组，默认使用全部目标。</div>
            </div>
          </GlassCard>
        </div>

        <section class="workflow-builder">
        <GlassCard class="workflow-builder-card">
          <div class="workflow-builder-head">
            <div>
              <div class="workflow-section-label">03 Funnel</div>
              <h2>消息阶段编排</h2>
              <p>每个阶段可以独立设置消息类型、内容和前置等待时间。</p>
            </div>
            <div class="workflow-builder-actions">
              <GlassButton variant="secondary" :disabled="form.steps.length >= 10" @click="addStep('text')">添加文本</GlassButton>
              <GlassButton variant="secondary" :disabled="form.steps.length >= 10" @click="addStep('voice')">添加语音</GlassButton>
              <GlassButton variant="secondary" :disabled="form.steps.length >= 10" @click="addStep('gif')">添加 GIF</GlassButton>
              <GlassButton variant="primary" :disabled="form.steps.length >= 10" @click="addStep('image')">添加图片</GlassButton>
            </div>
          </div>

          <div class="workflow-step-strip">
            <button
              v-for="(step, index) in form.steps"
              :key="step._uid"
              class="workflow-step-chip"
              :class="{ 'is-active': selectedStepIndex === index }"
              type="button"
              @click="selectedStepIndex = index"
            >
              <span>{{ index + 1 }}</span>
              {{ stepTypeLabel(step.type) }}
            </button>
          </div>

          <div class="workflow-step-list">
            <article v-for="(step, index) in form.steps" :key="step._uid" class="workflow-step-card" :class="{ 'is-selected': selectedStepIndex === index }">
              <div class="workflow-step-index">
                <span>{{ String(index + 1).padStart(2, '0') }}</span>
                <small>{{ stepTypeLabel(step.type) }}</small>
              </div>

              <div class="workflow-step-body">
                <div class="workflow-step-controls">
                  <label>
                    <span>消息类型</span>
                    <select v-model="step.type">
                      <option value="text">发送文本</option>
                      <option value="image">推送图片</option>
                      <option value="voice">发送语音</option>
                      <option value="gif">发送 GIF</option>
                      <option value="forward">转发消息</option>
                    </select>
                  </label>
                  <label>
                    <span>前置等待</span>
                    <input v-model.number="step.delay_seconds" min="0" type="number" />
                  </label>
                  <button class="workflow-icon-btn" type="button" title="上移" :disabled="index === 0" @click="moveStep(index, -1)">↑</button>
                  <button class="workflow-icon-btn" type="button" title="下移" :disabled="index === form.steps.length - 1" @click="moveStep(index, 1)">↓</button>
                  <button class="workflow-icon-btn is-danger" type="button" title="删除" :disabled="form.steps.length <= 1" @click="removeStep(index)">删除</button>
                </div>

                <textarea
                  v-if="step.type === 'text'"
                  v-model="step.content"
                  class="workflow-message-input"
                  placeholder="输入消息内容，支持 {您好|你好|Hi} 这种随机文案。"
                ></textarea>

                <div v-else-if="step.type === 'forward'" class="workflow-forward-grid">
                  <label>
                    <span>来源 Chat ID</span>
                    <input v-model="step.source_chat_id" placeholder="-100123456789" />
                  </label>
                  <label>
                    <span>Message ID</span>
                    <input v-model="step.message_id" placeholder="9481" />
                  </label>
                </div>

                <div
                  v-else
                  class="workflow-media-box"
                  :class="{ 'is-uploading': step.uploading }"
                  @dragover.prevent
                  @drop.prevent="handleMediaDrop(index, $event)"
                >
                  <div>
                    <strong>{{ stepTypeLabel(step.type) }}素材</strong>
                    <p>拖拽文件到这里，或选择本地文件上传；上传成功后会自动绑定素材 ID。</p>
                  </div>
                  <div class="workflow-media-actions">
                    <input
                      :id="`workflow-media-${step._uid}`"
                      class="hidden"
                      type="file"
                      :accept="mediaAccept(step.type)"
                      @change="handleMediaSelect(index, $event)"
                    />
                    <label class="workflow-upload-btn" :for="`workflow-media-${step._uid}`">
                      {{ step.uploading ? '上传中...' : '选择文件' }}
                    </label>
                    <input v-model="step.media_asset_id" placeholder="上传后自动填入素材 ID" />
                  </div>
                  <div v-if="step.media_name || step.media_url" class="workflow-media-meta">
                    <span>{{ step.media_name || '已绑定媒体' }}</span>
                    <a v-if="step.media_url" :href="step.media_url" target="_blank" rel="noreferrer">查看</a>
                  </div>
                </div>
              </div>
            </article>
          </div>
        </GlassCard>
        </section>

        <GlassCard class="workflow-control-card">
          <div class="workflow-panel-head">
            <div>
              <div class="workflow-section-label">04 Delivery</div>
              <h2>投递控制台</h2>
            </div>
            <span class="status-pill" :data-tone="realExecutionEnabled ? 'success' : 'warning'">
              {{ realExecutionEnabled ? '真实投递' : '演练任务' }}
            </span>
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-2 text-sm text-steel">
            <span class="status-pill" :data-tone="riskPolicyTone">{{ riskPolicyPresetText }}</span>
            <span>{{ riskPolicyHelpText }}</span>
          </div>

          <div class="workflow-control-layout">
            <div class="workflow-preview-controls">
              <div class="workflow-cadence">
                <label>
                  <span>发送次数</span>
                  <input v-model.number="form.send_count" min="1" max="100" step="1" type="number" placeholder="例如 3" @blur="normalizeCadence" />
                  <small>最多 100 次</small>
                </label>
                <label>
                  <span>发送时间间隔</span>
                  <div class="workflow-interval-control">
                    <input v-model.number="form.send_interval_value" min="0" step="1" type="number" placeholder="例如 1" @blur="normalizeCadence" />
                    <select v-model="form.send_interval_unit" @change="normalizeCadence">
                      <option value="seconds">秒</option>
                      <option value="minutes">分钟</option>
                      <option value="hours">小时</option>
                    </select>
                  </div>
                  <small>0 表示不等待，最大 24 小时</small>
                </label>
              </div>

              <div class="workflow-summary">
                <div>
                  <span>终端范围</span>
                  <strong>{{ form.terminal_group_ids.length ? `${form.terminal_group_ids.length} 个分组` : '全部终端组' }}</strong>
                </div>
                <div>
                  <span>目标范围</span>
                  <strong>{{ form.target_group_ids.length ? `${form.target_group_ids.length} 个分组` : '全部目标组' }}</strong>
                </div>
                <div>
                  <span>阶段数量</span>
                  <strong>{{ form.steps.length }} 个阶段</strong>
                </div>
                <div>
                  <span>发送次数</span>
                  <strong>{{ form.send_count }} 次</strong>
                </div>
                <div>
                  <span>发送间隔</span>
                  <strong>{{ intervalLabel(normalizedSendInterval) }}</strong>
                </div>
                <div>
                  <span>预计投递</span>
                  <strong>{{ plannedDeliveryText }}</strong>
                </div>
              </div>
            </div>

            <div class="workflow-submit-panel">
              <div class="workflow-submit-copy">
                <strong>提交当前通知工作流</strong>
                <span>实时写入任务中心，后续可继续查看全部任务与失败原因。</span>
              </div>
              <GlassButton variant="primary" class="w-full" :loading="submitting" :disabled="!canSubmit" @click="submitJob">
                {{ realExecutionEnabled ? '真实投递通知工作流' : '创建演练任务' }}
              </GlassButton>
            </div>
          </div>
        </GlassCard>
      </main>

      <aside class="workflow-preview">
        <GlassCard class="workflow-preview-card">
          <div class="workflow-panel-head">
            <div>
              <div class="workflow-section-label">Live Preview</div>
              <h2>动态发送仿真</h2>
            </div>
            <span class="status-pill" :data-tone="isPlaying ? 'success' : 'info'">{{ playbackStatusText }}</span>
          </div>

          <div class="workflow-preview-device">
            <div class="workflow-iphone-frame">
              <div class="workflow-iphone-side is-mute"></div>
              <div class="workflow-iphone-side is-volume-up"></div>
              <div class="workflow-iphone-side is-volume-down"></div>
              <div class="workflow-iphone-side is-action"></div>
              <div class="workflow-dynamic-island">
                <span></span>
                <i></i>
              </div>
              <div class="workflow-phone">
                <div class="tg-statusbar">
                  <span>{{ currentTime }}</span>
                  <div class="tg-status-icons">
                    <i></i>
                    <b></b>
                    <strong></strong>
                  </div>
                </div>
                <div class="tg-header">
                  <button type="button">‹</button>
                  <div class="tg-mini-badge">TG</div>
                  <div class="tg-title">
                    <strong>TG 全球科技交流群</strong>
                    <span>{{ activeMembersText }}</span>
                  </div>
                  <button class="tg-send-button" type="button">➤</button>
                </div>
                <div class="workflow-chat">
                  <div class="workflow-day">Today</div>
                  <transition-group name="workflow-send">
                    <div v-for="(step, index) in visiblePreviewSteps" :key="step._uid" class="workflow-bubble-wrap">
                      <div v-if="step.delay_seconds > 0 && index === activePreviewIndex && sendPhase === 'waiting'" class="workflow-delay">
                        等待 {{ step.delay_seconds }} 秒后发送
                      </div>
                      <div class="tg-message-row is-outgoing">
                        <div class="tg-member-avatar" data-tone="sender">我</div>
                        <div class="workflow-bubble" :data-type="step.type" :data-phase="messagePhase(index)">
                          <div class="tg-sender-line">
                            <strong>当前终端</strong>
                            <span>@active_sender</span>
                          </div>
                          <template v-if="step.type === 'text'">
                            <div class="tg-message-text">{{ resolveSpintax(step.content) || '空文本消息' }}</div>
                          </template>
                          <template v-else-if="step.type === 'forward'">
                            <div class="tg-forward-card">
                              <strong>Forwarded Message</strong>
                              <span>{{ step.source_chat_id || '未配置来源' }}</span>
                              <small>Message {{ step.message_id || '未配置' }}</small>
                            </div>
                          </template>
                          <template v-else>
                            <img v-if="(step.type === 'image' || step.type === 'gif') && step.media_url" class="workflow-preview-media" :src="step.media_url" :alt="step.media_name || stepTypeLabel(step.type)" />
                            <div v-else-if="step.type === 'image' || step.type === 'gif'" class="tg-media-placeholder" :data-type="step.type">
                              <span>{{ step.type === 'gif' ? 'GIF' : 'IMG' }}</span>
                            </div>
                            <div v-else-if="step.type === 'voice'" class="workflow-voice-preview">
                              <span>▶</span>
                              <div><i></i></div>
                              <b>{{ step.media_name || '语音消息' }}</b>
                            </div>
                            <div class="tg-message-text">{{ step.media_name || step.media_asset_id || `已发送${stepTypeLabel(step.type)}素材` }}</div>
                          </template>
                          <small>{{ simulatedMessageTime(index) }} <span>{{ messagePhase(index) === 'sending' ? '✓' : '✓✓' }}</span></small>
                        </div>
                      </div>
                    </div>
                  </transition-group>
                  <div v-if="isPlaying && sendPhase === 'sending'" class="tg-typing">
                    <span></span>
                    <span></span>
                    <span></span>
                  </div>
                </div>
                <div class="tg-composer">
                  <button type="button">＋</button>
                  <div>Message</div>
                  <button type="button">⌕</button>
                </div>
              </div>
            </div>
          </div>
          <div class="workflow-playback">
            <div class="workflow-playback-bar">
              <i :style="{ width: `${playbackProgress}%` }"></i>
            </div>
            <div class="workflow-playback-actions">
              <GlassButton variant="secondary" @click="resetSimulation">重置</GlassButton>
              <GlassButton variant="primary" @click="toggleSimulation">{{ isPlaying ? '暂停演示' : '播放流程' }}</GlassButton>
            </div>
          </div>
        </GlassCard>
      </aside>
    </div>

    <GlassCard class="workflow-task-card">
      <div class="workflow-panel-head">
        <div>
          <div class="workflow-section-label">Task Status</div>
          <h2>任务状态</h2>
        </div>
        <div class="workflow-task-toolbar">
          <GlassButton variant="secondary" size="sm" :disabled="!completedTasks.length || deletingTaskScope === 'completed'" :loading="deletingTaskScope === 'completed'" @click="deleteTasksByStatus('completed')">
            一键删除已完成任务
          </GlassButton>
          <GlassButton variant="secondary" size="sm" :disabled="!failedTasks.length || deletingTaskScope === 'failed'" :loading="deletingTaskScope === 'failed'" @click="deleteTasksByStatus('failed')">
            一键删除执行失败任务
          </GlassButton>
        </div>
      </div>

      <div v-if="workflowTasks.length" class="workflow-task-table">
        <div class="workflow-task-table-head">
          <span>任务</span>
          <span>目标</span>
          <span>总数</span>
          <span>执行</span>
          <span>进度</span>
          <span>状态</span>
          <span>操作</span>
        </div>

        <div class="workflow-task-list">
          <article v-for="task in workflowTasks" :key="task.id" class="workflow-task-row" :class="{ 'is-tracked': workflowTask?.id === task.id }">
            <div class="workflow-task-cell is-name">
              <strong>{{ taskDisplayName(task) }}</strong>
              <small>{{ formatTaskTime(task.updated_at || task.created_at) }} · {{ task.id.slice(0, 8) }}</small>
            </div>

            <div class="workflow-task-cell">
              <strong>{{ taskTargetText(task) }}</strong>
              <small>发送策略</small>
            </div>

            <div class="workflow-task-cell">
              <strong>{{ taskTotalCount(task) }}</strong>
              <small>目标量</small>
            </div>

            <div class="workflow-task-cell">
              <strong>{{ taskExecutedCount(task) }}</strong>
              <small>成 {{ taskSuccessCount(task) }} / 败 {{ taskFailedCount(task) }}</small>
            </div>

            <div class="workflow-task-cell is-progress">
              <div class="workflow-task-inline-progress">
                <div class="progress-track">
                  <div class="progress-fill" :style="{ width: `${taskProgressPercent(task)}%` }"></div>
                </div>
                <small>{{ `${taskProgressPercent(task)}%` }}</small>
              </div>
            </div>

            <div class="workflow-task-cell is-status">
              <span
                class="status-pill"
                :data-tone="taskStatusTone(task.status)"
                :title="taskFailureTooltip(task)"
                @mouseenter="prefetchTaskFailure(task)"
              >
                {{ taskStatusText(task.status) }}
              </span>
            </div>

            <div class="workflow-task-cell is-actions">
              <GlassButton variant="ghost" size="sm" :disabled="!!deletingTaskIDs[task.id]" :loading="!!deletingTaskIDs[task.id]" @click="deleteTask(task.id)">
                删除
              </GlassButton>
            </div>
          </article>
        </div>
      </div>
      <div v-else class="workflow-task-empty">
        还没有通知工作流任务，投递后这里会实时展示名称、目标和执行进度。
      </div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { api, type Group, type MassMessageStep, type SystemSettings, type Task } from '../api/client'
import { useUiStore } from '../stores/ui'
import { taskDisplayName } from '../utils/taskDisplay'

type StepType = 'text' | 'image' | 'voice' | 'gif' | 'forward'
type IntervalUnit = 'seconds' | 'minutes' | 'hours'
type RichStep = MassMessageStep & {
  _uid: string
  type: StepType
  media_url?: string
  media_name?: string
  uploading?: boolean
}

const ui = useUiStore()
const loading = ref(false)
const submitting = ref(false)
const terminalGroups = ref<Group[]>([])
const targetGroups = ref<Group[]>([])
const settings = ref<SystemSettings | null>(null)
const workflowTasks = ref<Task[]>([])
const workflowTaskID = ref('')
const deletingTaskScope = ref<'completed' | 'failed' | ''>('')
const selectedStepIndex = ref(0)
const currentTime = ref('00:00')
const previewTick = ref(0)
const isPlaying = ref(false)
const activePreviewIndex = ref(0)
const visibleStepCount = ref(1)
const sendPhase = ref<'idle' | 'waiting' | 'sending' | 'sent'>('idle')
const taskFailureReasons = reactive<Record<string, string>>({})
const deletingTaskIDs = reactive<Record<string, boolean>>({})
let clockTimer: number | null = null
let previewTimer: number | null = null
let playbackTimer: number | null = null
let workflowTaskTimer: number | null = null

const form = reactive({
  terminal_group_ids: [] as string[],
  target_group_ids: [] as string[],
  send_count: 1,
  send_interval_value: 0,
  send_interval_unit: 'seconds' as IntervalUnit,
  steps: [
    createStep('text', '您好，这是一条通知工作流消息：\n{欢迎|你好|Hi}，请查看今天的最新安排。', 0)
  ] as RichStep[]
})

const previewSteps = computed(() => {
  previewTick.value
  return form.steps
})

const visiblePreviewSteps = computed(() => previewSteps.value.slice(0, Math.max(1, visibleStepCount.value)))
const playbackProgress = computed(() => {
  if (!form.steps.length) return 0
  return Math.min(100, Math.round((Math.max(0, activePreviewIndex.value) / form.steps.length) * 100))
})
const playbackStatusText = computed(() => {
  if (!isPlaying.value) return '等待播放'
  if (sendPhase.value === 'waiting') return '等待发送'
  if (sendPhase.value === 'sending') return '正在发送'
  return '已送达'
})
const activeMembersText = computed(() => {
  const targetCount = form.target_group_ids.length || targetGroups.value.length || 1
  return `${targetCount * 128} members, ${Math.max(1, targetCount * 7)} online`
})
const plannedDeliveryText = computed(() => {
  const targetCount = form.target_group_ids.length || targetGroups.value.length || 1
  return `${targetCount * form.steps.length * normalizedSendCount.value} 条`
})
const allTerminalGroupsSelected = computed(() => !form.terminal_group_ids.length || form.terminal_group_ids.length === terminalGroups.value.length)
const allTargetGroupsSelected = computed(() => !form.target_group_ids.length || form.target_group_ids.length === targetGroups.value.length)
const terminalSelectionLabel = computed(() => (allTerminalGroupsSelected.value ? '全部' : `${form.terminal_group_ids.length}`))
const targetSelectionLabel = computed(() => (allTargetGroupsSelected.value ? '全部' : `${form.target_group_ids.length}`))
const normalizedSendCount = computed(() => clampInteger(form.send_count, 1, 100, 1))
const normalizedSendInterval = computed(() => {
  const value = clampInteger(form.send_interval_value, 0, maxIntervalValue(form.send_interval_unit), 0)
  return Math.min(86400, value * intervalUnitMultiplier(form.send_interval_unit))
})
const canSubmit = computed(() => form.steps.length > 0 && !submitting.value)
const realExecutionEnabled = computed(() => {
  const adapter = settings.value?.adapter
  return !!adapter?.telegram_apply_enabled && !adapter?.workflow_dry_run
})
const riskPolicyPresetText = computed(() => {
  const risk = settings.value?.risk_control
  if (!risk?.auto_bypass_high_risk) return '风控避让关闭'
  if (risk.auto_bypass_active_restrictions === 2 && risk.auto_bypass_failures_24h === 6) return '保守模式'
  if (risk.auto_bypass_active_restrictions === 3 && risk.auto_bypass_failures_24h === 10) return '平衡模式'
  if (risk.auto_bypass_active_restrictions === 5 && risk.auto_bypass_failures_24h === 16) return '激进模式'
  return '自定义风控'
})
const riskPolicyTone = computed(() => {
  if (!settings.value?.risk_control.auto_bypass_high_risk) return 'info'
  if (riskPolicyPresetText.value.includes('保守')) return 'warning'
  if (riskPolicyPresetText.value.includes('激进')) return 'success'
  return 'cyan'
})
const riskPolicyHelpText = computed(() => {
  const risk = settings.value?.risk_control
  if (!risk) return '风控策略读取中'
  if (!risk.auto_bypass_high_risk) return '当前只记录风险，不会在选号时自动避让高风险账号。'
  return `达到生效中限制 ${risk.auto_bypass_active_restrictions} 条或 24h 命中 ${risk.auto_bypass_failures_24h} 次后自动避让。`
})
const workflowTask = computed(() => workflowTasks.value.find((task) => task.id === workflowTaskID.value) || workflowTasks.value[0] || null)
const completedTasks = computed(() => workflowTasks.value.filter((task) => /^(success|dry_run|partial_success)$/i.test(task.status)))
const failedTasks = computed(() => workflowTasks.value.filter((task) => /failed/i.test(task.status)))

function createStep(type: StepType = 'text', content = '', delay = 0): RichStep {
  return {
    _uid: Math.random().toString(36).slice(2, 10),
    type,
    content,
    media_asset_id: '',
    source_chat_id: type === 'forward' ? '-100123456789' : '',
    message_id: type === 'forward' ? '900' : '',
    delay_seconds: delay
  }
}

function isTerminalGroupSelected(groupID: string) {
  return allTerminalGroupsSelected.value || form.terminal_group_ids.includes(groupID)
}

function isTargetGroupSelected(groupID: string) {
  return allTargetGroupsSelected.value || form.target_group_ids.includes(groupID)
}

function selectAllTerminalGroups() {
  form.terminal_group_ids = []
}

function selectAllTargetGroups() {
  form.target_group_ids = []
}

function toggleTerminalGroup(groupID: string, event: Event) {
  const input = event.target as HTMLInputElement
  const checked = input.checked
  let next = allTerminalGroupsSelected.value ? terminalGroups.value.map((group) => group.id) : [...form.terminal_group_ids]
  if (checked) {
    if (!next.includes(groupID)) next.push(groupID)
  } else {
    next = next.filter((id) => id !== groupID)
  }
  form.terminal_group_ids = next.length === terminalGroups.value.length ? [] : next
}

function toggleTargetGroup(groupID: string, event: Event) {
  const input = event.target as HTMLInputElement
  const checked = input.checked
  let next = allTargetGroupsSelected.value ? targetGroups.value.map((group) => group.id) : [...form.target_group_ids]
  if (checked) {
    if (!next.includes(groupID)) next.push(groupID)
  } else {
    next = next.filter((id) => id !== groupID)
  }
  form.target_group_ids = next.length === targetGroups.value.length ? [] : next
}

function addStep(type: StepType) {
  if (form.steps.length >= 10) return
  form.steps.push(createStep(type, type === 'text' ? '这是一条新消息...' : '', 3))
  selectedStepIndex.value = form.steps.length - 1
  if (!isPlaying.value) {
    visibleStepCount.value = Math.max(1, form.steps.length)
  }
}

function removeStep(index: number) {
  if (form.steps.length <= 1) return
  form.steps.splice(index, 1)
  selectedStepIndex.value = Math.max(0, Math.min(selectedStepIndex.value, form.steps.length - 1))
}

function moveStep(index: number, offset: number) {
  const next = index + offset
  if (next < 0 || next >= form.steps.length) return
  const [item] = form.steps.splice(index, 1)
  form.steps.splice(next, 0, item)
  selectedStepIndex.value = next
}

function messagePhase(index: number) {
  if (index < activePreviewIndex.value) return 'sent'
  if (index === activePreviewIndex.value) return sendPhase.value
  return 'queued'
}

function resetSimulation() {
  stopPlaybackTimer()
  isPlaying.value = false
  activePreviewIndex.value = 0
  visibleStepCount.value = Math.min(1, form.steps.length)
  sendPhase.value = 'idle'
}

function toggleSimulation() {
  if (isPlaying.value) {
    stopPlaybackTimer()
    isPlaying.value = false
    return
  }
  startSimulation()
}

function startSimulation() {
  if (!form.steps.length) return
  stopPlaybackTimer()
  isPlaying.value = true
  activePreviewIndex.value = 0
  visibleStepCount.value = 1
  runPlaybackStage()
}

function runPlaybackStage() {
  if (!isPlaying.value) return
  const step = form.steps[activePreviewIndex.value]
  if (!step) {
    sendPhase.value = 'sent'
    visibleStepCount.value = form.steps.length
    playbackTimer = window.setTimeout(() => {
      activePreviewIndex.value = 0
      visibleStepCount.value = 1
      sendPhase.value = 'idle'
      runPlaybackStage()
    }, 1300)
    return
  }

  visibleStepCount.value = Math.max(visibleStepCount.value, activePreviewIndex.value + 1)
  sendPhase.value = step.delay_seconds > 0 ? 'waiting' : 'sending'
  const waitMs = step.delay_seconds > 0 ? Math.min(2200, Math.max(700, step.delay_seconds * 260)) : 450
  playbackTimer = window.setTimeout(() => {
    sendPhase.value = 'sending'
    playbackTimer = window.setTimeout(() => {
      sendPhase.value = 'sent'
      activePreviewIndex.value += 1
      visibleStepCount.value = Math.min(form.steps.length, activePreviewIndex.value + 1)
      playbackTimer = window.setTimeout(runPlaybackStage, 520)
    }, 760)
  }, waitMs)
}

function stopPlaybackTimer() {
  if (playbackTimer) {
    window.clearTimeout(playbackTimer)
    playbackTimer = null
  }
}

function isStepReady(step: RichStep) {
  if (step.type === 'text') return !!step.content?.trim()
  if (step.type === 'forward') return !!step.source_chat_id?.trim() && !!step.message_id?.trim()
  return !!step.media_asset_id?.trim()
}

function invalidStepReason() {
  const index = form.steps.findIndex((step) => !isStepReady(step))
  if (index < 0) return ''
  const step = form.steps[index]
  if (step.type === 'text') return `第 ${index + 1} 阶段文本内容为空`
  if (step.type === 'forward') return `第 ${index + 1} 阶段转发来源或消息 ID 未填写`
  return `第 ${index + 1} 阶段${stepTypeLabel(step.type)}素材还没有上传`
}

function mediaAccept(type: StepType) {
  if (type === 'image') return 'image/jpeg,image/png,image/gif'
  if (type === 'gif') return 'image/gif'
  if (type === 'voice') return 'audio/mpeg,audio/mp4,audio/aac,audio/ogg,audio/wav,audio/webm,.mp3,.m4a,.aac,.ogg,.oga,.wav,.webm'
  return ''
}

function intervalLabel(seconds: number) {
  seconds = clampInteger(seconds, 0, 86400, 0)
  if (!seconds) return '不等待'
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.round(seconds / 60)} 分钟`
  return `${Math.round(seconds / 3600)} 小时`
}

function intervalUnitMultiplier(unit: IntervalUnit) {
  if (unit === 'hours') return 3600
  if (unit === 'minutes') return 60
  return 1
}

function maxIntervalValue(unit: IntervalUnit) {
  if (unit === 'hours') return 24
  if (unit === 'minutes') return 1440
  return 86400
}

function clampInteger(value: unknown, min: number, max: number, fallback: number) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return fallback
  return Math.min(max, Math.max(min, Math.floor(parsed)))
}

function normalizeCadence() {
  form.send_count = normalizedSendCount.value
  form.send_interval_value = clampInteger(form.send_interval_value, 0, maxIntervalValue(form.send_interval_unit), 0)
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
  const step = form.steps[index]
  if (!step) return
  if (!isFileAllowedForStep(step.type, file)) {
    ui.toast({ title: '文件格式不匹配', message: `${stepTypeLabel(step.type)}阶段不能使用 ${file.name}`, tone: 'warning' })
    return
  }

  step.uploading = true
  if (step.media_url?.startsWith('blob:')) {
    URL.revokeObjectURL(step.media_url)
  }
  step.media_url = URL.createObjectURL(file)
  step.media_name = file.name

  try {
    const result = await api.uploadWorkflowMedia([file])
    const item = result.items.find((entry) => entry.status === 'success' || entry.status === 'duplicate')
    if (!item?.id) {
      const reason = result.items.find((entry) => entry.reason)?.reason || '媒体上传失败'
      throw new Error(reason)
    }
    step.media_asset_id = item.id
    step.media_url = item.url || step.media_url
    step.media_name = item.name || file.name
    ui.toast({ title: '媒体已绑定', message: `${file.name} 已上传并绑定到第 ${index + 1} 阶段。`, tone: 'success' })
  } catch (error) {
    step.media_asset_id = ''
    ui.toast({ title: '媒体上传失败', message: error instanceof Error ? error.message : '请重新选择文件', tone: 'error' })
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

function stepTypeLabel(type: string) {
  const labels: Record<string, string> = {
    text: '文本',
    image: '图片',
    voice: '语音',
    gif: 'GIF',
    forward: '转发'
  }
  return labels[type] || type || '消息'
}

function simulatedMessageTime(index: number) {
  const [hourText, minuteText] = currentTime.value.split(':')
  const baseHour = Number(hourText) || 12
  const baseMinute = Number(minuteText) || 35
  const date = new Date()
  date.setHours(baseHour, baseMinute + index * 2, 0, 0)
  const h = date.getHours()
  const m = String(date.getMinutes()).padStart(2, '0')
  const suffix = h >= 12 ? 'PM' : 'AM'
  const displayHour = h % 12 || 12
  return `${displayHour}:${m} ${suffix}`
}

function resolveSpintax(text: string | undefined) {
  previewTick.value
  if (!text) return ''
  return text.replace(/\{([^{}]+)\}/g, (_, options: string) => {
    const parts = options.split('|').map((item) => item.trim()).filter(Boolean)
    if (!parts.length) return ''
    return parts[Math.floor(Math.random() * parts.length)]
  })
}

async function loadResources() {
  loading.value = true
  try {
    const [termGroups, targGroups, currentSettings] = await Promise.all([
      api.groups('terminal'),
      api.groups('target'),
      api.systemSettings()
    ])
    terminalGroups.value = termGroups
    targetGroups.value = targGroups
    settings.value = currentSettings
    await loadWorkflowTask()
  } catch (error) {
    ui.toast({ title: '资源加载失败', message: error instanceof Error ? error.message : '请稍后重试', tone: 'error' })
  } finally {
    loading.value = false
  }
}

async function loadWorkflowTask() {
  const tasks = await api.tasks({ type: 'mass_messaging', limit: 50 })
  workflowTasks.value = tasks
  if (!tasks.length) {
    workflowTaskID.value = ''
    return
  }
  const tracked = workflowTaskID.value ? tasks.find((task) => task.id === workflowTaskID.value) : null
  workflowTaskID.value = (tracked || tasks[0]).id
}

async function submitJob() {
  if (!canSubmit.value) return
  const reason = invalidStepReason()
  if (reason) {
    ui.toast({ title: '通知工作流未完成', message: reason, tone: 'warning' })
    return
  }
  normalizeCadence()
  submitting.value = true
  try {
    const steps = form.steps.map(({ _uid, media_url, media_name, uploading, ...step }) => step)
    const task = await api.createMassMessagingJob({
      terminal_group_ids: form.terminal_group_ids,
      target_group_ids: form.target_group_ids,
      send_count: normalizedSendCount.value,
      send_interval_seconds: normalizedSendInterval.value,
      steps
    })
    workflowTaskID.value = task.id
    await loadWorkflowTask()
    ui.toast({
      title: realExecutionEnabled.value ? '通知工作流已投递' : '演练任务已创建',
      message: realExecutionEnabled.value ? `任务 ${task.id} 已进入真实执行队列。` : `任务 ${task.id} 处于演练模式，不会真实发送 Telegram 消息。`,
      tone: realExecutionEnabled.value ? 'success' : 'warning'
    })
  } catch (error) {
    ui.toast({ title: '投递失败', message: error instanceof Error ? error.message : '任务创建失败', tone: 'error' })
  } finally {
    submitting.value = false
  }
}

function updateTime() {
  const now = new Date()
  currentTime.value = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
}

function taskSummaryRecord(task?: Task | null) {
  return (task?.summary as Record<string, unknown>) || {}
}

function numericSummaryValue(task: Task | null | undefined, key: string) {
  const value = taskSummaryRecord(task)?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function numericPayloadValue(task: Task | null | undefined, key: string) {
  const value = task?.payload?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function taskTotalNumber(task?: Task | null) {
  return numericSummaryValue(task, 'total')
}

function taskExecutedNumber(task?: Task | null) {
  return numericSummaryValue(task, 'success') + numericSummaryValue(task, 'failed')
}

function formatTaskMetric(value: number) {
  return `${Math.max(0, Math.floor(value || 0))}`
}

function taskProgressPercent(task?: Task | null) {
  if (!task) return 0
  const total = taskTotalNumber(task)
  const executed = taskExecutedNumber(task)
  if (total > 0) {
    return Math.min(100, Math.round((executed / total) * 100))
  }
  return Math.min(100, Math.max(0, Number(task.progress) || 0))
}

function taskTargetText(task?: Task | null) {
  if (!task) return '未设置'
  const count = numericSummaryValue(task, 'send_count') || numericPayloadValue(task, 'send_count') || 1
  return `发送 ${count} 次`
}

function taskTotalCount(task?: Task | null) {
  return formatTaskMetric(taskTotalNumber(task))
}

function taskExecutedCount(task?: Task | null) {
  return formatTaskMetric(taskExecutedNumber(task))
}

function taskSuccessCount(task?: Task | null) {
  return formatTaskMetric(numericSummaryValue(task, 'success'))
}

function taskFailedCount(task?: Task | null) {
  return formatTaskMetric(numericSummaryValue(task, 'failed'))
}

function taskStatusTone(status: string) {
  if (/dry_run/i.test(status)) return 'info'
  if (/partial_success/i.test(status)) return 'warning'
  if (/success|done|completed|finished|running/i.test(status)) return 'success'
  if (/fail|error|stopped/i.test(status)) return 'danger'
  if (/queue|pending|wait|pause/i.test(status)) return 'warning'
  return 'info'
}

function taskStatusText(status: string) {
  const normalized = (status || '').toLowerCase()
  if (normalized === 'dry_run') return '演练完成'
  if (normalized === 'success') return '执行成功'
  if (normalized === 'partial_success') return '部分成功'
  if (normalized === 'failed') return '执行失败'
  if (normalized === 'queued') return '排队中'
  if (normalized === 'running') return '执行中'
  if (normalized === 'pending') return '待执行'
  if (normalized === 'paused') return '已暂停'
  if (normalized === 'stopped') return '已停止'
  return status || '未知状态'
}

function formatTaskTime(value?: string) {
  if (!value) return '刚刚'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

async function prefetchTaskFailure(task: Task) {
  if (!/failed|partial_success/i.test(task.status)) return
  if (taskFailureReasons[task.id]) return
  try {
    const logs = await api.taskLogs(task.id, { limit: 300 })
    const failedLog = [...logs].reverse().find((log) => log.level === 'ERROR' || /失败|failed|error/i.test(log.details))
    taskFailureReasons[task.id] = failedLog?.details || '暂无失败详情'
  } catch {
    taskFailureReasons[task.id] = '失败原因读取失败'
  }
}

function taskFailureTooltip(task: Task) {
  if (!/failed|partial_success/i.test(task.status)) return ''
  return taskFailureReasons[task.id] || '鼠标停留后加载失败原因'
}

async function deleteTask(taskID: string) {
  deletingTaskIDs[taskID] = true
  try {
    await api.deleteTask(taskID)
    delete taskFailureReasons[taskID]
    if (workflowTaskID.value === taskID) {
      workflowTaskID.value = ''
    }
    await loadWorkflowTask()
    ui.toast({ title: '任务已删除', message: `任务 ${taskID.slice(0, 8)} 已移除。`, tone: 'success' })
  } catch (error) {
    ui.toast({ title: '删除失败', message: error instanceof Error ? error.message : '任务删除失败', tone: 'error' })
  } finally {
    delete deletingTaskIDs[taskID]
  }
}

async function deleteTasksByStatus(scope: 'completed' | 'failed') {
  const tasks = scope === 'completed' ? completedTasks.value : failedTasks.value
  if (!tasks.length) return
  deletingTaskScope.value = scope
  try {
    for (const task of tasks) {
      await api.deleteTask(task.id)
      delete taskFailureReasons[task.id]
      delete deletingTaskIDs[task.id]
    }
    if (tasks.some((task) => task.id === workflowTaskID.value)) {
      workflowTaskID.value = ''
    }
    await loadWorkflowTask()
    ui.toast({
      title: scope === 'completed' ? '已清理完成任务' : '已清理失败任务',
      message: `共删除 ${tasks.length} 条任务记录。`,
      tone: 'success'
    })
  } catch (error) {
    ui.toast({ title: '批量删除失败', message: error instanceof Error ? error.message : '请稍后重试', tone: 'error' })
  } finally {
    deletingTaskScope.value = ''
  }
}

onMounted(() => {
  void loadResources()
  updateTime()
  visibleStepCount.value = 1
  clockTimer = window.setInterval(updateTime, 10000)
  previewTimer = window.setInterval(() => {
    previewTick.value += 1
  }, 2400)
  workflowTaskTimer = window.setInterval(() => {
    void loadWorkflowTask()
  }, 3000)
  window.setTimeout(startSimulation, 400)
})

onBeforeUnmount(() => {
  if (clockTimer) window.clearInterval(clockTimer)
  if (previewTimer) window.clearInterval(previewTimer)
  if (workflowTaskTimer) window.clearInterval(workflowTaskTimer)
  stopPlaybackTimer()
  for (const step of form.steps) {
    if (step.media_url?.startsWith('blob:')) {
      URL.revokeObjectURL(step.media_url)
    }
  }
})
</script>
