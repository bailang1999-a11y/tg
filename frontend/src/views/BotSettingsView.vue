<template>
  <div class="page-shell bot-settings-shell">
    <div class="page-header">
      <div>
        <div class="eyebrow">BOT CONTROL</div>
        <h1 class="page-title">Bot 配置</h1>
        <p class="page-subtitle">管理员统一配置 Bot 连接、监听关键词、功能按钮、回复文案、卡密和用户权限。</p>
      </div>
      <div class="page-actions">
        <span class="status-pill" :data-tone="config.running ? 'success' : config.enabled ? 'warning' : 'info'">
          {{ config.running ? 'Bot 推送运行中' : config.enabled ? '已启用未运行' : '未启动' }}
        </span>
        <span class="status-pill" :data-tone="config.last_test_status === 'success' ? 'success' : config.last_test_status === 'failed' ? 'danger' : 'info'">
          {{ config.last_test_status === 'success' ? '连接正常' : config.last_test_status === 'failed' ? '连接失败' : '未测试' }}
        </span>
        <GlassButton variant="secondary" :loading="loading" @click="loadAll">刷新</GlassButton>
      </div>
    </div>

    <div class="bot-command-center">
      <GlassCard v-for="item in overviewCards" :key="item.label" class="overview-card" :data-tone="item.tone">
        <span>{{ item.icon }}</span>
        <div>
          <small>{{ item.label }}</small>
          <strong>{{ item.value }}</strong>
          <em>{{ item.help }}</em>
        </div>
      </GlassCard>
    </div>

    <div class="module-jumpbar">
      <button
        v-for="item in moduleViews"
        :key="item.key"
        type="button"
        :class="['module-chip', { active: activeModuleView === item.key }]"
        @click="activeModuleView = item.key"
      >
        {{ item.label }}
      </button>
    </div>

    <div class="bot-grid">
      <GlassCard v-show="showModule('link')" id="bot-link" class="bot-panel integration-panel">
        <div class="integration-header">
          <div class="panel-title">
            <span>🤖</span>
            <div>
              <h2>对接管理机器人</h2>
              <p>统一管理 Bot Token、Webhook、本地轮询和指令同步，先看状态再动手。</p>
            </div>
          </div>
          <div class="integration-badges">
            <span class="status-pill" :data-tone="config.running ? 'success' : 'warning'">{{ config.running ? 'Bot 推送运行中' : 'Bot 推送未运行' }}</span>
            <span class="status-pill" :data-tone="pollingStatus.running ? 'success' : 'info'">{{ pollingStatus.running ? '本地轮询运行中' : '本地轮询未启动' }}</span>
            <span class="status-pill" :data-tone="config.webhook_url ? 'success' : 'warning'">{{ config.webhook_url ? 'Webhook 已配置' : 'Webhook 未配置' }}</span>
          </div>
        </div>

        <div class="integration-grid">
          <section class="integration-cardlet main-connection">
            <div class="cardlet-title">
              <span>01</span>
              <div>
                <strong>基础连接</strong>
                <small>Token、管理员与推送会话</small>
              </div>
            </div>
            <div class="form-grid integration-form">
              <label>
                <span>Bot 名称</span>
                <input v-model="form.name" placeholder="Codex3 Bot" />
              </label>
              <label>
                <span>Bot Token</span>
                <input v-model="form.token" placeholder="123456:ABC..." />
              </label>
              <label>
                <span>推送 Chat ID</span>
                <input v-model="form.push_chat_id" placeholder="-100xxxxxxxxxx 或用户 ID" />
              </label>
              <label>
                <span>管理员 Chat ID</span>
                <input v-model="form.admin_chat_id" placeholder="管理员 Telegram ID" />
              </label>
              <label class="wide-field support-contact-field">
                <span>客服联系方式</span>
                <input v-model="form.admin_contact" placeholder="@your_support / Telegram 链接 / 微信 / 邮箱" />
                <small>这里会出现在 Bot 的“在线客服”回复里，用户点客服菜单时能直接看到。</small>
              </label>
            </div>

            <div class="bot-actions compact-actions">
              <GlassButton variant="primary" :loading="saving" @click="saveConfig">保存配置</GlassButton>
              <GlassButton variant="secondary" :loading="testing" @click="testBot">测试连接</GlassButton>
              <GlassButton variant="secondary" :loading="commandBusy" @click="syncCommands">同步菜单</GlassButton>
              <GlassButton v-if="!config.running" variant="success" :loading="botBusy" @click="startBot">启动推送</GlassButton>
              <GlassButton v-else variant="danger" :loading="botBusy" @click="stopBot">停止推送</GlassButton>
            </div>

            <div class="test-result compact-result">
              <strong>{{ config.username ? `@${config.username}` : '尚未识别 Bot 用户名' }}</strong>
              <span>{{ config.last_test_message || '点击测试连接后，这里会显示 Telegram 返回结果。' }}</span>
            </div>
          </section>

          <section class="integration-cardlet">
            <div class="cardlet-title">
              <span>02</span>
              <div>
                <strong>Webhook 对接</strong>
                <small>公网回调，适合正式部署</small>
              </div>
            </div>
            <label class="block-field compact-field">
              <span>已解析的 HTTPS 域名</span>
              <input v-model="form.webhook_url" placeholder="https://tg.huazhaikeji.cc" />
            </label>
            <div class="bot-actions compact-actions">
              <GlassButton variant="primary" :loading="webhookBusy" @click="setupWebhook">保存并检查</GlassButton>
              <GlassButton variant="secondary" :loading="webhookBusy" @click="checkWebhook">检查连接</GlassButton>
              <GlassButton variant="danger" :loading="webhookBusy" @click="clearWebhook">一键关闭</GlassButton>
            </div>
            <div class="test-result compact-result">
              <strong>{{ webhookConnectionTitle }}</strong>
              <span>{{ webhookStatusText || config.last_webhook_message || '没有公网时，可使用右侧本地轮询测试。' }}</span>
            </div>
          </section>

          <section class="integration-cardlet">
            <div class="cardlet-title">
              <span>03</span>
              <div>
                <strong>本地轮询测试</strong>
                <small>无公网时直接拉取 Telegram 消息</small>
              </div>
            </div>
            <div class="polling-metrics">
              <div><span>运行状态</span><strong>{{ pollingStatus.running ? '运行中' : '未启动' }}</strong></div>
              <div><span>已处理</span><strong>{{ pollingStatus.handled_count || 0 }} 条</strong></div>
              <div><span>最近消息</span><strong>{{ formatDateTime(pollingStatus.last_message_at) }}</strong></div>
            </div>
            <div class="bot-actions compact-actions">
              <GlassButton v-if="!pollingStatus.running" variant="success" :loading="pollingBusy" @click="startPolling">启动本地轮询</GlassButton>
              <GlassButton v-else variant="danger" :loading="pollingBusy" @click="stopPolling">停止本地轮询</GlassButton>
              <GlassButton variant="secondary" :loading="pollingBusy" @click="loadPollingStatus">刷新状态</GlassButton>
            </div>
            <div class="test-result compact-result">
              <strong>{{ pollingStatus.last_error ? '轮询有错误' : '轮询诊断' }}</strong>
              <span>{{ pollingStatus.last_error ? `错误：${pollingStatus.last_error}` : '本地轮询会复用同一套 Bot 指令逻辑。' }}</span>
            </div>
          </section>
        </div>
      </GlassCard>

      <div v-show="showModule('policy')" class="module-section-title">
        <span>STRATEGY</span>
        <div>
          <strong>业务策略</strong>
          <small>控制试用权限、关键词配额和强制加群策略。</small>
        </div>
      </div>

      <GlassCard v-show="showModule('policy')" id="bot-policy" class="bot-panel policy-panel">
        <div class="panel-title">
          <span>🎯</span>
          <div>
            <h2>监听与试用</h2>
            <p>只保留试用时间、关键词上限、强制加入公开群/频道和设置关键词功能。</p>
          </div>
        </div>

        <div class="policy-execution-grid">
          <div class="policy-editor">
            <div class="trial-control compact-trial-control">
              <div class="trial-duration">
                <label>
                  <span>试用时间</span>
                  <input v-model.number="trialDurationValue" min="1" type="number" />
                </label>
                <label>
                  <span>时间单位</span>
                  <select v-model="trialDurationUnit">
                    <option value="hour">小时</option>
                    <option value="day">天</option>
                  </select>
                </label>
              </div>
            </div>

            <div class="form-grid">
              <label>
                <span>关键词上限</span>
                <input v-model.number="form.default_keyword_limit" min="1" type="number" placeholder="20" />
              </label>
            </div>

            <div class="force-join-box">
              <label>
                <input v-model="form.force_join_enabled" type="checkbox" />
                <span>强制加入公开群/频道后才能试用和设置关键词</span>
              </label>
              <input v-model="form.force_join_url" placeholder="https://t.me/your_channel 或 @your_channel" />
              <small>公开链接会自动解析；Bot 需要在该群/频道中，建议设置为管理员。</small>
            </div>
          </div>

          <div class="policy-status-card">
            <strong>执行链路</strong>
            <p>保存后会直接影响 Bot 端 /trial、/setkeywords 和设置中心关键词入口。</p>
            <div class="policy-status-list">
              <div v-for="item in policyExecutionItems" :key="item.label">
                <span :data-tone="item.ok ? 'success' : 'warning'">{{ item.ok ? '已接入' : '待完善' }}</span>
                <div>
                  <b>{{ item.label }}</b>
                  <small>{{ item.detail }}</small>
                </div>
              </div>
            </div>
            <GlassButton variant="primary" :loading="saving" @click="saveConfig">保存并生效</GlassButton>
          </div>
        </div>
      </GlassCard>

      <GlassCard v-show="showModule('commands')" id="bot-commands" class="bot-panel command-panel">
        <div class="panel-title">
          <span>⌘</span>
          <div>
            <h2>功能命令</h2>
            <p>勾选后会出现在 Telegram 指令菜单和机器人设置中心里，避免用户手动记命令。</p>
          </div>
        </div>

        <div class="command-toolbar">
          <GlassButton variant="secondary" @click="selectAllCommands">全选功能</GlassButton>
          <GlassButton variant="secondary" @click="clearOptionalCommands">仅保留试用/关键词</GlassButton>
          <GlassButton variant="primary" :loading="commandBusy" @click="syncCommands">保存并同步菜单</GlassButton>
        </div>

        <div class="command-grid">
          <div v-for="item in commandOptions" :key="item.key" class="command-option" :class="{ active: form.enabled_commands.includes(item.key) }">
            <label class="command-check">
              <input :checked="form.enabled_commands.includes(item.key)" :disabled="isPolicyCommand(item.key)" type="checkbox" @change="toggleCommand(item.key)" />
              <span class="command-title">{{ item.title }}</span>
              <code>{{ item.command }}</code>
            </label>
            <input v-model="form.command_labels[item.key]" class="command-name-input" :placeholder="item.description" />
            <small>{{ isPolicyCommand(item.key) ? '基础必开：机器人入口或监听与试用执行链路依赖此命令。' : item.description }}</small>
          </div>
        </div>
      </GlassCard>

      <div v-show="showModule('visual')" class="module-section-title">
        <span>EXPERIENCE</span>
        <div>
          <strong>机器人体验</strong>
          <small>统一配置按钮、文案和 Telegram 手机端预览。</small>
        </div>
      </div>

      <GlassCard v-show="showModule('visual')" id="bot-visual" class="bot-panel visual-bot-panel">
        <div class="panel-title">
          <span>🎛</span>
          <div>
            <h2>可视化机器人</h2>
            <p>直接配置欢迎语、底部菜单、FAQ 和客服内容，保存后会用于 Telegram 真实回复。</p>
          </div>
        </div>

        <div class="visual-editor-grid">
          <div class="bot-copy-editor">
            <div class="visual-command-editor">
              <div class="form-grid">
                <label>
                  <span>配置类型</span>
                  <select v-model="visualSelection.type" @change="syncVisualSelection()">
                    <option value="reply">回复文案</option>
                    <option value="button">功能按钮</option>
                    <option value="command">Bot 命令</option>
                  </select>
                </label>
                <label>
                  <span>功能分组</span>
                  <select v-model="visualSelection.group" @change="syncVisualSelectionKey()">
                    <option v-for="group in visualGroupOptions" :key="group.key" :value="group.key">{{ group.label }}</option>
                  </select>
                </label>
                <label>
                  <span>功能 / 命令</span>
                  <select v-model="visualSelection.key">
                    <option v-for="item in visualEditableOptions" :key="item.key" :value="item.key">{{ item.title }}</option>
                  </select>
                </label>
                <label class="compact-field">
                  <span>当前编辑项说明</span>
                  <input :value="selectedVisualItem?.help || '选择后可在下方直接编辑并实时预览'" readonly />
                </label>
              </div>

              <label v-if="visualSelection.type === 'reply'" class="block-field">
                <span>{{ selectedVisualItem?.title || '回复文案' }}</span>
                <textarea v-model="selectedReplyTemplate.text" :placeholder="selectedVisualItem?.help || '请输入机器人返回的提示内容'"></textarea>
              </label>
              <label v-else-if="visualSelection.type === 'button'" class="block-field compact-field">
                <span>{{ selectedVisualItem?.title || '按钮名称' }}</span>
                <input v-model="selectedButtonDraft.label" :placeholder="selectedVisualItem?.help || '请输入按钮名称'" />
              </label>
              <label v-else class="block-field compact-field">
                <span>{{ selectedVisualItem?.title || '命令显示名' }}</span>
                <input v-model="form.command_labels[visualSelection.key]" placeholder="会同步到 Telegram 指令菜单描述" />
              </label>
            </div>

            <div class="form-grid">
              <label>
                <span>欢迎标题</span>
                <input v-model="form.welcome_title" placeholder="欢迎使用 Codex3 监听机器人" />
              </label>
              <label>
                <span>输入框提示</span>
                <input v-model="form.menu_placeholder" placeholder="选择功能或输入命令..." />
              </label>
            </div>

            <label class="block-field">
              <span>服务概述</span>
              <textarea v-model="form.service_overview" placeholder="监听目标群组中的关键词命中，并把线索实时汇聚到你的收件箱。"></textarea>
            </label>

            <label class="block-field">
              <span>快速开始说明</span>
              <textarea v-model="form.quick_start_text" placeholder="点击下方菜单进入设置中心，或发送 /keywords 查看当前关键词。"></textarea>
            </label>

            <div class="menu-label-grid">
              <label>
                <span>菜单 1</span>
                <input v-model="form.menu_info_label" />
              </label>
              <label>
                <span>菜单 2</span>
                <input v-model="form.menu_settings_label" />
              </label>
              <label>
                <span>菜单 3</span>
                <input v-model="form.menu_faq_label" />
              </label>
              <label>
                <span>菜单 4</span>
                <input v-model="form.menu_support_label" />
              </label>
            </div>

            <label class="block-field">
              <span>常见问题回复</span>
              <textarea v-model="form.faq_text" placeholder="常见问题内容"></textarea>
            </label>

            <label class="block-field">
              <span>在线客服回复</span>
              <textarea v-model="form.support_text" placeholder="例如：在线客服&#10;&#10;请联系 @your_support"></textarea>
              <small>留空时会自动使用上方客服联系方式；填写后会追加客服联系方式。</small>
            </label>
          </div>

          <div class="bot-phone-preview">
            <div class="phone-top">
              <span></span>
              <strong>{{ form.name || 'Codex3 Bot' }}</strong>
              <small>@{{ config.username || 'bot_username' }}</small>
            </div>
            <div class="chat-preview scrollbar-thin">
              <div class="bubble bot-bubble compact">
                <pre>{{ liveVisualPreview }}</pre>
              </div>
              <div class="bubble bot-bubble">
                <pre>{{ previewWelcome }}</pre>
              </div>
              <div class="bubble user-bubble">{{ menuLabels[1] }}</div>
              <div class="bubble bot-bubble compact">
                <pre>{{ previewSettings }}</pre>
              </div>
              <div class="bubble user-bubble">{{ menuLabels[3] }}</div>
              <div class="bubble bot-bubble compact">
                <pre>{{ supportContactPreview }}</pre>
              </div>
            </div>
            <div class="keyboard-preview">
              <button v-for="label in menuLabels" :key="label" type="button">{{ label }}</button>
            </div>
            <div class="placeholder-preview">{{ form.menu_placeholder || '选择功能或输入命令...' }}</div>
          </div>
        </div>

        <div class="bot-actions">
          <GlassButton variant="primary" :loading="saving" @click="saveConfig">保存机器人界面</GlassButton>
          <GlassButton variant="secondary" :loading="testing" @click="testBot">保存并测试连接</GlassButton>
        </div>
      </GlassCard>

    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { api, type BotConfig, type BotPollingStatus } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUiStore } from '../stores/ui'

const ui = useUiStore()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const botBusy = ref(false)
const commandBusy = ref(false)
const webhookBusy = ref(false)
const pollingBusy = ref(false)
const webhookStatusText = ref('')
const activeModuleView = ref<'all' | 'link' | 'policy' | 'commands' | 'visual'>('all')
const moduleViews = [
  { key: 'all', label: '全部模块' },
  { key: 'link', label: '对接管理' },
  { key: 'policy', label: '监听与试用' },
  { key: 'commands', label: '功能命令' },
  { key: 'visual', label: '可视化机器人' }
] as const
const commandOptions = [
  { key: 'start', command: '/start', title: '开始使用', description: '打开机器人并显示欢迎语。' },
  { key: 'help', command: '/help', title: '帮助说明', description: '查看机器人基础帮助。' },
  { key: 'trial', command: '/trial', title: '开通试用', description: '用户一键领取后台设置的试用时长。' },
  { key: 'activate', command: '/activate', title: '卡密激活', description: '用户输入卡密激活正式权限。' },
  { key: 'status', command: '/status', title: '账号状态', description: '查看试用、授权和到期状态。' },
  { key: 'keywords', command: '/keywords', title: '查看关键词', description: '查看当前监听关键词。' },
  { key: 'setkeywords', command: '/setkeywords', title: '监听关键词', description: '用户可修改监听关键词。' },
  { key: 'match', command: '/match', title: '匹配模式', description: '切换模糊匹配或精准匹配。' },
  { key: 'setpush', command: '/setpush', title: '推送位置', description: '设置线索推送到个人、群组或频道。' },
  { key: 'listen', command: '/listen', title: '监听开关', description: '启动或暂停关键词监听。' },
  { key: 'inbox', command: '/inbox', title: '收件箱', description: '查看最近命中的线索。' }
]
const policyCommandKeys = ['start', 'help', 'trial', 'keywords', 'setkeywords']
const commandLineMap: Record<string, string> = {
  start: '开始使用：/start',
  help: '帮助说明：/help',
  trial: '开通试用：/trial',
  activate: '卡密激活：/activate 卡密',
  status: '账号状态：/status',
  keywords: '查看关键词：/keywords',
  setkeywords: '监听关键词：/setkeywords 合作 请教 多少钱',
  match: '匹配模式：/match fuzzy 或 /match exact',
  setpush: '推送位置：/setpush @channel 或在群内发送 /setpush',
  listen: '监听控制：/listen start 或 /listen pause',
  inbox: '最新命中：/inbox'
}

const config = ref<BotConfig>({
  id: '',
  name: '',
  token: '',
  username: '',
  push_chat_id: '',
  admin_chat_id: '',
  admin_contact: '',
  enabled: false,
  force_join_enabled: false,
  force_join_url: '',
  running: false,
  trial_enabled: true,
  trial_hours: 5,
  trial_features: ['keyword_monitor'],
  enabled_commands: commandOptions.map((item) => item.key),
  command_labels: Object.fromEntries(commandOptions.map((item) => [item.key, item.description])),
  default_keywords: [],
  default_keyword_limit: 20,
  default_match_mode: 'fuzzy',
  private_terminal_ids: [],
  welcome_title: '',
  service_overview: '',
  quick_start_text: '',
  faq_text: '',
  support_text: '',
  menu_info_label: '📋 我的信息',
  menu_settings_label: '⚙️ 设置中心',
  menu_faq_label: '❓ 常见问题',
  menu_support_label: '💬 在线客服',
  menu_placeholder: '选择功能或输入命令...',
  button_labels: {},
  reply_templates: {},
  default_dm_messages: [],
  dm_min_delay_seconds: 4,
  dm_max_delay_seconds: 8,
  dm_max_messages: 3,
  webhook_url: '',
  webhook_secret: '',
  last_webhook_status: '',
  last_webhook_message: '',
  last_test_status: '',
  last_test_message: '',
  updated_at: ''
})

const form = reactive({
  name: '',
  token: '',
  push_chat_id: '',
  admin_chat_id: '',
  admin_contact: '',
  enabled: false,
  force_join_enabled: false,
  force_join_url: '',
  trial_enabled: true,
  trial_hours: 5,
  trial_features: ['keyword_monitor'],
  enabled_commands: commandOptions.map((item) => item.key),
  command_labels: Object.fromEntries(commandOptions.map((item) => [item.key, item.description])) as Record<string, string>,
  default_keyword_limit: 20,
  default_match_mode: 'fuzzy' as 'fuzzy' | 'exact',
  private_terminal_ids: [] as string[],
  welcome_title: '',
  service_overview: '',
  quick_start_text: '',
  faq_text: '',
  support_text: '',
  menu_info_label: '📋 我的信息',
  menu_settings_label: '⚙️ 设置中心',
  menu_faq_label: '❓ 常见问题',
  menu_support_label: '💬 在线客服',
  menu_placeholder: '选择功能或输入命令...',
  webhook_url: ''
})
const webhookConnectionTitle = computed(() => {
  if (!config.value.webhook_url) return '尚未设置 Webhook'
  if (config.value.last_webhook_status === 'success') return '连接成功'
  if (config.value.last_webhook_status === 'failed') return '连接不成功'
  return config.value.webhook_url
})
const keywordsText = ref('')
const trialDurationValue = ref(5)
const trialDurationUnit = ref<'hour' | 'day'>('hour')
const pollingStatus = ref<BotPollingStatus>({ running: false })
const botButtonDrafts = reactive([
  { key: 'listen_keywords', title: '监听关键词按钮', label: '🔑 监听关键词', help: '替代原“私信关键词”，用于设置监听关键词。' },
  { key: 'listen_start', title: '启动监听按钮', label: '▶️ 启动监听', help: '启动当前 Bot 用户独立监听进程。' },
  { key: 'listen_pause', title: '暂停监听按钮', label: '⏸ 暂停监听', help: '暂停监听任务。' },
  { key: 'membership', title: '开通会员按钮', label: '💳 开通会员', help: '输入管理员生成的卡密激活。' }
])
const replyTemplates = reactive([
  { key: 'welcome', title: '欢迎语', text: '欢迎使用监听机器人，请点击设置中心开始配置。' },
  { key: 'keywords_prompt', title: '监听关键词提示', text: '请输入监听关键词，一行一个，或用逗号分隔。' },
  { key: 'admin_contact', title: '联系管理员提示', text: '请联系管理员开通权限或处理使用问题。' }
])
const visualSelection = reactive({
  type: 'reply' as 'reply' | 'button' | 'command',
  group: 'intro',
  key: 'welcome'
})

const visualEditableByType = computed(() => {
  if (visualSelection.type === 'button') {
    return botButtonDrafts.map((item) => ({ key: item.key, title: item.title, help: item.help, group: item.key.includes('listen') ? 'listener' : 'account' }))
  }
  if (visualSelection.type === 'command') {
    return commandOptions.map((item) => {
      const group = ['start', 'help', 'status'].includes(item.key)
        ? 'intro'
        : ['keywords', 'setkeywords', 'match', 'listen', 'inbox'].includes(item.key)
          ? 'listener'
          : ['setpush', 'trial', 'activate'].includes(item.key)
            ? 'account'
            : 'account'
      return { key: item.key, title: `${item.title} ${item.command}`, help: item.description, group }
    })
  }
  return replyTemplates.map((item) => ({ key: item.key, title: item.title, help: '设置该功能返回给用户的文案。', group: ['welcome'].includes(item.key) ? 'intro' : ['keywords_prompt'].includes(item.key) ? 'listener' : 'account' }))
})
const visualGroupOptions = computed(() => {
  const groups = [
    { key: 'intro', label: '基础引导' },
    { key: 'listener', label: '监听控制' },
    { key: 'account', label: '账户与会员' }
  ]
  const keys = new Set(visualEditableByType.value.map((item) => item.group))
  return groups.filter((item) => keys.has(item.key))
})
const visualEditableOptions = computed(() => {
  const scoped = visualEditableByType.value.filter((item) => item.group === visualSelection.group)
  if (scoped.length) return scoped
  return visualEditableByType.value
})
const selectedVisualItem = computed(() => visualEditableOptions.value.find((item) => item.key === visualSelection.key))
const selectedReplyTemplate = computed(() => replyTemplates.find((item) => item.key === visualSelection.key) || replyTemplates[0])
const selectedButtonDraft = computed(() => botButtonDrafts.find((item) => item.key === visualSelection.key) || botButtonDrafts[0])
const liveVisualPreview = computed(() => {
  if (visualSelection.type === 'command') {
    const cmd = commandOptions.find((item) => item.key === visualSelection.key)
    const label = form.command_labels[visualSelection.key] || cmd?.description || '未设置命令文案'
    return `命令：${cmd?.command || '/unknown'}\n名称：${cmd?.title || '未命名'}\n显示：${label}`
  }
  if (visualSelection.type === 'button') {
    const button = botButtonDrafts.find((item) => item.key === visualSelection.key)
    return `功能按钮\n${button?.title || '未命名按钮'}\n按钮文案：${button?.label || '未设置'}`
  }
  const reply = replyTemplates.find((item) => item.key === visualSelection.key)
  return `回复文案\n${reply?.title || '未命名回复'}\n\n${reply?.text || '未设置回复内容'}`
})
const enabledCommandCount = computed(() => new Set([...form.enabled_commands, 'start', 'help']).size)

const overviewCards = computed(() => [
  {
    icon: '●',
    label: 'Bot 服务',
    value: config.value.running ? '运行中' : config.value.enabled ? '已启用' : '未启动',
    help: config.value.username ? `@${config.value.username}` : '等待连接测试',
    tone: config.value.running ? 'success' : config.value.enabled ? 'warning' : 'muted'
  },
  {
    icon: '⌁',
    label: '监听关键词',
    value: `${keywordList().length} / ${form.default_keyword_limit || 20}`,
    help: form.default_match_mode === 'exact' ? '精准匹配' : '模糊匹配',
    tone: 'info'
  },
  {
    icon: '⌘',
    label: '可用命令',
    value: `${enabledCommandCount.value} 个`,
    help: '含固定 /start、/help',
    tone: 'pink'
  },
  {
    icon: '✦',
    label: '试用策略',
    value: form.trial_enabled ? `${trialHoursFromVisual()} 小时` : '已关闭',
    help: form.force_join_enabled ? '需先加入公开群/频道' : '无需强制加群',
    tone: form.trial_enabled ? 'success' : 'muted'
  }
])

const policyExecutionItems = computed(() => [
  {
    label: '试用时长',
    ok: form.trial_enabled && trialHoursFromVisual() > 0,
    detail: `用户领取试用后写入到期时间：${trialHoursFromVisual()} 小时`
  },
  {
    label: '关键词上限',
    ok: Number(form.default_keyword_limit || 0) > 0,
    detail: `/setkeywords 和设置中心都会按 ${form.default_keyword_limit || 20} 个关键词拦截`
  },
  {
    label: '强制加群',
    ok: !form.force_join_enabled || Boolean(form.force_join_url.trim()),
    detail: form.force_join_enabled ? '试用、设置关键词前会先校验公开群/频道成员关系' : '当前未启用强制加群'
  },
  {
    label: '命令开关',
    ok: form.enabled_commands.includes('setkeywords'),
    detail: '后台会拦截未开启的 Bot 命令，不能靠手动输入绕过'
  }
])

const menuLabels = computed(() => [
  form.menu_info_label || '📋 我的信息',
  form.menu_settings_label || '⚙️ 设置中心',
  form.menu_faq_label || '❓ 常见问题',
  form.menu_support_label || '💬 在线客服'
])
const previewWelcome = computed(() => [
  form.welcome_title || '欢迎使用 Codex3 监听机器人',
  '',
  '服务概述',
  form.service_overview || '监听目标群组中的关键词命中，并把线索实时汇聚到你的收件箱。',
  '',
  '立即开始',
  form.quick_start_text || '点击下方菜单进入设置中心，或发送 /keywords 查看当前关键词。',
  '',
  `试用用户仅可使用关键词监听功能。`
].join('\n'))
const previewSettings = computed(() => {
  const lines = form.enabled_commands.map((key) => {
    const base = commandLineMap[key]
    if (!base) return ''
    const label = form.command_labels[key]
    return label ? `${label}：${base.split('：').slice(1).join('：') || base}` : base
  }).filter(Boolean)
  return ['设置中心', '', ...(lines.length ? lines : ['当前没有开放可自助操作的功能命令。'])].join('\n')
})

const supportContactPreview = computed(() => {
  const contact = form.admin_contact.trim()
  const text = form.support_text.trim()
  if (text && contact) return `${text}\n\n客服联系方式：${contact}`
  if (text) return text
  if (contact) return `在线客服\n\n客服联系方式：${contact}`
  return '在线客服\n\n管理员暂未配置客服联系方式，请稍后再试。'
})

function showModule(key: Exclude<(typeof moduleViews)[number]['key'], 'all'>) {
  return activeModuleView.value === 'all' || activeModuleView.value === key
}

function syncVisualSelection() {
  const nextGroup = visualGroupOptions.value[0]?.key || 'intro'
  visualSelection.group = nextGroup
  syncVisualSelectionKey()
}

function syncVisualSelectionKey() {
  const current = visualEditableOptions.value.find((item) => item.key === visualSelection.key)
  if (current) return
  visualSelection.key = visualEditableOptions.value[0]?.key || ''
}

async function loadAll() {
  loading.value = true
  try {
    const [botConfig, pollingData] = await Promise.all([
      api.botConfig(),
      api.botPollingStatus()
    ])
    applyConfig(botConfig)
    pollingStatus.value = pollingData
  } catch (err) {
    ui.toast({ title: 'Bot 配置读取失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    loading.value = false
  }
}

function applyConfig(next: BotConfig) {
  config.value = next
  form.name = next.name || ''
  form.token = next.token || ''
  form.push_chat_id = next.push_chat_id || ''
  form.admin_chat_id = next.admin_chat_id || ''
  form.admin_contact = next.admin_contact || ''
  form.enabled = Boolean(next.enabled)
  form.force_join_enabled = Boolean(next.force_join_enabled)
  form.force_join_url = next.force_join_url || ''
  form.trial_enabled = true
  form.trial_hours = next.trial_hours || 5
  form.trial_features = next.trial_features?.length ? [...next.trial_features] : ['keyword_monitor']
  form.enabled_commands = Array.isArray(next.enabled_commands) ? [...next.enabled_commands] : commandOptions.map((item) => item.key)
  form.command_labels = {
    ...Object.fromEntries(commandOptions.map((item) => [item.key, item.description])),
    ...(next.command_labels || {})
  }
  form.default_keyword_limit = Number(next.default_keyword_limit || 20)
  form.default_match_mode = next.default_match_mode === 'exact' ? 'exact' : 'fuzzy'
  form.private_terminal_ids = Array.isArray(next.private_terminal_ids) ? [...next.private_terminal_ids] : []
  form.welcome_title = next.welcome_title || ''
  form.service_overview = next.service_overview || ''
  form.quick_start_text = next.quick_start_text || ''
  form.faq_text = next.faq_text || ''
  form.support_text = next.support_text || ''
  form.menu_info_label = next.menu_info_label || '📋 我的信息'
  form.menu_settings_label = next.menu_settings_label || '⚙️ 设置中心'
  form.menu_faq_label = next.menu_faq_label || '❓ 常见问题'
  form.menu_support_label = next.menu_support_label || '💬 在线客服'
  form.menu_placeholder = next.menu_placeholder || '选择功能或输入命令...'
  form.webhook_url = next.webhook_url || ''
  applyButtonLabels(next.button_labels || {})
  applyReplyTemplates(next.reply_templates || {})
  keywordsText.value = Array.isArray(next.default_keywords) ? next.default_keywords.join('\n') : ''
  applyTrialDuration(form.trial_hours)
}

function keywordList() {
  return Array.from(new Set(keywordsText.value.split('\n').map((item) => item.trim()).filter(Boolean)))
}

function applyTrialDuration(hours: number) {
  if (hours > 0 && hours % 24 === 0) {
    trialDurationUnit.value = 'day'
    trialDurationValue.value = Math.max(1, hours / 24)
    return
  }
  trialDurationUnit.value = 'hour'
  trialDurationValue.value = Math.max(1, hours || 5)
}

function trialHoursFromVisual() {
  const raw = Math.max(1, Number(trialDurationValue.value) || 1)
  return trialDurationUnit.value === 'day' ? raw * 24 : raw
}

function toggleCommand(key: string) {
  if (isPolicyCommand(key)) return
  form.enabled_commands = form.enabled_commands.includes(key)
    ? form.enabled_commands.filter((item) => item !== key)
    : [...form.enabled_commands, key]
}

function selectAllCommands() {
  form.enabled_commands = commandOptions.map((item) => item.key)
}

function clearOptionalCommands() {
  form.enabled_commands = [...policyCommandKeys]
}

function isPolicyCommand(key: string) {
  return policyCommandKeys.includes(key)
}

function commandKeysForSave() {
  return Array.from(new Set([...form.enabled_commands, ...policyCommandKeys]))
}

async function saveConfig() {
  saving.value = true
  try {
    form.trial_hours = trialHoursFromVisual()
    form.trial_enabled = true
    form.trial_features = ['keyword_monitor']
    const updated = await api.updateBotConfig({
      ...form,
      enabled_commands: commandKeysForSave(),
      default_keywords: keywordList(),
      button_labels: buttonLabelPayload(),
      reply_templates: replyTemplatePayload(),
      default_dm_messages: config.value.default_dm_messages || [],
      dm_min_delay_seconds: config.value.dm_min_delay_seconds || 4,
      dm_max_delay_seconds: config.value.dm_max_delay_seconds || 8,
      dm_max_messages: config.value.dm_max_messages || 3
    })
    applyConfig(updated)
    ui.toast({ title: 'Bot 配置已保存', message: '连接、试用策略、关键词权限和机器人界面已更新。', tone: 'success' })
  } catch (err) {
    ui.toast({ title: '保存失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    saving.value = false
  }
}

function applyButtonLabels(values: Record<string, string>) {
  for (const item of botButtonDrafts) {
    item.label = values[item.key] || item.label
  }
}

function applyReplyTemplates(values: Record<string, string>) {
  for (const item of replyTemplates) {
    item.text = values[item.key] ?? item.text
  }
}

function buttonLabelPayload() {
  return Object.fromEntries(botButtonDrafts.map((item) => [item.key, item.label]))
}

function replyTemplatePayload() {
  return Object.fromEntries(replyTemplates.map((item) => [item.key, item.text]))
}

async function syncCommands() {
  commandBusy.value = true
  try {
    await saveConfig()
    const result = await api.syncBotCommands()
    ui.toast({ title: '指令菜单已同步', message: `已同步 ${result.commands.length} 个 Telegram 指令。`, tone: 'success' })
  } catch (err) {
    ui.toast({ title: '同步失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    commandBusy.value = false
  }
}

async function setupWebhook() {
  webhookBusy.value = true
  try {
    await saveConfig()
    const result = await api.setupBotWebhook(form.webhook_url)
    applyConfig(result.config)
    webhookStatusText.value = result.message
    ui.toast({ title: result.connected ? 'Webhook 连接成功' : 'Webhook 连接不成功', message: result.message, tone: result.connected ? 'success' : 'error' })
  } catch (err) {
    ui.toast({ title: 'Webhook 设置失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    webhookBusy.value = false
  }
}

async function checkWebhook() {
  webhookBusy.value = true
  try {
    const info = await api.botWebhookStatus()
    webhookStatusText.value = `${info.message}；待处理：${info.pending_update_count ?? 0}`
    if (info.webhook_url) {
      config.value.webhook_url = info.webhook_url
    }
    config.value.last_webhook_status = info.connected ? 'success' : 'failed'
    config.value.last_webhook_message = info.message
    ui.toast({ title: info.connected ? 'Webhook 连接成功' : 'Webhook 连接不成功', message: webhookStatusText.value, tone: info.connected ? 'success' : 'error' })
  } catch (err) {
    ui.toast({ title: '读取失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    webhookBusy.value = false
  }
}

async function clearWebhook() {
  webhookBusy.value = true
  try {
    const result = await api.clearBotWebhook()
    applyConfig(result.config)
    webhookStatusText.value = result.message
    ui.toast({ title: 'Webhook 已关闭', message: '现在可以使用本地轮询测试模式。', tone: 'success' })
  } catch (err) {
    ui.toast({ title: '关闭失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    webhookBusy.value = false
  }
}

async function loadPollingStatus() {
  pollingBusy.value = true
  try {
    pollingStatus.value = await api.botPollingStatus()
  } catch (err) {
    ui.toast({ title: '轮询状态读取失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    pollingBusy.value = false
  }
}

async function startPolling() {
  pollingBusy.value = true
  try {
    await saveConfig()
    await api.startBotPolling()
    pollingStatus.value = await api.botPollingStatus()
    ui.toast({ title: '本地轮询已启动', message: '现在可以直接在 Telegram 给机器人发送 /start 测试。', tone: 'success' })
  } catch (err) {
    ui.toast({ title: '启动轮询失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    pollingBusy.value = false
  }
}

async function stopPolling() {
  pollingBusy.value = true
  try {
    await api.stopBotPolling()
    pollingStatus.value = await api.botPollingStatus()
    ui.toast({ title: '本地轮询已停止', message: 'Webhook 配置不受影响。', tone: 'success' })
  } catch (err) {
    ui.toast({ title: '停止轮询失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    pollingBusy.value = false
  }
}

async function testBot() {
  await saveConfig()
  testing.value = true
  try {
    const result = await api.testBotConfig()
    applyConfig(result.config)
    ui.toast({ title: 'Bot 连接正常', message: result.message, tone: 'success' })
  } catch (err) {
    await loadAll()
    ui.toast({ title: 'Bot 测试失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    testing.value = false
  }
}

async function startBot() {
  botBusy.value = true
  try {
    const result = await api.startBotPush()
    applyConfig(result.config)
    ui.toast({ title: 'Bot 推送已启动', message: '监听命中后会按任务配置推送。', tone: 'success' })
  } catch (err) {
    ui.toast({ title: '启动失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    botBusy.value = false
  }
}

async function stopBot() {
  botBusy.value = true
  try {
    const result = await api.stopBotPush()
    applyConfig(result.config)
    ui.toast({ title: 'Bot 推送已停止', message: '配置仍保留，可随时重新启动。', tone: 'success' })
  } catch (err) {
    ui.toast({ title: '停止失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    botBusy.value = false
  }
}

function formatDateTime(value?: string | null) {
  if (!value) return '--'
  return new Date(value).toLocaleString()
}

onMounted(loadAll)
</script>

<style scoped>
.bot-settings-shell {
  min-height: calc(100vh - 8.5rem);
}

.eyebrow {
  color: rgba(147, 164, 198, 0.9);
  font-size: 0.68rem;
  font-weight: 900;
  letter-spacing: 0;
}

.bot-command-center {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 1rem;
  margin-bottom: 1rem;
}

.module-jumpbar {
  position: sticky;
  top: 0.75rem;
  z-index: 3;
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
  margin-bottom: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.72);
  padding: 0.65rem;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
}

.module-chip {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.045);
  color: rgba(226, 232, 240, 0.9);
  padding: 0.58rem 0.78rem;
  font-size: 0.82rem;
  font-weight: 900;
  line-height: 1;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.module-chip:hover {
  border-color: rgba(34, 197, 94, 0.42);
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.16), rgba(79, 172, 254, 0.09));
  color: white;
  transform: translateY(-1px);
}

.module-chip.active {
  border-color: rgba(0, 242, 254, 0.44);
  background: linear-gradient(135deg, rgba(0, 242, 254, 0.2), rgba(79, 172, 254, 0.12));
  color: #d8fbff;
  box-shadow: 0 4px 15px rgba(0, 242, 254, 0.28);
}

.overview-card {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  min-height: 7.2rem;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.075), rgba(255, 255, 255, 0.025)),
    rgba(15, 23, 42, 0.58);
}

.overview-card > span {
  display: grid;
  width: 2.65rem;
  height: 2.65rem;
  place-items: center;
  border-radius: 8px;
  background: rgba(148, 163, 184, 0.16);
  color: #cbd5e1;
  font-weight: 900;
}

.overview-card[data-tone='success'] > span {
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.28), rgba(16, 185, 129, 0.12));
  color: #86efac;
}

.overview-card[data-tone='warning'] > span {
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.28), rgba(251, 191, 36, 0.1));
  color: #fbbf24;
}

.overview-card[data-tone='info'] > span {
  background: linear-gradient(135deg, rgba(0, 242, 254, 0.22), rgba(79, 172, 254, 0.12));
  color: #67e8f9;
}

.overview-card[data-tone='pink'] > span {
  background: linear-gradient(135deg, rgba(244, 114, 182, 0.24), rgba(168, 85, 247, 0.12));
  color: #f9a8d4;
}

.overview-card small,
.overview-card em {
  display: block;
  color: var(--app-text-muted);
  font-style: normal;
  font-size: 0.76rem;
}

.overview-card strong {
  display: block;
  margin: 0.18rem 0;
  color: white;
  font-size: 1.12rem;
  font-weight: 950;
}

.bot-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 1rem;
}

.bot-panel {
  display: flex;
  min-height: 16rem;
  flex-direction: column;
  gap: 1rem;
}

.integration-panel {
  grid-column: 1 / -1;
  min-height: auto;
  background:
    radial-gradient(circle at 8% 12%, rgba(34, 197, 94, 0.14), transparent 28%),
    radial-gradient(circle at 88% 0%, rgba(244, 114, 182, 0.12), transparent 30%),
    rgba(15, 23, 42, 0.62);
}

.module-section-title {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 0.85rem;
  margin-top: 0.4rem;
  padding: 0.25rem 0.15rem;
}

.module-section-title > span {
  border: 1px solid rgba(0, 242, 254, 0.2);
  border-radius: 999px;
  background: rgba(0, 242, 254, 0.08);
  color: #67e8f9;
  padding: 0.25rem 0.58rem;
  font-size: 0.66rem;
  font-weight: 950;
  letter-spacing: 0;
}

.module-section-title strong {
  display: block;
  color: white;
  font-size: 1rem;
  font-weight: 950;
}

.module-section-title small {
  display: block;
  margin-top: 0.14rem;
  color: var(--app-text-muted);
  font-size: 0.78rem;
}

.integration-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.integration-badges {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.55rem;
}

.integration-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: 1rem;
}

.integration-cardlet {
  display: flex;
  min-width: 0;
  min-height: 14rem;
  flex-direction: column;
  gap: 0.85rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.16);
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.02)),
    rgba(2, 6, 23, 0.28);
  padding: 1rem;
}

.main-connection {
  background:
    linear-gradient(135deg, rgba(34, 197, 94, 0.12), rgba(79, 172, 254, 0.06)),
    rgba(2, 6, 23, 0.25);
}

.cardlet-title {
  display: flex;
  align-items: center;
  gap: 0.72rem;
}

.cardlet-title > span {
  display: grid;
  width: 2.1rem;
  height: 2.1rem;
  place-items: center;
  border-radius: 8px;
  background: rgba(0, 242, 254, 0.13);
  color: #67e8f9;
  font-size: 0.78rem;
  font-weight: 950;
}

.cardlet-title strong {
  display: block;
  color: white;
  font-weight: 950;
}

.cardlet-title small {
  display: block;
  margin-top: 0.18rem;
  color: var(--app-text-muted);
  font-size: 0.74rem;
}

.integration-form {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.wide-field {
  grid-column: 1 / -1;
}

.compact-actions {
  gap: 0.52rem;
}

.compact-actions :deep(button) {
  min-height: 2.45rem;
}

.compact-result {
  margin-top: auto;
}

.polling-metrics {
  display: grid;
  gap: 0.62rem;
}

.polling-metrics > div {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.7rem 0.8rem;
}

.polling-metrics span {
  display: block;
  color: var(--app-text-muted);
  font-size: 0.72rem;
  font-weight: 800;
}

.polling-metrics strong {
  display: block;
  margin-top: 0.18rem;
  color: white;
}

.policy-panel {
  min-height: auto;
}

.subscribers-panel {
  grid-column: 1 / -1;
}

.visual-bot-panel {
  grid-column: span 12;
  min-height: 26rem;
}

.command-panel {
  grid-column: span 8;
}

.bot-copy-panel {
  grid-column: span 12;
}

.policy-panel {
  grid-column: span 4;
}

.panel-title {
  display: flex;
  align-items: flex-start;
  gap: 0.8rem;
}

.panel-title > span {
  display: grid;
  width: 2.4rem;
  height: 2.4rem;
  place-items: center;
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.2), rgba(244, 114, 182, 0.14));
}

.panel-title h2 {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 900;
}

.panel-title p {
  margin: 0.25rem 0 0;
  color: var(--app-text-muted);
  font-size: 0.82rem;
  line-height: 1.55;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.8rem;
}

.form-grid label,
.block-field {
  display: grid;
  gap: 0.42rem;
}

.form-grid span,
.block-field span {
  color: var(--app-text-muted);
  font-size: 0.76rem;
  font-weight: 800;
}

.support-contact-field small,
.block-field small {
  color: var(--app-text-muted);
  font-size: 0.72rem;
  line-height: 1.45;
}

.form-grid input,
.form-grid select,
.menu-label-grid input,
.block-field input,
.block-field select,
.block-field textarea {
  min-height: 2.65rem;
  border-radius: 8px;
  padding: 0.75rem 0.85rem;
}

.block-field textarea {
  min-height: 8rem;
  resize: vertical;
}

.trial-control {
  display: grid;
  gap: 0.75rem;
}

.policy-execution-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.85rem;
}

.policy-editor {
  display: grid;
  gap: 0.85rem;
}

.policy-status-card {
  display: grid;
  gap: 0.75rem;
  border: 1px solid rgba(0, 242, 254, 0.16);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(0, 242, 254, 0.08), rgba(34, 197, 94, 0.08));
  padding: 0.85rem;
}

.policy-status-card > strong {
  color: white;
  font-size: 0.95rem;
  font-weight: 950;
}

.policy-status-card > p {
  margin: 0;
  color: var(--app-text-muted);
  font-size: 0.78rem;
  line-height: 1.5;
}

.policy-status-list {
  display: grid;
  gap: 0.55rem;
}

.policy-status-list > div {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: start;
  gap: 0.55rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(2, 6, 23, 0.26);
  padding: 0.62rem;
}

.policy-status-list span {
  border-radius: 999px;
  padding: 0.22rem 0.48rem;
  font-size: 0.66rem;
  font-weight: 950;
}

.policy-status-list span[data-tone='success'] {
  background: rgba(34, 197, 94, 0.16);
  color: #86efac;
}

.policy-status-list span[data-tone='warning'] {
  background: rgba(245, 158, 11, 0.16);
  color: #fbbf24;
}

.policy-status-list b {
  display: block;
  color: white;
  font-size: 0.82rem;
}

.policy-status-list small {
  display: block;
  margin-top: 0.15rem;
  color: var(--app-text-muted);
  font-size: 0.72rem;
  line-height: 1.45;
}

.trial-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.13), rgba(99, 102, 241, 0.09));
  padding: 0.9rem;
}

.trial-switch strong {
  display: block;
  color: white;
  font-size: 0.95rem;
}

.trial-switch div > span {
  display: block;
  margin-top: 0.3rem;
  color: var(--app-text-muted);
  font-size: 0.78rem;
}

.switch {
  display: inline-flex;
  cursor: pointer;
}

.switch input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.switch span {
  position: relative;
  width: 3.2rem;
  height: 1.7rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.28);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.switch span::after {
  position: absolute;
  top: 0.24rem;
  left: 0.25rem;
  width: 1.22rem;
  height: 1.22rem;
  border-radius: 50%;
  background: white;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.28);
  content: '';
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.switch input:checked + span {
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.95), rgba(16, 185, 129, 0.85));
  box-shadow: 0 0 18px rgba(34, 197, 94, 0.28);
}

.switch input:checked + span::after {
  transform: translateX(1.48rem);
}

.trial-duration {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.75rem;
}

.trial-duration label {
  display: grid;
  gap: 0.42rem;
}

.trial-duration span {
  color: var(--app-text-muted);
  font-size: 0.76rem;
  font-weight: 800;
}

.trial-duration input,
.trial-duration select {
  min-height: 2.65rem;
  border-radius: 8px;
  padding: 0.75rem 0.85rem;
}

.command-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
}

.command-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.7rem;
}

.command-option {
  display: grid;
  gap: 0.35rem;
  min-height: 9.2rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.8rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.command-option:hover,
.command-option.active {
  border-color: rgba(34, 197, 94, 0.38);
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.13), rgba(79, 172, 254, 0.08));
  transform: translateY(-1px);
}

.command-check {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.45rem;
  align-items: center;
}

.command-check input {
  width: 1rem;
  height: 1rem;
}

.command-title {
  color: white;
  font-weight: 900;
}

.command-option code {
  grid-column: 1 / -1;
  width: fit-content;
  border-radius: 999px;
  background: rgba(0, 242, 254, 0.1);
  color: #67e8f9;
  padding: 0.24rem 0.5rem;
  font-size: 0.76rem;
}

.command-name-input {
  width: 100%;
  min-height: 2.35rem;
  border-radius: 8px;
  padding: 0.62rem 0.72rem;
}

.visual-command-editor {
  display: grid;
  gap: 0.75rem;
  border: 1px solid rgba(79, 172, 254, 0.18);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(0, 242, 254, 0.08), rgba(244, 114, 182, 0.06));
  padding: 0.85rem;
}

.compact-field textarea {
  min-height: 4rem;
}

.compact-field input {
  min-height: 2.65rem;
  border-radius: 8px;
  padding: 0.75rem 0.85rem;
}

.command-option small {
  color: var(--app-text-muted);
  line-height: 1.45;
}

.button-copy-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.75rem;
}

.copy-item,
.reply-template {
  display: grid;
  gap: 0.42rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.8rem;
}

.copy-item span,
.reply-template span {
  color: white;
  font-size: 0.82rem;
  font-weight: 900;
}

.copy-item input,
.reply-template textarea {
  min-height: 2.55rem;
  border-radius: 8px;
  padding: 0.72rem 0.82rem;
}

.copy-item small {
  color: var(--app-text-muted);
  font-size: 0.74rem;
  line-height: 1.45;
}

.reply-template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.75rem;
}

.reply-template textarea {
  min-height: 7.2rem;
  resize: vertical;
}

.visual-editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(24rem, 0.85fr);
  gap: 1rem;
  align-items: stretch;
}

.bot-copy-editor {
  display: grid;
  gap: 0.8rem;
}

.menu-label-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.65rem;
}

.menu-label-grid label {
  display: grid;
  gap: 0.42rem;
}

.menu-label-grid span {
  color: var(--app-text-muted);
  font-size: 0.76rem;
  font-weight: 800;
}

.bot-phone-preview {
  display: grid;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(231, 245, 255, 0.98)),
    radial-gradient(circle at 20% 10%, rgba(34, 197, 94, 0.22), transparent 38%);
  box-shadow: inset 0 0 0 8px rgba(6, 12, 24, 0.85), 0 20px 42px rgba(0, 0, 0, 0.32);
  color: #0f172a;
  grid-template-rows: auto minmax(18rem, 1fr) auto auto;
  width: min(430px, 100%);
  aspect-ratio: 430 / 932;
  min-height: 0;
  justify-self: center;
  padding: 0.85rem;
}

.phone-top {
  display: grid;
  justify-items: center;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08);
  padding: 0.6rem 0.5rem 0.75rem;
  position: relative;
}

.phone-top > span {
  width: 5.6rem;
  height: 1.35rem;
  border-radius: 999px;
  background: #050816;
  margin-bottom: 0.35rem;
}

.phone-top strong {
  font-size: 0.9rem;
}

.phone-top small {
  color: rgba(15, 23, 42, 0.52);
  font-weight: 700;
}

.chat-preview {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  overflow: auto;
  padding: 1rem 0.45rem;
}

.bubble {
  max-width: 88%;
  border-radius: 8px;
  padding: 0.75rem 0.85rem;
  font-weight: 800;
  line-height: 1.55;
}

.bubble pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
}

.bot-bubble {
  align-self: flex-start;
  background: white;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.1);
}

.bot-bubble.compact {
  font-size: 0.82rem;
}

.user-bubble {
  align-self: flex-end;
  background: #9bec6f;
}

.keyboard-preview {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
  border-top: 1px solid rgba(15, 23, 42, 0.08);
  padding-top: 0.75rem;
}

.keyboard-preview button {
  min-height: 2.8rem;
  border: 0;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.12);
  color: #0f172a;
  font-weight: 900;
}

.placeholder-preview {
  margin-top: 0.55rem;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.08);
  color: rgba(15, 23, 42, 0.52);
  padding: 0.65rem 0.85rem;
  font-size: 0.78rem;
}

.bot-actions,
.feature-row,
.license-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.65rem;
}

.feature-row label {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  padding: 0.75rem 0.9rem;
}

.force-join-box {
  display: grid;
  gap: 0.65rem;
  border: 1px solid rgba(244, 114, 182, 0.22);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(244, 114, 182, 0.11), rgba(34, 197, 94, 0.08));
  padding: 0.85rem;
}

.force-join-box label {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  color: white;
  font-weight: 900;
}

.force-join-box input:not([type='checkbox']) {
  min-height: 2.65rem;
  border-radius: 8px;
  padding: 0.75rem 0.85rem;
}

.force-join-box small {
  color: var(--app-text-muted);
  line-height: 1.5;
}

.test-result,
.license-card {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  padding: 0.9rem;
}

.test-result strong,
.license-card strong {
  display: block;
  color: white;
  font-weight: 900;
}

.test-result span,
.license-card span {
  display: block;
  margin-top: 0.35rem;
  color: var(--app-text-muted);
  font-size: 0.82rem;
  line-height: 1.55;
}

.license-list {
  display: grid;
  gap: 0.55rem;
  overflow: auto;
}

.license-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.55rem;
}

.license-form input {
  min-width: 0;
  border-radius: 8px;
  padding: 0.7rem 0.8rem;
}

.license-list {
  max-height: 18rem;
}

.license-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.license-actions button {
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.05);
  color: white;
  padding: 0.55rem 0.7rem;
}

.subscriber-table {
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
}

.subscriber-table td span:not(.status-pill) {
  display: block;
  color: var(--app-text-muted);
  font-size: 0.76rem;
}

.empty-line {
  border: 1px dashed rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  padding: 1rem;
  color: var(--app-text-muted);
  text-align: center;
}

@media (max-width: 1280px) {
  .bot-grid {
    grid-template-columns: 1fr;
  }

  .integration-header {
    flex-direction: column;
  }

  .integration-badges {
    justify-content: flex-start;
  }

  .integration-grid {
    grid-template-columns: 1fr;
  }

  .visual-bot-panel {
    grid-column: 1;
  }

  .command-panel,
  .bot-copy-panel {
    grid-column: 1;
  }

  .visual-editor-grid,
  .bot-command-center,
  .button-copy-grid,
  .reply-template-grid,
  .menu-label-grid,
  .command-grid,
  .trial-duration,
  .license-form,
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
