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

type MassMessageStep struct {
	Type         string `json:"type" binding:"required"` // "text", "image", "voice", "gif", "forward"
	Content      string `json:"content"`
	MediaAssetID string `json:"media_asset_id"`
	SourceChatID string `json:"source_chat_id"`
	MessageID    string `json:"message_id"`
	DelaySeconds int    `json:"delay_seconds"`
}

type MassMessagingRequest struct {
	TerminalGroupIDs     []string          `json:"terminal_group_ids"`
	TargetGroupIDs       []string          `json:"target_group_ids"`
	Steps                []MassMessageStep `json:"steps" binding:"required"`
	SendCount            int               `json:"send_count"`
	SendIntervalSeconds  int               `json:"send_interval_seconds"`
	RepeatCount          int               `json:"repeat_count"`
	RepeatIntervalSecond int               `json:"repeat_interval_seconds"`
}

func (s *Server) CreateMassMessagingTask(c *gin.Context) {
	var req MassMessagingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "无效的任务参数")
		return
	}

	if len(req.Steps) == 0 || len(req.Steps) > 10 {
		utils.Fail(c, http.StatusBadRequest, "消息漏斗需要包含 1 到 10 个阶段")
		return
	}
	sendCount := normalizeMassSendCount(req.SendCount, req.RepeatCount)
	sendIntervalSeconds := normalizeMassSendInterval(req.SendIntervalSeconds, req.RepeatIntervalSecond)

	payloadInfo := map[string]any{
		"step_index":            0,
		"terminal_group_ids":    req.TerminalGroupIDs,
		"target_group_ids":      req.TargetGroupIDs,
		"steps":                 req.Steps,
		"send_count":            sendCount,
		"send_interval_seconds": sendIntervalSeconds,
	}

	b, _ := json.Marshal(payloadInfo)
	rawPayload, _ := datatypes.JSON.MarshalJSON(datatypes.JSON(b))

	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantFilterID(c),
		Name:      "消息群发任务",
		Type:      "mass_messaging",
		Status:    "queued",
		Progress:  0,
		Payload:   rawPayload,
		CreatedBy: s.userIDPtr(c),
	}

	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建群发任务失败")
		return
	}

	s.logTask(c, task, "INFO", "created", "通知工作流任务已创建，等待执行器消费")
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.runMassMessagingTask(task.ID)
	}
	utils.Created(c, task)
}

func (s *Server) RunMassMessagingTask(taskID uuid.UUID) {
	s.runMassMessagingTask(taskID)
}

func (s *Server) runMassMessagingTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "mass_messaging")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return
	}
	if task.Type != "mass_messaging" {
		return
	}

	var payload struct {
		StepIndex            int               `json:"step_index"`
		TerminalGroupIDs     []string          `json:"terminal_group_ids"`
		TargetGroupIDs       []string          `json:"target_group_ids"`
		Steps                []MassMessageStep `json:"steps"`
		SendCount            int               `json:"send_count"`
		SendIntervalSeconds  int               `json:"send_interval_seconds"`
		RepeatCount          int               `json:"repeat_count"`
		RepeatIntervalSecond int               `json:"repeat_interval_seconds"`
	}
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		s.failMassMessagingTask(ctx, task, "payload", "通知工作流参数解析失败："+err.Error())
		return
	}
	if len(payload.Steps) == 0 {
		s.failMassMessagingTask(ctx, task, "payload", "通知工作流没有可执行的消息阶段")
		return
	}
	sendCount := normalizeMassSendCount(payload.SendCount, payload.RepeatCount)
	sendIntervalSeconds := normalizeMassSendInterval(payload.SendIntervalSeconds, payload.RepeatIntervalSecond)

	terminals, err := s.loadMassMessagingTerminals(ctx, task.TenantID, payload.TerminalGroupIDs)
	if err != nil {
		s.failMassMessagingTask(ctx, task, "terminals", err.Error())
		return
	}
	targets, err := s.loadMassMessagingTargets(ctx, task.TenantID, payload.TargetGroupIDs)
	if err != nil {
		s.failMassMessagingTask(ctx, task, "targets", err.Error())
		return
	}
	terminalCount := len(terminals)
	targetCount := len(targets)
	if terminalCount == 0 || targetCount == 0 {
		s.failMassMessagingTask(ctx, task, "selection", fmt.Sprintf("通知工作流无法执行：终端 %d 个，目标 %d 个，请先选择或导入可用终端和目标池", terminalCount, targetCount))
		return
	}

	s.updateTaskState(ctx, task.ID, "running", 5, nil)
	s.logTaskBackground(ctx, task, "INFO", "start", fmt.Sprintf("通知工作流开始执行：终端 %d 个，目标 %d 个，阶段 %d 个，发送 %d 次，投递间隔 %d 秒", terminalCount, targetCount, len(payload.Steps), sendCount, sendIntervalSeconds))

	settings := s.readSystemSettings(ctx, task.TenantID)
	totalSteps := len(payload.Steps)
	totalMessages := int64(targetCount * totalSteps * sendCount)
	realExecution := settings.Adapter.TelegramApplyEnabled && !settings.Adapter.WorkflowDryRun
	if realExecution {
		s.logTaskBackground(ctx, task, "INFO", "adapter", "Telegram 执行适配器已开启，通知工作流将尝试真实发送")
	} else {
		s.logTaskBackground(ctx, task, "WARN", "adapter", "当前为 dry-run：未开启执行适配器或工作流 dry-run 仍开启，只做本地编排与模拟推进")
	}

	assets, err := s.loadMassMessagingAssets(ctx, task.TenantID, payload.Steps)
	if err != nil {
		s.failMassMessagingTask(ctx, task, "assets", err.Error())
		return
	}

	successCount := int64(0)
	failedCount := int64(0)
	deliveryIndex := int64(0)
	skipReasonCounts := map[string]int{}
	messenger := telegram_client.NewMessenger(s.cfg)
	s.logTaskBackground(ctx, task, "INFO", "dispatch_policy", fmt.Sprintf("单目标单账号发送：不设置固定冷却时间，每个目标每条消息只选择 1 个终端发送，优先选择 %d 个终端中距离上一次发送时间最长的账号", terminalCount))
	for round := 1; round <= sendCount; round++ {
		if sendCount > 1 {
			s.logTaskBackground(ctx, task, "INFO", "round", fmt.Sprintf("第 %d/%d 次发送开始", round, sendCount))
		}
		for index, step := range payload.Steps {
			progress := 10 + int(float64(deliveryIndex)/float64(maxInt64(totalMessages, 1))*75)
			s.updateTaskState(ctx, task.ID, "running", progress, nil)
			s.logTaskBackground(ctx, task, "INFO", "step", fmt.Sprintf("第 %d 次发送，第 %d/%d 阶段：%s，预计覆盖 %d 条投递", round, index+1, totalSteps, massStepRunLabel(step), targetCount))

			if realExecution {
				for _, target := range targets {
					terminalIndex, terminal, quotaErr := s.pickMassMessagingTerminal(ctx, task.TenantID, terminals, target)
					if quotaErr != nil {
						failedCount++
						deliveryIndex++
						accumulateSkipReason(skipReasonCounts, quotaErr.Error())
						s.logTaskBackground(ctx, task, "WARN", "quota_exhausted", quotaErr.Error())
						if !s.waitMassMessagingInterval(ctx, task, sendIntervalSeconds, deliveryIndex, totalMessages) {
							return
						}
						continue
					}
					result, sendErr := messenger.Send(ctx, telegram_client.MessageRequest{
						FilePath:     terminal.FilePath,
						AccessType:   terminal.AccessType,
						TargetType:   target.Type,
						Target:       target.Identifier,
						StepType:     step.Type,
						Content:      step.Content,
						MediaPath:    massStepMediaPath(step, assets),
						SourceChatID: step.SourceChatID,
						MessageID:    step.MessageID,
					})
					deliveryIndex++
					markMassMessagingTerminalUsed(terminals, terminalIndex, time.Now())
					if sendErr != nil || !result.OK {
						failedCount++
						reason := result.Reason
						if reason == "" && sendErr != nil {
							reason = sendErr.Error()
						}
						s.applyTerminalOutboundFailure(ctx, terminal.ID, reason)
						s.applyTerminalTargetFailure(ctx, task.TenantID, terminal.ID, terminalQuotaActionDM, target.Type, target.Identifier, reason)
						s.incrementTerminalDelivery(ctx, terminal.ID, false)
						s.logMassMessagingDelivery(ctx, task, "ERROR", terminal, target, round, index, reason)
						if !s.waitMassMessagingInterval(ctx, task, sendIntervalSeconds, deliveryIndex, totalMessages) {
							return
						}
						continue
					}
					successCount++
					s.incrementTerminalDelivery(ctx, terminal.ID, true)
					s.incrementTargetNotification(ctx, target.ID)
					s.logMassMessagingDelivery(ctx, task, "INFO", terminal, target, round, index, result.Reason)
					if !s.waitMassMessagingInterval(ctx, task, sendIntervalSeconds, deliveryIndex, totalMessages) {
						return
					}
				}
			} else {
				successCount += int64(targetCount)
				deliveryIndex += int64(targetCount)
				time.Sleep(300 * time.Millisecond)
			}

			if step.DelaySeconds > 0 {
				if realExecution {
					s.logTaskBackground(ctx, task, "INFO", "delay", fmt.Sprintf("第 %d 次发送，第 %d 阶段设置前置/阶段延迟 %d 秒，真实执行等待中", round, index+1, step.DelaySeconds))
					select {
					case <-ctx.Done():
						s.failMassMessagingTask(ctx, task, "timeout", "通知工作流执行超时")
						return
					case <-time.After(time.Duration(step.DelaySeconds) * time.Second):
					}
				} else {
					s.logTaskBackground(ctx, task, "INFO", "delay", fmt.Sprintf("第 %d 次发送，第 %d 阶段设置延迟 %d 秒，已写入执行计划", round, index+1, step.DelaySeconds))
				}
			}
		}
	}

	status := "success"
	if !realExecution {
		status = "dry_run"
	} else if failedCount > 0 && successCount == 0 {
		status = "failed"
	} else if failedCount > 0 {
		status = "partial_success"
	}
	detail := fmt.Sprintf("通知工作流执行完成：可选终端 %d 个，目标 %d 个，阶段 %d 个，发送 %d 次，投递间隔 %d 秒，总投递 %d 条，成功 %d 条，失败 %d 条", terminalCount, targetCount, totalSteps, sendCount, sendIntervalSeconds, totalMessages, successCount, failedCount)
	if !realExecution {
		detail += "；当前为 dry-run，本次未真实发送 Telegram 消息"
	}
	summary, _ := json.Marshal(gin.H{
		"terminal_count":   terminalCount,
		"target_count":     targetCount,
		"step_count":       totalSteps,
		"send_count":       sendCount,
		"send_interval":    sendIntervalSeconds,
		"dispatch_policy":  "one_terminal_per_target",
		"selection_policy": "oldest_last_message_at_without_fixed_cooldown",
		"total":            totalMessages,
		"success":          successCount,
		"failed":           failedCount,
		"top_skip_reason":  topSkipReason(skipReasonCounts),
		"skip_reasons":     skipReasonCounts,
		"dry_run":          !realExecution,
	})
	s.updateTaskState(ctx, task.ID, status, 100, datatypes.JSON(summary))
	s.logTaskBackground(ctx, task, "INFO", "summary", detail)
}

func (s *Server) failMassMessagingTask(ctx context.Context, task models.Task, action string, detail string) {
	s.updateTaskState(ctx, task.ID, "failed", 100, nil)
	s.logTaskBackground(ctx, task, "ERROR", action, detail)
}

func (s *Server) loadMassMessagingTerminals(ctx context.Context, tenantID uuid.UUID, groupIDs []string) ([]models.Terminal, error) {
	var terminals []models.Terminal
	query := s.db.WithContext(ctx).
		Where("file_path <> ''").
		Where("status NOT IN ?", []string{"abnormal", "banned", "disabled"}).
		Where("(sleep_until IS NULL OR sleep_until <= ?)", time.Now()).
		Order("last_message_at asc nulls first").
		Order("created_at asc")
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	ids, err := parseMassMessagingGroupIDs(groupIDs)
	if err != nil {
		return nil, fmt.Errorf("终端分组 ID 无效：%w", err)
	}
	if len(ids) > 0 {
		query = query.Where("group_id IN ?", ids)
	}
	if err := query.Find(&terminals).Error; err != nil {
		return nil, fmt.Errorf("读取终端失败：%w", err)
	}
	return terminals, nil
}

func (s *Server) loadMassMessagingTargets(ctx context.Context, tenantID uuid.UUID, groupIDs []string) ([]models.Target, error) {
	var targets []models.Target
	query := s.db.WithContext(ctx).Order("created_at asc")
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	ids, err := parseMassMessagingGroupIDs(groupIDs)
	if err != nil {
		return nil, fmt.Errorf("目标分组 ID 无效：%w", err)
	}
	if len(ids) > 0 {
		clauses := make([]string, 0, len(ids))
		args := make([]any, 0, len(ids)*2)
		for _, groupID := range ids {
			subQuery := s.db.WithContext(ctx).
				Model(&models.TargetGroupBinding{}).
				Select("target_id").
				Where("group_id = ?", groupID)
			if tenantID != uuid.Nil {
				subQuery = subQuery.Where("tenant_id = ?", tenantID)
			}
			clauses = append(clauses, "(group_id = ? OR id IN (?))")
			args = append(args, groupID, subQuery)
		}
		query = query.Where(strings.Join(clauses, " OR "), args...)
	}
	if err := query.Find(&targets).Error; err != nil {
		return nil, fmt.Errorf("读取目标失败：%w", err)
	}
	return targets, nil
}

func parseMassMessagingGroupIDs(rawIDs []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, raw := range rawIDs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func massStepRunLabel(step MassMessageStep) string {
	switch step.Type {
	case "text":
		content := strings.TrimSpace(step.Content)
		if content == "" {
			return "文本消息"
		}
		if len([]rune(content)) > 24 {
			content = string([]rune(content)[:24]) + "..."
		}
		return "文本消息「" + content + "」"
	case "image":
		return "推送图片素材 " + step.MediaAssetID
	case "voice":
		return "发送语音素材 " + step.MediaAssetID
	case "gif":
		return "发送 GIF 素材 " + step.MediaAssetID
	case "forward":
		return "转发消息 " + step.SourceChatID + "/" + step.MessageID
	default:
		return "未知阶段 " + step.Type
	}
}

func (s *Server) loadMassMessagingAssets(ctx context.Context, tenantID uuid.UUID, steps []MassMessageStep) (map[string]models.Asset, error) {
	ids := make([]uuid.UUID, 0)
	for _, step := range steps {
		if step.Type != "image" && step.Type != "voice" && step.Type != "gif" {
			continue
		}
		raw := strings.TrimSpace(step.MediaAssetID)
		if raw == "" {
			return nil, fmt.Errorf("%s 阶段缺少媒体素材", step.Type)
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("媒体素材 ID 无效：%w", err)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[string]models.Asset{}, nil
	}
	var assets []models.Asset
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("读取媒体素材失败：%w", err)
	}
	assetMap := make(map[string]models.Asset, len(assets))
	for _, asset := range assets {
		assetMap[asset.ID.String()] = asset
	}
	for _, id := range ids {
		if _, ok := assetMap[id.String()]; !ok {
			return nil, fmt.Errorf("媒体素材不存在：%s", id.String())
		}
	}
	return assetMap, nil
}

func massStepMediaPath(step MassMessageStep, assets map[string]models.Asset) string {
	asset, ok := assets[strings.TrimSpace(step.MediaAssetID)]
	if !ok {
		return ""
	}
	return asset.FilePath
}

func (s *Server) pickMassMessagingTerminal(ctx context.Context, tenantID uuid.UUID, terminals []models.Terminal, target models.Target) (int, models.Terminal, error) {
	candidateIndexes := make([]int, 0, len(terminals))
	for index := range terminals {
		if terminalReadyForOutboundAction(terminals[index]) && s.terminalTargetAvailable(ctx, tenantID, terminals[index].ID, terminalQuotaActionDM, target.Type, target.Identifier) {
			candidateIndexes = append(candidateIndexes, index)
		}
	}
	if len(candidateIndexes) == 0 {
		return -1, models.Terminal{}, fmt.Errorf("没有可用的群发账号")
	}
	sort.Slice(candidateIndexes, func(i, j int) bool {
		return terminalOlderForMessaging(terminals[candidateIndexes[i]], terminals[candidateIndexes[j]])
	})
	skipReasons := make([]string, 0, len(candidateIndexes))
	for _, index := range candidateIndexes {
		terminal, err := s.reserveTerminalQuota(ctx, terminals[index].ID, terminalQuotaActionDM)
		if err == nil {
			terminals[index] = terminal
			return index, terminal, nil
		}
		skipReasons = append(skipReasons, err.Error())
	}
	return -1, models.Terminal{}, fmt.Errorf("全部群发账号已跳过：%s", summarizeTerminalSkipReasons(skipReasons, "账号限额已满或不可用"))
}

func terminalOlderForMessaging(candidate models.Terminal, current models.Terminal) bool {
	if candidate.LastMessageAt == nil && current.LastMessageAt != nil {
		return true
	}
	if candidate.LastMessageAt != nil && current.LastMessageAt == nil {
		return false
	}
	if candidate.LastMessageAt != nil && current.LastMessageAt != nil {
		if candidate.LastMessageAt.Before(*current.LastMessageAt) {
			return true
		}
		if current.LastMessageAt.Before(*candidate.LastMessageAt) {
			return false
		}
	}
	return candidate.CreatedAt.Before(current.CreatedAt)
}

func markMassMessagingTerminalUsed(terminals []models.Terminal, index int, usedAt time.Time) {
	if index < 0 || index >= len(terminals) {
		return
	}
	terminals[index].LastMessageAt = &usedAt
}

func (s *Server) logMassMessagingDelivery(ctx context.Context, task models.Task, level string, terminal models.Terminal, target models.Target, round int, stepIndex int, detail string) {
	if strings.TrimSpace(detail) == "" {
		detail = "已完成"
	}
	_ = s.db.WithContext(ctx).Create(&models.TaskLog{
		ID:          uuid.New(),
		TenantID:    task.TenantID,
		TaskID:      task.ID,
		Level:       level,
		Category:    task.Type,
		TerminalRef: terminalDisplayName(terminal),
		TargetRef:   target.Name,
		Action:      "deliver",
		Details:     fmt.Sprintf("第 %d 次发送，第 %d 阶段 -> %s：%s", round, stepIndex+1, target.Identifier, detail),
		TraceID:     uuid.NewString(),
		CreatedAt:   time.Now(),
	}).Error
}

func (s *Server) waitMassMessagingInterval(ctx context.Context, task models.Task, intervalSeconds int, deliveryIndex int64, totalMessages int64) bool {
	if intervalSeconds <= 0 || deliveryIndex >= totalMessages {
		return true
	}
	s.logTaskBackground(ctx, task, "INFO", "interval", fmt.Sprintf("投递间隔 %d 秒，进度 %d/%d", intervalSeconds, deliveryIndex, totalMessages))
	select {
	case <-ctx.Done():
		s.failMassMessagingTask(ctx, task, "timeout", "通知工作流执行超时")
		return false
	case <-time.After(time.Duration(intervalSeconds) * time.Second):
		return true
	}
}

func normalizeMassSendCount(sendCount int, repeatCount int) int {
	if sendCount <= 0 {
		sendCount = repeatCount
	}
	if sendCount <= 0 {
		return 1
	}
	if sendCount > 100 {
		return 100
	}
	return sendCount
}

func normalizeMassSendInterval(intervalSeconds int, repeatIntervalSeconds int) int {
	if intervalSeconds <= 0 && repeatIntervalSeconds > 0 {
		intervalSeconds = repeatIntervalSeconds
	}
	if intervalSeconds < 0 {
		return 0
	}
	if intervalSeconds > 86400 {
		return 86400
	}
	return intervalSeconds
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *Server) incrementTerminalDelivery(ctx context.Context, terminalID uuid.UUID, success bool) {
	updates := map[string]any{}
	updates["last_message_at"] = time.Now()
	if success {
		updates["today_success"] = gormExpr("today_success + 1")
		updates["total_success"] = gormExpr("total_success + 1")
	} else {
		updates["today_failed"] = gormExpr("today_failed + 1")
		updates["total_failed"] = gormExpr("total_failed + 1")
	}
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Where("id = ?", terminalID).Updates(updates).Error
}

func (s *Server) incrementTargetNotification(ctx context.Context, targetID uuid.UUID) {
	_ = s.db.WithContext(ctx).Model(&models.Target{}).Where("id = ?", targetID).Update("notification_count", gormExpr("notification_count + 1")).Error
}

func gormExpr(sql string) any {
	return gorm.Expr(sql)
}
