package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codex3/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Server) ensureBotDefaultAccountGroup(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) (models.BotPrivateAccountGroup, error) {
	var group models.BotPrivateAccountGroup
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND subscriber_id = ? AND is_default = ?", config.TenantID, subscriber.ID, true).First(&group).Error
	if err == nil {
		return group, nil
	}
	if err != gorm.ErrRecordNotFound {
		return group, err
	}
	group = models.BotPrivateAccountGroup{
		ID:           uuid.New(),
		TenantID:     config.TenantID,
		SubscriberID: subscriber.ID,
		Name:         "全部账号",
		IsDefault:    true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return group, s.db.WithContext(ctx).Create(&group).Error
}

func (s *Server) sendBotAccountGroupPicker(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, purpose string) error {
	if normalizeBotAccountGroupPurpose(purpose) == "dm" {
		return s.sendBotTerminalGroupPicker(ctx, config, subscriber, chatID)
	}
	_, _ = s.ensureBotDefaultAccountGroup(ctx, config, subscriber)
	title := "请选择账号分组。1号默认为全部账号。"
	if purpose == "upload" {
		title = "上传前请先选择要加入的账号分组。"
	}
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, title, botAccountGroupPickerKeyboard(ctx, s, config, subscriber, purpose))
}

func (s *Server) sendBotTerminalGroupPicker(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string) error {
	remaining := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if remaining <= 0 {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botAdminAssistText(config, "当前私信额度不足，请联系管理员添加额度后再开启私信任务。"), botBackToSettingsKeyboard())
	}
	groups := s.botSubscriberAllowedTerminalGroups(ctx, config, subscriber)
	if len(groups) == 0 {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botAdminAssistText(config, "管理员尚未为你配置可用私信账号池分组，请联系管理员在 Web 后台设置。"), botBackToSettingsKeyboard())
	}
	rows := [][]gin.H{}
	for index, group := range groups {
		rows = append(rows, []gin.H{{"text": fmt.Sprintf("%d. %s", index+1, group.Name), "callback_data": "dm_group:" + group.ID.String()}})
	}
	rows = append(rows, []gin.H{{"text": "⬅️ 返回上一层", "callback_data": "nav:settings"}})
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, fmt.Sprintf("请选择管理员分配给你的私信账号池分组。\n当前可用额度：%d 条", remaining), gin.H{"inline_keyboard": rows})
}

func (s *Server) startBotDMComposer(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string) error {
	var activeCount int64
	_ = s.db.WithContext(ctx).
		Model(&models.BotDMTask{}).
		Where("tenant_id = ? AND subscriber_id = ? AND status = ?", config.TenantID, subscriber.ID, "active").
		Count(&activeCount).Error
	if activeCount > 0 {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "你当前已有一个进行中的私信任务。请先点击“暂停私信”停止当前任务，或在“任务查看”里查看详情。", botBackToSettingsKeyboard())
	}
	remaining := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if remaining <= 0 {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botAdminAssistText(config, "当前私信额度不足，请联系管理员添加额度后再开启私信任务。"), botBackToSettingsKeyboard())
	}
	groups := s.botSubscriberAllowedTerminalGroups(ctx, config, subscriber)
	if len(groups) == 0 {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botAdminAssistText(config, "管理员尚未为你配置可用私信账号池分组，请联系管理员在 Web 后台设置。"), botBackToSettingsKeyboard())
	}
	minDelay, maxDelay := normalizeBotDMDelay(config.DMMinDelaySeconds, config.DMMaxDelaySeconds)
	maxMessages := normalizeBotDMMaxMessages(config.DMMaxMessages)
	payload := gin.H{
		"group_id":   groups[0].ID.String(),
		"group_name": groups[0].Name,
		"min_delay":  strconv.Itoa(minDelay),
		"max_delay":  strconv.Itoa(maxDelay),
		"max_count":  strconv.Itoa(maxMessages),
		"next_index": "1",
	}
	s.saveBotConversationState(ctx, config, subscriber, "await_dm_message", payload)
	prompt := botReplyTemplate(config, "dm_prompt")
	if strings.TrimSpace(prompt) == "" {
		prompt = "请设置需要发送的第 1 条私信内容。"
	}
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, strings.Join([]string{
		"✉️ 开启私信",
		"",
		prompt,
		fmt.Sprintf("当前账号池：%s", groups[0].Name),
		fmt.Sprintf("消息延迟：%d-%d 秒", minDelay, maxDelay),
		fmt.Sprintf("同一个账号最多发送 %d 条消息给目标。", maxMessages),
	}, "\n"), botBackToSettingsKeyboard())
}

func (s *Server) continueBotDMComposer(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, done bool) error {
	state, ok := s.loadBotConversationState(ctx, subscriber.ID)
	if !ok || (state.State != "await_dm_next" && state.State != "await_dm_message" && state.State != "await_dm_ready") {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "当前没有进行中的私信编排，请重新点击“开启私信”。", s.botSettingsMarkup(ctx, config, subscriber))
	}
	payload := map[string]string{}
	_ = json.Unmarshal(state.Payload, &payload)
	count, _ := strconv.Atoi(payload["message_count"])
	if count <= 0 {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "请先输入第 1 条私信内容。", botBackToSettingsKeyboard())
	}
	maxMessages, _ := strconv.Atoi(payload["max_count"])
	if maxMessages <= 0 {
		maxMessages = config.DMMaxMessages
	}
	maxMessages = normalizeBotDMMaxMessages(maxMessages)
	if done || count >= maxMessages {
		s.saveBotConversationState(ctx, config, subscriber, "await_dm_ready", stringMapToGinH(payload))
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botDMTaskReadyText(ctx, config, subscriber, payload), botDMTaskReadyKeyboard())
	}
	next := count + 1
	payload["next_index"] = strconv.Itoa(next)
	s.saveBotConversationState(ctx, config, subscriber, "await_dm_message", stringMapToGinH(payload))
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, fmt.Sprintf("请设置需要发送的第 %d 条私信内容。", next), botBackToSettingsKeyboard())
}

func botAccountGroupPickerKeyboard(ctx context.Context, s *Server, config models.BotConfig, subscriber models.BotSubscriber, purpose string) gin.H {
	purpose = normalizeBotAccountGroupPurpose(purpose)
	var groups []models.BotPrivateAccountGroup
	_ = s.db.WithContext(ctx).Where("tenant_id = ? AND subscriber_id = ?", config.TenantID, subscriber.ID).Order("is_default desc, created_at asc").Find(&groups).Error
	rows := [][]gin.H{}
	for index, group := range groups {
		prefix := "upload_group:"
		if purpose == "dm" {
			prefix = "dm_group:"
		}
		rows = append(rows, []gin.H{{"text": fmt.Sprintf("%d. %s", index+1, group.Name), "callback_data": prefix + group.ID.String()}})
	}
	rows = append(rows, []gin.H{{"text": "➕ 新建分组", "callback_data": "group:new:" + purpose}})
	if len(groups) > 1 {
		deleteRow := []gin.H{}
		for _, group := range groups {
			if !group.IsDefault {
				deleteRow = append(deleteRow, gin.H{"text": "删除 " + group.Name, "callback_data": "group:delete:" + purpose + ":" + group.ID.String()})
			}
		}
		if len(deleteRow) > 0 {
			rows = append(rows, deleteRow)
		}
	}
	rows = append(rows, []gin.H{{"text": "⬅️ 返回上一层", "callback_data": "nav:settings"}})
	return gin.H{"inline_keyboard": rows}
}

func (s *Server) deleteBotPrivateAccountGroup(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, groupIDText string, purpose string) error {
	groupID, err := uuid.Parse(groupIDText)
	if err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "分组 ID 无效。", botBackToSettingsKeyboard())
	}
	var group models.BotPrivateAccountGroup
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND subscriber_id = ? AND id = ?", config.TenantID, subscriber.ID, groupID).First(&group).Error; err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "分组不存在。", botBackToSettingsKeyboard())
	}
	if group.IsDefault {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "默认全部账号分组不能删除。", botBackToGroupPickerKeyboard(purpose))
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", group.ID).Delete(&models.BotPrivateAccount{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.BotPrivateAccountGroup{}, "id = ?", group.ID).Error
	}); err != nil {
		return err
	}
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, "已删除分组及该分组下账号："+group.Name, botAccountGroupPickerKeyboard(ctx, s, config, subscriber, purpose))
}

func (s *Server) startBotDMTaskFromState(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string) error {
	state, ok := s.loadBotConversationState(ctx, subscriber.ID)
	if !ok || state.State != "await_dm_ready" {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "当前没有可启动的私信编排，请重新点击“开启私信”。", s.botSettingsMarkup(ctx, config, subscriber))
	}
	var activeCount int64
	if err := s.db.WithContext(ctx).Model(&models.BotDMTask{}).
		Where("tenant_id = ? AND subscriber_id = ? AND status = ?", config.TenantID, subscriber.ID, "active").
		Count(&activeCount).Error; err != nil {
		return err
	}
	if activeCount > 0 {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "你当前已有一个进行中的私信任务。每个 Bot 用户只能同时启动一个私信任务。", botBackToSettingsKeyboard())
	}
	payload := map[string]string{}
	_ = json.Unmarshal(state.Payload, &payload)
	task, err := s.createBotDMTaskFromState(ctx, config, subscriber, payload)
	if err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, err.Error(), botBackToSettingsKeyboard())
	}
	s.clearBotConversationState(ctx, subscriber.ID)
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, botDMTaskCreatedText(task), botBackToSettingsKeyboard())
}

func (s *Server) createBotDMTaskFromState(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, payload map[string]string) (models.BotDMTask, error) {
	remaining := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if remaining <= 0 {
		return models.BotDMTask{}, errors.New(botAdminAssistText(config, "当前私信额度不足，请联系管理员添加额度"))
	}
	groupID, _ := uuid.Parse(payload["group_id"])
	cooldown, _ := strconv.Atoi(payload["cooldown"])
	minDelay, _ := strconv.Atoi(payload["min_delay"])
	maxDelay, _ := strconv.Atoi(payload["max_delay"])
	minDelay, maxDelay = normalizeBotDMDelay(minDelay, maxDelay)
	messages := botDMMessagesFromPayload(payload)
	if len(messages) == 0 {
		messages = jsonStringSlice(config.DefaultDMMessages)
	}
	messages = normalizeBotDMMessages(messages)
	if len(messages) == 0 {
		return models.BotDMTask{}, errors.New("请至少设置 1 条私信内容")
	}
	groupName := "全部账号"
	if groupID != uuid.Nil {
		var group models.BotPrivateAccountGroup
		if err := s.db.WithContext(ctx).First(&group, "id = ? AND subscriber_id = ?", groupID, subscriber.ID).Error; err == nil {
			groupName = group.Name
		} else {
			var terminalGroup models.Group
			if err := s.db.WithContext(ctx).First(&terminalGroup, "id = ? AND tenant_id = ? AND resource_type = ?", groupID, config.TenantID, "terminal").Error; err == nil {
				groupName = terminalGroup.Name
			}
		}
	}
	keywordsJSON, _ := json.Marshal(jsonStringSlice(subscriber.Keywords))
	messagesJSON, _ := json.Marshal(messages)
	now := time.Now()
	name := "私信任务 " + now.Format("2006-01-02 15:04")
	task := models.BotDMTask{
		ID:               uuid.New(),
		TenantID:         config.TenantID,
		SubscriberID:     subscriber.ID,
		Name:             name,
		AccountGroupName: groupName,
		CooldownMinutes:  cooldown,
		Keywords:         datatypes.JSON(keywordsJSON),
		Messages:         datatypes.JSON(messagesJSON),
		MinDelaySeconds:  minDelay,
		MaxDelaySeconds:  maxDelay,
		Status:           "active",
		StartedAt:        &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if groupID != uuid.Nil {
		task.AccountGroupID = &groupID
	}
	return task, s.db.WithContext(ctx).Create(&task).Error
}

func botDMMessagesFromPayload(payload map[string]string) []string {
	messages := make([]string, 0, 10)
	maxCount, _ := strconv.Atoi(payload["max_count"])
	maxCount = normalizeBotDMMaxMessages(maxCount)
	for index := 1; index <= maxCount; index++ {
		value := strings.TrimSpace(payload[fmt.Sprintf("message_%d", index)])
		if value != "" {
			messages = append(messages, value)
		}
	}
	return messages
}

func (s *Server) botDMTaskReadyText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, payload map[string]string) string {
	minDelay, _ := strconv.Atoi(payload["min_delay"])
	maxDelay, _ := strconv.Atoi(payload["max_delay"])
	minDelay, maxDelay = normalizeBotDMDelay(minDelay, maxDelay)
	messages := normalizeBotDMMessages(botDMMessagesFromPayload(payload))
	remaining := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if remaining < 0 {
		remaining = 0
	}
	groups := s.botSubscriberAllowedTerminalGroups(ctx, config, subscriber)
	groupNames := make([]string, 0, len(groups))
	for _, group := range groups {
		groupNames = append(groupNames, group.Name)
	}
	if len(groupNames) == 0 {
		groupNames = append(groupNames, "管理员未配置")
	}
	return strings.Join([]string{
		"私信内容编排完成。",
		"",
		"任务信息：",
		"账号池：" + strings.Join(groupNames, "、"),
		fmt.Sprintf("编排消息：%d 条", len(messages)),
		fmt.Sprintf("发送间隔：%d-%d 秒", minDelay, maxDelay),
		fmt.Sprintf("私信额度：%d/%d", subscriber.DMQuotaUsed, subscriber.DMQuotaTotal),
		"启动后会自动使用管理员分配的账号池，给后续推送出的线索用户发送私信。",
	}, "\n")
}

func (s *Server) botSubscriberAllowedTerminalGroups(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) []models.Group {
	ids := jsonStringSlice(subscriber.PrivateTerminalGroupIDs)
	if len(ids) == 0 {
		return []models.Group{}
	}
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, rawID := range ids {
		id, err := uuid.Parse(strings.TrimSpace(rawID))
		if err == nil {
			parsed = append(parsed, id)
		}
	}
	if len(parsed) == 0 {
		return []models.Group{}
	}
	var groups []models.Group
	_ = s.db.WithContext(ctx).
		Where("tenant_id = ? AND resource_type = ? AND id IN ?", config.TenantID, "terminal", parsed).
		Order("created_at asc").
		Find(&groups).Error
	return groups
}

func (s *Server) botDMQuotaText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) string {
	remaining := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if remaining < 0 {
		remaining = 0
	}
	groups := s.botSubscriberAllowedTerminalGroups(ctx, config, subscriber)
	groupNames := make([]string, 0, len(groups))
	for _, group := range groups {
		groupNames = append(groupNames, group.Name)
	}
	if len(groupNames) == 0 {
		groupNames = append(groupNames, "管理员未配置")
	}
	return strings.Join([]string{
		"💬 私信额度",
		"",
		fmt.Sprintf("可用额度：%d 条", remaining),
		fmt.Sprintf("总额度：%d 条", subscriber.DMQuotaTotal),
		fmt.Sprintf("已使用：%d 条", subscriber.DMQuotaUsed),
		"可用账号池：" + strings.Join(groupNames, "、"),
		"",
		"每发送 1 条私信会扣除 1 条额度。",
	}, "\n")
}

func (s *Server) botMyInfoText(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) string {
	keywords := jsonStringSlice(subscriber.Keywords)
	keywordLimit := botEffectiveKeywordLimit(config, subscriber)
	keywordQuota := fmt.Sprintf("%d/%s", len(keywords), botQuotaLimitText(keywordLimit))
	dmQuota := fmt.Sprintf("%d/%d", clampMinInt64(subscriber.DMQuotaUsed, 0), clampMinInt64(subscriber.DMQuotaTotal, 0))
	inviteLink := s.botInviteLink(config, subscriber)
	expireText := botSubscriberExpireText(subscriber)
	statusIcon := "✅"
	if subscriber.Status != "active" {
		statusIcon = "❌"
	}
	activeTask := s.botActiveDMTaskInfoText(ctx, config, subscriber)
	return strings.Join([]string{
		fmt.Sprintf("👤 昵称：%s", botSubscriberDisplayName(subscriber)),
		fmt.Sprintf("🆔 ID：%s", firstNonEmpty(subscriber.TelegramUserID, "未记录")),
		"💰 积分：0 分",
		fmt.Sprintf("📦 订阅状态：%s %s", statusIcon, botSubscriberStatusLabel(subscriber.Status)),
		"└ 到期：" + expireText,
		"",
		"———— 配额使用 ————",
		"🔑 关键词：" + keywordQuota,
		"✉️ 私信：" + dmQuota,
		"🧾 用户关键词：" + botKeywordListText(keywords, "未设置"),
		"",
		"———— 推送设置 ————",
		"推送渠道：" + botPushChannelText(config, subscriber),
		"推送状态：" + boolCheckText(subscriber.PushEnabled),
		s.botSubscriberFeatureStatusText(ctx, config, subscriber),
		"",
		"———— 当前任务 ————",
		activeTask,
		"",
		"🔗 邀请链接：",
		inviteLink,
		"💡 邀请好友试用可获得积分奖励",
	}, "\n")
}
