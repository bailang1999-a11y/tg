package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codex3/backend/internal/middleware"
	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type terminalCheckSummary struct {
	TaskID   uuid.UUID                 `json:"task_id"`
	Total    int                       `json:"total"`
	Online   int                       `json:"online"`
	Offline  int                       `json:"offline"`
	Abnormal int                       `json:"abnormal"`
	Items    []terminalCheckResultItem `json:"items"`
}

type terminalCheckResultItem struct {
	TerminalID   string     `json:"terminal_id"`
	Phone        string     `json:"phone,omitempty"`
	Nickname     string     `json:"nickname,omitempty"`
	Bio          string     `json:"bio,omitempty"`
	Homepage     string     `json:"homepage,omitempty"`
	Status       string     `json:"status"`
	LastOnlineAt *time.Time `json:"last_online_at,omitempty"`
	Reason       string     `json:"reason,omitempty"`
}

func (s *Server) tenantID(c *gin.Context) uuid.UUID {
	if claims := middleware.CurrentClaims(c); claims != nil {
		return claims.TenantID
	}
	return uuid.Nil
}

func (s *Server) userIDPtr(c *gin.Context) *uuid.UUID {
	if claims := middleware.CurrentClaims(c); claims != nil {
		return &claims.UserID
	}
	return nil
}

func (s *Server) ListGroups(c *gin.Context) {
	resource := c.Param("resource")
	if resource == "avatar" {
		s.ListAvatarGroups(c)
		return
	}
	var groups []models.Group
	query := s.db.WithContext(c.Request.Context()).Where("resource_type = ?", resource)
	query = s.applyTenantAccess(c, query)
	if err := query.Order("created_at desc").Find(&groups).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取分组失败")
		return
	}
	utils.OK(c, groups)
}

func (s *Server) CreateGroup(c *gin.Context) {
	resource := c.Param("resource")
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入分组名称")
		return
	}
	group := models.Group{
		ID:           uuid.New(),
		TenantID:     s.tenantID(c),
		ResourceType: resource,
		Name:         req.Name,
		Description:  req.Description,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&group).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建分组失败")
		return
	}
	utils.Created(c, group)
}

func (s *Server) RenameGroup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "分组 ID 无效")
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入分组名称")
		return
	}
	query := s.db.WithContext(c.Request.Context()).Model(&models.Group{}).Where("id = ? AND resource_type = ?", id, c.Param("resource"))
	query = s.applyTenantAccess(c, query)
	if err := query.Update("name", req.Name).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "重命名分组失败")
		return
	}
	utils.OK(c, gin.H{"id": id, "name": req.Name})
}

func (s *Server) DeleteGroup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "分组 ID 无效")
		return
	}
	if c.Param("resource") == "avatar" {
		s.DeleteAvatarGroup(c, id)
		return
	}
	if c.Param("resource") == "target" {
		if err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			targetQuery := tx.Model(&models.Target{}).Where("group_id = ?", id)
			targetQuery = s.applyTenantAccess(c, targetQuery)
			if err := targetQuery.Update("group_id", nil).Error; err != nil {
				return err
			}
			bindingQuery := tx.Where("group_id = ?", id)
			bindingQuery = s.applyTenantAccess(c, bindingQuery)
			if err := bindingQuery.Delete(&models.TargetGroupBinding{}).Error; err != nil {
				return err
			}
			groupQuery := tx.Where("id = ? AND resource_type = ?", id, c.Param("resource"))
			groupQuery = s.applyTenantAccess(c, groupQuery)
			return groupQuery.Delete(&models.Group{}).Error
		}); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "删除分组失败")
			return
		}
		utils.OK(c, gin.H{"deleted": id})
		return
	}
	query := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", id, c.Param("resource"))
	query = s.applyTenantAccess(c, query)
	if err := query.Delete(&models.Group{}).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除分组失败")
		return
	}
	utils.OK(c, gin.H{"deleted": id})
}

func (s *Server) ListTerminals(c *gin.Context) {
	var items []models.Terminal
	query := s.db.WithContext(c.Request.Context()).Order("created_at desc")
	query = s.applyTenantAccess(c, query)
	if group := c.Query("group_id"); group != "" {
		query = query.Where("group_id = ?", group)
	}
	if err := query.Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取终端失败")
		return
	}

	result := make([]terminalListItem, 0, len(items))
	for _, item := range items {
		normalizedPhone, originCountry, originFlag := syncTerminalPhoneIdentity(item.Phone, item.OriginCountry, item.OriginFlag)
		if normalizedPhone != item.Phone || originCountry != item.OriginCountry || originFlag != item.OriginFlag {
			_ = s.db.WithContext(c.Request.Context()).Model(&models.Terminal{}).Where("id = ?", item.ID).Updates(map[string]any{
				"phone":          normalizedPhone,
				"origin_country": originCountry,
				"origin_flag":    originFlag,
			}).Error
			item.Phone = normalizedPhone
			item.OriginCountry = originCountry
			item.OriginFlag = originFlag
		}
		result = append(result, buildTerminalListItem(item))
	}

	utils.OK(c, result)
}

func (s *Server) DeleteTerminal(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "终端 ID 无效")
		return
	}
	if err := s.db.WithContext(c.Request.Context()).Delete(&models.Terminal{}, "id = ?", id).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除终端失败")
		return
	}
	s.markAccountJoinRecordsUnavailable(c.Request.Context(), uuid.Nil, accountJoinKindTerminal, id, "账号已删除")
	utils.OK(c, gin.H{"deleted": id})
}

func (s *Server) ListWorkflows(c *gin.Context) {
	var items []models.Workflow
	if err := s.db.WithContext(c.Request.Context()).Order("created_at desc").Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取工作流失败")
		return
	}
	utils.OK(c, items)
}

func (s *Server) CreateWorkflow(c *gin.Context) {
	var req struct {
		Name        string         `json:"name" binding:"required"`
		Description string         `json:"description"`
		Definition  datatypes.JSON `json:"definition"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "工作流名称不能为空")
		return
	}
	workflow := models.Workflow{
		ID:          uuid.New(),
		TenantID:    s.tenantID(c),
		Name:        req.Name,
		Description: req.Description,
		Definition:  req.Definition,
		Status:      "draft",
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&workflow).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建工作流失败")
		return
	}
	utils.Created(c, workflow)
}

func (s *Server) CreateCheckTerminalsTask(c *gin.Context) {
	var req struct {
		GroupID    string `json:"group_id"`
		TerminalID string `json:"terminal_id"`
	}
	_ = c.ShouldBindJSON(&req)

	groupID, _, err := validateTerminalCheckSelection(req.GroupID, req.TerminalID)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      "一键检查终端状态",
		Type:      "account_status_check",
		Status:    "queued",
		Progress:  0,
		CreatedBy: s.userIDPtr(c),
	}
	if groupID != nil {
		task.TerminalGroupID = groupID
	}
	payloadInfo := map[string]string{
		"group_id":    req.GroupID,
		"terminal_id": req.TerminalID,
	}
	payloadBytes, _ := json.Marshal(payloadInfo)
	task.Payload = datatypes.JSON(payloadBytes)

	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建状态检查任务失败")
		return
	}
	s.logTask(c, task, "INFO", "created", "终端状态检查任务已创建，等待执行器消费")
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.RunCheckTerminalsTask(task.ID)
	}
	utils.Created(c, gin.H{"task": task})
}

func (s *Server) RunCheckTerminalsTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "account_status_check")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "account_status_check").First(&task).Error; err != nil {
		return
	}
	var payload struct {
		GroupID    string `json:"group_id"`
		TerminalID string `json:"terminal_id"`
	}
	_ = json.Unmarshal(task.Payload, &payload)

	groupID, terminalID, err := validateTerminalCheckSelection(payload.GroupID, payload.TerminalID)
	if err != nil {
		s.failCheckTerminalsTask(ctx, task, err.Error())
		return
	}
	query := s.db.WithContext(ctx).Model(&models.Terminal{}).Where("tenant_id = ?", task.TenantID).Order("created_at desc")
	if terminalID != nil {
		query = query.Where("id = ?", *terminalID)
	} else if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
		task.TerminalGroupID = groupID
	}

	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		s.failCheckTerminalsTask(ctx, task, "读取终端失败："+err.Error())
		return
	}

	s.updateTaskState(ctx, task.ID, "running", 5, nil)
	summary := terminalCheckSummary{
		TaskID: task.ID,
		Items:  []terminalCheckResultItem{},
	}
	now := time.Now()
	syncer := telegram_client.NewInspector(s.cfg)
	tenantID := task.TenantID
	s.logTaskBackground(ctx, task, "INFO", "start", "开始检查终端状态")
	for _, terminal := range terminals {
		summary.Total++
		status, reason, lastOnline, riskStatus, banStatus := assessTerminalLocally(terminal, now)
		phone := terminal.Phone
		nickname := terminal.Nickname
		bio := terminal.Bio
		homepage := terminal.Homepage
		avatarURL := terminal.AvatarURL

		if terminal.FilePath != "" && isStoredTerminalFileReady(terminal.FilePath) {
			avatarDir, avatarDirErr := prepareTerminalAvatarSyncDir(tenantID, terminal.ID)
			if avatarDirErr != nil {
				reason = firstNonEmpty(reason, "准备头像缓存目录失败")
			}
			syncResult, syncErr := syncer.Sync(ctx, telegram_client.SyncRequest{
				FilePath:   terminal.FilePath,
				AccessType: terminal.AccessType,
				AvatarDir:  avatarDir,
			})
			if syncErr == nil {
				status = firstNonEmpty(syncResult.Status, status)
				reason = firstNonEmpty(syncResult.Reason, reason)
				if syncResult.LastOnlineAt != nil {
					lastOnline = syncResult.LastOnlineAt
				} else if syncResult.Status == "abnormal" {
					lastOnline = nil
				} else if syncResult.Status != "online" {
					lastOnline = terminal.LastOnlineAt
				}
				phone = firstNonEmpty(syncResult.Phone, phone)
				nickname = firstNonEmpty(syncResult.Nickname, nickname)
				bio = syncResult.Bio
				homepage = syncResult.Homepage
				avatarURL, reason = s.persistTerminalAvatar(tenantID, terminal, avatarURL, syncResult, reason)
				riskStatus = mergeTerminalRiskStatus(riskStatus, syncResult.RiskStatus)
				banStatus = mergeTerminalBanStatus(banStatus, syncResult.BanStatus)
			} else if strings.TrimSpace(syncResult.Reason) != "" {
				status = firstNonEmpty(syncResult.Status, status)
				reason = syncResult.Reason
				if syncResult.LastOnlineAt != nil {
					lastOnline = syncResult.LastOnlineAt
				} else if syncResult.Status == "abnormal" {
					lastOnline = nil
				} else if syncResult.Status != "online" {
					lastOnline = terminal.LastOnlineAt
				}
				phone = firstNonEmpty(syncResult.Phone, phone)
				nickname = firstNonEmpty(syncResult.Nickname, nickname)
				bio = firstNonEmpty(syncResult.Bio, bio)
				homepage = firstNonEmpty(syncResult.Homepage, homepage)
				riskStatus = mergeTerminalRiskStatus(riskStatus, syncResult.RiskStatus)
				banStatus = mergeTerminalBanStatus(banStatus, syncResult.BanStatus)
			} else {
				reason = reason + "；账号资料同步失败"
			}
			if avatarDir != "" {
				_ = os.RemoveAll(avatarDir)
			}
		}

		countTerminalStatus(&summary, status)
		normalizedPhone, originCountry, originFlag := syncTerminalPhoneIdentity(phone, terminal.OriginCountry, terminal.OriginFlag)

		if err := s.db.WithContext(ctx).Model(&models.Terminal{}).Where("tenant_id = ? AND id = ?", task.TenantID, terminal.ID).Updates(map[string]any{
			"phone":          normalizedPhone,
			"nickname":       nickname,
			"avatar_url":     avatarURL,
			"bio":            bio,
			"homepage":       homepage,
			"status":         status,
			"last_online_at": lastOnline,
			"origin_country": originCountry,
			"origin_flag":    originFlag,
			"risk_status":    riskStatus,
			"ban_status":     banStatus,
		}).Error; err != nil {
			s.failCheckTerminalsTask(ctx, task, "更新终端状态失败："+err.Error())
			return
		}
		updatedTerminal := terminal
		updatedTerminal.Phone = normalizedPhone
		updatedTerminal.Nickname = nickname
		updatedTerminal.AvatarURL = avatarURL
		updatedTerminal.Bio = bio
		updatedTerminal.Homepage = homepage
		updatedTerminal.Status = status
		updatedTerminal.LastOnlineAt = lastOnline
		updatedTerminal.OriginCountry = originCountry
		updatedTerminal.OriginFlag = originFlag
		updatedTerminal.RiskStatus = riskStatus
		updatedTerminal.BanStatus = banStatus
		if !terminalReadyForOutboundAction(updatedTerminal) {
			removed := s.markAccountJoinRecordsUnavailable(ctx, task.TenantID, accountJoinKindTerminal, terminal.ID, reason)
			if removed > 0 {
				s.logTaskBackground(ctx, task, "WARN", "membership_pruned", fmt.Sprintf("%s 已不可用，已从 %d 条目标群有效账号状态中移除", terminalDisplayName(updatedTerminal), removed))
			}
		}

		summary.Items = append(summary.Items, terminalCheckResultItem{
			TerminalID:   terminal.ID.String(),
			Phone:        formatTerminalPhoneDisplay(normalizedPhone),
			Nickname:     nickname,
			Bio:          bio,
			Homepage:     homepage,
			Status:       status,
			LastOnlineAt: lastOnline,
			Reason:       reason,
		})
		s.logTaskBackground(ctx, task, "INFO", "check", terminalDisplayName(terminal)+" -> "+status+"（"+reason+"）")
	}

	summaryBytes, _ := json.Marshal(summary)
	if err := s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":            "success",
		"progress":          100,
		"terminal_group_id": task.TerminalGroupID,
		"summary":           datatypes.JSON(summaryBytes),
	}).Error; err != nil {
		s.failCheckTerminalsTask(ctx, task, "更新状态检查汇总失败："+err.Error())
		return
	}

	s.logTaskBackground(ctx, task, "INFO", "summary", "状态检查完成")
}

func (s *Server) failCheckTerminalsTask(ctx context.Context, task models.Task, reason string) {
	summaryBytes, _ := json.Marshal(terminalCheckSummary{
		TaskID: task.ID,
		Items:  []terminalCheckResultItem{},
	})
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":   "failed",
		"progress": 100,
		"summary":  datatypes.JSON(summaryBytes),
	}).Error
	s.logTaskBackground(ctx, task, "ERROR", "failed", reason)
}

func prepareTerminalAvatarSyncDir(tenantID, terminalID uuid.UUID) (string, error) {
	base := filepath.Join("storage", "uploads", tenantID.String(), "terminal-avatar-sync")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, sanitizeFilename(terminalID.String())+"-")
}

func (s *Server) persistTerminalAvatar(tenantID uuid.UUID, terminal models.Terminal, currentAvatarURL string, syncResult telegram_client.SyncResult, reason string) (string, string) {
	if !syncResult.AvatarChecked {
		return currentAvatarURL, appendTerminalReason(reason, syncResult.AvatarError)
	}

	if strings.TrimSpace(syncResult.AvatarError) != "" {
		return currentAvatarURL, appendTerminalReason(reason, "头像回拉失败："+strings.TrimSpace(syncResult.AvatarError))
	}

	oldAvatarPath := storedPathFromPublicURL(currentAvatarURL)
	if !syncResult.AvatarPresent || strings.TrimSpace(syncResult.AvatarPath) == "" {
		if oldAvatarPath != "" {
			_ = removeStoredAssetFile(oldAvatarPath)
		}
		return "", reason
	}

	data, err := os.ReadFile(syncResult.AvatarPath)
	if err != nil {
		return currentAvatarURL, appendTerminalReason(reason, "读取回拉头像失败")
	}

	savedPath, err := saveUploadedBytes(tenantID, "terminal-avatars", filepath.Base(syncResult.AvatarPath), data)
	if err != nil {
		return currentAvatarURL, appendTerminalReason(reason, "保存回拉头像失败")
	}

	if oldAvatarPath != "" && oldAvatarPath != savedPath {
		_ = removeStoredAssetFile(oldAvatarPath)
	}

	return "/" + filepath.ToSlash(savedPath), reason
}

func storedPathFromPublicURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "/storage/uploads/") {
		clean := filepath.Clean(strings.TrimPrefix(value, "/"))
		if strings.HasPrefix(filepath.ToSlash(clean), "storage/uploads/") {
			return clean
		}
	}
	return ""
}

func appendTerminalReason(current string, extra string) string {
	current = strings.TrimSpace(current)
	extra = strings.TrimSpace(extra)
	switch {
	case current == "":
		return extra
	case extra == "":
		return current
	case strings.Contains(current, extra):
		return current
	default:
		return current + "；" + extra
	}
}

func (s *Server) CreateNetworkTestTask(c *gin.Context) {
	s.createNamedTask(c, "network_batch_test", "网络节点批量测试", "已创建批量测试任务")
}

func (s *Server) RunWorkflow(c *gin.Context) {
	workflowID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "工作流 ID 无效")
		return
	}

	var workflow models.Workflow
	if err := s.db.WithContext(c.Request.Context()).First(&workflow, "id = ?", workflowID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Fail(c, http.StatusNotFound, "工作流不存在")
			return
		}
		utils.Fail(c, http.StatusInternalServerError, "读取工作流失败")
		return
	}

	payload, _ := json.Marshal(gin.H{
		"workflow_id":   workflow.ID,
		"workflow_name": workflow.Name,
	})
	settings := s.readSystemSettings(c.Request.Context(), s.tenantID(c))

	detail := "已创建工作流执行任务「" + workflow.Name + "」"
	if settings.Adapter.WorkflowDryRun {
		detail += "，当前按 dry-run 排队"
	} else {
		detail += "，已进入执行队列"
	}

	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      "通知工作流执行 - " + workflow.Name,
		Type:      "workflow_execution",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payload),
		CreatedBy: s.userIDPtr(c),
	}

	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		return tx.Create(&models.TaskLog{
			ID:        uuid.New(),
			TenantID:  task.TenantID,
			TaskID:    task.ID,
			Level:     "INFO",
			Category:  task.Type,
			Action:    "created",
			Details:   detail,
			TraceID:   uuid.NewString(),
			CreatedAt: time.Now(),
		}).Error
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建工作流任务失败")
		return
	}

	utils.Created(c, task)
}

func (s *Server) CreateOutreachTask(c *gin.Context) {
	var req struct {
		Name             string `json:"name"`
		JobType          string `json:"job_type"`
		TerminalGroupID  string `json:"terminal_group_id"`
		TargetGroupID    string `json:"target_group_id"`
		Keyword          string `json:"keyword"`
		Message          string `json:"message"`
		SyncProfile      bool   `json:"sync_profile"`
		ContentCleanup   bool   `json:"content_cleanup"`
		ComplianceReview bool   `json:"compliance_review"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "主动触达任务参数格式不正确")
		return
	}

	req.JobType = normalizeOutreachJobType(req.JobType)
	req.Name = strings.TrimSpace(req.Name)
	req.Keyword = strings.TrimSpace(req.Keyword)
	req.Message = strings.TrimSpace(req.Message)

	if req.JobType == "" {
		req.JobType = "bulk_message"
	}
	if req.Name == "" {
		req.Name = defaultOutreachTaskName(req.JobType)
	}
	if needsOutreachMessage(req.JobType) && req.Message == "" {
		utils.Fail(c, http.StatusBadRequest, "请填写触达内容")
		return
	}
	if req.JobType == "keyword_reply" && req.Keyword == "" {
		utils.Fail(c, http.StatusBadRequest, "关键词自动响应需要填写关键词")
		return
	}

	terminalGroupID, err := parseOptionalUUID(req.TerminalGroupID, "终端组 ID 无效")
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	targetGroupID, err := parseOptionalUUID(req.TargetGroupID, "目标组 ID 无效")
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	settings := s.readSystemSettings(c.Request.Context(), s.tenantID(c))
	payload, _ := json.Marshal(gin.H{
		"job_type":           req.JobType,
		"keyword":            req.Keyword,
		"message":            req.Message,
		"sync_profile":       req.SyncProfile,
		"content_cleanup":    req.ContentCleanup,
		"compliance_review":  req.ComplianceReview,
		"outreach_dry_run":   settings.Adapter.OutreachDryRun,
		"adapter_enabled":    settings.Adapter.TelegramApplyEnabled,
		"require_admin_gate": settings.Security.RequireAdminApproval,
	})

	task := models.Task{
		ID:              uuid.New(),
		TenantID:        s.tenantID(c),
		Name:            req.Name,
		Type:            "event_outreach",
		TerminalGroupID: terminalGroupID,
		TargetGroupID:   targetGroupID,
		Status:          "queued",
		Progress:        0,
		Payload:         datatypes.JSON(payload),
		CreatedBy:       s.userIDPtr(c),
	}

	detailParts := []string{"已创建" + outreachJobTypeLabel(req.JobType)}
	if settings.Adapter.OutreachDryRun {
		detailParts = append(detailParts, "当前按 dry-run 排队")
	} else {
		detailParts = append(detailParts, "当前允许进入执行队列")
	}
	if settings.Security.RequireAdminApproval || req.ComplianceReview {
		detailParts = append(detailParts, "需管理员复核")
	}
	if !settings.Adapter.TelegramApplyEnabled {
		detailParts = append(detailParts, "Telegram 执行适配器未开启")
	}

	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		return tx.Create(&models.TaskLog{
			ID:        uuid.New(),
			TenantID:  task.TenantID,
			TaskID:    task.ID,
			Level:     "INFO",
			Category:  task.Type,
			Action:    "created",
			Details:   strings.Join(detailParts, "；"),
			TraceID:   uuid.NewString(),
			CreatedAt: time.Now(),
		}).Error
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建主动触达任务失败")
		return
	}

	utils.Created(c, task)
}

func (s *Server) createNamedTask(c *gin.Context, taskType, name, detail string) {
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      name,
		Type:      taskType,
		Status:    "queued",
		Progress:  0,
		CreatedBy: s.userIDPtr(c),
	}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		return tx.Create(&models.TaskLog{
			ID:        uuid.New(),
			TenantID:  task.TenantID,
			TaskID:    task.ID,
			Level:     "INFO",
			Category:  taskType,
			Action:    "created",
			Details:   detail,
			TraceID:   uuid.NewString(),
			CreatedAt: time.Now(),
		}).Error
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建任务失败")
		return
	}
	utils.Created(c, task)
}

func isStoredTerminalFileReady(path string) bool {
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return false
	}
	info, err := os.Stat(clean)
	return err == nil && !info.IsDir()
}

func terminalDisplayName(terminal models.Terminal) string {
	if terminal.Nickname != "" {
		return terminal.Nickname
	}
	if terminal.Phone != "" {
		return terminal.Phone
	}
	return terminal.ID.String()
}

func assessTerminalLocally(terminal models.Terminal, checkedAt time.Time) (string, string, *time.Time, string, string) {
	riskStatus := strings.TrimSpace(terminal.RiskStatus)
	banStatus := strings.TrimSpace(terminal.BanStatus)
	if riskStatus == "" {
		riskStatus = "正常"
	}
	if banStatus == "" {
		banStatus = "正常"
	}

	switch {
	case terminal.FilePath == "":
		return "abnormal", "缺少本地会话文件", terminal.LastOnlineAt, "需重新导入", banStatus
	case !isStoredTerminalFileReady(terminal.FilePath):
		return "abnormal", "本地会话文件不存在", terminal.LastOnlineAt, "需重新导入", banStatus
	case strings.TrimSpace(terminal.SessionHash) == "":
		return "offline", "缺少会话哈希", terminal.LastOnlineAt, riskStatus, banStatus
	default:
		lastOnline := checkedAt
		return "online", "会话文件存在", &lastOnline, riskStatus, banStatus
	}
}

func validateTerminalCheckSelection(groupIDText string, terminalIDText string) (*uuid.UUID, *uuid.UUID, error) {
	groupIDText = strings.TrimSpace(groupIDText)
	terminalIDText = strings.TrimSpace(terminalIDText)

	if groupIDText != "" && terminalIDText != "" {
		return nil, nil, fmt.Errorf("终端组和终端不能同时选择")
	}

	if terminalIDText != "" {
		terminalID, err := uuid.Parse(terminalIDText)
		if err != nil {
			return nil, nil, fmt.Errorf("终端 ID 无效")
		}
		return nil, &terminalID, nil
	}

	if groupIDText != "" {
		groupID, err := uuid.Parse(groupIDText)
		if err != nil {
			return nil, nil, fmt.Errorf("终端组 ID 无效")
		}
		return &groupID, nil, nil
	}

	return nil, nil, nil
}

func countTerminalStatus(summary *terminalCheckSummary, status string) {
	switch status {
	case "online":
		summary.Online++
	case "offline":
		summary.Offline++
	default:
		summary.Abnormal++
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseOptionalUUID(text string, invalidMessage string) (*uuid.UUID, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}

	parsed, err := uuid.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%s", invalidMessage)
	}
	return &parsed, nil
}

func normalizeOutreachJobType(value string) string {
	switch strings.TrimSpace(value) {
	case "keyword_reply", "member_invite", "bulk_message", "identity_sync", "content_cleanup":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func needsOutreachMessage(jobType string) bool {
	switch jobType {
	case "keyword_reply", "bulk_message":
		return true
	default:
		return false
	}
}

func defaultOutreachTaskName(jobType string) string {
	return "主动触达 - " + outreachJobTypeLabel(jobType)
}

func outreachJobTypeLabel(jobType string) string {
	switch jobType {
	case "keyword_reply":
		return "关键词自动响应"
	case "member_invite":
		return "授权成员邀请"
	case "identity_sync":
		return "身份同步"
	case "content_cleanup":
		return "内容清洗"
	default:
		return "批量消息"
	}
}
