package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

type targetMembershipStat struct {
	ActiveCount   int64
	InvalidCount  int64
	LastCheckedAt *time.Time
}

type targetMembershipStatRow struct {
	TargetKey     string
	ActiveCount   int64
	InvalidCount  int64
	LastCheckedAt *time.Time
}

type targetMembershipRow struct {
	ID            string  `json:"id"`
	AccountKind   string  `json:"account_kind"`
	AccountID     string  `json:"account_id"`
	AccountLabel  string  `json:"account_label"`
	Phone         string  `json:"phone"`
	Nickname      string  `json:"nickname"`
	AccountStatus string  `json:"account_status"`
	RiskStatus    string  `json:"risk_status"`
	Status        string  `json:"status"`
	StatusText    string  `json:"status_text"`
	StatusReason  string  `json:"status_reason"`
	Active        bool    `json:"active"`
	JoinedAt      *string `json:"joined_at"`
	LastCheckedAt *string `json:"last_checked_at"`
	LastSeenAt    *string `json:"last_seen_at"`
	RemovedAt     *string `json:"removed_at"`
}

type targetMembershipRefreshRequest struct {
	AccountKind   string `json:"account_kind"`
	TargetScope   string `json:"target_scope"`
	TargetID      string `json:"target_id"`
	TargetGroupID string `json:"target_group_id"`
	AdminScope    bool   `json:"admin_scope,omitempty"`
}

type targetMembershipRefreshSummary struct {
	TaskID  string                              `json:"task_id"`
	Total   int                                 `json:"total"`
	Active  int                                 `json:"active"`
	Removed int                                 `json:"removed"`
	Skipped int                                 `json:"skipped"`
	Failed  int                                 `json:"failed"`
	Items   []targetMembershipRefreshResultItem `json:"items"`
}

type targetMembershipRefreshResultItem struct {
	AccountKind string `json:"account_kind"`
	Account     string `json:"account"`
	Target      string `json:"target"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
}

const maxTargetMembershipRefreshSummaryItems = 500
const maxMembershipBatchTargets = 50

type membershipRefreshBatchItem struct {
	Join models.AccountTargetJoin
}

func (s *Server) loadTargetMembershipStats(ctx context.Context, tenantID uuid.UUID, adminScope bool, accountKind string, targets []models.Target) map[string]targetMembershipStat {
	stats := make(map[string]targetMembershipStat, len(targets))
	keys := make([]string, 0, len(targets))
	for _, target := range targets {
		key := accountTargetJoinKey(target.Type, target.Identifier)
		if key == "" {
			continue
		}
		stats[key] = targetMembershipStat{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return stats
	}
	s.pruneUnavailableAccountTargetJoins(ctx, tenantID, adminScope, accountKind, keys)
	var rows []targetMembershipStatRow
	query := s.db.WithContext(ctx).Model(&models.AccountTargetJoin{}).
		Select(`
			target_key,
			COALESCE(SUM(CASE WHEN active THEN 1 ELSE 0 END), 0) AS active_count,
			COALESCE(SUM(CASE WHEN active THEN 0 ELSE 1 END), 0) AS invalid_count,
			MAX(last_checked_at) AS last_checked_at
		`).
		Where("account_kind = ? AND target_key IN ?", accountKind, keys).
		Group("target_key")
	if !adminScope {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return stats
	}
	for _, row := range rows {
		stats[row.TargetKey] = targetMembershipStat{
			ActiveCount:   row.ActiveCount,
			InvalidCount:  row.InvalidCount,
			LastCheckedAt: row.LastCheckedAt,
		}
	}
	return stats
}

func (s *Server) ListTargetMemberships(c *gin.Context) {
	targetID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "目标 ID 无效")
		return
	}
	var target models.Target
	query := s.applyTenantAccess(c, s.db.WithContext(c.Request.Context()).Where("id = ?", targetID))
	if err := query.First(&target).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "目标不存在")
		return
	}
	key := accountTargetJoinKey(target.Type, target.Identifier)
	if key == "" {
		utils.OK(c, []targetMembershipRow{})
		return
	}
	accountKind := normalizeMembershipAccountKind(c.Query("account_kind"))
	if accountKind != "all" {
		s.pruneUnavailableAccountTargetJoins(c.Request.Context(), target.TenantID, s.isAdmin(c), accountKind, []string{key})
	} else {
		s.pruneUnavailableAccountTargetJoins(c.Request.Context(), target.TenantID, s.isAdmin(c), accountJoinKindTerminal, []string{key})
		s.pruneUnavailableAccountTargetJoins(c.Request.Context(), target.TenantID, s.isAdmin(c), accountJoinKindListener, []string{key})
	}
	joinQuery := s.db.WithContext(c.Request.Context()).
		Where("target_key = ?", key).
		Order("active desc, updated_at desc")
	if accountKind != "all" {
		joinQuery = joinQuery.Where("account_kind = ?", accountKind)
	}
	if !s.isAdmin(c) {
		joinQuery = joinQuery.Where("tenant_id = ?", target.TenantID)
	}
	var joins []models.AccountTargetJoin
	if err := joinQuery.Find(&joins).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取目标群账号状态失败")
		return
	}
	rows := s.buildTargetMembershipRows(c.Request.Context(), joins)
	utils.OK(c, rows)
}

func (s *Server) CreateRefreshTargetMembershipsTask(c *gin.Context) {
	var req targetMembershipRefreshRequest
	_ = c.ShouldBindJSON(&req)
	req.AccountKind = normalizeMembershipAccountKind(req.AccountKind)
	req.TargetScope = normalizeMembershipTargetScope(req.TargetScope)
	if req.TargetScope == "target" && strings.TrimSpace(req.TargetID) == "" {
		utils.Fail(c, http.StatusBadRequest, "请选择要刷新的目标")
		return
	}
	if req.TargetScope == "group" && strings.TrimSpace(req.TargetGroupID) == "" {
		utils.Fail(c, http.StatusBadRequest, "请选择要刷新的目标分组")
		return
	}
	req.AdminScope = s.isAdmin(c)
	payload, _ := json.Marshal(req)
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      "刷新目标群账号状态",
		Type:      "target_membership_refresh",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payload),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建目标群状态刷新任务失败")
		return
	}
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.runRefreshTargetMembershipsTask(context.Background(), task, req, req.AdminScope)
	}
	utils.Created(c, gin.H{"task": task})
}

func (s *Server) RunRefreshTargetMembershipsTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "target_membership_refresh")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "target_membership_refresh").First(&task).Error; err != nil {
		return
	}
	var req targetMembershipRefreshRequest
	if err := json.Unmarshal(task.Payload, &req); err != nil {
		s.finishTargetMembershipRefreshTask(ctx, task, "failed", targetMembershipRefreshSummary{TaskID: task.ID.String(), Items: []targetMembershipRefreshResultItem{}}, "目标群状态刷新参数解析失败："+err.Error())
		return
	}
	req.AccountKind = normalizeMembershipAccountKind(req.AccountKind)
	req.TargetScope = normalizeMembershipTargetScope(req.TargetScope)
	s.runRefreshTargetMembershipsTask(ctx, task, req, req.AdminScope)
}

func (s *Server) runRefreshTargetMembershipsTask(ctx context.Context, task models.Task, req targetMembershipRefreshRequest, adminScope bool) {
	s.updateTaskState(ctx, task.ID, "running", 1, nil)
	summary := targetMembershipRefreshSummary{TaskID: task.ID.String(), Items: []targetMembershipRefreshResultItem{}}
	keys, err := s.loadMembershipRefreshTargetKeys(ctx, task.TenantID, adminScope, req)
	if err != nil {
		s.finishTargetMembershipRefreshTask(ctx, task, "failed", summary, err.Error())
		return
	}
	kinds := membershipAccountKinds(req.AccountKind)
	query := s.db.WithContext(ctx).Where("active = ? AND account_kind IN ?", true, kinds).Order("updated_at asc")
	if len(keys) > 0 {
		query = query.Where("target_key IN ?", keys)
	}
	if !adminScope {
		query = query.Where("tenant_id = ?", task.TenantID)
	}
	var joins []models.AccountTargetJoin
	if err := query.Find(&joins).Error; err != nil {
		s.finishTargetMembershipRefreshTask(ctx, task, "failed", summary, "读取目标群账号状态失败："+err.Error())
		return
	}
	summary.Total = len(joins)
	if len(joins) == 0 {
		s.finishTargetMembershipRefreshTask(ctx, task, "success", summary, "没有需要刷新的目标群账号状态")
		return
	}
	checker := telegram_client.NewMembershipChecker(s.cfg)
	accounts := s.loadMembershipCheckAccounts(ctx, joins)
	batches := make(map[string][]membershipRefreshBatchItem)
	batchOrder := make([]string, 0)
	listenersToRefresh := make(map[uuid.UUID]struct{})
	processed := 0
	for _, join := range joins {
		accountKey := membershipAccountMapKey(join.AccountKind, join.AccountID)
		account, ok := accounts[accountKey]
		if !ok {
			account = membershipCheckAccount{
				Label:  join.AccountID.String(),
				Ready:  false,
				Reason: membershipMissingAccountReason(join.AccountKind),
			}
		}
		if !account.Ready {
			result := telegram_client.MembershipCheckResult{
				Status:     accountTargetStatusAccountUnavailable,
				Reason:     account.Reason,
				Target:     join.TargetValue,
				TargetType: join.TargetType,
				Ref:        join.ID.String(),
			}
			s.applyMembershipRefreshResult(ctx, task, &summary, join, account, result, nil)
			if join.AccountKind == accountJoinKindListener {
				listenersToRefresh[join.AccountID] = struct{}{}
			}
			processed++
			if shouldUpdateMembershipProgress(processed, len(joins)) {
				s.updateTargetMembershipRefreshProgress(ctx, task.ID, processed, len(joins))
			}
			continue
		}
		if _, exists := batches[accountKey]; !exists {
			batchOrder = append(batchOrder, accountKey)
		}
		batches[accountKey] = append(batches[accountKey], membershipRefreshBatchItem{Join: join})
	}

	for _, accountKey := range batchOrder {
		account := accounts[accountKey]
		items := batches[accountKey]
		floodWaitReason := ""
		for start := 0; start < len(items); start += maxMembershipBatchTargets {
			end := start + maxMembershipBatchTargets
			if end > len(items) {
				end = len(items)
			}
			chunk := items[start:end]
			if floodWaitReason != "" {
				for _, item := range chunk {
					result := telegram_client.MembershipCheckResult{
						Status:     "flood_wait",
						Reason:     floodWaitReason,
						Target:     item.Join.TargetValue,
						TargetType: item.Join.TargetType,
						Ref:        item.Join.ID.String(),
					}
					s.applyMembershipRefreshResult(ctx, task, &summary, item.Join, account, result, nil)
					if item.Join.AccountKind == accountJoinKindListener {
						listenersToRefresh[item.Join.AccountID] = struct{}{}
					}
					processed++
					if shouldUpdateMembershipProgress(processed, len(joins)) {
						s.updateTargetMembershipRefreshProgress(ctx, task.ID, processed, len(joins))
					}
				}
				continue
			}
			targets := make([]telegram_client.MembershipCheckTarget, 0, len(chunk))
			for _, item := range chunk {
				targets = append(targets, telegram_client.MembershipCheckTarget{
					Ref:        item.Join.ID.String(),
					TargetType: item.Join.TargetType,
					Identifier: item.Join.TargetValue,
				})
			}
			results, batchErr := checker.CheckBatch(ctx, telegram_client.MembershipBatchCheckRequest{
				FilePath:   account.FilePath,
				AccessType: account.AccessType,
				Targets:    targets,
			})
			resultsByRef := make(map[string]telegram_client.MembershipCheckResult, len(results))
			for _, result := range results {
				if strings.TrimSpace(result.Ref) != "" {
					resultsByRef[result.Ref] = result
				}
			}
			for offset, item := range chunk {
				result, ok := resultsByRef[item.Join.ID.String()]
				if !ok && offset < len(results) {
					result = results[offset]
					ok = true
				}
				if !ok {
					result = telegram_client.MembershipCheckResult{
						Status:     "failed",
						Reason:     membershipBatchErrorReason(batchErr),
						Target:     item.Join.TargetValue,
						TargetType: item.Join.TargetType,
						Ref:        item.Join.ID.String(),
					}
				}
				if strings.EqualFold(strings.TrimSpace(result.Status), "flood_wait") && floodWaitReason == "" {
					floodWaitReason = firstNonEmpty(result.Reason, "触发 Telegram 限流")
				}
				s.applyMembershipRefreshResult(ctx, task, &summary, item.Join, account, result, batchErr)
				if item.Join.AccountKind == accountJoinKindListener {
					listenersToRefresh[item.Join.AccountID] = struct{}{}
				}
				processed++
				if shouldUpdateMembershipProgress(processed, len(joins)) {
					s.updateTargetMembershipRefreshProgress(ctx, task.ID, processed, len(joins))
				}
			}
		}
	}
	for listenerID := range listenersToRefresh {
		s.refreshListenerJoinedTargetCount(ctx, listenerID)
	}
	status := "success"
	if summary.Removed > 0 || summary.Failed > 0 || summary.Skipped > 0 {
		status = "partial_success"
	}
	detail := fmt.Sprintf("目标群账号状态刷新完成：仍有效 %d，移除 %d，跳过 %d，失败 %d", summary.Active, summary.Removed, summary.Skipped, summary.Failed)
	s.finishTargetMembershipRefreshTask(ctx, task, status, summary, detail)
}

func (s *Server) applyMembershipRefreshResult(ctx context.Context, task models.Task, summary *targetMembershipRefreshSummary, join models.AccountTargetJoin, account membershipCheckAccount, result telegram_client.MembershipCheckResult, checkErr error) {
	status := normalizeMembershipCheckStatus(result.Status, checkErr)
	active := join.Active
	if accountTargetMembershipActive(status) {
		active = true
	} else if membershipStatusDisablesAccountTarget(status) {
		active = false
	}
	reason := firstNonEmpty(result.Reason, membershipStatusFallbackReason(status, checkErr))
	if strings.EqualFold(status, "flood_wait") {
		s.applyMembershipAccountFloodWait(ctx, join, reason)
	}
	s.updateAccountTargetJoinMembership(ctx, join, status, reason, active)
	item := targetMembershipRefreshResultItem{
		AccountKind: join.AccountKind,
		Account:     account.Label,
		Target:      firstNonEmpty(strings.TrimSpace(join.TargetValue), join.TargetKey),
		Status:      status,
		Reason:      reason,
	}
	switch {
	case active && accountTargetMembershipActive(status):
		summary.Active++
	case !active:
		summary.Removed++
	default:
		summary.Skipped++
	}
	if checkErr != nil && !membershipStatusDisablesAccountTarget(status) {
		summary.Failed++
	}
	appendTargetMembershipRefreshItem(summary, item)
	if shouldLogMembershipRefreshResult(active, status, checkErr) {
		_ = s.createTaskLog(ctx, task, membershipRefreshLogLevel(active, status, checkErr), "membership_check", fmt.Sprintf("%s：%s", accountTargetMembershipStatusText(status, active), reason), item.Account, item.Target)
	}
}

func (s *Server) applyMembershipAccountFloodWait(ctx context.Context, join models.AccountTargetJoin, reason string) {
	switch join.AccountKind {
	case accountJoinKindTerminal:
		s.applyTerminalOutboundFailure(ctx, join.AccountID, reason)
	case accountJoinKindListener:
		until := terminalFloodWaitUntil(reason)
		if until == nil {
			return
		}
		_ = s.db.WithContext(ctx).Model(&models.ListenerAccount{}).Where("id = ?", join.AccountID).Updates(map[string]any{
			"risk_status":         "限流冷却",
			"join_cooldown_until": until,
			"updated_at":          time.Now(),
		}).Error
	}
}

func (s *Server) buildTargetMembershipRows(ctx context.Context, joins []models.AccountTargetJoin) []targetMembershipRow {
	terminalIDs := make([]uuid.UUID, 0)
	listenerIDs := make([]uuid.UUID, 0)
	for _, join := range joins {
		switch join.AccountKind {
		case accountJoinKindTerminal:
			terminalIDs = append(terminalIDs, join.AccountID)
		case accountJoinKindListener:
			listenerIDs = append(listenerIDs, join.AccountID)
		}
	}
	terminals := map[uuid.UUID]models.Terminal{}
	if len(terminalIDs) > 0 {
		var items []models.Terminal
		_ = s.db.WithContext(ctx).Where("id IN ?", terminalIDs).Find(&items).Error
		for _, item := range items {
			terminals[item.ID] = item
		}
	}
	listeners := map[uuid.UUID]models.ListenerAccount{}
	if len(listenerIDs) > 0 {
		var items []models.ListenerAccount
		_ = s.db.WithContext(ctx).Where("id IN ?", listenerIDs).Find(&items).Error
		for _, item := range items {
			listeners[item.ID] = item
		}
	}
	rows := make([]targetMembershipRow, 0, len(joins))
	for _, join := range joins {
		row := targetMembershipRow{
			ID:            join.ID.String(),
			AccountKind:   join.AccountKind,
			AccountID:     join.AccountID.String(),
			Status:        normalizeAccountTargetMembershipStatus(join.Status, join.Active),
			StatusText:    accountTargetMembershipStatusText(join.Status, join.Active),
			StatusReason:  join.StatusReason,
			Active:        join.Active,
			JoinedAt:      formatOptionalTime(&join.JoinedAt),
			LastCheckedAt: formatOptionalTime(join.LastCheckedAt),
			LastSeenAt:    formatOptionalTime(join.LastSeenAt),
			RemovedAt:     formatOptionalTime(join.RemovedAt),
		}
		if join.AccountKind == accountJoinKindTerminal {
			terminal := terminals[join.AccountID]
			row.Phone = formatTerminalPhoneDisplay(terminal.Phone)
			row.Nickname = terminal.Nickname
			row.AccountStatus = terminal.Status
			row.RiskStatus = firstNonEmpty(terminal.RiskStatus, terminal.BanStatus)
			row.AccountLabel = terminalTargetImportLabel(terminal)
		} else {
			account := listeners[join.AccountID]
			row.Phone = formatTerminalPhoneDisplay(account.Phone)
			row.Nickname = account.Nickname
			row.AccountStatus = account.Status
			row.RiskStatus = account.RiskStatus
			row.AccountLabel = listenerMembershipAccountLabel(account)
		}
		if strings.TrimSpace(row.AccountLabel) == "" {
			row.AccountLabel = row.AccountID
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Active != rows[j].Active {
			return rows[i].Active
		}
		return rows[i].AccountLabel < rows[j].AccountLabel
	})
	return rows
}

type membershipCheckAccount struct {
	Label      string
	FilePath   string
	AccessType string
	Ready      bool
	Reason     string
}

func (s *Server) loadMembershipCheckAccounts(ctx context.Context, joins []models.AccountTargetJoin) map[string]membershipCheckAccount {
	terminalIDs := make([]uuid.UUID, 0)
	listenerIDs := make([]uuid.UUID, 0)
	seenTerminals := map[uuid.UUID]struct{}{}
	seenListeners := map[uuid.UUID]struct{}{}
	for _, join := range joins {
		switch join.AccountKind {
		case accountJoinKindTerminal:
			if _, ok := seenTerminals[join.AccountID]; ok {
				continue
			}
			seenTerminals[join.AccountID] = struct{}{}
			terminalIDs = append(terminalIDs, join.AccountID)
		case accountJoinKindListener:
			if _, ok := seenListeners[join.AccountID]; ok {
				continue
			}
			seenListeners[join.AccountID] = struct{}{}
			listenerIDs = append(listenerIDs, join.AccountID)
		}
	}
	accounts := make(map[string]membershipCheckAccount, len(terminalIDs)+len(listenerIDs))
	if len(terminalIDs) > 0 {
		var terminals []models.Terminal
		_ = s.db.WithContext(ctx).Where("id IN ?", terminalIDs).Find(&terminals).Error
		for _, terminal := range terminals {
			account := membershipCheckAccount{
				Label:      terminalTargetImportLabel(terminal),
				FilePath:   terminal.FilePath,
				AccessType: terminal.AccessType,
				Ready:      terminalReadyForOutboundAction(terminal),
				Reason:     "账号不可用或已被风控限制",
			}
			if account.Ready {
				account.Reason = ""
			}
			accounts[membershipAccountMapKey(accountJoinKindTerminal, terminal.ID)] = account
		}
	}
	if len(listenerIDs) > 0 {
		var listeners []models.ListenerAccount
		_ = s.db.WithContext(ctx).Where("id IN ?", listenerIDs).Find(&listeners).Error
		for _, listener := range listeners {
			account := membershipCheckAccount{
				Label:      listenerMembershipAccountLabel(listener),
				FilePath:   listener.FilePath,
				AccessType: listener.AccessType,
				Ready:      listenerAccountReadyForJoin(listener),
				Reason:     "监听账号不可用或已被风控限制",
			}
			if account.Ready {
				account.Reason = ""
			}
			accounts[membershipAccountMapKey(accountJoinKindListener, listener.ID)] = account
		}
	}
	return accounts
}

func membershipAccountMapKey(accountKind string, accountID uuid.UUID) string {
	return strings.TrimSpace(strings.ToLower(accountKind)) + ":" + accountID.String()
}

func membershipMissingAccountReason(accountKind string) string {
	if accountKind == accountJoinKindListener {
		return "监听账号不存在或已删除"
	}
	if accountKind == accountJoinKindTerminal {
		return "账号不存在或已删除"
	}
	return "未知账号类型"
}

func (s *Server) loadMembershipRefreshTargetKeys(ctx context.Context, tenantID uuid.UUID, adminScope bool, req targetMembershipRefreshRequest) ([]string, error) {
	switch normalizeMembershipTargetScope(req.TargetScope) {
	case "all":
		return nil, nil
	case "target":
		targetID, err := uuid.Parse(strings.TrimSpace(req.TargetID))
		if err != nil {
			return nil, fmt.Errorf("目标 ID 无效")
		}
		var target models.Target
		query := s.db.WithContext(ctx).Where("id = ?", targetID)
		if !adminScope {
			query = query.Where("tenant_id = ?", tenantID)
		}
		if err := query.First(&target).Error; err != nil {
			return nil, fmt.Errorf("目标不存在")
		}
		key := accountTargetJoinKey(target.Type, target.Identifier)
		if key == "" {
			return nil, fmt.Errorf("目标标识无效")
		}
		return []string{key}, nil
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TargetGroupID))
		if err != nil {
			return nil, fmt.Errorf("目标分组 ID 无效")
		}
		query := s.db.WithContext(ctx).Model(&models.Target{}).Order("created_at asc")
		if !adminScope {
			query = query.Where("tenant_id = ?", tenantID)
		}
		filterTenantID := tenantID
		if adminScope {
			filterTenantID = uuid.Nil
		}
		query = s.applyTargetGroupFilter(ctx, query, filterTenantID, groupID)
		var targets []models.Target
		if err := query.Find(&targets).Error; err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(targets))
		for _, target := range targets {
			if key := accountTargetJoinKey(target.Type, target.Identifier); key != "" {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("目标分组内没有可刷新的目标")
		}
		return keys, nil
	default:
		return nil, fmt.Errorf("目标刷新范围无效")
	}
}

func (s *Server) updateTargetMembershipRefreshProgress(ctx context.Context, taskID uuid.UUID, done int, total int) {
	progress := 1
	if total > 0 {
		progress = 1 + int(float64(done)/float64(total)*98)
	}
	if progress > 99 {
		progress = 99
	}
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).Update("progress", progress).Error
}

func (s *Server) finishTargetMembershipRefreshTask(ctx context.Context, task models.Task, status string, summary targetMembershipRefreshSummary, detail string) {
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

func appendTargetMembershipRefreshItem(summary *targetMembershipRefreshSummary, item targetMembershipRefreshResultItem) {
	if len(summary.Items) >= maxTargetMembershipRefreshSummaryItems {
		return
	}
	summary.Items = append(summary.Items, item)
}

func shouldUpdateMembershipProgress(done int, total int) bool {
	if done <= 1 || done >= total || total <= 100 {
		return true
	}
	step := total / 100
	if step < 1 {
		step = 1
	}
	return done%step == 0
}

func (s *Server) refreshListenerJoinedTargetCount(ctx context.Context, accountID uuid.UUID) {
	count := s.countActiveAccountTargetJoins(ctx, uuid.Nil, accountJoinKindListener, accountID)
	_ = s.db.WithContext(ctx).Model(&models.ListenerAccount{}).Where("id = ?", accountID).Updates(map[string]any{
		"joined_targets": count,
		"updated_at":     time.Now(),
	}).Error
}

func (s *Server) pruneUnavailableAccountTargetJoins(ctx context.Context, tenantID uuid.UUID, adminScope bool, accountKind string, keys []string) int64 {
	if len(keys) == 0 {
		return 0
	}
	now := time.Now()
	var sub *gorm.DB
	var allAccountIDs *gorm.DB
	switch accountKind {
	case accountJoinKindTerminal:
		allAccountIDs = s.db.Model(&models.Terminal{}).Select("id")
		sub = s.db.Model(&models.Terminal{}).
			Select("id").
			Where(`
				status IN ?
				OR COALESCE(file_path, '') = ''
				OR (COALESCE(ban_status, '') <> '' AND ban_status <> ?)
				OR COALESCE(risk_status, '') ILIKE ?
				OR COALESCE(risk_status, '') ILIKE ?
				OR COALESCE(risk_status, '') ILIKE ?
			`, []string{"abnormal", "disabled", "banned"}, "正常", "%冻结%", "%受限%", "%frozen%")
		if !adminScope {
			sub = sub.Where("tenant_id = ?", tenantID)
			allAccountIDs = allAccountIDs.Where("tenant_id = ?", tenantID)
		}
	case accountJoinKindListener:
		allAccountIDs = s.db.Model(&models.ListenerAccount{}).Select("id")
		sub = s.db.Model(&models.ListenerAccount{}).
			Select("id").
			Where("status IN ? OR COALESCE(file_path, '') = ''", []string{"abnormal", "failed", "disabled", "banned"})
		if !adminScope {
			sub = sub.Where("tenant_id = ?", tenantID)
			allAccountIDs = allAccountIDs.Where("tenant_id = ?", tenantID)
		}
	default:
		return 0
	}
	baseUpdates := map[string]any{
		"status":          accountTargetStatusAccountUnavailable,
		"status_reason":   "账号当前不可用，后台已从目标群有效账号中移除",
		"active":          false,
		"last_checked_at": now,
		"removed_at":      now,
		"updated_at":      now,
	}
	query := s.db.WithContext(ctx).Model(&models.AccountTargetJoin{}).
		Where("account_kind = ? AND active = ? AND target_key IN ? AND account_id IN (?)", accountKind, true, keys, sub)
	if !adminScope {
		query = query.Where("tenant_id = ?", tenantID)
	}
	result := query.Updates(baseUpdates)
	if result.Error != nil {
		return 0
	}
	missingQuery := s.db.WithContext(ctx).Model(&models.AccountTargetJoin{}).
		Where("account_kind = ? AND active = ? AND target_key IN ?", accountKind, true, keys).
		Where("account_id NOT IN (?)", allAccountIDs)
	if !adminScope {
		missingQuery = missingQuery.Where("tenant_id = ?", tenantID)
	}
	missingUpdates := map[string]any{}
	for key, value := range baseUpdates {
		missingUpdates[key] = value
	}
	missingUpdates["status_reason"] = "账号已不存在，后台已从目标群有效账号中移除"
	missingResult := missingQuery.Updates(missingUpdates)
	if missingResult.Error != nil {
		return result.RowsAffected
	}
	return result.RowsAffected + missingResult.RowsAffected
}

func normalizeMembershipAccountKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case accountJoinKindTerminal:
		return accountJoinKindTerminal
	case accountJoinKindListener:
		return accountJoinKindListener
	default:
		return "all"
	}
}

func membershipAccountKinds(value string) []string {
	switch normalizeMembershipAccountKind(value) {
	case accountJoinKindTerminal:
		return []string{accountJoinKindTerminal}
	case accountJoinKindListener:
		return []string{accountJoinKindListener}
	default:
		return []string{accountJoinKindTerminal, accountJoinKindListener}
	}
}

func normalizeMembershipTargetScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "target":
		return "target"
	case "group":
		return "group"
	default:
		return "all"
	}
}

func normalizeMembershipCheckStatus(status string, err error) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		if err != nil {
			return accountTargetStatusCheckFailed
		}
		return accountTargetStatusActive
	}
	if normalized == "already_joined" || normalized == "success" {
		return accountTargetStatusActive
	}
	if normalized == "account_invalid" {
		return accountTargetStatusAccountUnavailable
	}
	if normalized == "failed" && err != nil {
		return accountTargetStatusCheckFailed
	}
	return normalized
}

func membershipStatusDisablesAccountTarget(status string) bool {
	switch normalizeMembershipCheckStatus(status, nil) {
	case accountTargetStatusAccountUnavailable, accountTargetStatusNotMember, accountTargetStatusKicked, accountTargetStatusBanned, accountTargetStatusInaccessible, accountTargetStatusTargetInvalid:
		return true
	default:
		return false
	}
}

func membershipStatusFallbackReason(status string, err error) string {
	if err != nil {
		return err.Error()
	}
	return accountTargetMembershipStatusText(status, false)
}

func membershipBatchErrorReason(err error) string {
	if err == nil {
		return "成员状态检测失败"
	}
	return err.Error()
}

func membershipRefreshLogLevel(active bool, status string, err error) string {
	if !active || membershipStatusDisablesAccountTarget(status) {
		return "WARN"
	}
	if strings.EqualFold(strings.TrimSpace(status), "flood_wait") {
		return "WARN"
	}
	if err != nil {
		return "ERROR"
	}
	return "INFO"
}

func shouldLogMembershipRefreshResult(active bool, status string, err error) bool {
	if err != nil || !active {
		return true
	}
	return !accountTargetMembershipActive(status)
}

func listenerMembershipAccountLabel(account models.ListenerAccount) string {
	phone := formatTerminalPhoneDisplay(account.Phone)
	if strings.TrimSpace(account.Nickname) != "" {
		return strings.TrimSpace(phone + " / " + account.Nickname)
	}
	return firstNonEmpty(phone, account.ID.String())
}
