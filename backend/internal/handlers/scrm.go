package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type scrmRecentSearchItem struct {
	Keyword        string    `json:"keyword"`
	Message        string    `json:"message"`
	SourceChatName string    `json:"source_chat_name"`
	HitAt          time.Time `json:"hit_at"`
}

type scrmLeadResponse struct {
	models.SCRMLead
	HitTime       string                 `json:"hit_time"`
	RecentHistory []scrmRecentSearchItem `json:"recent_history"`
}

func (s *Server) ListSCRMRules(c *gin.Context) {
	var rules []models.SCRMKeywordRule
	query := s.db.WithContext(c.Request.Context()).Order("updated_at desc")
	query = s.applySCRMOwnerScope(c, query)
	if err := query.Find(&rules).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 SCRM 规则失败")
		return
	}
	utils.OK(c, rules)
}

func (s *Server) CreateSCRMRule(c *gin.Context) {
	var input struct {
		ID       string `json:"id"`
		Name     string `json:"name" binding:"required"`
		Keywords struct {
			List []string `json:"list"`
			Text string   `json:"text"`
		} `json:"keywords" binding:"required"`
		ListenGroupID      string   `json:"listen_group_id"`
		StrikeGroupID      string   `json:"strike_group_id"`
		MonitorGroupID     string   `json:"monitor_group_id"`
		MonitorTerminalIDs []string `json:"monitor_terminal_ids"`
		MatchMode          string   `json:"match_mode"`
		PushToBot          bool     `json:"push_to_bot"`
		StrikeEnabled      *bool    `json:"strike_enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Fail(c, http.StatusBadRequest, "参数无效")
		return
	}

	keywordList := collectSCRMKeywords(input.Keywords.List, input.Keywords.Text)
	if len(keywordList) == 0 {
		utils.Fail(c, http.StatusBadRequest, "请至少输入一个关键词")
		return
	}

	var listenGroupID *uuid.UUID
	var strikeGroupID *uuid.UUID
	var monitorGroupID *uuid.UUID
	var err error

	matchMode := normalizeSCRMMatchMode(input.MatchMode)
	strikeEnabled := false
	rawMonitorTerminalIDs, err := json.Marshal([]string{})
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "监听号格式无效")
		return
	}
	rawKeywords, err := json.Marshal(gin.H{
		"list": keywordList,
		"text": strings.Join(keywordList, "\n"),
	})
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "关键词规则格式无效")
		return
	}

	tenantID := s.tenantID(c)
	name := strings.TrimSpace(input.Name)
	now := time.Now()
	ownerUserID := s.userIDPtr(c)

	var rule models.SCRMKeywordRule
	tx := s.db.WithContext(c.Request.Context())
	if strings.TrimSpace(input.ID) != "" {
		ruleID, parseErr := uuid.Parse(strings.TrimSpace(input.ID))
		if parseErr != nil {
			utils.Fail(c, http.StatusBadRequest, "监听任务 ID 无效")
			return
		}
		err = s.applySCRMOwnerScope(c, tx.Where("id = ?", ruleID)).First(&rule).Error
	} else {
		err = tx.Where("tenant_id = ? AND name = ?", tenantID, name).First(&rule).Error
	}
	switch {
	case err == nil:
		if rule.OwnerUserID != nil {
			ownerUserID = rule.OwnerUserID
		} else if rule.TenantID != uuid.Nil {
			tenantOwnerID := rule.TenantID
			ownerUserID = &tenantOwnerID
		}
		rule.ListenGroupID = listenGroupID
		rule.StrikeGroupID = strikeGroupID
		rule.MonitorGroupID = monitorGroupID
		rule.MonitorTerminalIDs = datatypes.JSON(rawMonitorTerminalIDs)
		rule.Keywords = datatypes.JSON(rawKeywords)
		rule.MatchMode = matchMode
		rule.PushToBot = input.PushToBot
		rule.StrikeEnabled = strikeEnabled
		rule.Status = "active"
		rule.OwnerUserID = ownerUserID
		rule.UpdatedAt = now
		if err := tx.Save(&rule).Error; err != nil {
			utils.Fail(c, http.StatusInternalServerError, "更新 SCRM 规则失败")
			return
		}
		utils.OK(c, rule)
	case errors.Is(err, gorm.ErrRecordNotFound):
		rule = models.SCRMKeywordRule{
			ID:                 uuid.New(),
			TenantID:           tenantID,
			OwnerUserID:        ownerUserID,
			Name:               name,
			ListenGroupID:      listenGroupID,
			StrikeGroupID:      strikeGroupID,
			MonitorGroupID:     monitorGroupID,
			MonitorTerminalIDs: datatypes.JSON(rawMonitorTerminalIDs),
			Keywords:           datatypes.JSON(rawKeywords),
			MatchMode:          matchMode,
			PushToBot:          input.PushToBot,
			StrikeEnabled:      strikeEnabled,
			Status:             "active",
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := tx.Create(&rule).Error; err != nil {
			utils.Fail(c, http.StatusInternalServerError, "创建 SCRM 规则失败")
			return
		}
		utils.Created(c, rule)
	default:
		utils.Fail(c, http.StatusInternalServerError, "读取 SCRM 规则失败")
	}
}

func (s *Server) DeleteSCRMRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "规则 ID 无效")
		return
	}

	result := s.db.WithContext(c.Request.Context()).
		Where("id = ?", ruleID)
	result = s.applySCRMOwnerScope(c, result).Delete(&models.SCRMKeywordRule{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除 SCRM 规则失败")
		return
	}
	if result.RowsAffected == 0 {
		utils.Fail(c, http.StatusNotFound, "未找到对应规则")
		return
	}
	utils.OK(c, gin.H{"deleted": ruleID})
}

func (s *Server) ListSCRMLeads(c *gin.Context) {
	var leads []models.SCRMLead
	query := s.db.WithContext(c.Request.Context()).Order("COALESCE(hit_at, created_at) desc")
	query = s.applySCRMOwnerScope(c, query)
	if !truthyQuery(c.Query("include_blacklisted")) {
		query = query.Where("(status IS NULL OR status <> ?)", "blacklisted")
	}
	if taskIDText := strings.TrimSpace(c.Query("task_id")); taskIDText != "" && !strings.EqualFold(taskIDText, "all") {
		taskID, err := uuid.Parse(taskIDText)
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "监听任务 ID 无效")
			return
		}
		query = query.Where("source_task_id = ?", taskID)
	}
	if err := query.Find(&leads).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "检索客资追踪源失败")
		return
	}

	responses := make([]scrmLeadResponse, 0, len(leads))
	for _, lead := range leads {
		recentHistory, err := s.resolveSCRMRecentHistory(c, lead)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "整理客资搜索记录失败")
			return
		}
		responses = append(responses, scrmLeadResponse{
			SCRMLead:      lead,
			HitTime:       scrmLeadHitTime(lead).Format(time.RFC3339),
			RecentHistory: recentHistory,
		})
	}

	utils.OK(c, responses)
}

func (s *Server) BlacklistSCRMLeadUser(c *gin.Context) {
	leadID, err := uuid.Parse(c.Param("lead_id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "线索 ID 无效")
		return
	}
	lead, ok := s.loadScopedSCRMLead(c, leadID)
	if !ok {
		return
	}
	if lead.SourceTaskID == nil || *lead.SourceTaskID == uuid.Nil {
		utils.Fail(c, http.StatusBadRequest, "线索缺少监听任务，无法加入任务黑名单")
		return
	}

	userKey := scrmLeadUserKey(lead)
	if userKey == "" {
		utils.Fail(c, http.StatusBadRequest, "线索缺少用户身份，无法加入黑名单")
		return
	}

	now := time.Now()
	targetID := lead.TargetID
	account := normalizeSCRMLeadAccount(lead.UserAccount)
	blacklist := models.SCRMTaskUserBlacklist{
		ID:           uuid.New(),
		TenantID:     lead.TenantID,
		TaskID:       *lead.SourceTaskID,
		OwnerUserID:  lead.OwnerUserID,
		UserKey:      userKey,
		UserAccount:  account,
		UserNickname: strings.TrimSpace(lead.UserNickname),
		TargetID:     &targetID,
		SourceLeadID: &lead.ID,
		CreatedBy:    s.userIDPtr(c),
		Reason:       "lead_card_block",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var existing models.SCRMTaskUserBlacklist
		query := tx.Where("tenant_id = ? AND task_id = ?", blacklist.TenantID, blacklist.TaskID)
		if targetID != uuid.Nil {
			query = query.Where("(user_key = ? OR target_id = ?)", userKey, targetID)
		} else {
			query = query.Where("user_key = ?", userKey)
		}
		if err := query.First(&existing).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Create(&blacklist).Error; err != nil {
				return err
			}
		} else {
			existing.UserKey = userKey
			existing.UserAccount = account
			existing.UserNickname = strings.TrimSpace(lead.UserNickname)
			existing.TargetID = &targetID
			existing.SourceLeadID = &lead.ID
			existing.UpdatedAt = now
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			blacklist = existing
		}

		return tx.Model(&models.SCRMLead{}).
			Where("id = ? AND tenant_id = ?", lead.ID, lead.TenantID).
			Updates(map[string]any{
				"status":     "blacklisted",
				"updated_at": now,
			}).Error
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "加入任务黑名单失败")
		return
	}

	utils.OK(c, gin.H{
		"status":    "ok",
		"lead_id":   lead.ID,
		"task_id":   *lead.SourceTaskID,
		"user_key":  userKey,
		"blacklist": blacklist,
	})
}

func (s *Server) ListSCRMMessages(c *gin.Context) {
	var messages []models.SCRMMessage
	leadID, err := uuid.Parse(c.Param("lead_id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "线索 ID 无效")
		return
	}
	lead, ok := s.loadScopedSCRMLead(c, leadID)
	if !ok {
		return
	}
	if err := s.db.Where("tenant_id = ? AND lead_id = ?", lead.TenantID, leadID).Order("message_time asc").Find(&messages).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "拉取客服通讯记录失败")
		return
	}
	utils.OK(c, messages)
}

func (s *Server) SendSCRMMessage(c *gin.Context) {
	var input struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Fail(c, http.StatusBadRequest, "发送内容不能为空")
		return
	}
	leadID, err := uuid.Parse(c.Param("lead_id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "线索 ID 无效")
		return
	}

	var lead models.SCRMLead
	var ok bool
	lead, ok = s.loadScopedSCRMLead(c, leadID)
	if !ok {
		return
	}
	tenantID := lead.TenantID

	now := time.Now()
	message := models.SCRMMessage{
		ID:          uuid.New(),
		TenantID:    tenantID,
		LeadID:      leadID,
		SenderType:  "user",
		TerminalID:  lead.AssignedWorker,
		Content:     strings.TrimSpace(input.Content),
		IsRead:      true,
		MessageTime: now,
		CreatedAt:   now,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&message).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "消息入库失败")
		return
	}

	if err := s.db.WithContext(c.Request.Context()).
		Model(&models.SCRMLead{}).
		Where("id = ? AND tenant_id = ?", leadID, tenantID).
		Updates(map[string]any{
			"status":     "replied",
			"updated_at": now,
		}).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "线索状态更新失败")
		return
	}

	utils.OK(c, gin.H{"message": message, "status": "queued"})
}

func collectSCRMKeywords(list []string, text string) []string {
	items := make([]string, 0, len(list)+8)
	items = append(items, list...)
	if strings.TrimSpace(text) != "" {
		items = append(items, strings.Split(text, "\n")...)
	}

	seen := map[string]struct{}{}
	keywords := make([]string, 0, len(items))
	for _, item := range items {
		keyword := strings.TrimSpace(item)
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	return keywords
}

func normalizeSCRMMatchMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "exact":
		return "exact"
	default:
		return "fuzzy"
	}
}

func truthyQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func normalizeSCRMLeadAccount(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.TrimPrefix(value, "@")
}

func scrmLeadUserKey(lead models.SCRMLead) string {
	return scrmLeadUserKeyFromValues(lead.UserAccount, lead.UserNickname, lead.TargetID)
}

func scrmLeadUserKeyFromValues(account string, nickname string, targetID uuid.UUID) string {
	if normalized := normalizeSCRMLeadAccount(account); normalized != "" {
		return "account:" + normalized
	}
	if targetID != uuid.Nil {
		return "target:" + targetID.String()
	}
	if normalized := normalizeTextForSCRMKey(nickname); normalized != "" {
		return "nick:" + normalized
	}
	return ""
}

func normalizeTextForSCRMKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *Server) scrmTaskUserBlacklisted(ctx context.Context, tenantID uuid.UUID, taskID uuid.UUID, account string, nickname string, targetID uuid.UUID) bool {
	userKey := scrmLeadUserKeyFromValues(account, nickname, targetID)
	if userKey == "" && targetID == uuid.Nil {
		return false
	}
	query := s.db.WithContext(ctx).Model(&models.SCRMTaskUserBlacklist{}).
		Where("tenant_id = ? AND task_id = ?", tenantID, taskID)
	if targetID != uuid.Nil && userKey != "" {
		query = query.Where("(user_key = ? OR target_id = ?)", userKey, targetID)
	} else if targetID != uuid.Nil {
		query = query.Where("target_id = ?", targetID)
	} else {
		query = query.Where("user_key = ?", userKey)
	}
	var count int64
	return query.Count(&count).Error == nil && count > 0
}

func (s *Server) resolveSCRMGroupID(c *gin.Context, rawID string, resourceType string) (*uuid.UUID, error) {
	if rawID == "" {
		return nil, nil
	}
	groupID, err := uuid.Parse(rawID)
	if err != nil {
		return nil, errors.New("分组 ID 无效")
	}
	var group models.Group
	query := s.db.WithContext(c.Request.Context()).
		Where("id = ? AND resource_type = ?", groupID, resourceType)
	query = s.applyTenantAccess(c, query)
	if err := query.First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if resourceType == "target" || resourceType == "listener_target" {
				return nil, errors.New("监听组不存在")
			}
			return nil, errors.New("出击组不存在")
		}
		return nil, err
	}
	return &groupID, nil
}

func (s *Server) resolveSCRMAdminGroupID(c *gin.Context, rawID string, resourceType string, missingMessage string) (*uuid.UUID, error) {
	if rawID == "" {
		return nil, nil
	}
	groupID, err := uuid.Parse(rawID)
	if err != nil {
		return nil, errors.New("分组 ID 无效")
	}
	var group models.Group
	if err := s.db.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ? AND resource_type = ?", groupID, uuid.Nil, resourceType).
		First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(missingMessage)
		}
		return nil, err
	}
	return &groupID, nil
}

func (s *Server) applySCRMOwnerScope(c *gin.Context, query *gorm.DB) *gorm.DB {
	if s.isAdmin(c) {
		userIDText := strings.TrimSpace(c.Query("user_id"))
		if userIDText == "" || strings.EqualFold(userIDText, "all") {
			return query
		}
		userID, err := uuid.Parse(userIDText)
		if err != nil {
			return query.Where("1 = 0")
		}
		return query.Where("(tenant_id = ? OR owner_user_id = ?)", userID, userID)
	}
	userID := s.userIDPtr(c)
	if userID == nil {
		return query.Where("1 = 0")
	}
	return query.Where("(tenant_id = ? OR owner_user_id = ?)", s.tenantID(c), *userID)
}

func (s *Server) loadScopedSCRMLead(c *gin.Context, leadID uuid.UUID) (models.SCRMLead, bool) {
	var lead models.SCRMLead
	query := s.db.WithContext(c.Request.Context()).Where("id = ?", leadID)
	query = s.applySCRMOwnerScope(c, query)
	if err := query.First(&lead).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(c, http.StatusNotFound, "未找到对应线索")
			return models.SCRMLead{}, false
		}
		utils.Fail(c, http.StatusInternalServerError, "读取线索失败")
		return models.SCRMLead{}, false
	}
	return lead, true
}

func (s *Server) resolveSCRMRecentHistory(c *gin.Context, lead models.SCRMLead) ([]scrmRecentSearchItem, error) {
	if len(lead.RecentSearches) > 0 {
		var items []scrmRecentSearchItem
		if err := json.Unmarshal(lead.RecentSearches, &items); err == nil {
			sort.Slice(items, func(i, j int) bool { return items[i].HitAt.After(items[j].HitAt) })
			if len(items) > 5 {
				items = items[:5]
			}
			return items, nil
		}
	}

	query := s.db.WithContext(c.Request.Context()).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -15)).
		Order("COALESCE(hit_at, created_at) desc")
	if lead.OwnerUserID != nil {
		query = query.Where("(tenant_id = ? OR owner_user_id = ?)", lead.TenantID, *lead.OwnerUserID)
	} else {
		query = query.Where("tenant_id = ?", lead.TenantID)
	}

	account := strings.TrimSpace(lead.UserAccount)
	if account != "" {
		query = query.Where("user_account = ?", account)
	} else {
		query = query.Where("target_id = ?", lead.TargetID)
	}

	var historyLeads []models.SCRMLead
	if err := query.Limit(20).Find(&historyLeads).Error; err != nil {
		return nil, err
	}

	items := make([]scrmRecentSearchItem, 0, len(historyLeads))
	for _, item := range historyLeads {
		items = append(items, scrmRecentSearchItem{
			Keyword:        item.TriggerWord,
			Message:        item.TriggerMessage,
			SourceChatName: item.SourceChatName,
			HitAt:          scrmLeadHitTime(item),
		})
	}
	if len(items) > 5 {
		items = items[:5]
	}
	return items, nil
}

func scrmLeadHitTime(lead models.SCRMLead) time.Time {
	if lead.HitAt != nil && !lead.HitAt.IsZero() {
		return *lead.HitAt
	}
	return lead.CreatedAt
}

func (s *Server) resolveSCRMMonitorTerminalIDs(c *gin.Context, rawIDs []string) ([]string, error) {
	if len(rawIDs) == 0 {
		return []string{}, nil
	}

	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(rawIDs))
	parsedIDs := make([]uuid.UUID, 0, len(rawIDs))

	for _, rawID := range rawIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, errors.New("监听号 ID 无效")
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
		parsedIDs = append(parsedIDs, parsed)
	}

	if len(parsedIDs) == 0 {
		return []string{}, nil
	}

	var count int64
	if err := s.db.WithContext(c.Request.Context()).
		Model(&models.ListenerAccount{}).
		Where("tenant_id = ? AND id IN ?", uuid.Nil, parsedIDs).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count != int64(len(parsedIDs)) {
		query := s.db.WithContext(c.Request.Context()).Model(&models.Terminal{}).Where("id IN ?", parsedIDs)
		query = s.applyTenantAccess(c, query)
		if err := query.Count(&count).Error; err != nil {
			return nil, err
		}
	}
	if count != int64(len(parsedIDs)) {
		return nil, errors.New("监听号里包含不存在的账号")
	}

	return normalized, nil
}
