package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type botUserDashboardItem struct {
	models.BotSubscriber
	WebUser           *models.User `json:"web_user,omitempty"`
	AccountCount      int64        `json:"account_count"`
	AccountGroupCount int64        `json:"account_group_count"`
	TaskCount         int64        `json:"task_count"`
	InviteCount       int64        `json:"invite_count"`
}

func (s *Server) ListBotUserDashboard(c *gin.Context) {
	var subscribers []models.BotSubscriber
	if err := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", s.tenantID(c)).Order("updated_at desc").Limit(1000).Find(&subscribers).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 用户失败")
		return
	}
	if len(subscribers) == 0 {
		utils.OK(c, []botUserDashboardItem{})
		return
	}

	subscriberIDs := make([]uuid.UUID, 0, len(subscribers))
	userIDs := make([]uuid.UUID, 0, len(subscribers))
	seenUsers := map[uuid.UUID]struct{}{}
	for _, subscriber := range subscribers {
		subscriberIDs = append(subscriberIDs, subscriber.ID)
		if subscriber.UserID != nil {
			if _, ok := seenUsers[*subscriber.UserID]; !ok {
				userIDs = append(userIDs, *subscriber.UserID)
				seenUsers[*subscriber.UserID] = struct{}{}
			}
		}
	}

	webUsers := map[uuid.UUID]models.User{}
	if len(userIDs) > 0 {
		var users []models.User
		_ = s.db.WithContext(c.Request.Context()).Where("id IN ?", userIDs).Find(&users).Error
		for _, user := range users {
			webUsers[user.ID] = user
		}
	}

	accountCounts := s.countBotRowsBySubscriber(c, &models.BotPrivateAccount{}, subscriberIDs)
	groupCounts := s.countBotRowsBySubscriber(c, &models.BotPrivateAccountGroup{}, subscriberIDs)
	dmTaskCounts := s.countBotRowsBySubscriber(c, &models.BotDMTask{}, subscriberIDs)
	inviteCounts := s.countBotRowsByInviter(c, subscriberIDs)
	listenerTaskCounts := s.countListenerTasksBySubscriberPayload(c, subscriberIDs)

	out := make([]botUserDashboardItem, 0, len(subscribers))
	for _, subscriber := range subscribers {
		item := botUserDashboardItem{BotSubscriber: subscriber}
		if subscriber.UserID != nil {
			if user, ok := webUsers[*subscriber.UserID]; ok {
				item.WebUser = &user
			}
		}
		item.AccountCount = accountCounts[subscriber.ID]
		item.AccountGroupCount = groupCounts[subscriber.ID]
		item.TaskCount = dmTaskCounts[subscriber.ID] + listenerTaskCounts[subscriber.ID]
		item.InviteCount = inviteCounts[subscriber.ID]
		out = append(out, item)
	}
	utils.OK(c, out)
}

type subscriberCountRow struct {
	SubscriberID uuid.UUID `gorm:"column:subscriber_id"`
	Total        int64     `gorm:"column:total"`
}

func (s *Server) countBotRowsBySubscriber(c *gin.Context, model any, subscriberIDs []uuid.UUID) map[uuid.UUID]int64 {
	counts := map[uuid.UUID]int64{}
	var rows []subscriberCountRow
	_ = s.db.WithContext(c.Request.Context()).
		Model(model).
		Select("subscriber_id, COUNT(*) AS total").
		Where("tenant_id = ? AND subscriber_id IN ?", s.tenantID(c), subscriberIDs).
		Group("subscriber_id").
		Scan(&rows).Error
	for _, row := range rows {
		counts[row.SubscriberID] = row.Total
	}
	return counts
}

func (s *Server) countBotRowsByInviter(c *gin.Context, subscriberIDs []uuid.UUID) map[uuid.UUID]int64 {
	counts := map[uuid.UUID]int64{}
	var rows []struct {
		InviterID uuid.UUID `gorm:"column:inviter_id"`
		Total     int64     `gorm:"column:total"`
	}
	_ = s.db.WithContext(c.Request.Context()).
		Model(&models.BotReferral{}).
		Select("inviter_id, COUNT(*) AS total").
		Where("tenant_id = ? AND inviter_id IN ?", s.tenantID(c), subscriberIDs).
		Group("inviter_id").
		Scan(&rows).Error
	for _, row := range rows {
		counts[row.InviterID] = row.Total
	}
	return counts
}

func (s *Server) countListenerTasksBySubscriberPayload(c *gin.Context, subscriberIDs []uuid.UUID) map[uuid.UUID]int64 {
	counts := map[uuid.UUID]int64{}
	type payloadRow struct {
		Payload string `gorm:"column:payload_text"`
	}
	var rows []payloadRow
	_ = s.db.WithContext(c.Request.Context()).
		Model(&models.Task{}).
		Select("CAST(payload AS TEXT) AS payload_text").
		Where("tenant_id = ? AND payload IS NOT NULL", s.tenantID(c)).
		Scan(&rows).Error
	for _, row := range rows {
		for _, subscriberID := range subscriberIDs {
			if strings.Contains(row.Payload, subscriberID.String()) {
				counts[subscriberID]++
			}
		}
	}
	return counts
}

func (s *Server) GetBotUserDashboard(c *gin.Context) {
	subscriber, ok := s.loadBotSubscriberParam(c)
	if !ok {
		return
	}
	var groups []models.BotPrivateAccountGroup
	var accounts []models.BotPrivateAccount
	var uploads []models.BotPrivateUpload
	var tasks []models.BotDMTask
	var allTasks []models.Task
	var referrals []models.BotReferral
	var terminalGroups []models.Group
	_ = s.db.WithContext(c.Request.Context()).Where("subscriber_id = ?", subscriber.ID).Order("is_default desc, created_at asc").Find(&groups).Error
	_ = s.db.WithContext(c.Request.Context()).Where("subscriber_id = ?", subscriber.ID).Order("created_at desc").Limit(500).Find(&accounts).Error
	_ = s.db.WithContext(c.Request.Context()).Where("subscriber_id = ?", subscriber.ID).Order("created_at desc").Limit(100).Find(&uploads).Error
	_ = s.db.WithContext(c.Request.Context()).Where("subscriber_id = ?", subscriber.ID).Order("created_at desc").Limit(100).Find(&tasks).Error
	_ = s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND CAST(payload AS TEXT) LIKE ?", subscriber.TenantID, "%"+subscriber.ID.String()+"%").Order("created_at desc").Limit(100).Find(&allTasks).Error
	_ = s.db.WithContext(c.Request.Context()).Where("inviter_id = ? OR invitee_id = ?", subscriber.ID, subscriber.ID).Order("created_at desc").Limit(100).Find(&referrals).Error
	_ = s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND resource_type = ?", subscriber.TenantID, "terminal").Order("created_at asc").Find(&terminalGroups).Error

	var webUser *models.User
	if subscriber.UserID != nil {
		var user models.User
		if err := s.db.WithContext(c.Request.Context()).First(&user, "id = ?", *subscriber.UserID).Error; err == nil {
			webUser = &user
		}
	}
	utils.OK(c, gin.H{
		"subscriber":      subscriber,
		"web_user":        webUser,
		"groups":          groups,
		"accounts":        accounts,
		"uploads":         uploads,
		"tasks":           tasks,
		"all_tasks":       s.enrichTasks(c.Request.Context(), allTasks),
		"referrals":       referrals,
		"terminal_groups": terminalGroups,
	})
}

func (s *Server) UpdateBotUserDashboard(c *gin.Context) {
	subscriber, ok := s.loadBotSubscriberParam(c)
	if !ok {
		return
	}
	var req struct {
		UserID                  string   `json:"user_id"`
		Status                  string   `json:"status"`
		Plan                    string   `json:"plan"`
		TrialDays               int      `json:"trial_days"`
		Keywords                []string `json:"keywords"`
		KeywordLimit            *int     `json:"keyword_limit"`
		MatchMode               string   `json:"match_mode"`
		PushEnabled             *bool    `json:"push_enabled"`
		PushChatID              *string  `json:"push_chat_id"`
		UserBlacklistEnabled    *bool    `json:"user_blacklist_enabled"`
		RiskControlEnabled      *bool    `json:"risk_control_enabled"`
		MessageDedupMinutes     *int     `json:"message_dedup_minutes"`
		DMQuotaTotal            *int64   `json:"dm_quota_total"`
		DMQuotaUsed             *int64   `json:"dm_quota_used"`
		PrivateTerminalGroupIDs []string `json:"private_terminal_group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "参数无效")
		return
	}
	updates := map[string]any{"updated_at": time.Now()}
	if req.UserID != "" {
		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "Web 用户 ID 无效")
			return
		}
		updates["user_id"] = userID
		_ = s.db.WithContext(c.Request.Context()).Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
			"telegram_user_id":  subscriber.TelegramUserID,
			"telegram_username": subscriber.Username,
			"updated_at":        time.Now(),
		}).Error
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Plan != "" {
		updates["plan"] = req.Plan
	}
	if req.TrialDays > 0 {
		trialEnds := time.Now().Add(time.Duration(req.TrialDays) * 24 * time.Hour)
		updates["trial_ends_at"] = &trialEnds
		updates["status"] = "active"
		updates["plan"] = "trial"
	}
	if req.MatchMode != "" {
		updates["match_mode"] = normalizeBotMatchMode(req.MatchMode)
	}
	if req.KeywordLimit != nil {
		updates["keyword_limit"] = maxInt(*req.KeywordLimit, 0)
	}
	if req.PushChatID != nil {
		updates["push_chat_id"] = strings.TrimSpace(*req.PushChatID)
	}
	setBoolUpdate(updates, "push_enabled", req.PushEnabled)
	setBoolUpdate(updates, "user_blacklist_enabled", req.UserBlacklistEnabled)
	setBoolUpdate(updates, "risk_control_enabled", req.RiskControlEnabled)
	if req.MessageDedupMinutes != nil {
		updates["message_dedup_minutes"] = maxInt(*req.MessageDedupMinutes, 0)
	}
	if req.DMQuotaTotal != nil {
		updates["dm_quota_total"] = clampMinInt64(*req.DMQuotaTotal, 0)
	}
	if req.DMQuotaUsed != nil {
		used := clampMinInt64(*req.DMQuotaUsed, 0)
		if req.DMQuotaTotal != nil && used > clampMinInt64(*req.DMQuotaTotal, 0) {
			used = clampMinInt64(*req.DMQuotaTotal, 0)
		}
		updates["dm_quota_used"] = used
	}
	if req.PrivateTerminalGroupIDs != nil {
		raw, _ := json.Marshal(req.PrivateTerminalGroupIDs)
		updates["private_terminal_group_ids"] = datatypes.JSON(raw)
	}
	if err := s.db.WithContext(c.Request.Context()).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(updates).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "更新 Bot 用户失败")
		return
	}
	utils.OK(c, gin.H{"id": subscriber.ID})
}

func (s *Server) loadBotSubscriberParam(c *gin.Context) (models.BotSubscriber, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Bot 用户 ID 无效")
		return models.BotSubscriber{}, false
	}
	var subscriber models.BotSubscriber
	if err := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).First(&subscriber).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "Bot 用户不存在")
		return models.BotSubscriber{}, false
	}
	return subscriber, true
}

func setBoolUpdate(updates map[string]any, key string, value *bool) {
	if value != nil {
		updates[key] = *value
	}
}

func clampMinInt64(value int64, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}

func jsonStringList(value []string) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
