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

func (s *Server) handleBotUpdate(ctx context.Context, config models.BotConfig, update telegramUpdate) error {
	if strings.TrimSpace(update.CallbackQuery.Data) != "" {
		return s.handleBotCallback(ctx, config, update)
	}
	return s.handleBotCommand(ctx, config, update)
}

func (s *Server) handleBotCommand(ctx context.Context, config models.BotConfig, update telegramUpdate) error {
	chatID := updateChatID(update)
	userID := updateUserID(update)
	text := strings.TrimSpace(update.Message.Text)
	if userID == "0" || chatID == "0" {
		return nil
	}

	subscriber, err := s.ensureBotSubscriber(ctx, config, update)
	if err != nil {
		return err
	}
	if strings.TrimSpace(update.Message.Document.FileID) != "" {
		return s.handleBotDocument(ctx, config, subscriber, update)
	}
	if text == "" {
		return nil
	}
	if isBotSettingsCenterText(config, text) {
		s.clearBotConversationState(ctx, subscriber.ID)
		if blocked, err := s.sendForceJoinPromptIfNeeded(ctx, config, subscriber, chatID); blocked || err != nil {
			return err
		}
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSettingsPanelText(subscriber), s.botSettingsMarkup(ctx, config, subscriber))
	}
	if state, ok := s.loadBotConversationState(ctx, subscriber.ID); ok && !strings.HasPrefix(text, "/") {
		return s.handleBotStateInput(ctx, config, subscriber, chatID, state, text)
	}

	switch strings.TrimSpace(text) {
	case "☰ 呼出设置面板", "呼出设置面板", "☰ 呼出菜单栏", "呼出菜单栏":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		if blocked, err := s.sendForceJoinPromptIfNeeded(ctx, config, subscriber, chatID); blocked || err != nil {
			return err
		}
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSettingsPanelText(subscriber), s.botSettingsMarkup(ctx, config, subscriber))
	case botMenuSettingsLabel(config), "⚙️ 设置中心", "设置中心":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		if blocked, err := s.sendForceJoinPromptIfNeeded(ctx, config, subscriber, chatID); blocked || err != nil {
			return err
		}
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSettingsPanelText(subscriber), s.botSettingsMarkup(ctx, config, subscriber))
	case botMenuInfoLabel(config), "📋 我的信息", "我的信息", "👤 我的", "我的":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botMyInfoText(ctx, config, subscriber), botMyInfoKeyboard())
	case botMenuFAQLabel(config), "❓ 常见问题", "常见问题":
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botFAQText(config), botMainKeyboard(config))
	case botMenuSupportLabel(config), "💬 在线客服", "在线客服":
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSupportText(config), botMainKeyboard(config))
	}

	command, args := parseBotCommand(text)
	if command != "" && !botCommandEnabled(config, command) {
		return sendTelegramBotMessage(config.Token, chatID, "该功能暂未开放，请在设置中心使用当前已开启的功能。")
	}
	switch command {
	case "/start", "/help":
		if command, args := parseBotCommand(text); command == "/start" && strings.HasPrefix(args, "ref_") {
			_ = s.applyBotReferral(ctx, config, &subscriber, strings.TrimPrefix(args, "ref_"))
		}
		reply := s.startTrialIfAvailable(ctx, config, &subscriber)
		if command == "/help" {
			reply = botHelpText() + "\n\n" + reply
		}
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botWelcomeText(config, reply), botMainKeyboard(config))
	case "/trial":
		if blocked, err := s.sendForceJoinPromptIfNeeded(ctx, config, subscriber, chatID); blocked || err != nil {
			return err
		}
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.startTrialIfAvailable(ctx, config, &subscriber), botMainKeyboard(config))
	case "/activate":
		reply, err := s.activateBotLicense(ctx, config, &subscriber, args)
		if err != nil {
			reply = err.Error()
		}
		return sendTelegramBotMessage(config.Token, chatID, reply)
	case "/status":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botMyInfoText(ctx, config, subscriber), botMyInfoKeyboard())
	case "/keywords":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessage(config.Token, chatID, s.botKeywordsText(config, subscriber))
	case "/setkeywords":
		if blocked, err := s.sendForceJoinPromptIfNeeded(ctx, config, subscriber, chatID); blocked || err != nil {
			return err
		}
		if !botSubscriberCanUse(subscriber) {
			return sendTelegramBotMessage(config.Token, chatID, "试用已过期或账号未授权，请发送 /activate 卡密 激活。")
		}
		keywords := parseBotKeywordInput(args)
		if len(keywords) == 0 {
			return sendTelegramBotMessage(config.Token, chatID, "请这样发送：\n/setkeywords 合作，请教，多少钱\n也可以换行输入多个关键词。")
		}
		if limit := botEffectiveKeywordLimit(config, subscriber); limit > 0 && len(keywords) > limit {
			return sendTelegramBotMessage(config.Token, chatID, fmt.Sprintf("关键词最多只能设置 %d 个，当前识别到 %d 个。", limit, len(keywords)))
		}
		if err := s.updateBotKeywords(ctx, subscriber, keywords); err != nil {
			return err
		}
		return sendTelegramBotMessage(config.Token, chatID, "关键词已更新：\n"+strings.Join(keywords, "\n"))
	case "/match":
		if !botSubscriberCanUse(subscriber) {
			return sendTelegramBotMessage(config.Token, chatID, "试用已过期或账号未授权，请发送 /activate 卡密 激活。")
		}
		mode := normalizeBotMatchMode(args)
		if err := s.updateBotMatchMode(ctx, subscriber, mode); err != nil {
			return err
		}
		label := "模糊匹配"
		if mode == "exact" {
			label = "精准匹配"
		}
		return sendTelegramBotMessage(config.Token, chatID, "匹配模式已切换为："+label)
	case "/setpush":
		if !botSubscriberCanUse(subscriber) {
			return sendTelegramBotMessage(config.Token, chatID, "试用已过期或账号未授权，请发送 /activate 卡密 激活。")
		}
		reply := s.setBotSubscriberPushChat(ctx, config, subscriber, chatID, args)
		return sendTelegramBotMessage(config.Token, chatID, reply)
	case "/listen":
		if !botSubscriberCanUse(subscriber) {
			return sendTelegramBotMessage(config.Token, chatID, "试用已过期或账号未授权，请发送 /activate 卡密 激活。")
		}
		reply := s.handleBotListenCommand(ctx, config, subscriber, args)
		return sendTelegramBotMessage(config.Token, chatID, reply)
	case "/inbox":
		if !botSubscriberCanUse(subscriber) {
			return sendTelegramBotMessage(config.Token, chatID, "试用已过期或账号未授权，请发送 /activate 卡密 激活。")
		}
		return sendTelegramBotMessage(config.Token, chatID, s.botInboxText(ctx, config.TenantID))
	default:
		return sendTelegramBotMessage(config.Token, chatID, botHelpText())
	}
}

func (s *Server) handleBotCallback(ctx context.Context, config models.BotConfig, update telegramUpdate) error {
	chatID := updateChatID(update)
	data := strings.TrimSpace(update.CallbackQuery.Data)
	if update.CallbackQuery.ID != "" {
		_ = answerTelegramCallbackQuery(config.Token, update.CallbackQuery.ID, "")
	}
	subscriber, err := s.ensureBotSubscriber(ctx, config, update)
	if err != nil {
		return err
	}
	if data == "upload_accounts" || data == "task:list" || strings.HasPrefix(data, "dm:") || strings.HasPrefix(data, "dm_msg:") || strings.HasPrefix(data, "dm_task:") || strings.HasPrefix(data, "dm_group:") || strings.HasPrefix(data, "upload_group:") || data == "group:new" || strings.HasPrefix(data, "group:new:") || strings.HasPrefix(data, "group:delete:") {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "该功能已在后台关闭，请使用当前已开放的监听和关键词功能。", s.botSettingsMarkup(ctx, config, subscriber))
	}
	if data == "keywords:set" && !botCommandEnabled(config, "setkeywords") {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "设置关键词功能暂未开放。", s.botSettingsMarkup(ctx, config, subscriber))
	}
	if strings.HasPrefix(data, "listen:") && !botCommandEnabled(config, "listen") {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "监听开关暂未开放。", s.botSettingsMarkup(ctx, config, subscriber))
	}
	if data == "push:setup" && !botCommandEnabled(config, "setpush") {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "推送位置功能暂未开放。", s.botSettingsMarkup(ctx, config, subscriber))
	}
	if strings.HasPrefix(data, "cfg:match:") && !botCommandEnabled(config, "match") {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "匹配模式功能暂未开放。", s.botSettingsMarkup(ctx, config, subscriber))
	}
	if strings.HasPrefix(data, "cfg:") || data == "upload_accounts" || strings.HasPrefix(data, "dm:") || strings.HasPrefix(data, "dm_msg:") || strings.HasPrefix(data, "dm_task:") || data == "task:list" || data == "keywords:set" || data == "push:setup" || strings.HasPrefix(data, "listen:") {
		if data == "cfg:settings" || data == "upload_accounts" || strings.HasPrefix(data, "dm:") || strings.HasPrefix(data, "dm_msg:") || strings.HasPrefix(data, "dm_task:") || data == "keywords:set" || data == "push:setup" || strings.HasPrefix(data, "listen:") {
			if blocked, err := s.sendForceJoinPromptIfNeeded(ctx, config, subscriber, chatID); blocked || err != nil {
				return err
			}
		}
	}
	switch {
	case data == "nav:main":
		s.clearBotConversationState(ctx, subscriber.ID)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "已返回主菜单。", botMainKeyboard(config))
	case data == "nav:settings":
		s.clearBotConversationState(ctx, subscriber.ID)
		if blocked, err := s.sendForceJoinPromptIfNeeded(ctx, config, subscriber, chatID); blocked || err != nil {
			return err
		}
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSettingsPanelText(subscriber), s.botSettingsMarkup(ctx, config, subscriber))
	case strings.HasPrefix(data, "nav:groups:"):
		s.clearBotConversationState(ctx, subscriber.ID)
		purpose := normalizeBotAccountGroupPurpose(strings.TrimPrefix(data, "nav:groups:"))
		return s.sendBotAccountGroupPicker(ctx, config, subscriber, chatID, purpose)
	case data == "cfg:settings":
		s.clearBotConversationState(ctx, subscriber.ID)
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSettingsPanelText(subscriber), s.botSettingsMarkup(ctx, config, subscriber))
	case data == "cfg:refresh":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		joined := "未配置强制加群链接"
		if botForceJoinRequired(config) {
			ok, _ := s.botSubscriberJoinedRequiredChat(ctx, config, subscriber)
			if ok {
				joined = "已加入指定群/频道"
			} else {
				joined = "未检测到加入指定群/频道"
			}
		}
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSettingsSummary(config, subscriber, joined), s.botSettingsMarkup(ctx, config, subscriber))
	case data == "invite:link":
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botInviteText(config, subscriber), botBackToSettingsKeyboard())
	case data == "invite:detail":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botInviteDetailText(ctx, config, subscriber), botMyInfoKeyboard())
	case data == "keywords:set":
		s.saveBotConversationState(ctx, config, subscriber, "await_keywords", gin.H{})
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botKeywordPromptText(config, subscriber), botBackToSettingsKeyboard())
	case data == "push:setup":
		s.saveBotConversationState(ctx, config, subscriber, "await_push_chat", gin.H{})
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botPushChatPromptText(config, subscriber), botBackToSettingsKeyboard())
	case strings.HasPrefix(data, "listen:"):
		return s.handleBotListenCallback(ctx, config, subscriber, chatID, data)
	case strings.HasPrefix(data, "lead:block:"):
		return s.handleBotLeadBlockCallback(ctx, config, subscriber, chatID, strings.TrimPrefix(data, "lead:block:"))
	case strings.HasPrefix(data, "lead:block_source:"):
		return s.handleBotLeadBlockSourceCallback(ctx, config, subscriber, chatID, strings.TrimPrefix(data, "lead:block_source:"))
	case strings.HasPrefix(data, "lead:chat:"):
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "这个用户没有可跳转的公开用户名，暂时无法直接打开对话。", botBackToSettingsKeyboard())
	case data == "dm:quota":
		s.saveBotConversationState(ctx, config, subscriber, "await_keywords", gin.H{})
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botKeywordPromptText(config, subscriber), botBackToSettingsKeyboard())
	case data == "profile:settings":
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botProfileSettingsText(ctx, config, subscriber), botMyInfoKeyboard())
	case data == "member:activate":
		s.saveBotConversationState(ctx, config, subscriber, "await_activate_code", gin.H{})
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botAdminAssistText(config, "请输入管理员后台生成的卡号进行激活。"), botBackToSettingsKeyboard())
	case data == "upload_accounts":
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botDMQuotaText(ctx, config, subscriber), botBackToSettingsKeyboard())
	case data == "group:new" || strings.HasPrefix(data, "group:new:"):
		purpose := normalizeBotAccountGroupPurpose(strings.TrimPrefix(data, "group:new:"))
		s.saveBotConversationState(ctx, config, subscriber, "await_group_name", gin.H{"purpose": purpose})
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "请输入要新建的账号分组名称。", botBackToGroupPickerKeyboard(purpose))
	case strings.HasPrefix(data, "group:delete:"):
		purpose, groupID := parseBotGroupDeleteCallback(data)
		return s.deleteBotPrivateAccountGroup(ctx, config, subscriber, chatID, groupID, purpose)
	case strings.HasPrefix(data, "upload_group:"):
		groupID := strings.TrimPrefix(data, "upload_group:")
		s.saveBotConversationState(ctx, config, subscriber, "await_account_upload", gin.H{"group_id": groupID})
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "请上传 zip 压缩包或 session/tdata 文件。文件夹请先压缩为 zip 后发送。", botBackToGroupPickerKeyboard("upload"))
	case data == "dm:on":
		return s.startBotDMComposer(ctx, config, subscriber, chatID)
	case data == "dm:off":
		if err := s.stopBotDMTasks(ctx, config, subscriber, "用户手动停止"); err != nil {
			return err
		}
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "私信任务已停止，当前 Bot 用户的自动私信进程已在后台清理。", s.botSettingsMarkup(ctx, config, subscriber))
	case data == "dm_msg:add":
		return s.continueBotDMComposer(ctx, config, subscriber, chatID, false)
	case data == "dm_msg:done":
		return s.continueBotDMComposer(ctx, config, subscriber, chatID, true)
	case data == "dm_task:start":
		return s.startBotDMTaskFromState(ctx, config, subscriber, chatID)
	case strings.HasPrefix(data, "dm_group:"):
		groupID := strings.TrimPrefix(data, "dm_group:")
		s.saveBotConversationState(ctx, config, subscriber, "await_dm_cooldown", gin.H{"group_id": groupID})
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "请输入账号最小冷却时间，单位分钟。例如输入 30。最小值 0，表示不设置冷却。", botBackToGroupPickerKeyboard("dm"))
	case data == "task:list":
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, s.botDMTaskListText(ctx, config, subscriber), botBackToSettingsKeyboard())
	case strings.HasPrefix(data, "cfg:"):
		return s.applyBotSettingCallback(ctx, config, subscriber, chatID, data)
	default:
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, botSettingsPanelText(subscriber), s.botSettingsMarkup(ctx, config, subscriber))
	}
}

func (s *Server) applyBotSettingCallback(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, data string) error {
	updates := map[string]any{"updated_at": time.Now()}
	switch data {
	case "cfg:push:on":
		updates["push_enabled"] = true
	case "cfg:push:off":
		updates["push_enabled"] = false
	case "cfg:match:exact":
		updates["match_mode"] = "exact"
	case "cfg:match:fuzzy":
		updates["match_mode"] = "fuzzy"
	case "cfg:userblack:on":
		updates["user_blacklist_enabled"] = true
	case "cfg:userblack:off":
		updates["user_blacklist_enabled"] = false
	case "cfg:risk:on":
		updates["risk_control_enabled"] = true
	case "cfg:risk:off":
		updates["risk_control_enabled"] = false
	default:
		if strings.HasPrefix(data, "cfg:dedup:") {
			value, _ := strconv.Atoi(strings.TrimPrefix(data, "cfg:dedup:"))
			if value < 0 {
				value = 0
			}
			updates["message_dedup_minutes"] = value
		} else if strings.HasPrefix(data, "cfg:interval:") {
			value, _ := strconv.Atoi(strings.TrimPrefix(data, "cfg:interval:"))
			updates["message_dedup_minutes"] = maxInt(value, 0)
		} else {
			return nil
		}
	}
	if err := s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(updates).Error; err != nil {
		return err
	}
	var fresh models.BotSubscriber
	_ = s.db.WithContext(ctx).First(&fresh, "id = ?", subscriber.ID).Error
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, "设置已实时同步到后台。\n\n"+botSettingsPanelText(fresh), s.botSettingsMarkup(ctx, config, fresh))
}
