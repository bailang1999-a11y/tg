package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type terminalQuotaAction string

const (
	terminalQuotaActionDM   terminalQuotaAction = "dm"
	terminalQuotaActionJoin terminalQuotaAction = "join"
)

var floodWaitPattern = regexp.MustCompile(`需等待\s+(\d+)\s+秒`)

type terminalRestrictionItem struct {
	ID            string  `json:"id"`
	Action        string  `json:"action"`
	ActionText    string  `json:"action_text"`
	TargetType    string  `json:"target_type"`
	TargetValue   string  `json:"target_value"`
	Reason        string  `json:"reason"`
	FailCount     int     `json:"fail_count"`
	CooldownUntil *string `json:"cooldown_until"`
	LastFailedAt  *string `json:"last_failed_at"`
	Active        bool    `json:"active"`
}

type terminalRiskStats struct {
	ActiveRestrictionCount  int     `json:"active_restriction_count"`
	ExpiredRestrictionCount int     `json:"expired_restriction_count"`
	Restriction24HDM        int64   `json:"restriction_24h_dm"`
	Restriction24HJoin      int64   `json:"restriction_24h_join"`
	Failure24HTotal         int64   `json:"failure_24h_total"`
	CooldownActive          bool    `json:"cooldown_active"`
	CooldownUntil           *string `json:"cooldown_until"`
	DMHourlyUsage           int     `json:"dm_hourly_usage"`
	DMDailyUsage            int     `json:"dm_daily_usage"`
	JoinHourlyUsage         int     `json:"join_hourly_usage"`
	JoinDailyUsage          int     `json:"join_daily_usage"`
	DMHourlyLimit           int     `json:"dm_hourly_limit"`
	DMDailyLimit            int     `json:"dm_daily_limit"`
	JoinHourlyLimit         int     `json:"join_hourly_limit"`
	JoinDailyLimit          int     `json:"join_daily_limit"`
	RiskScore               string  `json:"risk_score"`
}

type terminalRestrictionAgg struct {
	ActiveCount  int64
	ExpiredCount int64
	DM24H        int64
	Join24H      int64
}

type terminalRiskBoardItem struct {
	TerminalID              string  `json:"terminal_id"`
	ActiveRestrictionCount  int     `json:"active_restriction_count"`
	ExpiredRestrictionCount int     `json:"expired_restriction_count"`
	Restriction24HDM        int64   `json:"restriction_24h_dm"`
	Restriction24HJoin      int64   `json:"restriction_24h_join"`
	Failure24HTotal         int64   `json:"failure_24h_total"`
	CooldownActive          bool    `json:"cooldown_active"`
	CooldownUntil           *string `json:"cooldown_until"`
	DMHourlyUsage           int     `json:"dm_hourly_usage"`
	DMDailyUsage            int     `json:"dm_daily_usage"`
	JoinHourlyUsage         int     `json:"join_hourly_usage"`
	JoinDailyUsage          int     `json:"join_daily_usage"`
	RiskScore               string  `json:"risk_score"`
}

type terminalBatchResult struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

func (s *Server) UpdateTerminalLimits(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "终端 ID 无效")
		return
	}

	var req struct {
		DMHourlyLimit   *int `json:"dm_hourly_limit"`
		DMDailyLimit    *int `json:"dm_daily_limit"`
		JoinHourlyLimit *int `json:"join_hourly_limit"`
		JoinDailyLimit  *int `json:"join_daily_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "限额参数无效")
		return
	}

	updates := map[string]any{}
	if req.DMHourlyLimit != nil {
		updates["dm_hourly_limit"] = normalizeTerminalLimit(*req.DMHourlyLimit)
	}
	if req.DMDailyLimit != nil {
		updates["dm_daily_limit"] = normalizeTerminalLimit(*req.DMDailyLimit)
	}
	if req.JoinHourlyLimit != nil {
		updates["join_hourly_limit"] = normalizeTerminalLimit(*req.JoinHourlyLimit)
	}
	if req.JoinDailyLimit != nil {
		updates["join_daily_limit"] = normalizeTerminalLimit(*req.JoinDailyLimit)
	}
	if len(updates) == 0 {
		utils.Fail(c, http.StatusBadRequest, "请至少提交一个限额字段")
		return
	}
	updates["updated_at"] = time.Now()

	query := s.db.WithContext(c.Request.Context()).Model(&models.Terminal{}).Where("id = ?", id)
	query = s.applyTenantAccess(c, query)
	if err := query.Updates(updates).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "更新终端限额失败")
		return
	}

	var item models.Terminal
	readQuery := s.db.WithContext(c.Request.Context()).Where("id = ?", id)
	readQuery = s.applyTenantAccess(c, readQuery)
	if err := readQuery.First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Fail(c, http.StatusNotFound, "终端不存在")
			return
		}
		utils.Fail(c, http.StatusInternalServerError, "读取终端失败")
		return
	}

	utils.OK(c, buildTerminalListItem(item))
}

func (s *Server) ListTerminalRestrictions(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "终端 ID 无效")
		return
	}

	action := normalizeRestrictionAction(c.Query("action"))
	state := normalizeRestrictionState(c.Query("state"))

	query := s.db.WithContext(c.Request.Context()).Where("terminal_id = ?", id)
	query = s.applyTenantAccess(c, query)
	if action != "" {
		query = query.Where("action = ?", action)
	}
	now := time.Now()
	switch state {
	case "active":
		query = query.Where("cooldown_until IS NOT NULL AND cooldown_until > ?", now)
	case "expired":
		query = query.Where("cooldown_until IS NULL OR cooldown_until <= ?", now)
	}

	var items []models.TerminalTargetRestriction
	if err := query.
		Order("COALESCE(cooldown_until, updated_at) desc").
		Limit(30).
		Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取账号限制记录失败")
		return
	}

	result := make([]terminalRestrictionItem, 0, len(items))
	for _, item := range items {
		result = append(result, terminalRestrictionItem{
			ID:            item.ID.String(),
			Action:        item.Action,
			ActionText:    terminalQuotaActionText(item.Action),
			TargetType:    item.TargetType,
			TargetValue:   item.TargetValue,
			Reason:        item.Reason,
			FailCount:     item.FailCount,
			CooldownUntil: formatOptionalTime(item.CooldownUntil),
			LastFailedAt:  formatOptionalTime(item.LastFailedAt),
			Active:        item.CooldownUntil != nil && item.CooldownUntil.After(now),
		})
	}

	utils.OK(c, result)
}

func (s *Server) GetTerminalRiskStats(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "终端 ID 无效")
		return
	}

	var terminal models.Terminal
	terminalQuery := s.db.WithContext(c.Request.Context()).Where("id = ?", id)
	terminalQuery = s.applyTenantAccess(c, terminalQuery)
	if err := terminalQuery.First(&terminal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Fail(c, http.StatusNotFound, "终端不存在")
			return
		}
		utils.Fail(c, http.StatusInternalServerError, "读取终端失败")
		return
	}

	now := time.Now()
	since := now.Add(-24 * time.Hour)

	var agg terminalRestrictionAgg
	if err := s.db.WithContext(c.Request.Context()).
		Model(&models.TerminalTargetRestriction{}).
		Select(`
			COALESCE(SUM(CASE WHEN cooldown_until IS NOT NULL AND cooldown_until > ? THEN 1 ELSE 0 END), 0) AS active_count,
			COALESCE(SUM(CASE WHEN cooldown_until IS NULL OR cooldown_until <= ? THEN 1 ELSE 0 END), 0) AS expired_count,
			COALESCE(SUM(CASE WHEN action = ? AND last_failed_at IS NOT NULL AND last_failed_at >= ? THEN fail_count ELSE 0 END), 0) AS dm24_h,
			COALESCE(SUM(CASE WHEN action = ? AND last_failed_at IS NOT NULL AND last_failed_at >= ? THEN fail_count ELSE 0 END), 0) AS join24_h
		`, now, now, string(terminalQuotaActionDM), since, string(terminalQuotaActionJoin), since).
		Where("terminal_id = ?", id).
		Scan(&agg).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取风控统计失败")
		return
	}

	stats := terminalRiskStats{
		ActiveRestrictionCount:  int(agg.ActiveCount),
		ExpiredRestrictionCount: int(agg.ExpiredCount),
		Restriction24HDM:        agg.DM24H,
		Restriction24HJoin:      agg.Join24H,
		Failure24HTotal:         agg.DM24H + agg.Join24H,
		CooldownActive:          terminal.SleepUntil != nil && terminal.SleepUntil.After(now),
		CooldownUntil:           formatOptionalTime(terminal.SleepUntil),
		DMHourlyUsage:           percentUsage(terminal.DMHourlyCount, terminal.DMHourlyLimit),
		DMDailyUsage:            percentUsage(terminal.DMDailyCount, terminal.DMDailyLimit),
		JoinHourlyUsage:         percentUsage(terminal.JoinHourlyCount, terminal.JoinHourlyLimit),
		JoinDailyUsage:          percentUsage(terminal.JoinDailyCount, terminal.JoinDailyLimit),
		DMHourlyLimit:           terminal.DMHourlyLimit,
		DMDailyLimit:            terminal.DMDailyLimit,
		JoinHourlyLimit:         terminal.JoinHourlyLimit,
		JoinDailyLimit:          terminal.JoinDailyLimit,
		RiskScore:               riskScoreLabel(terminal, agg),
	}

	utils.OK(c, stats)
}

func (s *Server) ListTerminalRiskBoard(c *gin.Context) {
	tenantID := s.tenantID(c)
	groupID, err := parseOptionalUUID(strings.TrimSpace(c.Query("group_id")), "分组 ID 无效")
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "分组 ID 无效")
		return
	}

	query := s.db.WithContext(c.Request.Context()).
		Model(&models.Terminal{})
	query = s.applyTenantAccess(c, query)
	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	}

	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取账号风控看板失败")
		return
	}
	if len(terminals) == 0 {
		utils.OK(c, []terminalRiskBoardItem{})
		return
	}

	ids := make([]uuid.UUID, 0, len(terminals))
	for _, item := range terminals {
		ids = append(ids, item.ID)
	}

	now := time.Now()
	since := now.Add(-24 * time.Hour)

	type riskBoardRow struct {
		TerminalID   uuid.UUID
		ActiveCount  int64
		ExpiredCount int64
		DM24H        int64
		Join24H      int64
	}
	var rows []riskBoardRow
	aggQuery := s.db.WithContext(c.Request.Context()).
		Model(&models.TerminalTargetRestriction{}).
		Select(`
			terminal_id,
			COALESCE(SUM(CASE WHEN cooldown_until IS NOT NULL AND cooldown_until > ? THEN 1 ELSE 0 END), 0) AS active_count,
			COALESCE(SUM(CASE WHEN cooldown_until IS NULL OR cooldown_until <= ? THEN 1 ELSE 0 END), 0) AS expired_count,
			COALESCE(SUM(CASE WHEN action = ? AND last_failed_at IS NOT NULL AND last_failed_at >= ? THEN fail_count ELSE 0 END), 0) AS dm24_h,
			COALESCE(SUM(CASE WHEN action = ? AND last_failed_at IS NOT NULL AND last_failed_at >= ? THEN fail_count ELSE 0 END), 0) AS join24_h
		`, now, now, string(terminalQuotaActionDM), since, string(terminalQuotaActionJoin), since).
		Where("terminal_id IN ?", ids)
	if !s.isAdmin(c) {
		aggQuery = aggQuery.Where("tenant_id = ?", tenantID)
	}
	if err := aggQuery.Group("terminal_id").Scan(&rows).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取账号风控聚合失败")
		return
	}

	aggMap := make(map[uuid.UUID]terminalRestrictionAgg, len(rows))
	for _, row := range rows {
		aggMap[row.TerminalID] = terminalRestrictionAgg{
			ActiveCount:  row.ActiveCount,
			ExpiredCount: row.ExpiredCount,
			DM24H:        row.DM24H,
			Join24H:      row.Join24H,
		}
	}

	result := make([]terminalRiskBoardItem, 0, len(terminals))
	for _, item := range terminals {
		agg := aggMap[item.ID]
		result = append(result, terminalRiskBoardItem{
			TerminalID:              item.ID.String(),
			ActiveRestrictionCount:  int(agg.ActiveCount),
			ExpiredRestrictionCount: int(agg.ExpiredCount),
			Restriction24HDM:        agg.DM24H,
			Restriction24HJoin:      agg.Join24H,
			Failure24HTotal:         agg.DM24H + agg.Join24H,
			CooldownActive:          item.SleepUntil != nil && item.SleepUntil.After(now),
			CooldownUntil:           formatOptionalTime(item.SleepUntil),
			DMHourlyUsage:           percentUsage(item.DMHourlyCount, item.DMHourlyLimit),
			DMDailyUsage:            percentUsage(item.DMDailyCount, item.DMDailyLimit),
			JoinHourlyUsage:         percentUsage(item.JoinHourlyCount, item.JoinHourlyLimit),
			JoinDailyUsage:          percentUsage(item.JoinDailyCount, item.JoinDailyLimit),
			RiskScore:               riskScoreLabel(item, agg),
		})
	}

	utils.OK(c, result)
}

func (s *Server) BatchTerminalOperation(c *gin.Context) {
	var req struct {
		IDs        []string `json:"ids"`
		Action     string   `json:"action"`
		Multiplier float64  `json:"multiplier"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "批量操作参数无效")
		return
	}
	if len(req.IDs) == 0 {
		utils.Fail(c, http.StatusBadRequest, "请至少选择一个账号")
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "reduce_limits", "clear_cooldown", "clear_expired_restrictions":
	default:
		utils.Fail(c, http.StatusBadRequest, "不支持的批量操作")
		return
	}

	multiplier := req.Multiplier
	if action == "reduce_limits" {
		if multiplier <= 0 || multiplier >= 1 {
			multiplier = 0.5
		}
	}

	results := make([]terminalBatchResult, 0, len(req.IDs))
	for _, rawID := range req.IDs {
		id, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			results = append(results, terminalBatchResult{ID: rawID, OK: false, Message: "终端 ID 无效"})
			continue
		}

		message, err := s.applyTerminalBatchOperation(c.Request.Context(), s.tenantID(c), id, action, multiplier)
		if err != nil {
			results = append(results, terminalBatchResult{ID: id.String(), OK: false, Message: err.Error()})
			continue
		}
		results = append(results, terminalBatchResult{ID: id.String(), OK: true, Message: message})
	}

	utils.OK(c, gin.H{
		"action":  action,
		"results": results,
	})
}

func (s *Server) ClearTerminalRestrictions(c *gin.Context) {
	terminalID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "终端 ID 无效")
		return
	}

	var req struct {
		Mode   string `json:"mode"`
		Action string `json:"action"`
		State  string `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "批量清理参数无效")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "expired"
	}
	if mode != "expired" && mode != "filtered" && mode != "all" {
		utils.Fail(c, http.StatusBadRequest, "不支持的清理模式")
		return
	}

	action := normalizeRestrictionAction(req.Action)
	state := normalizeRestrictionState(req.State)

	query := s.db.WithContext(c.Request.Context()).Where("terminal_id = ?", terminalID)
	query = s.applyTenantAccess(c, query)

	now := time.Now()
	switch mode {
	case "expired":
		query = query.Where("cooldown_until IS NULL OR cooldown_until <= ?", now)
	case "filtered":
		if action != "" {
			query = query.Where("action = ?", action)
		}
		switch state {
		case "active":
			query = query.Where("cooldown_until IS NOT NULL AND cooldown_until > ?", now)
		case "expired":
			query = query.Where("cooldown_until IS NULL OR cooldown_until <= ?", now)
		}
	case "all":
		if action != "" {
			query = query.Where("action = ?", action)
		}
		switch state {
		case "active":
			query = query.Where("cooldown_until IS NOT NULL AND cooldown_until > ?", now)
		case "expired":
			query = query.Where("cooldown_until IS NULL OR cooldown_until <= ?", now)
		}
	}

	result := query.Delete(&models.TerminalTargetRestriction{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "批量清理账号限制失败")
		return
	}

	utils.OK(c, gin.H{
		"deleted_count": result.RowsAffected,
	})
}

func (s *Server) ClearTerminalCooldown(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "终端 ID 无效")
		return
	}
	query := s.db.WithContext(c.Request.Context()).Model(&models.Terminal{}).Where("id = ?", id)
	query = s.applyTenantAccess(c, query)
	if err := query.Updates(map[string]any{
		"sleep_until":         nil,
		"dm_cooldown_until":   nil,
		"join_cooldown_until": nil,
		"risk_status":         gorm.Expr("CASE WHEN risk_status = ? THEN ? ELSE risk_status END", "限流冷却", "正常"),
		"updated_at":          time.Now(),
	}).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "清除账号冷却失败")
		return
	}
	utils.OK(c, gin.H{"cleared": id.String()})
}

func (s *Server) DeleteTerminalRestriction(c *gin.Context) {
	terminalID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "终端 ID 无效")
		return
	}
	restrictionID, err := uuid.Parse(strings.TrimSpace(c.Param("restriction_id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "限制记录 ID 无效")
		return
	}
	query := s.db.WithContext(c.Request.Context()).
		Where("terminal_id = ? AND id = ?", terminalID, restrictionID)
	query = s.applyTenantAccess(c, query)
	result := query.Delete(&models.TerminalTargetRestriction{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "解除目标限制失败")
		return
	}
	utils.OK(c, gin.H{"deleted": restrictionID.String()})
}

func normalizeTerminalLimit(value int) int {
	if value <= 0 {
		return 0
	}
	if value > 100000 {
		return 100000
	}
	return value
}

func percentUsage(count int, limit int) int {
	if count <= 0 || limit <= 0 {
		return 0
	}
	usage := count * 100 / limit
	if usage < 0 {
		return 0
	}
	if usage > 999 {
		return 999
	}
	return usage
}

func (s *Server) applyTerminalBatchOperation(ctx context.Context, tenantID, terminalID uuid.UUID, action string, multiplier float64) (string, error) {
	switch action {
	case "clear_cooldown":
		query := s.db.WithContext(ctx).Model(&models.Terminal{}).Where("id = ?", terminalID)
		if tenantID != uuid.Nil {
			query = query.Where("tenant_id = ?", tenantID)
		}
		result := query.Updates(map[string]any{
			"sleep_until":         nil,
			"dm_cooldown_until":   nil,
			"join_cooldown_until": nil,
			"risk_status":         gorm.Expr("CASE WHEN risk_status = ? THEN ? ELSE risk_status END", "限流冷却", "正常"),
			"updated_at":          time.Now(),
		})
		if result.Error != nil {
			return "", result.Error
		}
		if result.RowsAffected == 0 {
			return "", fmt.Errorf("账号不存在")
		}
		return "已解除冷却", nil
	case "clear_expired_restrictions":
		query := s.db.WithContext(ctx).
			Where("terminal_id = ? AND (cooldown_until IS NULL OR cooldown_until <= ?)", terminalID, time.Now())
		if tenantID != uuid.Nil {
			query = query.Where("tenant_id = ?", tenantID)
		}
		result := query.Delete(&models.TerminalTargetRestriction{})
		if result.Error != nil {
			return "", result.Error
		}
		return fmt.Sprintf("已清理 %d 条过期限制", result.RowsAffected), nil
	case "reduce_limits":
		var terminal models.Terminal
		query := s.db.WithContext(ctx).Where("id = ?", terminalID)
		if tenantID != uuid.Nil {
			query = query.Where("tenant_id = ?", tenantID)
		}
		if err := query.First(&terminal).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return "", fmt.Errorf("账号不存在")
			}
			return "", err
		}
		updates := map[string]any{
			"dm_hourly_limit":   scaledTerminalLimit(terminal.DMHourlyLimit, multiplier),
			"dm_daily_limit":    scaledTerminalLimit(terminal.DMDailyLimit, multiplier),
			"join_hourly_limit": scaledTerminalLimit(terminal.JoinHourlyLimit, multiplier),
			"join_daily_limit":  scaledTerminalLimit(terminal.JoinDailyLimit, multiplier),
			"updated_at":        time.Now(),
		}
		updateQuery := s.db.WithContext(ctx).Model(&models.Terminal{}).Where("id = ?", terminalID)
		if tenantID != uuid.Nil {
			updateQuery = updateQuery.Where("tenant_id = ?", tenantID)
		}
		if err := updateQuery.Updates(updates).Error; err != nil {
			return "", err
		}
		return "已按比例下调额度", nil
	default:
		return "", fmt.Errorf("未知批量操作")
	}
}

func scaledTerminalLimit(value int, multiplier float64) int {
	if value <= 0 {
		return 0
	}
	scaled := int(float64(value) * multiplier)
	if scaled < 1 {
		return 1
	}
	return normalizeTerminalLimit(scaled)
}

func riskScoreLabel(terminal models.Terminal, agg terminalRestrictionAgg) string {
	if terminal.SleepUntil != nil && terminal.SleepUntil.After(time.Now()) {
		return "高"
	}
	if agg.ActiveCount >= 3 || agg.DM24H+agg.Join24H >= 10 {
		return "高"
	}
	if agg.ActiveCount > 0 || agg.DM24H+agg.Join24H >= 3 {
		return "中"
	}
	return "低"
}

func normalizeRestrictionAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dm":
		return string(terminalQuotaActionDM)
	case "join":
		return string(terminalQuotaActionJoin)
	default:
		return ""
	}
}

func normalizeRestrictionState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active":
		return "active"
	case "expired":
		return "expired"
	default:
		return ""
	}
}

func (s *Server) reserveTerminalQuota(ctx context.Context, terminalID uuid.UUID, action terminalQuotaAction) (models.Terminal, error) {
	var terminal models.Terminal
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", terminalID).
			First(&terminal).Error; err != nil {
			return err
		}

		now := time.Now()
		if terminal.SleepUntil != nil && terminal.SleepUntil.After(now) {
			return fmt.Errorf("账号冷却中，需等待到 %s", terminal.SleepUntil.Local().Format("2006-01-02 15:04:05"))
		}
		if !terminalReadyForOutboundAction(terminal) {
			return fmt.Errorf("账号当前不可用于%s", terminalQuotaActionLabel(action))
		}
		riskAgg, err := s.loadTerminalRestrictionAgg(tx, terminal.TenantID, terminal.ID, now)
		if err != nil {
			return err
		}
		settings := s.readSystemSettings(ctx, terminal.TenantID)
		if terminalShouldAutoBypass(terminal, riskAgg, settings.RiskControl) {
			return fmt.Errorf("账号当前风险过高，已自动跳过%s调度", terminalQuotaActionLabel(action))
		}

		updates := map[string]any{}
		switch action {
		case terminalQuotaActionDM:
			if terminal.DMCooldownUntil != nil && terminal.DMCooldownUntil.After(now) {
				return fmt.Errorf("账号发信冷却中，需等待到 %s", terminal.DMCooldownUntil.Local().Format("2006-01-02 15:04:05"))
			}
			hourlyCount, hourlyResetAt := resetTerminalQuotaWindow(now, terminal.DMHourlyCount, terminal.DMHourlyResetAt, "hour")
			dailyCount, dailyResetAt := resetTerminalQuotaWindow(now, terminal.DMDailyCount, terminal.DMDailyResetAt, "day")
			if terminal.DMHourlyLimit > 0 && hourlyCount >= terminal.DMHourlyLimit {
				return fmt.Errorf("账号每小时私信限额已达上限（%d）", terminal.DMHourlyLimit)
			}
			if terminal.DMDailyLimit > 0 && dailyCount >= terminal.DMDailyLimit {
				return fmt.Errorf("账号每日私信限额已达上限（%d）", terminal.DMDailyLimit)
			}
			hourlyCount++
			dailyCount++
			terminal.DMHourlyCount = hourlyCount
			terminal.DMDailyCount = dailyCount
			terminal.DMHourlyResetAt = hourlyResetAt
			terminal.DMDailyResetAt = dailyResetAt
			nextCooldown := terminalNextCooldownAt(now, settings.RiskControl.MessageCooldownMinutes, settings.RiskControl.MessageJitterMinutes)
			terminal.DMCooldownUntil = &nextCooldown
			terminal.LastMessageAt = &now
			updates["dm_hourly_count"] = hourlyCount
			updates["dm_daily_count"] = dailyCount
			updates["dm_hourly_reset_at"] = hourlyResetAt
			updates["dm_daily_reset_at"] = dailyResetAt
			updates["dm_cooldown_until"] = nextCooldown
			updates["last_message_at"] = now
		case terminalQuotaActionJoin:
			if terminal.JoinCooldownUntil != nil && terminal.JoinCooldownUntil.After(now) {
				return fmt.Errorf("账号加群冷却中，需等待到 %s", terminal.JoinCooldownUntil.Local().Format("2006-01-02 15:04:05"))
			}
			hourlyCount, hourlyResetAt := resetTerminalQuotaWindow(now, terminal.JoinHourlyCount, terminal.JoinHourlyResetAt, "hour")
			dailyCount, dailyResetAt := resetTerminalQuotaWindow(now, terminal.JoinDailyCount, terminal.JoinDailyResetAt, "day")
			dailyLimit := terminal.JoinDailyLimit
			if dailyLimit <= 0 {
				dailyLimit = settings.RiskControl.JoinDailyLimit
			}
			if terminal.JoinHourlyLimit > 0 && hourlyCount >= terminal.JoinHourlyLimit {
				return fmt.Errorf("账号每小时加群限额已达上限（%d）", terminal.JoinHourlyLimit)
			}
			if dailyLimit > 0 && dailyCount >= dailyLimit {
				return fmt.Errorf("账号每日加群限额已达上限（%d）", dailyLimit)
			}
			hourlyCount++
			dailyCount++
			terminal.JoinHourlyCount = hourlyCount
			terminal.JoinDailyCount = dailyCount
			terminal.JoinHourlyResetAt = hourlyResetAt
			terminal.JoinDailyResetAt = dailyResetAt
			nextCooldown := terminalNextCooldownAt(now, settings.RiskControl.JoinIntervalMinutes, settings.RiskControl.JoinJitterMinutes)
			terminal.JoinCooldownUntil = &nextCooldown
			terminal.LastJoinAt = &now
			updates["join_hourly_count"] = hourlyCount
			updates["join_daily_count"] = dailyCount
			updates["join_hourly_reset_at"] = hourlyResetAt
			updates["join_daily_reset_at"] = dailyResetAt
			updates["join_cooldown_until"] = nextCooldown
			updates["last_join_at"] = now
		default:
			return fmt.Errorf("未知的账号限额动作")
		}
		updates["updated_at"] = now
		return tx.Model(&models.Terminal{}).Where("id = ?", terminalID).Updates(updates).Error
	})
	return terminal, err
}

func terminalNextCooldownAt(now time.Time, baseMinutes int, jitterMinutes int) time.Time {
	if baseMinutes <= 0 {
		return now
	}
	offset := terminalJitterOffsetMinutes(now, jitterMinutes)
	minutes := baseMinutes + offset
	if minutes < 1 {
		minutes = 1
	}
	return now.Add(time.Duration(minutes) * time.Minute)
}

func terminalJitterOffsetMinutes(now time.Time, jitterMinutes int) int {
	if jitterMinutes <= 0 {
		return 0
	}
	width := jitterMinutes*2 + 1
	value := int(now.UnixNano() % int64(width))
	if value < 0 {
		value = -value
	}
	return value - jitterMinutes
}

func (s *Server) loadTerminalRestrictionAgg(db *gorm.DB, tenantID uuid.UUID, terminalID uuid.UUID, now time.Time) (terminalRestrictionAgg, error) {
	since := now.Add(-24 * time.Hour)
	var agg terminalRestrictionAgg
	err := db.Model(&models.TerminalTargetRestriction{}).
		Select(`
			COALESCE(SUM(CASE WHEN cooldown_until IS NOT NULL AND cooldown_until > ? THEN 1 ELSE 0 END), 0) AS active_count,
			COALESCE(SUM(CASE WHEN cooldown_until IS NULL OR cooldown_until <= ? THEN 1 ELSE 0 END), 0) AS expired_count,
			COALESCE(SUM(CASE WHEN action = ? AND last_failed_at IS NOT NULL AND last_failed_at >= ? THEN fail_count ELSE 0 END), 0) AS dm24_h,
			COALESCE(SUM(CASE WHEN action = ? AND last_failed_at IS NOT NULL AND last_failed_at >= ? THEN fail_count ELSE 0 END), 0) AS join24_h
		`, now, now, string(terminalQuotaActionDM), since, string(terminalQuotaActionJoin), since).
		Where("tenant_id = ? AND terminal_id = ?", tenantID, terminalID).
		Scan(&agg).Error
	return agg, err
}

func resetTerminalQuotaWindow(now time.Time, count int, resetAt *time.Time, window string) (int, *time.Time) {
	if resetAt == nil || !resetAt.After(now) {
		nextResetAt := nextTerminalQuotaResetAt(now, window)
		return 0, &nextResetAt
	}
	return count, resetAt
}

func nextTerminalQuotaResetAt(now time.Time, window string) time.Time {
	location := terminalQuotaLocation()
	local := now.In(location)
	switch window {
	case "hour":
		return local.Truncate(time.Hour).Add(time.Hour)
	case "day":
		year, month, day := local.Date()
		return time.Date(year, month, day+1, 0, 0, 0, 0, local.Location())
	default:
		return now.Add(time.Hour)
	}
}

func terminalQuotaLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return location
}

func terminalReadyForOutboundAction(item models.Terminal) bool {
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "abnormal" || status == "disabled" || status == "banned" {
		return false
	}
	if isProfileRestrictedStatus(item.RiskStatus, item.BanStatus) {
		return false
	}
	if strings.TrimSpace(item.FilePath) == "" || !isStoredTerminalFileReady(item.FilePath) {
		return false
	}
	return true
}

func terminalShouldAutoBypass(item models.Terminal, agg terminalRestrictionAgg, settings systemRiskControlSettings) bool {
	if !settings.AutoBypassHighRisk {
		return false
	}
	if item.SleepUntil != nil && item.SleepUntil.After(time.Now()) {
		return true
	}
	if riskScoreLabel(item, agg) != "高" {
		return false
	}
	if agg.ActiveCount >= int64(settings.AutoBypassActiveRestrictions) {
		return true
	}
	return agg.DM24H+agg.Join24H >= int64(settings.AutoBypassFailures24H)
}

func summarizeTerminalSkipReasons(reasons []string, fallback string) string {
	if len(reasons) == 0 {
		return fallback
	}
	counts := make(map[string]int)
	order := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		normalized := normalizeTerminalSkipReason(reason)
		if normalized == "" {
			continue
		}
		if _, exists := counts[normalized]; !exists {
			order = append(order, normalized)
		}
		counts[normalized]++
	}
	if len(order) == 0 {
		return fallback
	}
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] == counts[order[j]] {
			return order[i] < order[j]
		}
		return counts[order[i]] > counts[order[j]]
	})
	parts := make([]string, 0, minInt(len(order), 3))
	for _, reason := range order {
		parts = append(parts, fmt.Sprintf("%s %d 个", reason, counts[reason]))
		if len(parts) >= 3 {
			break
		}
	}
	return strings.Join(parts, "；")
}

func normalizeTerminalSkipReason(reason string) string {
	text := strings.TrimSpace(reason)
	switch {
	case strings.Contains(text, "风险过高"):
		return "高风险自动跳过"
	case strings.Contains(text, "冷却中"):
		return "冷却中"
	case strings.Contains(text, "发信冷却中"), strings.Contains(text, "加群冷却中"):
		return "冷却中"
	case strings.Contains(text, "私信限额已达上限"), strings.Contains(text, "加群限额已达上限"):
		return "账号限额已满"
	case strings.Contains(text, "不可用于"):
		return "账号当前不可用"
	default:
		return text
	}
}

func accumulateSkipReason(counts map[string]int, reason string) {
	if counts == nil {
		return
	}
	normalized := normalizeTerminalSkipReason(reason)
	if normalized == "" {
		return
	}
	counts[normalized]++
}

func topSkipReason(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	reasons := make([]string, 0, len(counts))
	for reason := range counts {
		reasons = append(reasons, reason)
	}
	sort.SliceStable(reasons, func(i, j int) bool {
		if counts[reasons[i]] == counts[reasons[j]] {
			return reasons[i] < reasons[j]
		}
		return counts[reasons[i]] > counts[reasons[j]]
	})
	return fmt.Sprintf("%s %d 次", reasons[0], counts[reasons[0]])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func terminalQuotaActionLabel(action terminalQuotaAction) string {
	switch action {
	case terminalQuotaActionDM:
		return "私信"
	case terminalQuotaActionJoin:
		return "加群"
	default:
		return "外发"
	}
}

func terminalQuotaActionText(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case string(terminalQuotaActionDM):
		return "私信"
	case string(terminalQuotaActionJoin):
		return "加群"
	default:
		return firstNonEmpty(strings.TrimSpace(action), "未知动作")
	}
}

func (s *Server) applyTerminalOutboundFailure(ctx context.Context, terminalID uuid.UUID, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return
	}
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	switch {
	case terminalFloodWaitUntil(reason) != nil:
		until := terminalFloodWaitUntil(reason)
		updates["sleep_until"] = until
		updates["risk_status"] = "限流冷却"
	case terminalNeedsRelogin(reason):
		updates["status"] = "abnormal"
		updates["risk_status"] = "需重新登录"
	case terminalNeedsReimport(reason):
		updates["status"] = "abnormal"
		updates["risk_status"] = "需重新导入"
	}
	if len(updates) == 1 {
		return
	}
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Where("id = ?", terminalID).Updates(updates).Error
	if _, ok := updates["status"]; ok {
		s.markAccountJoinRecordsUnavailable(ctx, uuid.Nil, accountJoinKindTerminal, terminalID, reason)
	}
}

func (s *Server) terminalTargetAvailable(ctx context.Context, tenantID uuid.UUID, terminalID uuid.UUID, action terminalQuotaAction, targetType string, targetValue string) bool {
	key := terminalTargetKey(action, targetType, targetValue)
	if key == "" {
		return true
	}
	query := s.db.WithContext(ctx).
		Where("terminal_id = ? AND action = ? AND target_key = ?", terminalID, string(action), key)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	var restrictions []models.TerminalTargetRestriction
	if err := query.Find(&restrictions).Error; err != nil || len(restrictions) == 0 {
		return true
	}
	now := time.Now()
	for _, restriction := range restrictions {
		if restriction.CooldownUntil == nil || restriction.CooldownUntil.After(now) {
			return false
		}
	}
	return true
}

func (s *Server) applyTerminalTargetFailure(ctx context.Context, tenantID uuid.UUID, terminalID uuid.UUID, action terminalQuotaAction, targetType string, targetValue string, reason string) {
	cooldownUntil := terminalTargetCooldownUntil(reason)
	if cooldownUntil == nil {
		return
	}
	now := time.Now()
	key := terminalTargetKey(action, targetType, targetValue)
	if key == "" {
		return
	}
	restriction := models.TerminalTargetRestriction{
		ID:            uuid.New(),
		TenantID:      tenantID,
		TerminalID:    terminalID,
		Action:        string(action),
		TargetType:    strings.TrimSpace(targetType),
		TargetValue:   strings.TrimSpace(targetValue),
		TargetKey:     key,
		Reason:        strings.TrimSpace(reason),
		FailCount:     1,
		CooldownUntil: cooldownUntil,
		LastFailedAt:  &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "target_key"}, {Name: "action"}, {Name: "terminal_id"}, {Name: "tenant_id"}},
		DoUpdates: clause.Assignments(map[string]any{"reason": restriction.Reason, "cooldown_until": restriction.CooldownUntil, "last_failed_at": restriction.LastFailedAt, "updated_at": restriction.UpdatedAt, "fail_count": gorm.Expr("terminal_target_restrictions.fail_count + 1")}),
	}).Create(&restriction).Error
}

func terminalTargetCooldownUntil(reason string) *time.Time {
	text := strings.ToLower(strings.TrimSpace(reason))
	now := time.Now()
	switch {
	case strings.Contains(text, "账号已被该目标限制"):
		until := now.Add(7 * 24 * time.Hour)
		return &until
	case strings.Contains(text, "目标不可写入"), strings.Contains(text, "需要加入"), strings.Contains(text, "需要授权"), strings.Contains(text, "不可访问"):
		until := now.Add(6 * time.Hour)
		return &until
	default:
		return nil
	}
}

func terminalTargetKey(action terminalQuotaAction, targetType string, targetValue string) string {
	value := strings.TrimSpace(strings.ToLower(targetValue))
	if value == "" {
		return ""
	}
	return strings.Join([]string{string(action), strings.TrimSpace(strings.ToLower(targetType)), value}, ":")
}

func terminalFloodWaitUntil(reason string) *time.Time {
	matches := floodWaitPattern.FindStringSubmatch(strings.TrimSpace(reason))
	if len(matches) != 2 {
		return nil
	}
	seconds, err := strconv.Atoi(matches[1])
	if err != nil || seconds <= 0 {
		return nil
	}
	until := time.Now().Add(time.Duration(seconds) * time.Second)
	return &until
}

func terminalNeedsRelogin(reason string) bool {
	text := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(text, "未授权") ||
		strings.Contains(text, "重新登录") ||
		strings.Contains(text, "unauthorized") ||
		strings.Contains(text, "auth")
}

func terminalNeedsReimport(reason string) bool {
	text := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(text, "需重新导入") ||
		strings.Contains(text, "会话文件不存在") ||
		strings.Contains(text, "缺少本地会话文件") ||
		strings.Contains(text, "tdata 文件不完整")
}
