import { request } from '../request'
import type {
  BotConfig,
  BotLicense,
  BotPollingStatus,
  BotSubscriber,
  BotUserDashboardDetail,
  BotUserDashboardItem
} from '../types'

export const botApi = {
  botConfig: () => request<BotConfig>('/api/v1/bot/config'),
  updateBotConfig: (payload: {
    name: string
    token: string
    push_chat_id: string
    admin_chat_id: string
    admin_contact?: string
    enabled: boolean
    force_join_enabled?: boolean
    force_join_url?: string
    trial_enabled: boolean
    trial_hours: number
    trial_features: string[]
    enabled_commands: string[]
    command_labels?: Record<string, string>
    default_keywords: string[]
    default_keyword_limit?: number
    default_match_mode: 'fuzzy' | 'exact'
    private_terminal_ids: string[]
    welcome_title?: string
    service_overview?: string
    quick_start_text?: string
    faq_text?: string
    support_text?: string
    menu_info_label?: string
    menu_settings_label?: string
    menu_faq_label?: string
    menu_support_label?: string
    menu_placeholder?: string
    button_labels?: Record<string, string>
    reply_templates?: Record<string, string>
    default_dm_messages?: string[]
    dm_min_delay_seconds?: number
    dm_max_delay_seconds?: number
    dm_max_messages?: number
    webhook_url?: string
  }) => request<BotConfig>('/api/v1/bot/config', { method: 'PUT', body: JSON.stringify(payload) }),
  testBotConfig: () => request<{ status: string; username: string; message: string; config: BotConfig }>('/api/v1/bot/test', { method: 'POST', body: JSON.stringify({}) }),
  startBotPush: () => request<{ status: string; config: BotConfig }>('/api/v1/bot/start', { method: 'POST', body: JSON.stringify({}) }),
  stopBotPush: () => request<{ status: string; config: BotConfig }>('/api/v1/bot/stop', { method: 'POST', body: JSON.stringify({}) }),
  syncBotCommands: () => request<{ status: string; commands: Array<{ command: string; description: string }> }>('/api/v1/bot/commands/sync', { method: 'POST', body: JSON.stringify({}) }),
  setupBotWebhook: (webhookUrl: string) =>
    request<{ status: string; connected: boolean; webhook_url: string; message: string; config: BotConfig }>('/api/v1/bot/webhook/setup', { method: 'POST', body: JSON.stringify({ webhook_url: webhookUrl }) }),
  clearBotWebhook: () => request<{ status: string; message: string; config: BotConfig }>('/api/v1/bot/webhook/clear', { method: 'POST', body: JSON.stringify({}) }),
  botWebhookStatus: () => request<{ connected: boolean; status: string; message: string; webhook_url: string; pending_update_count: number; telegram?: Record<string, unknown> }>('/api/v1/bot/webhook/status'),
  botPollingStatus: () => request<BotPollingStatus>('/api/v1/bot/polling'),
  startBotPolling: () => request<{ status: string; started_at: string }>('/api/v1/bot/polling/start', { method: 'POST', body: JSON.stringify({}) }),
  stopBotPolling: () => request<{ status: string }>('/api/v1/bot/polling/stop', { method: 'POST', body: JSON.stringify({}) }),
  botLicenses: () => request<BotLicense[]>('/api/v1/bot/licenses'),
  createBotLicenses: (payload: { count: number; duration_hour: number; max_bind: number }) =>
    request<BotLicense[]>('/api/v1/bot/licenses', { method: 'POST', body: JSON.stringify(payload) }),
  updateBotLicenseStatus: (id: string, status: string) =>
    request<BotLicense>(`/api/v1/bot/licenses/${id}/status`, { method: 'PUT', body: JSON.stringify({ status }) }),
  deleteBotLicense: (id: string) => request<{ deleted: string }>(`/api/v1/bot/licenses/${id}`, { method: 'DELETE' }),
  botSubscribers: () => request<BotSubscriber[]>('/api/v1/bot/subscribers'),
  botUsers: () => request<BotUserDashboardItem[]>('/api/v1/bot-users'),
  botUserDetail: (id: string) => request<BotUserDashboardDetail>(`/api/v1/bot-users/${id}`),
  updateBotUser: (id: string, payload: Record<string, unknown>) =>
    request<{ id: string }>(`/api/v1/bot-users/${id}`, { method: 'PUT', body: JSON.stringify(payload) })
}
