package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codex3/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func normalizeTelegramPublicHandle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "https://t.me/")
	value = strings.TrimPrefix(value, "http://t.me/")
	value = strings.TrimPrefix(value, "t.me/")
	value = strings.Trim(value, "/")
	if strings.HasPrefix(value, "+") || strings.Contains(value, "/") {
		return ""
	}
	if strings.HasPrefix(value, "@") {
		return value
	}
	return "@" + value
}

func (s *Server) sendForceJoinPromptIfNeeded(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string) (bool, error) {
	if !config.ForceJoinEnabled {
		return false, nil
	}
	joined, err := s.botSubscriberJoinedRequiredChat(ctx, config, subscriber)
	if err != nil {
		return true, sendTelegramBotMessageWithMarkup(config.Token, chatID, botAdminAssistText(config, "暂时无法校验加群状态，请稍后重试。"), forceJoinInlineKeyboard(config))
	}
	if joined {
		_ = s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]any{"force_joined": true, "updated_at": time.Now()}).Error
		return false, nil
	}
	return true, sendTelegramBotMessageWithMarkup(config.Token, chatID, forceJoinPromptText(config), forceJoinInlineKeyboard(config))
}

func forceJoinPromptText(config models.BotConfig) string {
	handle := firstNonEmpty(config.ForceJoinHandle, config.ForceJoinURL)
	return botAdminAssistText(config, fmt.Sprintf("⚠️ 请先关注/加入指定群组或频道\n\n使用此功能需要加入 %s\n加入后重新发送命令或点击刷新设置即可。", handle))
}

func forceJoinInlineKeyboard(config models.BotConfig) gin.H {
	url := strings.TrimSpace(config.ForceJoinURL)
	if url == "" && strings.TrimSpace(config.ForceJoinHandle) != "" {
		url = "https://t.me/" + strings.TrimPrefix(config.ForceJoinHandle, "@")
	}
	return gin.H{"inline_keyboard": [][]gin.H{
		{{"text": "📣 前往关注/加入", "url": url}},
		{{"text": "⬅️ 返回主菜单", "callback_data": "nav:main"}},
	}}
}

func (s *Server) botSubscriberJoinedRequiredChat(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber) (bool, error) {
	handle := firstNonEmpty(strings.TrimSpace(config.ForceJoinHandle), normalizeTelegramPublicHandle(config.ForceJoinURL))
	if handle == "" {
		return true, nil
	}
	status, err := getTelegramChatMemberStatus(config.Token, handle, subscriber.TelegramUserID)
	if err != nil {
		return false, err
	}
	switch status {
	case "creator", "administrator", "member", "restricted":
		return true, nil
	default:
		return false, nil
	}
}

func (s *Server) loadBotConversationState(ctx context.Context, subscriberID uuid.UUID) (models.BotConversationState, bool) {
	var state models.BotConversationState
	if err := s.db.WithContext(ctx).Where("subscriber_id = ?", subscriberID).Order("updated_at desc").First(&state).Error; err != nil {
		return models.BotConversationState{}, false
	}
	return state, true
}

func (s *Server) saveBotConversationState(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, state string, payload gin.H) {
	raw, _ := json.Marshal(payload)
	now := time.Now()
	var existing models.BotConversationState
	err := s.db.WithContext(ctx).Where("subscriber_id = ?", subscriber.ID).First(&existing).Error
	if err == nil {
		existing.State = state
		existing.Payload = datatypes.JSON(raw)
		existing.UpdatedAt = now
		_ = s.db.WithContext(ctx).Save(&existing).Error
		return
	}
	record := models.BotConversationState{
		ID:             uuid.New(),
		TenantID:       config.TenantID,
		SubscriberID:   subscriber.ID,
		TelegramUserID: subscriber.TelegramUserID,
		State:          state,
		Payload:        datatypes.JSON(raw),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_ = s.db.WithContext(ctx).Create(&record).Error
}

func (s *Server) clearBotConversationState(ctx context.Context, subscriberID uuid.UUID) {
	_ = s.db.WithContext(ctx).Where("subscriber_id = ?", subscriberID).Delete(&models.BotConversationState{}).Error
}

func (s *Server) handleBotStateInput(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, state models.BotConversationState, text string) error {
	payload := map[string]string{}
	_ = json.Unmarshal(state.Payload, &payload)
	switch state.State {
	case "await_keywords":
		keywords := parseBotKeywordInput(text)
		if len(keywords) == 0 {
			return sendTelegramBotMessageWithMarkup(config.Token, chatID, "没有识别到关键词，请一行一个输入，或者使用“，”分开。", botBackToSettingsKeyboard())
		}
		if limit := botEffectiveKeywordLimit(config, subscriber); limit > 0 && len(keywords) > limit {
			return sendTelegramBotMessageWithMarkup(config.Token, chatID, fmt.Sprintf("关键词最多只能设置 %d 个，当前识别到 %d 个，请减少后重新发送。", limit, len(keywords)), botBackToSettingsKeyboard())
		}
		if err := s.updateBotKeywords(ctx, subscriber, keywords); err != nil {
			return err
		}
		s.clearBotConversationState(ctx, subscriber.ID)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "监听关键词已保存：\n"+strings.Join(keywords, "\n"), botBackToSettingsKeyboard())
	case "await_push_chat":
		reply := s.setBotSubscriberPushChat(ctx, config, subscriber, chatID, text)
		s.clearBotConversationState(ctx, subscriber.ID)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, reply, botBackToSettingsKeyboard())
	case "await_activate_code":
		reply, err := s.activateBotLicense(ctx, config, &subscriber, strings.TrimSpace(text))
		if err != nil {
			return sendTelegramBotMessageWithMarkup(config.Token, chatID, botAdminAssistText(config, "卡号激活失败："+err.Error()+"\n请重新点击“开通会员”并输入正确卡号。"), botBackToSettingsKeyboard())
		}
		s.clearBotConversationState(ctx, subscriber.ID)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, reply, botBackToSettingsKeyboard())
	case "await_group_name":
		name := strings.TrimSpace(text)
		purpose := normalizeBotAccountGroupPurpose(payload["purpose"])
		if name == "" {
			return sendTelegramBotMessageWithMarkup(config.Token, chatID, "分组名称不能为空，请重新输入。", botBackToGroupPickerKeyboard(purpose))
		}
		group := models.BotPrivateAccountGroup{
			ID:           uuid.New(),
			TenantID:     config.TenantID,
			SubscriberID: subscriber.ID,
			Name:         name,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := s.db.WithContext(ctx).Create(&group).Error; err != nil {
			return err
		}
		s.clearBotConversationState(ctx, subscriber.ID)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "账号分组已创建："+name, botAccountGroupPickerKeyboard(ctx, s, config, subscriber, purpose))
	case "await_dm_cooldown":
		minutes, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || minutes < 0 {
			return sendTelegramBotMessageWithMarkup(config.Token, chatID, "请输入大于等于 0 的数字，例如 30。", botBackToGroupPickerKeyboard("dm"))
		}
		payload["cooldown"] = strconv.Itoa(minutes)
		s.saveBotConversationState(ctx, config, subscriber, "await_dm_name", stringMapToGinH(payload))
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "请为该私信任务设置名称，可以输入中文、数字或字母。", botBackToGroupPickerKeyboard("dm"))
	case "await_dm_message":
		index, _ := strconv.Atoi(payload["next_index"])
		if index <= 0 {
			index = 1
		}
		content := strings.TrimSpace(text)
		if content == "" {
			return sendTelegramBotMessageWithMarkup(config.Token, chatID, fmt.Sprintf("第 %d 条私信内容不能为空，请重新输入。", index), botBackToSettingsKeyboard())
		}
		payload[fmt.Sprintf("message_%d", index)] = content
		payload["message_count"] = strconv.Itoa(index)
		if index >= normalizeBotDMMaxMessages(config.DMMaxMessages) {
			s.saveBotConversationState(ctx, config, subscriber, "await_dm_ready", stringMapToGinH(payload))
			return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botDMTaskReadyText(ctx, config, subscriber, payload), botDMTaskReadyKeyboard())
		}
		s.saveBotConversationState(ctx, config, subscriber, "await_dm_next", stringMapToGinH(payload))
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, fmt.Sprintf("第 %d 条私信已保存。\n你可以继续添加下一条，或完成编排后直接启动任务。", index), botDMComposerNextKeyboard())
	default:
		return nil
	}
}

func stringMapToGinH(input map[string]string) gin.H {
	out := gin.H{}
	for key, value := range input {
		out[key] = value
	}
	return out
}
