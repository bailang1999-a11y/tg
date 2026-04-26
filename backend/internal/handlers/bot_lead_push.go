package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) pushSCRMLeadToBot(ctx context.Context, task models.Task, lead models.SCRMLead) {
	var config models.BotConfig
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", task.TenantID).First(&config).Error; err != nil {
		s.logTaskBackground(ctx, task, "WARN", "bot_push", "Bot 配置不存在，已跳过推送")
		return
	}
	if !config.Enabled || !config.Running || strings.TrimSpace(config.Token) == "" {
		s.logTaskBackground(ctx, task, "WARN", "bot_push", "Bot 推送未启动或缺少 Token，已跳过")
		return
	}
	subscriber, hasSubscriber := s.resolveBotPushSubscriberForTask(ctx, task, config)
	pushChatID := strings.TrimSpace(config.PushChatID)
	if hasSubscriber {
		subscriber = s.reloadBotSubscriber(ctx, subscriber)
		normalizeBotSubscriberAccessState(&subscriber, time.Now())
		if subscriber.Status != "active" || !subscriber.PushEnabled {
			s.logTaskBackground(ctx, task, "INFO", "bot_push", fmt.Sprintf("Bot 用户 %s 未开启推送或已过期，已跳过", botSubscriberDisplayName(subscriber)))
			return
		}
		pushChatID = botSubscriberPushChatID(subscriber)
		if subscriber.MessageDedupMinutes > 0 && s.scrmLeadHitDeduped(ctx, task.TenantID, lead, subscriber.MessageDedupMinutes) {
			s.logTaskBackground(ctx, task, "INFO", "bot_push", fmt.Sprintf("用户 %s 在 %d 分钟内已推送过，已去重跳过", firstNonEmpty(lead.UserAccount, lead.UserNickname, "未知用户"), subscriber.MessageDedupMinutes))
			return
		}
		if s.botLeadUserBlacklisted(ctx, subscriber, lead) {
			s.logTaskBackground(ctx, task, "INFO", "bot_push", fmt.Sprintf("用户 %s 已在当前 Bot 用户黑名单，已跳过推送", firstNonEmpty(lead.UserAccount, lead.UserNickname, "未知用户")))
			return
		}
		if s.botLeadSourceBlacklisted(ctx, subscriber, lead) {
			s.logTaskBackground(ctx, task, "INFO", "bot_push", fmt.Sprintf("来源 %s 已在当前 Bot 用户黑名单，已跳过推送", firstNonEmpty(lead.SourceChatName, lead.SourceChatID, "未知来源")))
			return
		}
	}
	if strings.TrimSpace(pushChatID) == "" {
		s.logTaskBackground(ctx, task, "WARN", "bot_push", "缺少推送 Chat ID，已跳过")
		return
	}
	filterHint := ""
	if hasSubscriber && subscriber.RiskControlEnabled {
		hitCount := s.scrmLeadHitCountWithin(ctx, task.TenantID, lead, 24*time.Hour)
		if hitCount >= 20 {
			filterHint = fmt.Sprintf("\nAI过滤：疑似广告/疑似营销（同一用户24小时内命中 %d 次）", hitCount)
		}
	}
	var subscriberForText *models.BotSubscriber
	if hasSubscriber {
		subscriberForText = &subscriber
	}
	text := s.botSCRMLeadPushText(ctx, task.TenantID, subscriberForText, lead, filterHint)
	if err := sendTelegramBotMessageHTML(config.Token, pushChatID, text, s.botSCRMLeadPushKeyboard(ctx, task.TenantID, lead)); err != nil {
		s.logTaskBackground(ctx, task, "ERROR", "bot_push", "Bot 推送失败："+err.Error())
		return
	}
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&models.SCRMLead{}).Where("id = ?", lead.ID).Updates(map[string]any{
		"bot_pushed_at": &now,
		"updated_at":    now,
	}).Error
	s.logTaskBackground(ctx, task, "INFO", "bot_push", "Bot 推送已发送到 "+pushChatID)
	if hasSubscriber {
		go s.dispatchBotPrivateMessages(context.Background(), config, task, subscriber, lead)
	}
}

func (s *Server) dispatchBotPrivateMessages(ctx context.Context, config models.BotConfig, task models.Task, subscriber models.BotSubscriber, lead models.SCRMLead) {
	target := strings.TrimSpace(lead.UserAccount)
	if target == "" {
		s.logTaskBackground(ctx, task, "INFO", "bot_dm", "命中线索没有用户账号，自动私信已跳过")
		return
	}
	var dmTasks []models.BotDMTask
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND subscriber_id = ? AND status = ?", config.TenantID, subscriber.ID, "active").
		Order("created_at desc").
		Find(&dmTasks).Error; err != nil || len(dmTasks) == 0 {
		return
	}
	remaining := subscriber.DMQuotaTotal - subscriber.DMQuotaUsed
	if remaining <= 0 {
		s.logTaskBackground(ctx, task, "INFO", "bot_dm", "私信额度不足，自动私信已跳过")
		_ = s.stopBotDMTasks(ctx, config, subscriber, "私信额度不足，自动终止任务")
		return
	}
	messenger := telegram_client.NewMessenger(s.cfg)
	for _, dmTask := range dmTasks {
		messages := normalizeBotDMMessages(jsonStringSlice(dmTask.Messages))
		if len(messages) == 0 {
			continue
		}
		sentForTask := int64(0)
		for index, message := range messages {
			if subscriber.DMQuotaTotal-subscriber.DMQuotaUsed <= 0 {
				s.logTaskBackground(ctx, task, "INFO", "bot_dm", "私信额度已用完，停止后续自动私信")
				_ = s.stopBotDMTasks(ctx, config, subscriber, "私信额度用尽，自动终止任务")
				break
			}
			terminal, ok, skipReason := s.pickBotDMTerminal(ctx, config, subscriber, dmTask, target)
			if !ok {
				s.logTaskBackground(ctx, task, "WARN", "bot_dm", firstNonEmpty(skipReason, "没有可用私信账号，自动私信已跳过"))
				break
			}
			result, err := messenger.Send(ctx, telegram_client.MessageRequest{
				FilePath:   terminal.FilePath,
				AccessType: terminal.AccessType,
				TargetType: "user",
				Target:     target,
				StepType:   "text",
				Content:    message,
			})
			if err != nil || !result.OK {
				reason := result.Reason
				if reason == "" && err != nil {
					reason = err.Error()
				}
				s.applyTerminalOutboundFailure(ctx, terminal.ID, reason)
				s.applyTerminalTargetFailure(ctx, config.TenantID, terminal.ID, terminalQuotaActionDM, "user", target, reason)
				s.logTaskBackground(ctx, task, "ERROR", "bot_dm", fmt.Sprintf("自动私信失败：账号 %s -> %s，第 %d 条，原因：%s", terminal.Phone, target, index+1, firstNonEmpty(reason, "未知错误")))
				break
			}
			now := time.Now()
			sentForTask++
			subscriber.DMQuotaUsed++
			_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Where("id = ?", terminal.ID).Updates(map[string]any{"last_message_at": &now, "updated_at": now}).Error
			_ = s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]any{"dm_quota_used": subscriber.DMQuotaUsed, "updated_at": now}).Error
			s.logTaskBackground(ctx, task, "INFO", "bot_dm", fmt.Sprintf("自动私信成功：账号 %s -> %s，第 %d/%d 条", terminal.Phone, target, index+1, len(messages)))
			if index < len(messages)-1 {
				delay := randomDelaySeconds(dmTask.MinDelaySeconds, dmTask.MaxDelaySeconds)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(delay) * time.Second):
				}
			}
		}
		if sentForTask > 0 {
			now := time.Now()
			_ = s.db.WithContext(ctx).Model(&models.BotDMTask{}).Where("id = ?", dmTask.ID).Updates(map[string]any{
				"sent_count": gorm.Expr("sent_count + ?", sentForTask),
				"updated_at": now,
			}).Error
		}
	}
}

func (s *Server) pickBotDMTerminal(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, dmTask models.BotDMTask, target string) (models.Terminal, bool, string) {
	groupIDs := []uuid.UUID{}
	if dmTask.AccountGroupID != nil {
		groupIDs = append(groupIDs, *dmTask.AccountGroupID)
	} else {
		for _, rawID := range jsonStringSlice(subscriber.PrivateTerminalGroupIDs) {
			if id, err := uuid.Parse(strings.TrimSpace(rawID)); err == nil {
				groupIDs = append(groupIDs, id)
			}
		}
	}
	if len(groupIDs) == 0 {
		return models.Terminal{}, false, "没有配置可用的私信账号组"
	}
	var terminals []models.Terminal
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND group_id IN ? AND file_path <> '' AND status NOT IN ?", config.TenantID, groupIDs, []string{"abnormal", "banned", "disabled"}).
		Where("(sleep_until IS NULL OR sleep_until <= ?)", time.Now()).
		Order("CASE WHEN last_message_at IS NULL THEN 0 ELSE 1 END asc, last_message_at asc, updated_at asc").
		Limit(20).
		Find(&terminals).Error; err != nil || len(terminals) == 0 {
		return models.Terminal{}, false, "没有找到可调度的私信账号"
	}
	skipReasons := make([]string, 0, len(terminals))
	for _, terminal := range terminals {
		if !s.terminalTargetAvailable(ctx, config.TenantID, terminal.ID, terminalQuotaActionDM, "user", target) {
			skipReasons = append(skipReasons, "该账号已被标记为暂不处理此目标")
			continue
		}
		reserved, err := s.reserveTerminalQuota(ctx, terminal.ID, terminalQuotaActionDM)
		if err == nil {
			return reserved, true, ""
		}
		skipReasons = append(skipReasons, err.Error())
	}
	return models.Terminal{}, false, "自动私信已跳过：" + summarizeTerminalSkipReasons(skipReasons, "没有可用私信账号")
}

func (s *Server) botSCRMLeadPushText(ctx context.Context, tenantID uuid.UUID, subscriber *models.BotSubscriber, lead models.SCRMLead, filterHint string) string {
	target := s.loadSCRMLeadTarget(ctx, tenantID, lead.TargetID)
	keywordPool := "未设置"
	if subscriber != nil {
		keywords := jsonStringSlice(subscriber.Keywords)
		if len(keywords) > 0 {
			keywordPool = strings.Join(keywords, " / ")
		}
	}
	userLabel := firstNonEmpty(lead.UserNickname, strings.TrimPrefix(lead.UserAccount, "@"), "未记录")
	userHTML := botHTMLLink(userLabel, telegramUserLink(lead.UserAccount))
	sourceLabel := firstNonEmpty(lead.SourceChatName, target.Name, lead.SourceChatID, "未记录")
	sourceHTML := botHTMLLink(sourceLabel, telegramTargetLink(target, lead))
	keywordHTML := botHTMLLink(firstNonEmpty(lead.TriggerWord, "未记录"), telegramMessageLink(target, lead))
	history := s.botSCRMLeadRecentHistoryText(ctx, tenantID, lead)
	lines := []string{
		"🛰 <b>雷达命中新线索</b>",
		"👤 用户昵称：" + userHTML,
		"🔗 用户账号：" + botHTMLLink(firstNonEmpty(lead.UserAccount, "未记录"), telegramUserLink(lead.UserAccount)),
		"📍 来源：" + sourceHTML,
		"🎯 命中关键词：" + keywordHTML,
		"📝 完整消息：" + html.EscapeString(firstNonEmpty(lead.TriggerMessage, "未记录")),
		"🗂 用户设置关键词：" + html.EscapeString(keywordPool),
		"⏰ 命中时间：" + html.EscapeString(scrmLeadHitTime(lead).Format("2006-01-02 15:04:05")),
		"",
		"📚 <b>最近 10 条搜索记录</b>",
		history,
	}
	if strings.TrimSpace(filterHint) != "" {
		lines = append(lines, "", html.EscapeString(strings.TrimSpace(filterHint)))
	}
	return strings.Join(lines, "\n")
}

func (s *Server) botSCRMLeadPushKeyboard(ctx context.Context, tenantID uuid.UUID, lead models.SCRMLead) gin.H {
	chatURL := telegramUserLink(lead.UserAccount)
	row := []gin.H{
		{"text": "屏蔽该用户", "callback_data": "lead:block:" + lead.ID.String()},
		{"text": "屏蔽来源", "callback_data": "lead:block_source:" + lead.ID.String()},
	}
	if chatURL != "" {
		row = append(row, gin.H{"text": "聊天", "url": chatURL})
	} else {
		row = append(row, gin.H{"text": "聊天", "callback_data": "lead:chat:" + lead.ID.String()})
	}
	return gin.H{"inline_keyboard": [][]gin.H{row}}
}

func (s *Server) botSCRMLeadRecentHistoryText(ctx context.Context, tenantID uuid.UUID, lead models.SCRMLead) string {
	query := s.db.WithContext(ctx).
		Where("tenant_id = ? AND created_at >= ?", tenantID, time.Now().AddDate(0, 0, -15)).
		Order("COALESCE(hit_at, created_at) desc")
	if strings.TrimSpace(lead.UserAccount) != "" {
		query = query.Where("user_account = ?", strings.TrimSpace(lead.UserAccount))
	} else if strings.TrimSpace(lead.UserNickname) != "" {
		query = query.Where("user_nickname = ?", strings.TrimSpace(lead.UserNickname))
	} else {
		query = query.Where("target_id = ?", lead.TargetID)
	}
	var leads []models.SCRMLead
	if err := query.Limit(10).Find(&leads).Error; err != nil || len(leads) == 0 {
		return "暂无记录"
	}
	targets := s.loadSCRMLeadTargets(ctx, tenantID, leads)
	parts := make([]string, 0, len(leads))
	for _, item := range leads {
		target := targets[item.TargetID]
		source := botHTMLLink(firstNonEmpty(item.SourceChatName, target.Name, item.SourceChatID, "未知来源"), telegramTargetLink(target, item))
		keyword := botHTMLLink(firstNonEmpty(item.TriggerWord, "未记录"), telegramMessageLink(target, item))
		message := listenerMessagePreview(firstNonEmpty(item.TriggerMessage, ""))
		date := scrmLeadHitTime(item).Format("01-02 15:04")
		parts = append(parts, fmt.Sprintf("%s｜%s｜%s｜%s", html.EscapeString(date), source, keyword, html.EscapeString(message)))
	}
	return strings.Join(parts, "\n")
}

func (s *Server) loadSCRMLeadTarget(ctx context.Context, tenantID uuid.UUID, targetID uuid.UUID) models.Target {
	var target models.Target
	_ = s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, targetID).First(&target).Error
	return target
}

func (s *Server) loadSCRMLeadTargets(ctx context.Context, tenantID uuid.UUID, leads []models.SCRMLead) map[uuid.UUID]models.Target {
	ids := make([]uuid.UUID, 0, len(leads))
	seen := map[uuid.UUID]struct{}{}
	for _, lead := range leads {
		if lead.TargetID == uuid.Nil {
			continue
		}
		if _, ok := seen[lead.TargetID]; ok {
			continue
		}
		seen[lead.TargetID] = struct{}{}
		ids = append(ids, lead.TargetID)
	}
	out := map[uuid.UUID]models.Target{}
	if len(ids) == 0 {
		return out
	}
	var targets []models.Target
	_ = s.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&targets).Error
	for _, target := range targets {
		out[target.ID] = target
	}
	return out
}

func botHTMLLink(label string, url string) string {
	escaped := html.EscapeString(firstNonEmpty(label, "未记录"))
	if strings.TrimSpace(url) == "" {
		return escaped
	}
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(url), escaped)
}

func telegramUserLink(account string) string {
	value := strings.TrimSpace(account)
	if value == "" || value == "未记录" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.HasPrefix(value, "@") {
		return "https://t.me/" + strings.TrimPrefix(value, "@")
	}
	return ""
}

func telegramTargetLink(target models.Target, lead models.SCRMLead) string {
	identifier := strings.TrimSpace(target.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(lead.SourceChatID)
	}
	if identifier == "" {
		return ""
	}
	if strings.HasPrefix(identifier, "http://") || strings.HasPrefix(identifier, "https://") {
		return identifier
	}
	if strings.HasPrefix(identifier, "@") {
		return "https://t.me/" + strings.TrimPrefix(identifier, "@")
	}
	if strings.HasPrefix(identifier, "+") || strings.HasPrefix(identifier, "joinchat/") || strings.HasPrefix(identifier, "c/") {
		return "https://t.me/" + strings.TrimPrefix(identifier, "/")
	}
	if strings.HasPrefix(identifier, "-") {
		return ""
	}
	return "https://t.me/" + strings.TrimPrefix(identifier, "@")
}

func telegramMessageLink(target models.Target, lead models.SCRMLead) string {
	messageID := strings.TrimSpace(lead.MessageID)
	if messageID == "" || messageID == "0" {
		return telegramTargetLink(target, lead)
	}
	identifier := strings.TrimSpace(target.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(lead.SourceChatID)
	}
	if identifier == "" {
		return ""
	}
	if strings.HasPrefix(identifier, "http://") || strings.HasPrefix(identifier, "https://") {
		base := strings.TrimRight(identifier, "/")
		if strings.Contains(base, "/+") || strings.Contains(base, "/joinchat/") {
			return base
		}
		return base + "/" + messageID
	}
	if strings.HasPrefix(identifier, "@") {
		return "https://t.me/" + strings.TrimPrefix(identifier, "@") + "/" + messageID
	}
	if strings.HasPrefix(identifier, "c/") {
		return "https://t.me/" + strings.Trim(identifier, "/") + "/" + messageID
	}
	if strings.HasPrefix(identifier, "-100") {
		return "https://t.me/c/" + strings.TrimPrefix(identifier, "-100") + "/" + messageID
	}
	if strings.HasPrefix(identifier, "-") {
		return ""
	}
	if strings.HasPrefix(identifier, "+") || strings.HasPrefix(identifier, "joinchat/") {
		return telegramTargetLink(target, lead)
	}
	return "https://t.me/" + strings.TrimPrefix(identifier, "@") + "/" + messageID
}

func (s *Server) handleBotLeadBlockCallback(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, leadID string) error {
	parsed, err := uuid.Parse(strings.TrimSpace(leadID))
	if err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "线索 ID 无效，无法屏蔽。", botBackToSettingsKeyboard())
	}
	var lead models.SCRMLead
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", config.TenantID, parsed).First(&lead).Error; err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "没有找到这条线索，无法屏蔽。", botBackToSettingsKeyboard())
	}
	label := firstNonEmpty(strings.TrimSpace(lead.UserAccount), strings.TrimSpace(lead.UserNickname))
	if label == "" {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "这条线索没有用户账号或昵称，无法加入黑名单。", botBackToSettingsKeyboard())
	}
	if err := s.addBotUserBlacklist(ctx, config.TenantID, subscriber, lead); err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "加入黑名单失败："+err.Error(), botBackToSettingsKeyboard())
	}
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, "已屏蔽该用户："+label+"\n后续该 Bot 用户不会再收到此用户的推送。", botBackToSettingsKeyboard())
}

func (s *Server) handleBotLeadBlockSourceCallback(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, leadID string) error {
	parsed, err := uuid.Parse(strings.TrimSpace(leadID))
	if err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "线索 ID 无效，无法屏蔽来源。", botBackToSettingsKeyboard())
	}
	var lead models.SCRMLead
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", config.TenantID, parsed).First(&lead).Error; err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "没有找到这条线索，无法屏蔽来源。", botBackToSettingsKeyboard())
	}
	label := firstNonEmpty(strings.TrimSpace(lead.SourceChatName), strings.TrimSpace(lead.SourceChatID), lead.TargetID.String())
	if err := s.addBotSourceBlacklist(ctx, config.TenantID, subscriber, lead); err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "屏蔽来源失败："+err.Error(), botBackToSettingsKeyboard())
	}
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, "已屏蔽来源："+label+"\n后续该 Bot 用户不会再收到此群组/频道的推送。", botBackToSettingsKeyboard())
}

func (s *Server) addBotUserBlacklist(ctx context.Context, tenantID uuid.UUID, subscriber models.BotSubscriber, lead models.SCRMLead) error {
	account := strings.TrimSpace(lead.UserAccount)
	nickname := strings.TrimSpace(lead.UserNickname)
	uniqueAccount := firstNonEmpty(account, "nick:"+nickname)
	now := time.Now()
	record := models.BotUserBlacklist{
		ID:             uuid.New(),
		TenantID:       tenantID,
		SubscriberID:   subscriber.ID,
		UserAccount:    uniqueAccount,
		UserNickname:   nickname,
		SourceChatName: lead.SourceChatName,
		Reason:         "bot_button_block",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.BotUserBlacklist
		err := tx.Where("tenant_id = ? AND subscriber_id = ? AND user_account = ?", tenantID, subscriber.ID, uniqueAccount).First(&existing).Error
		if err == nil {
			return tx.Model(&existing).Updates(map[string]any{
				"user_nickname":    nickname,
				"source_chat_name": lead.SourceChatName,
				"updated_at":       now,
			}).Error
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return tx.Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]any{
			"user_blacklist_enabled": true,
			"updated_at":             now,
		}).Error
	})
}

func (s *Server) addBotSourceBlacklist(ctx context.Context, tenantID uuid.UUID, subscriber models.BotSubscriber, lead models.SCRMLead) error {
	sourceKey := botLeadSourceKey(lead)
	if sourceKey == "" {
		return errors.New("来源信息为空")
	}
	now := time.Now()
	targetID := lead.TargetID
	record := models.BotSourceBlacklist{
		ID:             uuid.New(),
		TenantID:       tenantID,
		SubscriberID:   subscriber.ID,
		SourceKey:      sourceKey,
		SourceChatID:   strings.TrimSpace(lead.SourceChatID),
		SourceChatName: strings.TrimSpace(lead.SourceChatName),
		TargetID:       &targetID,
		Reason:         "bot_button_block_source",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.BotSourceBlacklist
		err := tx.Where("tenant_id = ? AND subscriber_id = ? AND source_key = ?", tenantID, subscriber.ID, sourceKey).First(&existing).Error
		if err == nil {
			return tx.Model(&existing).Updates(map[string]any{
				"source_chat_id":   record.SourceChatID,
				"source_chat_name": record.SourceChatName,
				"target_id":        record.TargetID,
				"updated_at":       now,
			}).Error
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return tx.Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]any{
			"group_blacklist_enabled": true,
			"updated_at":              now,
		}).Error
	})
}

func (s *Server) botLeadUserBlacklisted(ctx context.Context, subscriber models.BotSubscriber, lead models.SCRMLead) bool {
	account := strings.TrimSpace(lead.UserAccount)
	nickname := strings.TrimSpace(lead.UserNickname)
	if account == "" && nickname == "" {
		return false
	}
	query := s.db.WithContext(ctx).Model(&models.BotUserBlacklist{}).
		Where("tenant_id = ? AND subscriber_id = ?", subscriber.TenantID, subscriber.ID)
	if account != "" {
		query = query.Where("user_account = ?", account)
	} else {
		query = query.Where("user_account = ? OR user_nickname = ?", "nick:"+nickname, nickname)
	}
	var count int64
	_ = query.Count(&count).Error
	return count > 0
}

func (s *Server) botLeadSourceBlacklisted(ctx context.Context, subscriber models.BotSubscriber, lead models.SCRMLead) bool {
	sourceKey := botLeadSourceKey(lead)
	if sourceKey == "" {
		return false
	}
	var count int64
	_ = s.db.WithContext(ctx).Model(&models.BotSourceBlacklist{}).
		Where("tenant_id = ? AND subscriber_id = ? AND source_key = ?", subscriber.TenantID, subscriber.ID, sourceKey).
		Count(&count).Error
	return count > 0
}

func botLeadSourceKey(lead models.SCRMLead) string {
	return firstNonEmpty(strings.TrimSpace(lead.SourceChatID), strings.TrimSpace(lead.TargetID.String()), strings.TrimSpace(lead.SourceChatName))
}

func (s *Server) resolveBotPushSubscriberForTask(ctx context.Context, task models.Task, config models.BotConfig) (models.BotSubscriber, bool) {
	payload := struct {
		BotSubscriberID string `json:"bot_subscriber_id"`
		BotPushChatID   string `json:"bot_push_chat_id"`
	}{}
	if len(task.Payload) > 0 {
		_ = json.Unmarshal(task.Payload, &payload)
	}
	if id, err := uuid.Parse(strings.TrimSpace(payload.BotSubscriberID)); err == nil {
		var subscriber models.BotSubscriber
		if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", task.TenantID, id).First(&subscriber).Error; err == nil {
			if subscriber.PushChatID == "" && strings.TrimSpace(payload.BotPushChatID) != "" && strings.TrimSpace(payload.BotPushChatID) != subscriber.TelegramUserID {
				subscriber.PushChatID = strings.TrimSpace(payload.BotPushChatID)
			}
			return subscriber, true
		}
	}
	return s.resolveBotPushSubscriber(ctx, task.TenantID, strings.TrimSpace(config.PushChatID))
}

func (s *Server) resolveBotPushSubscriber(ctx context.Context, tenantID uuid.UUID, chatID string) (models.BotSubscriber, bool) {
	var subscriber models.BotSubscriber
	if chatID != "" {
		if err := s.db.WithContext(ctx).Where("tenant_id = ? AND telegram_user_id = ?", tenantID, chatID).First(&subscriber).Error; err == nil {
			return subscriber, true
		}
	}
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("updated_at desc").First(&subscriber).Error; err != nil {
		return models.BotSubscriber{}, false
	}
	return subscriber, true
}

func (s *Server) scrmLeadHitDeduped(ctx context.Context, tenantID uuid.UUID, lead models.SCRMLead, minutes int) bool {
	if minutes <= 0 {
		return false
	}
	identifier := strings.TrimSpace(lead.UserAccount)
	if identifier == "" {
		identifier = strings.TrimSpace(lead.UserNickname)
	}
	if identifier == "" {
		return false
	}
	cutoff := scrmLeadHitTime(lead).Add(-time.Duration(minutes) * time.Minute)
	query := s.db.WithContext(ctx).Model(&models.SCRMLead{}).
		Where("tenant_id = ? AND id <> ? AND bot_pushed_at IS NOT NULL AND bot_pushed_at >= ?", tenantID, lead.ID, cutoff)
	if strings.TrimSpace(lead.UserAccount) != "" {
		query = query.Where("user_account = ?", strings.TrimSpace(lead.UserAccount))
	} else {
		query = query.Where("user_nickname = ?", strings.TrimSpace(lead.UserNickname))
	}
	var count int64
	_ = query.Count(&count).Error
	return count > 0
}

func (s *Server) scrmLeadHitCountWithin(ctx context.Context, tenantID uuid.UUID, lead models.SCRMLead, window time.Duration) int64 {
	if window <= 0 {
		return 0
	}
	identifier := strings.TrimSpace(lead.UserAccount)
	if identifier == "" {
		identifier = strings.TrimSpace(lead.UserNickname)
	}
	if identifier == "" {
		return 0
	}
	cutoff := scrmLeadHitTime(lead).Add(-window)
	query := s.db.WithContext(ctx).Model(&models.SCRMLead{}).
		Where("tenant_id = ? AND COALESCE(hit_at, created_at) >= ?", tenantID, cutoff)
	if strings.TrimSpace(lead.UserAccount) != "" {
		query = query.Where("user_account = ?", strings.TrimSpace(lead.UserAccount))
	} else {
		query = query.Where("user_nickname = ?", strings.TrimSpace(lead.UserNickname))
	}
	var count int64
	_ = query.Count(&count).Error
	return count
}
