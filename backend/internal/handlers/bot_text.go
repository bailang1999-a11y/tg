package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codex3/backend/internal/models"

	"github.com/gin-gonic/gin"
)

func parseBotCommand(text string) (string, string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", ""
	}
	command := strings.ToLower(parts[0])
	if idx := strings.Index(command, "@"); idx >= 0 {
		command = command[:idx]
	}
	args := strings.TrimSpace(strings.TrimPrefix(text, parts[0]))
	return command, args
}

func botHelpText() string {
	lines := []string{"可用指令："}
	for _, item := range botCommandCatalog() {
		lines = append(lines, "/"+item.Command+" "+item.Description)
	}
	return strings.Join(lines, "\n")
}

func botWelcomeText(config models.BotConfig, statusText string) string {
	if custom := strings.TrimSpace(botReplyTemplate(config, "welcome")); custom != "" && custom != defaultBotReplyTemplates()["welcome"] {
		return strings.Join([]string{custom, "", statusText}, "\n")
	}
	return strings.Join([]string{
		firstNonEmpty(strings.TrimSpace(config.WelcomeTitle), "欢迎使用 Codex3 监听机器人"),
		"",
		"服务概述",
		firstNonEmpty(strings.TrimSpace(config.ServiceOverview), "监听目标群组中的关键词命中，并把线索实时汇聚到你的收件箱。"),
		"",
		"立即开始",
		firstNonEmpty(strings.TrimSpace(config.QuickStartText), "点击下方菜单进入设置中心，或发送 /keywords 查看当前关键词。"),
		"",
		statusText,
	}, "\n")
}

func botSettingsText(config models.BotConfig) string {
	enabled := map[string]struct{}{}
	for _, key := range botEnabledCommandKeys(config) {
		enabled[key] = struct{}{}
	}
	lines := []string{"设置中心", ""}
	for _, item := range botCommandCatalog() {
		if item.SettingsLine == "" {
			continue
		}
		if _, ok := enabled[item.Key]; ok {
			lines = append(lines, item.SettingsLine)
		}
	}
	if len(lines) == 2 {
		lines = append(lines, "当前没有开放可自助操作的功能命令。")
	}
	return strings.Join(lines, "\n")
}

func defaultBotFAQText() string {
	return strings.Join([]string{
		"常见问题",
		"",
		"1. 没有命中怎么办？",
		"确认监听任务已启动，并且目标池分组里有真实群组。",
		"",
		"2. 试用用户能用什么？",
		"试用用户仅可使用关键词监听功能。",
		"",
		"3. 如何继续使用？",
		"发送 /activate 卡密 激活正式权限。",
	}, "\n")
}

func botFAQText(config models.BotConfig) string {
	return firstNonEmpty(strings.TrimSpace(config.FAQText), defaultBotFAQText())
}

func botSupportText(config models.BotConfig) string {
	if strings.TrimSpace(config.SupportText) != "" {
		return strings.TrimSpace(config.SupportText) + botAdminContactSuffix(config)
	}
	if line := botAdminContactLine(config); line != "" {
		return "在线客服\n\n" + line
	}
	return "在线客服\n\n管理员暂未配置客服联系方式，请稍后再试。"
}

func botMainKeyboard(config models.BotConfig) gin.H {
	return gin.H{
		"keyboard": [][]gin.H{
			{
				{"text": botMenuInfoLabel(config)},
				{"text": botMenuSettingsLabel(config)},
			},
			{
				{"text": botMenuFAQLabel(config)},
				{"text": botMenuSupportLabel(config)},
			},
		},
		"resize_keyboard":         true,
		"one_time_keyboard":       false,
		"is_persistent":           true,
		"input_field_placeholder": firstNonEmpty(strings.TrimSpace(config.MenuPlaceholder), "选择功能或输入命令..."),
		"selective":               false,
	}
}

func botSettingsPanelText(subscriber models.BotSubscriber) string {
	quotaRemain := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if quotaRemain < 0 {
		quotaRemain = 0
	}
	return strings.Join([]string{
		"⚙️ 设置中心",
		"",
		fmt.Sprintf("私信额度：%d / %d", quotaRemain, subscriber.DMQuotaTotal),
		"点击下方按钮可实时同步到后台。",
	}, "\n")
}

func botSettingsSummary(_ models.BotConfig, subscriber models.BotSubscriber, joinStatus string) string {
	return strings.Join([]string{
		"🔄 状态已刷新",
		"",
		fmt.Sprintf("加群状态：%s", joinStatus),
		fmt.Sprintf("默认匹配：%s", firstNonEmpty(subscriber.MatchMode, "fuzzy")),
		fmt.Sprintf("账号状态：%s", firstNonEmpty(subscriber.Status, "unknown")),
	}, "\n")
}

func boolText(value bool) string {
	if value {
		return "开启"
	}
	return "关闭"
}

type botSettingsViewState struct {
	PrivateDMActive bool
	ListenerRunning bool
}

func (s *Server) botSettingsMarkup(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) gin.H {
	return botSettingsInlineKeyboard(config, subscriber, s.botSettingsState(ctx, config, subscriber))
}

func (s *Server) botSettingsState(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) botSettingsViewState {
	var activeCount int64
	_ = s.db.WithContext(ctx).
		Model(&models.BotDMTask{}).
		Where("tenant_id = ? AND subscriber_id = ? AND status = ?", config.TenantID, subscriber.ID, "active").
		Count(&activeCount).Error
	s.listenerMu.Lock()
	runtime := s.listeners[scrmListenerRuntimeKey(config.TenantID, subscriber.ID)]
	s.listenerMu.Unlock()
	return botSettingsViewState{
		PrivateDMActive: activeCount > 0,
		ListenerRunning: runtime != nil && runtime.activeWorkers.Load() > 0 && listenerRuntimeBelongsToSubscriber(runtime, subscriber.ID),
	}
}

func botSettingsInlineKeyboard(config models.BotConfig, subscriber models.BotSubscriber, state botSettingsViewState) gin.H {
	dedup := subscriber.MessageDedupMinutes
	enabled := map[string]bool{}
	for _, key := range botEnabledCommandKeys(config) {
		enabled[key] = true
	}
	rows := [][]gin.H{
		{{"text": "🔄 刷新状态", "callback_data": "cfg:refresh"}},
	}
	if enabled["setkeywords"] || enabled["keywords"] {
		rows = append(rows, []gin.H{{"text": botButtonLabel(config, "listen_keywords"), "callback_data": "keywords:set"}})
	}
	if enabled["listen"] {
		rows = append(rows, []gin.H{{"text": selectedLabel(!state.ListenerRunning, botButtonLabel(config, "listen_pause")), "callback_data": "listen:pause"}, {"text": selectedLabel(state.ListenerRunning, botButtonLabel(config, "listen_start")), "callback_data": "listen:start"}})
	}
	if enabled["setpush"] {
		rows = append(rows,
			[]gin.H{{"text": "🔔 推送状态", "callback_data": "noop"}, {"text": selectedLabel(subscriber.PushEnabled, "开启"), "callback_data": "cfg:push:on"}, {"text": selectedLabel(!subscriber.PushEnabled, "关闭"), "callback_data": "cfg:push:off"}},
			[]gin.H{{"text": "📡 推送位置", "callback_data": "push:setup"}},
			[]gin.H{{"text": botPushChannelText(config, subscriber), "callback_data": "push:setup"}},
		)
	}
	if enabled["match"] {
		rows = append(rows, []gin.H{{"text": "🎯 关键词匹配", "callback_data": "noop"}, {"text": selectedLabel(subscriber.MatchMode == "exact", "开启精准"), "callback_data": "cfg:match:exact"}, {"text": selectedLabel(subscriber.MatchMode != "exact", "开启模糊"), "callback_data": "cfg:match:fuzzy"}})
	}
	if enabled["setpush"] || enabled["inbox"] {
		rows = append(rows,
			[]gin.H{{"text": "🚫 用户黑名单", "callback_data": "noop"}, {"text": selectedLabel(subscriber.UserBlacklistEnabled, "开启"), "callback_data": "cfg:userblack:on"}, {"text": selectedLabel(!subscriber.UserBlacklistEnabled, "关闭"), "callback_data": "cfg:userblack:off"}},
			[]gin.H{{"text": "🧠 AI 过滤", "callback_data": "noop"}, {"text": selectedLabel(subscriber.RiskControlEnabled, "开启"), "callback_data": "cfg:risk:on"}, {"text": selectedLabel(!subscriber.RiskControlEnabled, "关闭"), "callback_data": "cfg:risk:off"}},
			[]gin.H{{"text": "🧹 消息去重（同一用户多久内不重复推送）", "callback_data": "noop"}},
			[]gin.H{{"text": selectedLabel(dedup == 20, "20"), "callback_data": "cfg:dedup:20"}, {"text": selectedLabel(dedup == 60, "60"), "callback_data": "cfg:dedup:60"}, {"text": selectedLabel(dedup == 120, "120"), "callback_data": "cfg:dedup:120"}, {"text": selectedLabel(dedup == 0, "不限制"), "callback_data": "cfg:dedup:0"}},
		)
	}
	bottom := []gin.H{}
	if enabled["status"] {
		bottom = append(bottom, gin.H{"text": "👤 我的设置", "callback_data": "profile:settings"})
	}
	if enabled["activate"] {
		bottom = append(bottom, gin.H{"text": botButtonLabel(config, "member"), "callback_data": "member:activate"})
	}
	if len(bottom) > 0 {
		rows = append(rows, bottom)
	}
	return gin.H{"inline_keyboard": rows}
}

func botAdminContactLine(config models.BotConfig) string {
	contact := strings.TrimSpace(config.AdminContact)
	if contact == "" && strings.TrimSpace(config.AdminChatID) != "" {
		contact = "Chat ID：" + strings.TrimSpace(config.AdminChatID)
	}
	if contact == "" {
		return ""
	}
	return "管理员联系方式：" + contact
}

func botAdminContactSuffix(config models.BotConfig) string {
	if line := botAdminContactLine(config); line != "" {
		return "\n\n" + line
	}
	return ""
}

func botAdminAssistText(config models.BotConfig, message string) string {
	return strings.TrimSpace(message) + botAdminContactSuffix(config)
}

func onOffLabel(active bool, positive bool) string {
	if active {
		if positive {
			return "✅ 开启"
		}
		return "✅ 关闭"
	}
	if positive {
		return "开启"
	}
	return "关闭"
}

func selectedLabel(selected bool, label string) string {
	if selected {
		return "✅ " + label
	}
	return label
}

func boolStateBadge(active bool) string {
	if active {
		return "🟢 开启"
	}
	return "🔴 关闭"
}

func matchStateBadge(exact bool) string {
	if exact {
		return "🎯 精准"
	}
	return "🎯 模糊"
}

func botBackToSettingsKeyboard() gin.H {
	return gin.H{"inline_keyboard": [][]gin.H{
		{{"text": "⬅️ 返回上一层", "callback_data": "nav:settings"}},
	}}
}

func botDMComposerNextKeyboard() gin.H {
	return gin.H{"inline_keyboard": [][]gin.H{
		{{"text": "➕ 添加下一条", "callback_data": "dm_msg:add"}, {"text": "✅ 完成编排", "callback_data": "dm_msg:done"}},
		{{"text": "⬅️ 返回上一层", "callback_data": "nav:settings"}},
	}}
}

func botDMTaskReadyKeyboard() gin.H {
	return gin.H{"inline_keyboard": [][]gin.H{
		{{"text": "▶️ 启动任务", "callback_data": "dm_task:start"}},
		{{"text": "➕ 继续添加", "callback_data": "dm_msg:add"}, {"text": "⬅️ 返回上一层", "callback_data": "nav:settings"}},
	}}
}

func botMyInfoKeyboard() gin.H {
	return gin.H{"inline_keyboard": [][]gin.H{
		{{"text": "👥 邀请详情", "callback_data": "invite:detail"}},
	}}
}

func botBackToGroupPickerKeyboard(purpose string) gin.H {
	return gin.H{"inline_keyboard": [][]gin.H{
		{{"text": "⬅️ 返回上一层", "callback_data": "nav:groups:" + normalizeBotAccountGroupPurpose(purpose)}},
	}}
}

func normalizeBotAccountGroupPurpose(purpose string) string {
	if strings.TrimSpace(purpose) == "dm" {
		return "dm"
	}
	return "upload"
}

func parseBotGroupDeleteCallback(data string) (string, string) {
	payload := strings.TrimPrefix(data, "group:delete:")
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) == 2 && (parts[0] == "upload" || parts[0] == "dm") {
		return parts[0], parts[1]
	}
	return "upload", payload
}

func (s *Server) botActiveDMTaskInfoText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) string {
	var task models.BotDMTask
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND subscriber_id = ? AND status = ?", config.TenantID, subscriber.ID, "active").
		Order("created_at desc").
		First(&task).Error; err != nil {
		return "当前没有进行中的私信任务"
	}
	messages := jsonStringSlice(task.Messages)
	return strings.Join([]string{
		"任务名称：" + task.Name,
		fmt.Sprintf("任务状态：%s", botDMTaskStatusText(task.Status)),
		fmt.Sprintf("编排消息：%d 条", len(messages)),
		fmt.Sprintf("成功发送：%d 条", task.SentCount),
		fmt.Sprintf("私信额度：%d/%d", subscriber.DMQuotaUsed, subscriber.DMQuotaTotal),
	}, "\n")
}

func (s *Server) botKeywordPromptText(config models.BotConfig, subscriber models.BotSubscriber) string {
	limit := botEffectiveKeywordLimit(config, subscriber)
	limitText := "不限制"
	if limit > 0 {
		limitText = fmt.Sprintf("%d 个", limit)
	}
	current := jsonStringSlice(subscriber.Keywords)
	currentText := "当前未设置"
	if len(current) > 0 {
		currentText = strings.Join(current, "、")
	}
	return strings.Join([]string{
		botButtonLabel(config, "listen_keywords"),
		"",
		botReplyTemplate(config, "keywords_prompt"),
		"关键词上限：" + limitText,
		"当前关键词：" + currentText,
	}, "\n")
}

func (s *Server) botProfileSettingsText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) string {
	keywords := jsonStringSlice(subscriber.Keywords)
	keywordLimit := botEffectiveKeywordLimit(config, subscriber)
	keywordQuota := fmt.Sprintf("%d/%s", len(keywords), botQuotaLimitText(keywordLimit))
	keywordText := botKeywordListText(keywords, "未设置")
	quotaRemain := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if quotaRemain < 0 {
		quotaRemain = 0
	}
	return strings.Join([]string{
		"👤 我的设置",
		"",
		botSubscriberStatusText(subscriber),
		"",
		"———— 配额使用 ————",
		fmt.Sprintf("🔑 关键词：%s", keywordQuota),
		fmt.Sprintf("私信额度：%d / %d", quotaRemain, subscriber.DMQuotaTotal),
		fmt.Sprintf("用户关键词：%s", keywordText),
		"",
		"———— 功能状态 ————",
		s.botSubscriberFeatureStatusText(ctx, config, subscriber),
	}, "\n")
}

func (s *Server) botSubscriberFeatureStatusText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) string {
	state := s.botSettingsState(ctx, config, subscriber)
	dedupText := "不限制"
	if subscriber.MessageDedupMinutes > 0 {
		dedupText = fmt.Sprintf("%d 分钟", subscriber.MessageDedupMinutes)
	}
	return strings.Join([]string{
		"监听状态：" + boolText(state.ListenerRunning),
		"私信状态：" + boolText(state.PrivateDMActive),
		"关键词匹配：" + botMatchModeText(subscriber.MatchMode),
		"用户黑名单：" + boolText(subscriber.UserBlacklistEnabled),
		"AI过滤：" + boolText(subscriber.RiskControlEnabled),
		"消息去重：" + dedupText,
	}, "\n")
}

func botKeywordListText(keywords []string, emptyText string) string {
	if len(keywords) == 0 {
		return emptyText
	}
	return strings.Join(keywords, "、")
}

func botMatchModeText(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "exact") {
		return "精准"
	}
	return "模糊"
}

func botDMTaskCreatedText(task models.BotDMTask) string {
	messages := jsonStringSlice(task.Messages)
	return fmt.Sprintf("私信任务已启动。\n\n任务名称：%s\n账号分组：%s\n消息数量：%d 条\n发送延迟：%d-%d 秒\n状态：%s\n创建时间：%s", task.Name, task.AccountGroupName, len(messages), task.MinDelaySeconds, task.MaxDelaySeconds, botDMTaskStatusText(task.Status), task.CreatedAt.Format("2006-01-02 15:04:05"))
}

func (s *Server) botDMTaskListText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) string {
	var tasks []models.BotDMTask
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND subscriber_id = ?", config.TenantID, subscriber.ID).Order("created_at desc").Limit(20).Find(&tasks).Error; err != nil || len(tasks) == 0 {
		return "当前还没有私信任务。"
	}
	currentKeywords := jsonStringSlice(subscriber.Keywords)
	keywordText := firstNonEmpty(strings.Join(currentKeywords, "、"), "未设置")
	quotaText := fmt.Sprintf("%d/%d", subscriber.DMQuotaUsed, subscriber.DMQuotaTotal)
	lines := []string{"你的私信任务："}
	for _, task := range tasks {
		end := "--"
		if task.EndedAt != nil {
			end = task.EndedAt.Format("2006-01-02 15:04:05")
		}
		messages := jsonStringSlice(task.Messages)
		messageLines := make([]string, 0, len(messages))
		for index, message := range messages {
			messageLines = append(messageLines, fmt.Sprintf("%d. %s", index+1, message))
		}
		if len(messageLines) == 0 {
			messageLines = append(messageLines, "未设置")
		}
		lines = append(lines, fmt.Sprintf("\n任务名称：%s\n创建时间：%s\n结束时间：%s\n关键词：%s\n编排消息：\n%s\n发送间隔：%d-%d 秒\n成功发送消息数量：%d\n任务状态：%s\n私信额度：%s", task.Name, task.CreatedAt.Format("2006-01-02 15:04:05"), end, keywordText, strings.Join(messageLines, "\n"), task.MinDelaySeconds, task.MaxDelaySeconds, task.SentCount, botDMTaskStatusText(task.Status), quotaText))
	}
	return strings.Join(lines, "\n")
}

func botDMTaskStatusText(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "running", "queued":
		return "进行中"
	case "completed", "done", "success":
		return "已完成"
	case "paused":
		return "已暂停"
	case "expired":
		return "已过期"
	case "failed":
		return "执行失败"
	default:
		return firstNonEmpty(status, "未知")
	}
}

func (s *Server) botInviteText(config models.BotConfig, subscriber models.BotSubscriber) string {
	link := s.botInviteLink(config, subscriber)
	return "邀请好友试用机器人\n\n你的专属邀请链接：\n" + link + "\n\n好友通过该链接进入并成功开通试用后，你的试用时间自动增加 2 小时。"
}

func (s *Server) botInviteLink(config models.BotConfig, subscriber models.BotSubscriber) string {
	code := firstNonEmpty(strings.TrimSpace(subscriber.InviteCode), subscriber.TelegramUserID)
	username := strings.TrimSpace(config.Username)
	if username == "" {
		username = "你的机器人用户名"
	}
	return "https://t.me/" + strings.TrimPrefix(username, "@") + "?start=ref_" + code
}

func (s *Server) botInviteDetailText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) string {
	var inviteCount int64
	var rewardHours int64
	_ = s.db.WithContext(ctx).Model(&models.BotReferral{}).Where("tenant_id = ? AND inviter_id = ?", config.TenantID, subscriber.ID).Count(&inviteCount).Error
	_ = s.db.WithContext(ctx).Model(&models.BotReferral{}).Where("tenant_id = ? AND inviter_id = ?", config.TenantID, subscriber.ID).Select("COALESCE(SUM(reward_hours), 0)").Scan(&rewardHours).Error
	return fmt.Sprintf("👥 邀请详情\n\n已邀请：%d 人\n累计奖励：%d 小时\n\n你的邀请链接：\n%s", inviteCount, rewardHours, s.botInviteLink(config, subscriber))
}

func mustParseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func botMenuInfoLabel(config models.BotConfig) string {
	return firstNonEmpty(strings.TrimSpace(config.MenuInfoLabel), "📋 我的信息")
}

func botMenuSettingsLabel(config models.BotConfig) string {
	return firstNonEmpty(strings.TrimSpace(config.MenuSettingsLabel), "⚙️ 设置中心")
}

func isBotSettingsCenterText(config models.BotConfig, text string) bool {
	value := strings.TrimSpace(text)
	return value == botMenuSettingsLabel(config) || value == "⚙️ 设置中心" || value == "设置中心"
}

func botMenuFAQLabel(config models.BotConfig) string {
	return firstNonEmpty(strings.TrimSpace(config.MenuFAQLabel), "❓ 常见问题")
}

func botMenuSupportLabel(config models.BotConfig) string {
	return firstNonEmpty(strings.TrimSpace(config.MenuSupportLabel), "💬 在线客服")
}

func normalizeBotMenuLabel(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeBotMatchMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "exact", "精准", "精确", "精准匹配", "精确匹配":
		return "exact"
	default:
		return "fuzzy"
	}
}

func botSubscriberCanUse(subscriber models.BotSubscriber) bool {
	return botSubscriberCanUseAt(subscriber, time.Now())
}

func botSubscriberStatusText(subscriber models.BotSubscriber) string {
	remaining := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if remaining < 0 {
		remaining = 0
	}
	lines := []string{
		"账号状态：" + botSubscriberStatusLabel(subscriber.Status),
		"权限类型：" + botSubscriberPlanLabel(subscriber.Plan),
		fmt.Sprintf("私信额度：%d / %d", remaining, subscriber.DMQuotaTotal),
	}
	if subscriber.TrialEndsAt != nil {
		lines = append(lines, "试用到期："+subscriber.TrialEndsAt.Format("2006-01-02 15:04:05"))
	}
	if subscriber.ExpiresAt != nil {
		lines = append(lines, "授权到期："+subscriber.ExpiresAt.Format("2006-01-02 15:04:05"))
	}
	if subscriber.Plan == "trial" {
		lines = append(lines, "试用限制：仅关键词监听")
	}
	return strings.Join(lines, "\n")
}

func botSubscriberStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return "正常"
	case "expired":
		return "已过期"
	case "disabled":
		return "已禁用"
	case "inactive":
		return "未激活"
	default:
		return "未知"
	}
}

func botSubscriberPlanLabel(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "trial":
		return "试用用户"
	case "license", "member":
		return "会员用户"
	case "none", "":
		return "未授权"
	default:
		return "未知"
	}
}

func botSubscriberDisplayName(subscriber models.BotSubscriber) string {
	name := firstNonEmpty(strings.TrimSpace(subscriber.Nickname), strings.TrimSpace(subscriber.Username), subscriber.TelegramUserID)
	if username := strings.TrimSpace(subscriber.Username); username != "" {
		return fmt.Sprintf("%s (@%s)", name, strings.TrimPrefix(username, "@"))
	}
	return name
}

func botSubscriberExpireText(subscriber models.BotSubscriber) string {
	if subscriber.Plan == "license" && subscriber.ExpiresAt != nil {
		return subscriber.ExpiresAt.Format("2006-01-02 15:04")
	}
	if subscriber.TrialEndsAt != nil {
		return subscriber.TrialEndsAt.Format("2006-01-02 15:04")
	}
	return "未设置"
}

func botQuotaLimitText(limit int) string {
	if limit <= 0 {
		return "不限"
	}
	return strconv.Itoa(limit)
}

func boolCheckText(value bool) string {
	if value {
		return "✅ 已开启"
	}
	return "❌ 已关闭"
}

func botPushChannelText(config models.BotConfig, subscriber models.BotSubscriber) string {
	channel := strings.TrimSpace(subscriber.PushChatID)
	if channel == "" && strings.TrimSpace(subscriber.TelegramUserID) != "" {
		return "个人 " + botSubscriberDisplayName(subscriber)
	}
	if channel == "" {
		channel = strings.TrimSpace(config.PushChatID)
	}
	if channel == "" {
		return "未设置"
	}
	if channel == subscriber.TelegramUserID {
		return "个人 " + botSubscriberDisplayName(subscriber)
	}
	if strings.HasPrefix(channel, "http://") || strings.HasPrefix(channel, "https://") || strings.HasPrefix(channel, "@") {
		return channel
	}
	if strings.HasPrefix(channel, "-") {
		return "群组/频道 " + channel
	}
	return "个人 " + channel
}

func botSubscriberPushChatID(subscriber models.BotSubscriber) string {
	return firstNonEmpty(strings.TrimSpace(subscriber.PushChatID), strings.TrimSpace(subscriber.TelegramUserID))
}

func botPushChatPromptText(config models.BotConfig, subscriber models.BotSubscriber) string {
	return strings.Join([]string{
		"📡 推送位置",
		"",
		"请发送你要接收监听推送的位置：",
		"1. 发送“个人”表示推送到当前私聊",
		"2. 公开群组/频道可发送 @用户名 或 https://t.me/用户名",
		"3. 私有群组/频道请发送 -100 开头的 Chat ID",
		"",
		"请先把机器人加入对应群组或频道，并给它发言权限。",
		"当前：" + botPushChannelText(config, subscriber),
	}, "\n")
}
