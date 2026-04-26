package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"codex3/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Server) ensureBotSubscriber(ctx context.Context, config models.BotConfig, update telegramUpdate) (models.BotSubscriber, error) {
	now := time.Now()
	userID := updateUserID(update)
	username := strings.TrimSpace(update.Message.From.Username)
	firstName := update.Message.From.FirstName
	lastName := update.Message.From.LastName
	if update.CallbackQuery.From.ID != 0 {
		username = strings.TrimSpace(update.CallbackQuery.From.Username)
		firstName = update.CallbackQuery.From.FirstName
		lastName = update.CallbackQuery.From.LastName
	}
	nickname := strings.TrimSpace(strings.Join([]string{firstName, lastName}, " "))

	var subscriber models.BotSubscriber
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND telegram_user_id = ?", config.TenantID, userID).
		First(&subscriber).Error
	if err == nil {
		wasUsable := botSubscriberCanUseAt(subscriber, now)
		subscriber.Username = username
		subscriber.Nickname = nickname
		if strings.TrimSpace(subscriber.InviteCode) == "" {
			subscriber.InviteCode = subscriber.TelegramUserID
		}
		subscriber.LastSeenAt = &now
		subscriber.UpdatedAt = now
		s.attachWebUserToSubscriber(ctx, &subscriber)
		normalizeBotSubscriberAccessState(&subscriber, now)
		s.enforceBotSubscriberProcessState(ctx, config.TenantID, &subscriber, wasUsable, now)
		_ = s.db.WithContext(ctx).Save(&subscriber).Error
		return subscriber, nil
	}
	if err != gorm.ErrRecordNotFound {
		return models.BotSubscriber{}, err
	}
	emptyGroupsJSON, _ := json.Marshal([]string{})
	emptyKeywordsJSON, _ := json.Marshal([]string{})
	if !config.TrialEnabled {
		subscriber = models.BotSubscriber{
			ID:                      uuid.New(),
			TenantID:                config.TenantID,
			TelegramUserID:          userID,
			Username:                username,
			Nickname:                nickname,
			InviteCode:              userID,
			PushEnabled:             false,
			Keywords:                datatypes.JSON(emptyKeywordsJSON),
			KeywordLimit:            0,
			MatchMode:               "fuzzy",
			PushIntervalMinutes:     0,
			MessageDedupMinutes:     0,
			PrivateTerminalGroupIDs: datatypes.JSON(emptyGroupsJSON),
			Status:                  "inactive",
			Plan:                    "none",
			LastSeenAt:              &now,
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		if err := s.db.WithContext(ctx).Create(&subscriber).Error; err != nil {
			return models.BotSubscriber{}, err
		}
		s.attachWebUserToSubscriber(ctx, &subscriber)
		return subscriber, nil
	}
	trialEnds := now.Add(time.Duration(maxInt(config.TrialHours, 5)) * time.Hour)
	subscriber = models.BotSubscriber{
		ID:                      uuid.New(),
		TenantID:                config.TenantID,
		TelegramUserID:          userID,
		Username:                username,
		Nickname:                nickname,
		InviteCode:              userID,
		PushEnabled:             false,
		Keywords:                datatypes.JSON(emptyKeywordsJSON),
		KeywordLimit:            0,
		MatchMode:               "fuzzy",
		PushIntervalMinutes:     0,
		MessageDedupMinutes:     0,
		PrivateTerminalGroupIDs: datatypes.JSON(emptyGroupsJSON),
		Status:                  "active",
		Plan:                    "trial",
		TrialStartedAt:          &now,
		TrialEndsAt:             &trialEnds,
		LastSeenAt:              &now,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := s.db.WithContext(ctx).Create(&subscriber).Error; err != nil {
		return models.BotSubscriber{}, err
	}
	s.attachWebUserToSubscriber(ctx, &subscriber)
	return subscriber, nil
}

func (s *Server) attachWebUserToSubscriber(ctx context.Context, subscriber *models.BotSubscriber) {
	if subscriber == nil || strings.TrimSpace(subscriber.TelegramUserID) == "" {
		return
	}
	var user models.User
	if subscriber.UserID != nil {
		if err := s.db.WithContext(ctx).Where("id = ?", *subscriber.UserID).First(&user).Error; err != nil {
			return
		}
	} else {
		if err := s.db.WithContext(ctx).Where("telegram_user_id = ?", subscriber.TelegramUserID).First(&user).Error; err != nil {
			return
		}
		subscriber.UserID = &user.ID
	}
	if user.Status == models.StatusDisabled {
		subscriber.Status = "disabled"
		return
	}
	if subscriber.Plan != "license" && user.TrialEndsAt != nil {
		subscriber.TrialEndsAt = user.TrialEndsAt
		subscriber.Plan = "trial"
		if user.TrialEndsAt.After(time.Now()) {
			subscriber.Status = "active"
		} else {
			subscriber.Status = "expired"
		}
	}
}

func (s *Server) reloadBotSubscriber(ctx context.Context, subscriber models.BotSubscriber) models.BotSubscriber {
	var fresh models.BotSubscriber
	if err := s.db.WithContext(ctx).First(&fresh, "id = ?", subscriber.ID).Error; err != nil {
		return subscriber
	}
	s.attachWebUserToSubscriber(ctx, &fresh)
	normalizeBotSubscriberAccessState(&fresh, time.Now())
	s.enforceBotSubscriberProcessState(ctx, fresh.TenantID, &fresh, true, time.Now())
	fresh.UpdatedAt = time.Now()
	_ = s.db.WithContext(ctx).Save(&fresh).Error
	return fresh
}

func botSubscriberCanUseAt(subscriber models.BotSubscriber, now time.Time) bool {
	if subscriber.Status != "active" {
		return false
	}
	if subscriber.Plan == "license" && subscriber.ExpiresAt != nil {
		return subscriber.ExpiresAt.After(now)
	}
	if subscriber.Plan == "trial" && subscriber.TrialEndsAt != nil {
		return subscriber.TrialEndsAt.After(now)
	}
	return false
}

func normalizeBotSubscriberAccessState(subscriber *models.BotSubscriber, now time.Time) {
	if subscriber == nil || subscriber.Status != "active" {
		return
	}
	switch subscriber.Plan {
	case "license":
		if subscriber.ExpiresAt != nil && subscriber.ExpiresAt.Before(now) {
			subscriber.Status = "expired"
		}
	case "trial":
		if subscriber.TrialEndsAt != nil && subscriber.TrialEndsAt.Before(now) {
			subscriber.Status = "expired"
		}
	}
}

func botTaskSubscriberID(task models.Task) uuid.UUID {
	if len(task.Payload) == 0 {
		return uuid.Nil
	}
	payload := struct {
		BotSubscriberID string `json:"bot_subscriber_id"`
	}{}
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return uuid.Nil
	}
	parsed, err := uuid.Parse(strings.TrimSpace(payload.BotSubscriberID))
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

func listenerRuntimeBelongsToSubscriber(runtime *scrmListenerRuntime, subscriberID uuid.UUID) bool {
	if runtime == nil || subscriberID == uuid.Nil {
		return false
	}
	return botTaskSubscriberID(runtime.task) == subscriberID
}

func (s *Server) enforceBotSubscriberProcessState(ctx context.Context, tenantID uuid.UUID, subscriber *models.BotSubscriber, wasUsable bool, now time.Time) {
	if subscriber == nil {
		return
	}
	isUsable := botSubscriberCanUseAt(*subscriber, now)
	if subscriber.Status == "disabled" || subscriber.Status == "expired" || (wasUsable && !isUsable) {
		taskStatus := "expired"
		ruleStatus := "stopped"
		reason := "Bot 用户已过期，已自动停止该用户的监听和私信任务"
		if subscriber.Status == "disabled" {
			taskStatus = "stopped"
			reason = "Bot 用户已禁用，已自动停止该用户的监听和私信任务"
		}
		s.stopBotSubscriberProcesses(ctx, tenantID, *subscriber, taskStatus, ruleStatus, reason)
	}
}

func (s *Server) stopBotSubscriberProcesses(ctx context.Context, tenantID uuid.UUID, subscriber models.BotSubscriber, taskStatus string, ruleStatus string, reason string) {
	s.stopBotSubscriberListenerProcesses(ctx, tenantID, subscriber, taskStatus, ruleStatus, reason)

	now := time.Now()
	endAt := any(nil)
	if taskStatus == "expired" || taskStatus == "stopped" || taskStatus == "completed" {
		endAt = &now
	}

	dmUpdates := map[string]any{
		"status":     taskStatus,
		"updated_at": now,
	}
	if endAt != nil {
		dmUpdates["ended_at"] = endAt
	}
	_ = s.db.WithContext(ctx).
		Model(&models.BotDMTask{}).
		Where("tenant_id = ? AND subscriber_id = ? AND status IN ?", tenantID, subscriber.ID, []string{"active", "queued", "running"}).
		Updates(dmUpdates).Error
	_ = s.db.WithContext(ctx).Where("subscriber_id = ?", subscriber.ID).Delete(&models.BotConversationState{}).Error
}

func (s *Server) stopBotDMTasks(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, reason string) error {
	now := time.Now()
	err := s.db.WithContext(ctx).
		Model(&models.BotDMTask{}).
		Where("tenant_id = ? AND subscriber_id = ? AND status IN ?", config.TenantID, subscriber.ID, []string{"active", "queued", "running"}).
		Updates(map[string]any{"status": "completed", "ended_at": &now, "updated_at": now}).Error
	if err != nil {
		return err
	}
	_ = s.db.WithContext(ctx).Where("subscriber_id = ?", subscriber.ID).Delete(&models.BotConversationState{}).Error
	if strings.TrimSpace(reason) != "" {
		log.Printf("bot dm tasks stopped: tenant=%s subscriber=%s reason=%s", config.TenantID, subscriber.ID, reason)
	}
	return nil
}

func (s *Server) stopBotSubscriberListenerProcesses(ctx context.Context, tenantID uuid.UUID, subscriber models.BotSubscriber, taskStatus string, ruleStatus string, reason string) {
	now := time.Now()

	s.listenerMu.Lock()
	runtime := s.listeners[scrmListenerRuntimeKey(tenantID, subscriber.ID)]
	if listenerRuntimeBelongsToSubscriber(runtime, subscriber.ID) {
		runtime.stopping.Store(true)
		runtime.cancel()
		delete(s.listeners, runtime.key)
	} else {
		runtime = nil
	}
	s.listenerMu.Unlock()

	if runtime != nil {
		s.updateTaskState(ctx, runtime.task.ID, taskStatus, 100, nil)
		_ = s.db.WithContext(ctx).Model(&models.SCRMKeywordRule{}).
			Where("tenant_id = ? AND id = ?", tenantID, runtime.rule.ID).
			Updates(map[string]any{"status": ruleStatus, "updated_at": now}).Error
		s.logTaskBackground(ctx, runtime.task, "INFO", "subscriber_stop", reason)
	}

	var listenerTasks []models.Task
	_ = s.db.WithContext(ctx).
		Where("tenant_id = ? AND type = ? AND status IN ?", tenantID, "scrm_listener", []string{"queued", "running", "paused"}).
		Order("created_at desc").
		Find(&listenerTasks).Error
	for _, task := range listenerTasks {
		if botTaskSubscriberID(task) != subscriber.ID {
			continue
		}
		s.updateTaskState(ctx, task.ID, taskStatus, 100, nil)
	}

	_ = s.db.WithContext(ctx).Where("subscriber_id = ?", subscriber.ID).Delete(&models.BotConversationState{}).Error
}

func (s *Server) startTrialIfAvailable(ctx context.Context, config models.BotConfig, subscriber *models.BotSubscriber) string {
	now := time.Now()
	if !config.TrialEnabled {
		return "当前暂未开放试用，请发送 /activate 卡密 激活正式权限。"
	}
	if subscriber.Plan == "license" && subscriber.ExpiresAt != nil && subscriber.ExpiresAt.After(now) {
		return "你已经是正式授权用户，可使用完整功能。\n" + botSubscriberStatusText(*subscriber)
	}
	if subscriber.TrialEndsAt != nil && subscriber.TrialEndsAt.After(now) {
		return fmt.Sprintf("试用已开启，到期时间：%s\n试用用户仅可使用关键词监听功能。", subscriber.TrialEndsAt.Format("2006-01-02 15:04:05"))
	}
	if subscriber.TrialStartedAt != nil {
		subscriber.Status = "expired"
		subscriber.UpdatedAt = now
		_ = s.db.WithContext(ctx).Save(subscriber).Error
		return "你的试用已过期，请发送 /activate 卡密 激活正式权限。"
	}
	trialEnds := now.Add(time.Duration(maxInt(config.TrialHours, 5)) * time.Hour)
	subscriber.Status = "active"
	subscriber.Plan = "trial"
	subscriber.TrialStartedAt = &now
	subscriber.TrialEndsAt = &trialEnds
	subscriber.UpdatedAt = now
	_ = s.db.WithContext(ctx).Save(subscriber).Error
	return fmt.Sprintf("欢迎使用 Codex3 Bot，已为你开通 %d 小时试用。\n试用用户仅可使用关键词监听功能。\n到期时间：%s", maxInt(config.TrialHours, 5), trialEnds.Format("2006-01-02 15:04:05"))
}

func (s *Server) activateBotLicense(ctx context.Context, config models.BotConfig, subscriber *models.BotSubscriber, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", errors.New("请发送：/activate 你的卡密")
	}
	now := time.Now()
	var license models.BotLicense
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND code = ?", config.TenantID, code).First(&license).Error; err != nil {
		return "", errors.New("卡密不存在")
	}
	if license.Status == "disabled" {
		return "", errors.New("卡密已被禁用")
	}
	if license.Status == "expired" || (license.ExpiresAt != nil && license.ExpiresAt.Before(now)) {
		return "", errors.New("卡密已过期")
	}
	if license.BoundCount >= license.MaxBind && license.BoundUserID != subscriber.TelegramUserID {
		return "", errors.New("卡密绑定人数已满")
	}
	expiresAt := now.Add(time.Duration(maxInt(license.DurationHour, 24*30)) * time.Hour)
	license.Status = "used"
	license.BoundCount = maxInt(license.BoundCount, 0) + 1
	license.BoundUserID = subscriber.TelegramUserID
	license.BoundUsername = subscriber.Username
	license.UsedAt = &now
	license.ExpiresAt = &expiresAt
	license.UpdatedAt = now

	subscriber.Status = "active"
	subscriber.Plan = "license"
	subscriber.LicenseID = &license.ID
	subscriber.AuthorizedAt = &now
	subscriber.ExpiresAt = &expiresAt
	subscriber.UpdatedAt = now

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&license).Error; err != nil {
			return err
		}
		return tx.Save(subscriber).Error
	}); err != nil {
		return "", err
	}
	return "激活成功，正式权限到期时间：" + expiresAt.Format("2006-01-02 15:04:05"), nil
}
