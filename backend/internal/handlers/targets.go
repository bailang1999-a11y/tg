package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var telegramUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)

type targetImportSummary struct {
	Success   int                      `json:"success"`
	Failed    int                      `json:"failed"`
	Duplicate int                      `json:"duplicate"`
	Skipped   int                      `json:"skipped"`
	GroupID   *uuid.UUID               `json:"group_id,omitempty"`
	GroupName string                   `json:"group_name,omitempty"`
	Items     []targetImportResultItem `json:"items"`
}

type targetImportResultItem struct {
	Line       string `json:"line"`
	Identifier string `json:"identifier,omitempty"`
	Type       string `json:"type,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}

type parsedTarget struct {
	Identifier string
	Name       string
	Type       string
}

type targetTerminalImportRequest struct {
	Scope           string `json:"scope" binding:"required"`
	TerminalID      string `json:"terminal_id"`
	TerminalGroupID string `json:"terminal_group_id"`
	GroupID         string `json:"group_id"`
	NewGroupName    string `json:"new_group_name"`
}

type targetListItem struct {
	models.Target
	GroupIDs              []string `json:"group_ids"`
	ActiveMemberCount     int64    `json:"active_member_count"`
	InvalidMemberCount    int64    `json:"invalid_member_count"`
	LastMembershipCheckAt *string  `json:"last_membership_check_at,omitempty"`
}

func (s *Server) ListTargets(c *gin.Context) {
	var items []models.Target
	filterTenantID := s.tenantFilterID(c)
	query := s.db.WithContext(c.Request.Context()).Order("created_at desc")
	query = s.applyTenantAccess(c, query)
	if groupID := c.Query("group_id"); groupID != "" {
		parsed, err := uuid.Parse(groupID)
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "分组 ID 无效")
			return
		}
		query = s.applyTargetGroupFilter(c.Request.Context(), query, filterTenantID, parsed)
	}
	if err := query.Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取目标池失败")
		return
	}
	groupIDsByTarget, err := s.loadTargetGroupIDs(c.Request.Context(), filterTenantID, items)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取目标池分组失败")
		return
	}
	membershipStats := s.loadTargetMembershipStats(c.Request.Context(), filterTenantID, s.isAdmin(c), accountJoinKindTerminal, items)

	result := make([]targetListItem, 0, len(items))
	for _, item := range items {
		stat := membershipStats[accountTargetJoinKey(item.Type, item.Identifier)]
		item.LinkedTerminals = stat.ActiveCount
		result = append(result, targetListItem{
			Target:                item,
			GroupIDs:              groupIDsByTarget[item.ID],
			ActiveMemberCount:     stat.ActiveCount,
			InvalidMemberCount:    stat.InvalidCount,
			LastMembershipCheckAt: formatOptionalTime(stat.LastCheckedAt),
		})
	}

	utils.OK(c, result)
}

func (s *Server) ImportTargets(c *gin.Context) {
	var req struct {
		Content      string `json:"content" binding:"required"`
		GroupID      string `json:"group_id"`
		NewGroupName string `json:"new_group_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入要导入的目标链接")
		return
	}

	groupID, groupName, err := s.resolveTargetGroup(c, strings.TrimSpace(req.GroupID), strings.TrimSpace(req.NewGroupName))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	summary := targetImportSummary{
		GroupID:   groupID,
		GroupName: groupName,
		Items:     []targetImportResultItem{},
	}

	lines := strings.Split(req.Content, "\n")
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for _, raw := range lines {
			line := cleanTargetImportLine(raw)
			if line == "" {
				summary.Skipped++
				continue
			}

			target, err := parseTargetLine(line)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, targetImportResultItem{Line: line, Status: "failed", Reason: err.Error()})
				continue
			}

			existing, err := s.findExistingTarget(c, tx, target)
			if err != nil {
				return err
			}
			if existing != nil {
				added, reason, err := s.attachTargetToGroup(c, tx, *existing, groupID)
				if err != nil {
					return err
				}
				status := "duplicate"
				if added {
					summary.Success++
					status = "success"
				} else {
					summary.Duplicate++
				}
				summary.Items = append(summary.Items, targetImportResultItem{
					Line:       line,
					Identifier: target.Identifier,
					Type:       target.Type,
					Status:     status,
					Reason:     reason,
				})
				continue
			}

			item := models.Target{
				ID:         uuid.New(),
				TenantID:   s.tenantID(c),
				Identifier: target.Identifier,
				Name:       target.Name,
				Type:       target.Type,
				GroupID:    groupID,
			}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			summary.Success++
			summary.Items = append(summary.Items, targetImportResultItem{
				Line:       line,
				Identifier: target.Identifier,
				Type:       target.Type,
				Status:     "success",
			})
		}
		return nil
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "导入目标池失败")
		return
	}

	utils.Created(c, summary)
}

func (s *Server) ImportTerminalTargets(c *gin.Context) {
	var req targetTerminalImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请选择终端范围")
		return
	}

	groupID, groupName, err := s.resolveTargetGroup(c, strings.TrimSpace(req.GroupID), strings.TrimSpace(req.NewGroupName))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	terminals, err := s.resolveTargetImportTerminals(c, req)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	summary := targetImportSummary{
		GroupID:   groupID,
		GroupName: groupName,
		Items:     []targetImportResultItem{},
	}

	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for _, terminal := range terminals {
			line := terminalTargetImportLabel(terminal)
			homepage := strings.TrimSpace(terminal.Homepage)
			if homepage == "" {
				summary.Failed++
				summary.Items = append(summary.Items, targetImportResultItem{
					Line:   line,
					Status: "failed",
					Reason: "终端未设置个人频道",
				})
				continue
			}

			target, err := parseTargetLine(homepage)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, targetImportResultItem{
					Line:       line,
					Identifier: homepage,
					Status:     "failed",
					Reason:     err.Error(),
				})
				continue
			}

			existing, err := s.findExistingTarget(c, tx, target)
			if err != nil {
				return err
			}
			if existing != nil {
				added, reason, err := s.attachTargetToGroup(c, tx, *existing, groupID)
				if err != nil {
					return err
				}
				status := "duplicate"
				if added {
					summary.Success++
					status = "success"
				} else {
					summary.Duplicate++
				}
				summary.Items = append(summary.Items, targetImportResultItem{
					Line:       line,
					Identifier: target.Identifier,
					Type:       target.Type,
					Status:     status,
					Reason:     reason,
				})
				continue
			}

			item := models.Target{
				ID:         uuid.New(),
				TenantID:   s.tenantID(c),
				AvatarURL:  terminal.AvatarURL,
				Identifier: target.Identifier,
				Name:       terminalTargetName(target, terminal),
				Type:       target.Type,
				GroupID:    groupID,
			}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			summary.Success++
			summary.Items = append(summary.Items, targetImportResultItem{
				Line:       line,
				Identifier: target.Identifier,
				Type:       target.Type,
				Status:     "success",
			})
		}
		return nil
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "从终端加入目标池失败")
		return
	}

	utils.Created(c, summary)
}

func (s *Server) resolveTargetGroup(c *gin.Context, groupIDText string, newGroupName string) (*uuid.UUID, string, error) {
	if groupIDText != "" && newGroupName != "" {
		return nil, "", fmt.Errorf("请选择已有分组或填写新分组，不能同时使用")
	}
	if newGroupName != "" {
		group := models.Group{
			ID:           uuid.New(),
			TenantID:     s.tenantID(c),
			ResourceType: "target",
			Name:         newGroupName,
		}
		if err := s.db.WithContext(c.Request.Context()).Create(&group).Error; err != nil {
			return nil, "", fmt.Errorf("创建新分组失败")
		}
		return &group.ID, group.Name, nil
	}
	if groupIDText == "" {
		return nil, "", nil
	}
	parsed, err := uuid.Parse(groupIDText)
	if err != nil {
		return nil, "", fmt.Errorf("分组 ID 无效")
	}
	var group models.Group
	query := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", parsed, "target")
	query = s.applyTenantAccess(c, query)
	if err := query.First(&group).Error; err != nil {
		return nil, "", fmt.Errorf("目标池分组不存在")
	}
	return &group.ID, group.Name, nil
}

func (s *Server) findExistingTarget(c *gin.Context, tx *gorm.DB, target parsedTarget) (*models.Target, error) {
	var existing models.Target
	query := tx.WithContext(c.Request.Context()).Model(&models.Target{}).Where("type = ?", target.Type)
	query = s.applyTenantAccess(c, query)
	if target.Type == "channel" {
		query = query.Where("LOWER(identifier) = ?", strings.ToLower(target.Identifier))
	} else {
		query = query.Where("identifier = ?", target.Identifier)
	}
	if err := query.First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &existing, nil
}

func (s *Server) attachTargetToGroup(c *gin.Context, tx *gorm.DB, target models.Target, groupID *uuid.UUID) (bool, string, error) {
	if groupID == nil {
		return false, "目标链接已存在", nil
	}

	if target.GroupID != nil && *target.GroupID == *groupID {
		return false, "目标已在当前分组", nil
	}

	if target.GroupID == nil {
		query := tx.WithContext(c.Request.Context()).Model(&models.Target{}).Where("id = ?", target.ID)
		query = s.applyTenantAccess(c, query)
		if err := query.Update("group_id", *groupID).Error; err != nil {
			return false, "", err
		}
		return true, "已加入分组", nil
	}

	var existing models.TargetGroupBinding
	query := tx.WithContext(c.Request.Context()).Where("target_id = ? AND group_id = ?", target.ID, *groupID)
	query = s.applyTenantAccess(c, query)
	if err := query.First(&existing).Error; err == nil {
		return false, "目标已在当前分组", nil
	} else if err != gorm.ErrRecordNotFound {
		return false, "", err
	}

	binding := models.TargetGroupBinding{
		ID:       uuid.New(),
		TenantID: s.tenantID(c),
		TargetID: target.ID,
		GroupID:  *groupID,
	}
	if err := tx.WithContext(c.Request.Context()).Create(&binding).Error; err != nil {
		return false, "", err
	}
	return true, "已加入分组", nil
}

func parseTargetLine(line string) (parsedTarget, error) {
	value := cleanTargetImportLine(line)
	if value == "" {
		return parsedTarget{}, fmt.Errorf("空行")
	}

	if strings.HasPrefix(value, "@") {
		return parseTelegramUsername(strings.TrimPrefix(value, "@"))
	}

	if !strings.Contains(value, "://") && (strings.HasPrefix(strings.ToLower(value), "t.me/") || strings.HasPrefix(strings.ToLower(value), "telegram.me/")) {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return parsedTarget{}, fmt.Errorf("目标格式应为 https://t.me/用户名")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return parsedTarget{}, fmt.Errorf("链接协议只支持 http 或 https")
	}

	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host != "t.me" && host != "telegram.me" {
		return parsedTarget{}, fmt.Errorf("只支持 t.me 或 telegram.me 链接")
	}

	segments := splitTargetPath(parsed.Path)
	if len(segments) == 0 {
		return parsedTarget{}, fmt.Errorf("链接缺少目标标识")
	}

	switch strings.ToLower(segments[0]) {
	case "joinchat":
		if len(segments) < 2 || strings.TrimSpace(segments[1]) == "" {
			return parsedTarget{}, fmt.Errorf("邀请链接缺少参数")
		}
		identifier := "joinchat/" + segments[1]
		return parsedTarget{Identifier: identifier, Name: identifier, Type: "invite"}, nil
	case "c":
		if len(segments) < 2 || strings.TrimSpace(segments[1]) == "" {
			return parsedTarget{}, fmt.Errorf("私有频道链接缺少标识")
		}
		identifier := "c/" + segments[1]
		return parsedTarget{Identifier: identifier, Name: identifier, Type: "private_channel"}, nil
	case "s":
		if len(segments) < 2 {
			return parsedTarget{}, fmt.Errorf("公开预览链接缺少用户名")
		}
		return parseTelegramUsername(segments[1])
	default:
		if strings.HasPrefix(segments[0], "+") {
			return parsedTarget{Identifier: segments[0], Name: segments[0], Type: "invite"}, nil
		}
		return parseTelegramUsername(segments[0])
	}
}

func parseTelegramUsername(value string) (parsedTarget, error) {
	username := strings.TrimSpace(strings.TrimPrefix(value, "@"))
	if !telegramUsernamePattern.MatchString(username) {
		return parsedTarget{}, fmt.Errorf("Telegram 用户名需为 5-32 位字母、数字或下划线")
	}
	return parsedTarget{Identifier: username, Name: "@" + username, Type: "channel"}, nil
}

func splitTargetPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	segments := make([]string, 0, len(raw))
	for _, segment := range raw {
		if segment == "" {
			continue
		}
		value, err := url.PathUnescape(segment)
		if err != nil {
			value = segment
		}
		segments = append(segments, strings.TrimSpace(value))
	}
	return segments
}

func cleanTargetImportLine(line string) string {
	return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "\ufeff")), " \t\r\n'\"，,")
}

func (s *Server) resolveTargetImportTerminals(c *gin.Context, req targetTerminalImportRequest) ([]models.Terminal, error) {
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	query := s.db.WithContext(c.Request.Context()).Order("created_at asc")
	query = s.applyTenantAccess(c, query)

	switch scope {
	case "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TerminalGroupID))
		if err != nil {
			return nil, fmt.Errorf("终端分组 ID 无效")
		}
		var group models.Group
		groupQuery := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", groupID, "terminal")
		groupQuery = s.applyTenantAccess(c, groupQuery)
		if err := groupQuery.First(&group).Error; err != nil {
			return nil, fmt.Errorf("终端分组不存在")
		}
		query = query.Where("group_id = ?", groupID)
	case "terminal":
		terminalID, err := uuid.Parse(strings.TrimSpace(req.TerminalID))
		if err != nil {
			return nil, fmt.Errorf("终端 ID 无效")
		}
		query = query.Where("id = ?", terminalID)
	default:
		return nil, fmt.Errorf("请选择全部终端、终端组或单个终端")
	}

	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		return nil, err
	}
	if len(terminals) == 0 {
		return nil, fmt.Errorf("当前范围内没有可加入目标池的终端")
	}
	return terminals, nil
}

func terminalTargetImportLabel(terminal models.Terminal) string {
	parts := make([]string, 0, 2)
	if phone := formatTerminalPhoneDisplay(normalizeTerminalPhone(terminal.Phone)); phone != "" {
		parts = append(parts, phone)
	}
	if nickname := strings.TrimSpace(terminal.Nickname); nickname != "" {
		parts = append(parts, nickname)
	}
	if len(parts) == 0 {
		return terminal.ID.String()
	}
	return strings.Join(parts, " / ")
}

func terminalTargetName(target parsedTarget, terminal models.Terminal) string {
	if name := strings.TrimSpace(target.Name); name != "" {
		return name
	}
	return target.Identifier
}

type JoinTargetsRequest struct {
	TerminalScope   string `json:"terminal_scope" binding:"required"`
	TerminalGroupID string `json:"terminal_group_id"`
	TerminalID      string `json:"terminal_id"`
	TargetScope     string `json:"target_scope" binding:"required"`
	TargetGroupID   string `json:"target_group_id"`
}

type joinTargetsSummary struct {
	TaskID        string                 `json:"task_id"`
	Total         int                    `json:"total"`
	Success       int                    `json:"success"`
	Failed        int                    `json:"failed"`
	Skipped       int                    `json:"skipped"`
	Terminals     int                    `json:"terminals"`
	Targets       int                    `json:"targets"`
	TopSkipReason string                 `json:"top_skip_reason,omitempty"`
	SkipReasons   map[string]int         `json:"skip_reasons,omitempty"`
	Items         []joinTargetsResultRow `json:"items"`
}

type joinTargetsResultRow struct {
	TerminalID string `json:"terminal_id"`
	Terminal   string `json:"terminal"`
	TargetID   string `json:"target_id"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
}

func (s *Server) CreateJoinTargetsTask(c *gin.Context) {
	var req JoinTargetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "无效的任务参数")
		return
	}

	terminals, err := s.resolveJoinTerminals(c, req)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	targets, err := s.resolveJoinTargets(c, req)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	payloadInfo := map[string]any{
		"terminal_scope":    req.TerminalScope,
		"terminal_id":       req.TerminalID,
		"terminal_group_id": req.TerminalGroupID,
		"target_scope":      req.TargetScope,
		"target_group_id":   req.TargetGroupID,
		"terminal_count":    len(terminals),
		"target_count":      len(targets),
	}

	rawPayload, _ := datatypes.JSON.MarshalJSON(datatypes.JSON(mustMarshal(payloadInfo)))

	var terminalGroupIDPtr *uuid.UUID
	if req.TerminalGroupID != "" {
		id, err := uuid.Parse(req.TerminalGroupID)
		if err == nil {
			terminalGroupIDPtr = &id
		}
	}

	var targetGroupIDPtr *uuid.UUID
	if req.TargetGroupID != "" {
		id, err := uuid.Parse(req.TargetGroupID)
		if err == nil {
			targetGroupIDPtr = &id
		}
	}
	taskTenantID := s.tenantFilterID(c)

	task := models.Task{
		ID:              uuid.New(),
		TenantID:        taskTenantID,
		Name:            "自动加群任务",
		Type:            "join_targets",
		Status:          "queued",
		Progress:        0,
		TerminalGroupID: terminalGroupIDPtr,
		TargetGroupID:   targetGroupIDPtr,
		Payload:         rawPayload,
		CreatedBy:       s.userIDPtr(c),
	}

	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建任务失败")
		return
	}

	_ = s.createTaskLog(context.Background(), task, "INFO", "created", fmt.Sprintf("自动加群任务已创建：%d 个终端 × %d 个目标", len(terminals), len(targets)), "", "")

	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.runJoinTargetsTask(context.Background(), task, terminals, targets)
	}

	utils.Created(c, gin.H{"task": task})
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (s *Server) resolveJoinTerminals(c *gin.Context, req JoinTargetsRequest) ([]models.Terminal, error) {
	scope := strings.ToLower(strings.TrimSpace(req.TerminalScope))
	query := s.db.WithContext(c.Request.Context()).Order("created_at asc")
	query = s.applyTenantAccess(c, query)

	switch scope {
	case "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TerminalGroupID))
		if err != nil {
			return nil, fmt.Errorf("终端分组 ID 无效")
		}
		var group models.Group
		groupQuery := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", groupID, "terminal")
		groupQuery = s.applyTenantAccess(c, groupQuery)
		if err := groupQuery.First(&group).Error; err != nil {
			return nil, fmt.Errorf("终端分组不存在")
		}
		query = query.Where("group_id = ?", groupID)
	case "terminal":
		terminalID, err := uuid.Parse(strings.TrimSpace(req.TerminalID))
		if err != nil {
			return nil, fmt.Errorf("终端 ID 无效")
		}
		query = query.Where("id = ?", terminalID)
	default:
		return nil, fmt.Errorf("请选择全部终端、终端组或单个终端")
	}

	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		return nil, err
	}
	if len(terminals) == 0 {
		return nil, fmt.Errorf("当前范围内没有终端")
	}
	return terminals, nil
}

func (s *Server) resolveJoinTargets(c *gin.Context, req JoinTargetsRequest) ([]models.Target, error) {
	scope := strings.ToLower(strings.TrimSpace(req.TargetScope))
	tenantID := s.tenantFilterID(c)
	query := s.db.WithContext(c.Request.Context()).Order("created_at asc")
	query = s.applyTenantAccess(c, query)

	switch scope {
	case "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TargetGroupID))
		if err != nil {
			return nil, fmt.Errorf("目标分组 ID 无效")
		}
		var group models.Group
		groupQuery := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", groupID, "target")
		groupQuery = s.applyTenantAccess(c, groupQuery)
		if err := groupQuery.First(&group).Error; err != nil {
			return nil, fmt.Errorf("目标分组不存在")
		}
		query = s.applyTargetGroupFilter(c.Request.Context(), query, tenantID, groupID)
	default:
		return nil, fmt.Errorf("请选择全部目标池或指定目标分组")
	}

	var targets []models.Target
	if err := query.Find(&targets).Error; err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("当前范围内没有目标")
	}
	return targets, nil
}

func (s *Server) runJoinTargetsTask(ctx context.Context, task models.Task, terminals []models.Terminal, targets []models.Target) {
	s.updateTaskState(ctx, task.ID, "running", 1, nil)
	targets = s.sortTargetsByJoinCoverage(ctx, task.TenantID, accountJoinKindTerminal, targets)
	summary := joinTargetsSummary{
		TaskID:      task.ID.String(),
		Terminals:   len(terminals),
		Targets:     len(targets),
		Total:       len(targets),
		SkipReasons: map[string]int{},
		Items:       []joinTargetsResultRow{},
	}

	joiner := telegram_client.NewJoiner(s.cfg)
	total := summary.Total
	if total == 0 {
		s.finishJoinTargetsTask(ctx, task, "failed", summary, "自动加群任务没有可执行的终端或目标")
		return
	}

	_ = s.createTaskLog(ctx, task, "INFO", "start", fmt.Sprintf("开始自动加群：%d 个候选终端，%d 个目标，按未覆盖目标优先调度", len(terminals), len(targets)), "", "")

	done := 0
	for _, target := range targets {
		done++
		targetRef := targetJoinLabel(target)
		row := joinTargetsResultRow{
			TargetID: target.ID.String(),
			Target:   targetRef,
		}
		if !isJoinableTargetType(target.Type) {
			row.Status = "skipped"
			row.Reason = joinUnsupportedReason(target.Type)
			summary.Skipped++
			accumulateSkipReason(summary.SkipReasons, row.Reason)
			summary.Items = append(summary.Items, row)
			_ = s.createTaskLog(ctx, task, "WARN", "join_skipped", row.Reason, "", targetRef)
			s.updateJoinTaskProgress(ctx, task.ID, done, total)
			continue
		}
		terminalIndex, terminal, quotaErr := s.pickJoinTargetTerminal(ctx, task.TenantID, terminals, target)
		if quotaErr != nil {
			row.Status = "skipped"
			row.Reason = quotaErr.Error()
			summary.Skipped++
			accumulateSkipReason(summary.SkipReasons, row.Reason)
			summary.Items = append(summary.Items, row)
			_ = s.createTaskLog(ctx, task, "WARN", "join_skipped", row.Reason, "", targetRef)
			s.updateJoinTaskProgress(ctx, task.ID, done, total)
			continue
		}
		terminalRef := terminalTargetImportLabel(terminal)
		row.TerminalID = terminal.ID.String()
		row.Terminal = terminalRef

		start := time.Now()
		result, err := joiner.Join(ctx, telegram_client.JoinRequest{
			FilePath:   terminal.FilePath,
			AccessType: terminal.AccessType,
			TargetType: target.Type,
			Identifier: target.Identifier,
		})
		duration := time.Since(start).Milliseconds()
		row.Status = result.Status
		row.Reason = result.Reason
		if strings.TrimSpace(row.Status) == "" {
			row.Status = "failed"
		}
		if strings.TrimSpace(row.Reason) == "" && err != nil {
			row.Reason = err.Error()
		}

		if err == nil && result.OK {
			summary.Success++
			terminals[terminalIndex] = terminal
			s.recordAccountTargetJoin(ctx, task.TenantID, accountJoinKindTerminal, terminal.ID, target, &task.ID)
			_ = s.createTaskLogWithDuration(ctx, task, "INFO", "join_success", firstNonEmpty(result.Reason, "已加入目标"), terminalRef, targetRef, duration)
		} else {
			summary.Failed++
			s.applyTerminalOutboundFailure(ctx, terminal.ID, row.Reason)
			s.applyTerminalTargetFailure(ctx, task.TenantID, terminal.ID, terminalQuotaActionJoin, target.Type, target.Identifier, row.Reason)
			_ = s.createTaskLogWithDuration(ctx, task, "ERROR", "join_failed", firstNonEmpty(row.Reason, "加群失败"), terminalRef, targetRef, duration)
		}
		summary.Items = append(summary.Items, row)
		s.updateJoinTaskProgress(ctx, task.ID, done, total)
	}

	status := "success"
	switch {
	case summary.Success == 0 && (summary.Failed > 0 || summary.Skipped > 0):
		status = "failed"
	case summary.Failed > 0 || summary.Skipped > 0:
		status = "partial_success"
	}
	summary.TopSkipReason = topSkipReason(summary.SkipReasons)
	detail := fmt.Sprintf("自动加群完成：成功 %d，失败 %d，跳过 %d", summary.Success, summary.Failed, summary.Skipped)
	s.finishJoinTargetsTask(ctx, task, status, summary, detail)
}

func (s *Server) RunJoinTargetsTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "join_targets")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "join_targets").First(&task).Error; err != nil {
		return
	}
	terminals, targets, err := s.loadJoinTargetsTaskSelection(ctx, task)
	if err != nil {
		s.finishJoinTargetsTask(ctx, task, "failed", joinTargetsSummary{TaskID: task.ID.String(), Items: []joinTargetsResultRow{}}, err.Error())
		return
	}
	s.runJoinTargetsTask(ctx, task, terminals, targets)
}

func (s *Server) loadJoinTargetsTaskSelection(ctx context.Context, task models.Task) ([]models.Terminal, []models.Target, error) {
	var payload JoinTargetsRequest
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return nil, nil, fmt.Errorf("自动加群任务参数解析失败：%s", err.Error())
	}
	terminals, err := s.loadJoinTerminalsFromPayload(ctx, task.TenantID, payload)
	if err != nil {
		return nil, nil, err
	}
	targets, err := s.loadJoinTargetsFromPayload(ctx, task.TenantID, payload)
	if err != nil {
		return nil, nil, err
	}
	return terminals, targets, nil
}

func (s *Server) loadJoinTerminalsFromPayload(ctx context.Context, tenantID uuid.UUID, req JoinTargetsRequest) ([]models.Terminal, error) {
	scope := strings.ToLower(strings.TrimSpace(req.TerminalScope))
	query := s.db.WithContext(ctx).Order("created_at asc")
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	switch scope {
	case "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TerminalGroupID))
		if err != nil {
			return nil, fmt.Errorf("终端分组 ID 无效")
		}
		query = query.Where("group_id = ?", groupID)
	case "terminal":
		terminalID, err := uuid.Parse(strings.TrimSpace(req.TerminalID))
		if err != nil {
			return nil, fmt.Errorf("终端 ID 无效")
		}
		query = query.Where("id = ?", terminalID)
	default:
		return nil, fmt.Errorf("请选择全部终端、终端组或单个终端")
	}
	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		return nil, err
	}
	if len(terminals) == 0 {
		return nil, fmt.Errorf("当前范围内没有终端")
	}
	return terminals, nil
}

func (s *Server) loadJoinTargetsFromPayload(ctx context.Context, tenantID uuid.UUID, req JoinTargetsRequest) ([]models.Target, error) {
	scope := strings.ToLower(strings.TrimSpace(req.TargetScope))
	query := s.db.WithContext(ctx).Order("created_at asc")
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	switch scope {
	case "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TargetGroupID))
		if err != nil {
			return nil, fmt.Errorf("目标分组 ID 无效")
		}
		query = s.applyTargetGroupFilter(ctx, query, tenantID, groupID)
	default:
		return nil, fmt.Errorf("请选择全部目标池或指定目标分组")
	}
	var targets []models.Target
	if err := query.Find(&targets).Error; err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("当前范围内没有目标")
	}
	return targets, nil
}

func (s *Server) pickJoinTargetTerminal(ctx context.Context, tenantID uuid.UUID, terminals []models.Terminal, target models.Target) (int, models.Terminal, error) {
	candidateIndexes := make([]int, 0, len(terminals))
	for index := range terminals {
		if terminalReadyForOutboundAction(terminals[index]) && s.terminalTargetAvailable(ctx, tenantID, terminals[index].ID, terminalQuotaActionJoin, target.Type, target.Identifier) {
			candidateIndexes = append(candidateIndexes, index)
		}
	}
	if len(candidateIndexes) == 0 {
		return -1, models.Terminal{}, fmt.Errorf("没有可用的加群账号")
	}
	sort.Slice(candidateIndexes, func(i, j int) bool {
		return terminalOlderForJoin(terminals[candidateIndexes[i]], terminals[candidateIndexes[j]])
	})
	skipReasons := make([]string, 0, len(candidateIndexes))
	for _, index := range candidateIndexes {
		terminal, err := s.reserveTerminalQuota(ctx, terminals[index].ID, terminalQuotaActionJoin)
		if err == nil {
			terminals[index] = terminal
			return index, terminal, nil
		}
		skipReasons = append(skipReasons, err.Error())
	}
	return -1, models.Terminal{}, fmt.Errorf("全部加群账号已跳过：%s", summarizeTerminalSkipReasons(skipReasons, "账号限额已满或不可用"))
}

func terminalOlderForJoin(candidate models.Terminal, current models.Terminal) bool {
	if candidate.LastJoinAt == nil && current.LastJoinAt != nil {
		return true
	}
	if candidate.LastJoinAt != nil && current.LastJoinAt == nil {
		return false
	}
	if candidate.LastJoinAt != nil && current.LastJoinAt != nil {
		if candidate.LastJoinAt.Before(*current.LastJoinAt) {
			return true
		}
		if current.LastJoinAt.Before(*candidate.LastJoinAt) {
			return false
		}
	}
	return candidate.CreatedAt.Before(current.CreatedAt)
}

func (s *Server) updateJoinTaskProgress(ctx context.Context, taskID uuid.UUID, done int, total int) {
	progress := 1
	if total > 0 {
		progress = 1 + int(float64(done)/float64(total)*98)
	}
	if progress > 99 {
		progress = 99
	}
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).Update("progress", progress).Error
}

func (s *Server) finishJoinTargetsTask(ctx context.Context, task models.Task, status string, summary joinTargetsSummary, detail string) {
	summaryBytes, _ := json.Marshal(summary)
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":   status,
		"progress": 100,
		"summary":  datatypes.JSON(summaryBytes),
	}).Error
	level := "INFO"
	if status == "failed" {
		level = "ERROR"
	} else if status == "partial_success" {
		level = "WARN"
	}
	_ = s.createTaskLog(ctx, task, level, "summary", detail, "", "")
}

func (s *Server) createTaskLog(ctx context.Context, task models.Task, level, action, detail, terminalRef, targetRef string) error {
	return s.createTaskLogWithDuration(ctx, task, level, action, detail, terminalRef, targetRef, 0)
}

func (s *Server) createTaskLogWithDuration(ctx context.Context, task models.Task, level, action, detail, terminalRef, targetRef string, durationMS int64) error {
	return s.db.WithContext(ctx).Create(&models.TaskLog{
		ID:          uuid.New(),
		TenantID:    task.TenantID,
		TaskID:      task.ID,
		Level:       level,
		Category:    task.Type,
		TerminalRef: terminalRef,
		TargetRef:   targetRef,
		Action:      action,
		Details:     detail,
		DurationMS:  durationMS,
		TraceID:     uuid.NewString(),
		CreatedAt:   time.Now(),
	}).Error
}

func isJoinableTargetType(targetType string) bool {
	switch strings.TrimSpace(targetType) {
	case "channel", "invite":
		return true
	default:
		return false
	}
}

func joinUnsupportedReason(targetType string) string {
	switch strings.TrimSpace(targetType) {
	case "private_channel":
		return "私有频道 c/... 不能直接加入，请改用邀请链接"
	case "terminal_account":
		return "终端账号不能作为加群目标"
	default:
		return "目标类型不支持自动加群"
	}
}

func targetJoinLabel(target models.Target) string {
	if strings.TrimSpace(target.Name) != "" {
		return target.Name
	}
	if strings.TrimSpace(target.Identifier) != "" {
		return target.Identifier
	}
	return target.ID.String()
}
