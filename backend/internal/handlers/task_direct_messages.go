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
)

type DirectMessageStep struct {
	Type         string `json:"type" binding:"required"`
	Content      string `json:"content"`
	MediaAssetID string `json:"media_asset_id"`
	SourceChatID string `json:"source_chat_id"`
	MessageID    string `json:"message_id"`
	DelaySeconds int    `json:"delay_seconds"`
}

type DirectMessageJobRequest struct {
	Name                  string              `json:"name"`
	LeadIDs               []string            `json:"lead_ids" binding:"required"`
	TerminalScope         string              `json:"terminal_scope"`
	TerminalGroupID       string              `json:"terminal_group_id"`
	TerminalID            string              `json:"terminal_id"`
	TerminalIDs           []string            `json:"terminal_ids"`
	Steps                 []DirectMessageStep `json:"steps" binding:"required"`
	MinDelaySeconds       int                 `json:"min_delay_seconds"`
	MaxDelaySeconds       int                 `json:"max_delay_seconds"`
	CooldownMinutes       int                 `json:"cooldown_minutes"`
	CooldownJitterMinutes int                 `json:"cooldown_jitter_minutes"`
	DedupeDays            int                 `json:"dedupe_days"`
	SkipNoAccount         bool                `json:"skip_no_account"`
	StopOnReply           bool                `json:"stop_on_reply"`
	DryRun                bool                `json:"dry_run"`
}

type directMessageTaskPayload DirectMessageJobRequest

func (s *Server) CreateDirectMessageTask(c *gin.Context) {
	var req DirectMessageJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "监听私信任务参数格式不正确")
		return
	}
	normalized, leadIDs, terminalIDs, err := normalizeDirectMessageRequest(req)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	var leads []models.SCRMLead
	query := s.db.WithContext(c.Request.Context()).
		Where("id IN ?", leadIDs).
		Where("(status IS NULL OR status <> ?)", "blacklisted")
	query = s.applySCRMOwnerScope(c, query)
	if err := query.Order("COALESCE(hit_at, created_at) desc").Find(&leads).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听线索失败")
		return
	}
	if len(leads) == 0 {
		utils.Fail(c, http.StatusBadRequest, "没有可用的监听线索")
		return
	}
	leadIDs = leadIDs[:0]
	for _, lead := range leads {
		leadIDs = append(leadIDs, lead.ID)
	}

	terminalPayload := directMessageTaskPayload(normalized)
	terminalPayload.TerminalIDs = uuidSliceStrings(terminalIDs)
	terminals, err := s.loadDirectMessageTerminals(c.Request.Context(), s.tenantFilterID(c), terminalPayload)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(terminals) == 0 {
		utils.Fail(c, http.StatusBadRequest, "当前账号池没有可用于私信的账号")
		return
	}

	contactable := 0
	for _, lead := range leads {
		if directMessageLeadTarget(lead) != "" {
			contactable++
		}
	}
	if contactable == 0 {
		utils.Fail(c, http.StatusBadRequest, "选中的线索没有可私信的 Telegram 用户名")
		return
	}

	payloadBytes, _ := json.Marshal(terminalPayload)
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantFilterID(c),
		Name:      firstNonEmpty(normalized.Name, "监听私信任务"),
		Type:      "direct_messages",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payloadBytes),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建监听私信任务失败")
		return
	}

	s.logTask(c, task, "INFO", "created", fmt.Sprintf("监听私信任务已创建：线索 %d 条，可私信 %d 条，候选账号 %d 个，消息阶段 %d 个", len(leads), contactable, len(terminals), len(normalized.Steps)))
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.runDirectMessagesTask(task.ID)
	}
	utils.Created(c, task)
}

func (s *Server) RunDirectMessagesTask(taskID uuid.UUID) {
	s.runDirectMessagesTask(taskID)
}

func (s *Server) runDirectMessagesTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "direct_messages")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "direct_messages").First(&task).Error; err != nil {
		return
	}

	var payload directMessageTaskPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		s.failDirectMessagesTask(ctx, task, "payload", "监听私信参数解析失败："+err.Error())
		return
	}

	leadIDs, err := parseUUIDStrings(payload.LeadIDs, "线索 ID 无效")
	if err != nil || len(leadIDs) == 0 {
		s.failDirectMessagesTask(ctx, task, "payload", "监听私信没有可执行线索")
		return
	}
	leads, err := s.loadDirectMessageLeads(ctx, task, leadIDs)
	if err != nil {
		s.failDirectMessagesTask(ctx, task, "leads", err.Error())
		return
	}
	terminals, err := s.loadDirectMessageTerminals(ctx, task.TenantID, payload)
	if err != nil {
		s.failDirectMessagesTask(ctx, task, "terminals", err.Error())
		return
	}
	if len(leads) == 0 || len(terminals) == 0 {
		s.failDirectMessagesTask(ctx, task, "selection", fmt.Sprintf("监听私信无法执行：线索 %d 条，账号 %d 个", len(leads), len(terminals)))
		return
	}
	assets, err := s.loadDirectMessageAssets(ctx, task.TenantID, payload.Steps)
	if err != nil {
		s.failDirectMessagesTask(ctx, task, "assets", err.Error())
		return
	}

	settings := s.readSystemSettings(ctx, directMessageSettingsTenant(task, leads))
	realExecution := settings.Adapter.TelegramApplyEnabled && !settings.Adapter.WorkflowDryRun && !payload.DryRun
	if realExecution {
		s.logTaskBackground(ctx, task, "INFO", "adapter", "Telegram 执行适配器已开启，监听私信将真实发送")
	} else {
		s.logTaskBackground(ctx, task, "WARN", "adapter", "当前为 dry-run：适配器未开启、工作流 dry-run 开启或任务手动 dry-run")
	}

	totalMessages := int64(maxInt(len(leads)*len(payload.Steps), 1))
	successCount := int64(0)
	failedCount := int64(0)
	skippedCount := int64(0)
	deliveryIndex := int64(0)
	skipReasonCounts := map[string]int{}
	messenger := telegram_client.NewMessenger(s.cfg)
	startedAt := time.Now()

	s.updateTaskState(ctx, task.ID, "running", 5, nil)
	s.logTaskBackground(ctx, task, "INFO", "start", fmt.Sprintf("开始监听私信：候选账号 %d 个，线索 %d 条，消息阶段 %d 个", len(terminals), len(leads), len(payload.Steps)))

	for _, lead := range leads {
		target := directMessageLeadTarget(lead)
		if target == "" {
			skippedCount++
			accumulateSkipReason(skipReasonCounts, "线索缺少 Telegram 用户名")
			s.logTaskBackground(ctx, task, "WARN", "lead_skipped", "线索缺少 Telegram 用户名，已跳过："+leadDisplayForDirectMessage(lead))
			continue
		}
		if payload.DedupeDays > 0 && s.directMessageLeadRecentlyContacted(ctx, lead, payload.DedupeDays) {
			skippedCount++
			accumulateSkipReason(skipReasonCounts, "去重窗口内已触达")
			s.logTaskBackground(ctx, task, "INFO", "lead_deduped", fmt.Sprintf("%s 在 %d 天内已触达，已跳过", target, payload.DedupeDays))
			continue
		}

		terminalIndex := -1
		var terminal models.Terminal
		if realExecution {
			var pickErr error
			terminalIndex, terminal, pickErr = s.pickDirectMessageTerminal(ctx, terminals, target, payload.CooldownMinutes, payload.CooldownJitterMinutes)
			if pickErr != nil {
				failedCount++
				accumulateSkipReason(skipReasonCounts, pickErr.Error())
				s.logTaskBackground(ctx, task, "WARN", "quota_exhausted", fmt.Sprintf("%s 分配私信账号失败：%s", target, pickErr.Error()))
				continue
			}
			s.logTaskBackground(ctx, task, "INFO", "terminal_assigned", fmt.Sprintf("线索 %s 已分配账号 %s，整套消息编排将使用该账号发送", target, terminalDisplayName(terminal)))
		}

		for stepIndex, step := range payload.Steps {
			if payload.StopOnReply && stepIndex > 0 && s.directMessageLeadHasInboundSince(ctx, lead.ID, startedAt) {
				skippedCount++
				accumulateSkipReason(skipReasonCounts, "线索已回复，停止后续编排")
				s.logTaskBackground(ctx, task, "INFO", "sequence_stopped", "检测到线索回复，停止后续私信编排："+target)
				break
			}

			deliveryIndex++
			progress := 10 + int(float64(deliveryIndex)/float64(totalMessages)*80)
			s.updateTaskState(ctx, task.ID, "running", minInt(progress, 95), nil)
			content := renderDirectMessageTemplate(step.Content, lead)
			if step.Type == "text" && strings.TrimSpace(content) == "" {
				failedCount++
				accumulateSkipReason(skipReasonCounts, "消息内容为空")
				s.logTaskBackground(ctx, task, "ERROR", "message_empty", fmt.Sprintf("第 %d 阶段消息内容为空，目标 %s 已跳过", stepIndex+1, target))
				break
			}

			if !realExecution {
				successCount++
				s.logTaskBackground(ctx, task, "INFO", "dry_run", fmt.Sprintf("dry-run 编排：第 %d 阶段 -> %s：%s", stepIndex+1, target, directMessageStepPreview(step, content)))
				continue
			}

			result, sendErr := messenger.Send(ctx, telegram_client.MessageRequest{
				FilePath:     terminal.FilePath,
				AccessType:   terminal.AccessType,
				TargetType:   "user",
				Target:       target,
				StepType:     step.Type,
				Content:      content,
				MediaPath:    directMessageStepMediaPath(step, assets),
				SourceChatID: renderDirectMessageTemplate(step.SourceChatID, lead),
				MessageID:    strings.TrimSpace(step.MessageID),
			})
			markMassMessagingTerminalUsed(terminals, terminalIndex, time.Now())
			if sendErr != nil || !result.OK {
				reason := strings.TrimSpace(result.Reason)
				if reason == "" && sendErr != nil {
					reason = sendErr.Error()
				}
				reason = firstNonEmpty(reason, "私信发送失败")
				failedCount++
				accumulateSkipReason(skipReasonCounts, reason)
				s.applyTerminalOutboundFailure(ctx, terminal.ID, reason)
				s.applyTerminalTargetFailure(ctx, terminal.TenantID, terminal.ID, terminalQuotaActionDM, "user", target, reason)
				s.incrementTerminalDelivery(ctx, terminal.ID, false)
				s.logTaskBackground(ctx, task, "ERROR", "dm_failed", fmt.Sprintf("账号 %s -> %s，第 %d 阶段失败：%s", terminalDisplayName(terminal), target, stepIndex+1, reason))
				break
			}

			successCount++
			s.incrementTerminalDelivery(ctx, terminal.ID, true)
			s.recordDirectMessageDelivery(ctx, lead, terminal, directMessageRecordContent(step, content))
			s.logTaskBackground(ctx, task, "INFO", "dm_sent", fmt.Sprintf("账号 %s -> %s，第 %d/%d 阶段已发送：%s", terminalDisplayName(terminal), target, stepIndex+1, len(payload.Steps), directMessageStepPreview(step, content)))

			if step.DelaySeconds > 0 && !s.waitDirectMessageDelay(ctx, task, step.DelaySeconds, "step_delay") {
				return
			}
			delay := randomDelaySeconds(payload.MinDelaySeconds, payload.MaxDelaySeconds)
			if delay > 0 && !s.waitDirectMessageDelay(ctx, task, delay, "send_interval") {
				return
			}
		}
	}

	status := "success"
	if !realExecution {
		status = "dry_run"
	} else if failedCount > 0 && successCount == 0 {
		status = "failed"
	} else if failedCount > 0 || skippedCount > 0 {
		status = "partial_success"
	}
	summary, _ := json.Marshal(gin.H{
		"lead_count":        len(leads),
		"terminal_count":    len(terminals),
		"step_count":        len(payload.Steps),
		"total":             totalMessages,
		"success":           successCount,
		"failed":            failedCount,
		"skipped":           skippedCount,
		"top_skip_reason":   topSkipReason(skipReasonCounts),
		"skip_reasons":      skipReasonCounts,
		"dry_run":           !realExecution,
		"selection_policy":  "one_terminal_per_lead_sequence_with_quota_guard",
		"terminal_scope":    payload.TerminalScope,
		"stop_on_reply":     payload.StopOnReply,
		"dedupe_days":       payload.DedupeDays,
		"min_delay_seconds": payload.MinDelaySeconds,
		"max_delay_seconds": payload.MaxDelaySeconds,
		"cooldown_minutes":  payload.CooldownMinutes,
		"cooldown_jitter":   payload.CooldownJitterMinutes,
	})
	s.updateTaskState(ctx, task.ID, status, 100, datatypes.JSON(summary))
	s.logTaskBackground(ctx, task, "INFO", "summary", fmt.Sprintf("监听私信完成：成功 %d 条，失败 %d 条，跳过 %d 条", successCount, failedCount, skippedCount))
}

func normalizeDirectMessageRequest(req DirectMessageJobRequest) (DirectMessageJobRequest, []uuid.UUID, []uuid.UUID, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "监听私信任务"
	}
	if len(req.LeadIDs) == 0 || len(req.LeadIDs) > 5000 {
		return req, nil, nil, fmt.Errorf("线索数量需要在 1 到 5000 条之间")
	}
	leadIDs, err := parseUUIDStrings(req.LeadIDs, "线索 ID 无效")
	if err != nil {
		return req, nil, nil, err
	}
	if len(req.Steps) == 0 || len(req.Steps) > 8 {
		return req, nil, nil, fmt.Errorf("消息编排需要包含 1 到 8 个阶段")
	}
	for index := range req.Steps {
		req.Steps[index].Type = normalizeDirectMessageStepType(req.Steps[index].Type)
		req.Steps[index].Content = strings.TrimSpace(req.Steps[index].Content)
		req.Steps[index].MediaAssetID = strings.TrimSpace(req.Steps[index].MediaAssetID)
		req.Steps[index].SourceChatID = strings.TrimSpace(req.Steps[index].SourceChatID)
		req.Steps[index].MessageID = strings.TrimSpace(req.Steps[index].MessageID)
		if req.Steps[index].Type == "" {
			return req, nil, nil, fmt.Errorf("第 %d 个消息阶段类型不支持", index+1)
		}
		switch req.Steps[index].Type {
		case "text":
			if req.Steps[index].Content == "" {
				return req, nil, nil, fmt.Errorf("第 %d 个文本阶段内容不能为空", index+1)
			}
		case "image", "voice", "gif":
			if req.Steps[index].MediaAssetID == "" {
				return req, nil, nil, fmt.Errorf("第 %d 个媒体阶段缺少素材", index+1)
			}
		case "forward":
			if req.Steps[index].SourceChatID == "" || req.Steps[index].MessageID == "" {
				return req, nil, nil, fmt.Errorf("第 %d 个引用消息阶段缺少来源或消息 ID", index+1)
			}
		}
		if req.Steps[index].Type == "text" && req.Steps[index].Content == "" {
			return req, nil, nil, fmt.Errorf("第 %d 个消息阶段内容不能为空", index+1)
		}
		req.Steps[index].DelaySeconds = clampInt(req.Steps[index].DelaySeconds, 0, 86400)
	}
	req.TerminalScope = strings.ToLower(strings.TrimSpace(req.TerminalScope))
	if req.TerminalScope == "" {
		req.TerminalScope = "all"
	}
	if req.TerminalScope != "all" && req.TerminalScope != "group" && req.TerminalScope != "terminal" {
		return req, nil, nil, fmt.Errorf("账号池范围无效")
	}
	req.TerminalGroupID = strings.TrimSpace(req.TerminalGroupID)
	req.TerminalID = strings.TrimSpace(req.TerminalID)
	terminalIDs := make([]uuid.UUID, 0, len(req.TerminalIDs)+1)
	if req.TerminalScope == "group" {
		if _, err := uuid.Parse(req.TerminalGroupID); err != nil {
			return req, nil, nil, fmt.Errorf("账号分组 ID 无效")
		}
	}
	if req.TerminalScope == "terminal" {
		if req.TerminalID != "" {
			req.TerminalIDs = append([]string{req.TerminalID}, req.TerminalIDs...)
		}
		terminalIDs, err = parseUUIDStrings(req.TerminalIDs, "账号 ID 无效")
		if err != nil {
			return req, nil, nil, err
		}
		if len(terminalIDs) == 0 {
			return req, nil, nil, fmt.Errorf("请选择至少一个私信账号")
		}
	}
	req.MinDelaySeconds = clampInt(req.MinDelaySeconds, 0, 86400)
	req.MaxDelaySeconds = clampInt(req.MaxDelaySeconds, 0, 86400)
	if req.MaxDelaySeconds < req.MinDelaySeconds {
		req.MaxDelaySeconds = req.MinDelaySeconds
	}
	req.CooldownMinutes = clampInt(req.CooldownMinutes, 0, 24*60)
	req.CooldownJitterMinutes = clampInt(req.CooldownJitterMinutes, 0, 120)
	req.DedupeDays = clampInt(req.DedupeDays, 0, 365)
	req.SkipNoAccount = true
	req.LeadIDs = uuidSliceStrings(leadIDs)
	req.TerminalIDs = uuidSliceStrings(terminalIDs)
	return req, leadIDs, terminalIDs, nil
}

func (s *Server) loadDirectMessageLeads(ctx context.Context, task models.Task, ids []uuid.UUID) ([]models.SCRMLead, error) {
	var leads []models.SCRMLead
	query := s.db.WithContext(ctx).
		Where("id IN ?", ids).
		Where("(status IS NULL OR status <> ?)", "blacklisted").
		Order("COALESCE(hit_at, created_at) desc")
	if task.TenantID != uuid.Nil {
		if task.CreatedBy != nil {
			query = query.Where("(tenant_id = ? OR owner_user_id = ?)", task.TenantID, *task.CreatedBy)
		} else {
			query = query.Where("tenant_id = ?", task.TenantID)
		}
	}
	if err := query.Find(&leads).Error; err != nil {
		return nil, fmt.Errorf("读取私信线索失败：%w", err)
	}
	return leads, nil
}

func (s *Server) loadDirectMessageTerminals(ctx context.Context, tenantID uuid.UUID, payload directMessageTaskPayload) ([]models.Terminal, error) {
	query := s.db.WithContext(ctx).
		Where("file_path <> ''").
		Where("status NOT IN ?", []string{"abnormal", "banned", "disabled"}).
		Where("(sleep_until IS NULL OR sleep_until <= ?)", time.Now()).
		Order("last_message_at asc nulls first").
		Order("created_at asc")
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	switch strings.ToLower(strings.TrimSpace(payload.TerminalScope)) {
	case "", "all":
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(payload.TerminalGroupID))
		if err != nil {
			return nil, fmt.Errorf("账号分组 ID 无效")
		}
		query = query.Where("group_id = ?", groupID)
	case "terminal":
		terminalIDs, err := parseUUIDStrings(payload.TerminalIDs, "账号 ID 无效")
		if err != nil {
			return nil, err
		}
		if len(terminalIDs) == 0 {
			return nil, fmt.Errorf("请选择至少一个私信账号")
		}
		query = query.Where("id IN ?", terminalIDs)
	default:
		return nil, fmt.Errorf("账号池范围无效")
	}
	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		return nil, fmt.Errorf("读取私信账号失败：%w", err)
	}
	ready := make([]models.Terminal, 0, len(terminals))
	for _, terminal := range terminals {
		if terminalReadyForOutboundAction(terminal) {
			ready = append(ready, terminal)
		}
	}
	return ready, nil
}

func (s *Server) pickDirectMessageTerminal(ctx context.Context, terminals []models.Terminal, target string, cooldownMinutes int, jitterMinutes int) (int, models.Terminal, error) {
	candidateIndexes := make([]int, 0, len(terminals))
	for index := range terminals {
		terminal := terminals[index]
		if terminalReadyForOutboundAction(terminal) && s.terminalTargetAvailable(ctx, terminal.TenantID, terminal.ID, terminalQuotaActionDM, "user", target) {
			candidateIndexes = append(candidateIndexes, index)
		}
	}
	if len(candidateIndexes) == 0 {
		return -1, models.Terminal{}, fmt.Errorf("没有可用的私信账号")
	}
	sort.Slice(candidateIndexes, func(i, j int) bool {
		return terminalOlderForMessaging(terminals[candidateIndexes[i]], terminals[candidateIndexes[j]])
	})
	skipReasons := make([]string, 0, len(candidateIndexes))
	for _, index := range candidateIndexes {
		terminal, err := s.reserveTerminalQuota(ctx, terminals[index].ID, terminalQuotaActionDM)
		if err == nil {
			if cooldownMinutes > 0 {
				now := time.Now()
				nextCooldown := terminalNextCooldownAt(now, cooldownMinutes, jitterMinutes)
				terminal.DMCooldownUntil = &nextCooldown
				terminal.LastMessageAt = &now
				_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Where("id = ?", terminal.ID).Updates(map[string]any{
					"dm_cooldown_until": nextCooldown,
					"last_message_at":   now,
					"updated_at":        now,
				}).Error
			}
			terminals[index] = terminal
			return index, terminal, nil
		}
		skipReasons = append(skipReasons, err.Error())
	}
	return -1, models.Terminal{}, fmt.Errorf("全部私信账号已跳过：%s", summarizeTerminalSkipReasons(skipReasons, "账号限额已满或不可用"))
}

func (s *Server) loadDirectMessageAssets(ctx context.Context, tenantID uuid.UUID, steps []DirectMessageStep) (map[string]models.Asset, error) {
	ids := make([]uuid.UUID, 0)
	for _, step := range steps {
		if step.Type != "image" && step.Type != "voice" && step.Type != "gif" {
			continue
		}
		id, err := uuid.Parse(strings.TrimSpace(step.MediaAssetID))
		if err != nil {
			return nil, fmt.Errorf("媒体素材 ID 无效：%w", err)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[string]models.Asset{}, nil
	}

	var assets []models.Asset
	query := s.db.WithContext(ctx).Where("id IN ?", ids)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("读取私信媒体素材失败：%w", err)
	}
	assetMap := make(map[string]models.Asset, len(assets))
	for _, asset := range assets {
		assetMap[asset.ID.String()] = asset
	}
	for _, id := range ids {
		if _, ok := assetMap[id.String()]; !ok {
			return nil, fmt.Errorf("私信媒体素材不存在：%s", id.String())
		}
	}
	return assetMap, nil
}

func directMessageStepMediaPath(step DirectMessageStep, assets map[string]models.Asset) string {
	asset, ok := assets[strings.TrimSpace(step.MediaAssetID)]
	if !ok {
		return ""
	}
	return asset.FilePath
}

func (s *Server) recordDirectMessageDelivery(ctx context.Context, lead models.SCRMLead, terminal models.Terminal, content string) {
	now := time.Now()
	message := models.SCRMMessage{
		ID:          uuid.New(),
		TenantID:    lead.TenantID,
		LeadID:      lead.ID,
		SenderType:  "terminal",
		TerminalID:  &terminal.ID,
		Content:     content,
		IsRead:      true,
		MessageTime: now,
		CreatedAt:   now,
	}
	_ = s.db.WithContext(ctx).Create(&message).Error
	_ = s.db.WithContext(ctx).Model(&models.SCRMLead{}).Where("id = ? AND tenant_id = ?", lead.ID, lead.TenantID).Updates(map[string]any{
		"assigned_worker": &terminal.ID,
		"status":          "dm_sent",
		"updated_at":      now,
	}).Error
}

func (s *Server) directMessageLeadRecentlyContacted(ctx context.Context, lead models.SCRMLead, days int) bool {
	if days <= 0 {
		return false
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	leadQuery := s.db.WithContext(ctx).Model(&models.SCRMLead{}).Select("id").Where("tenant_id = ? AND COALESCE(hit_at, created_at) >= ?", lead.TenantID, cutoff)
	if account := strings.TrimSpace(lead.UserAccount); account != "" {
		leadQuery = leadQuery.Where("user_account = ?", account)
	} else {
		leadQuery = leadQuery.Where("target_id = ?", lead.TargetID)
	}
	var count int64
	_ = s.db.WithContext(ctx).Model(&models.SCRMMessage{}).
		Where("lead_id IN (?) AND sender_type IN ? AND message_time >= ?", leadQuery, []string{"user", "terminal"}, cutoff).
		Count(&count).Error
	return count > 0
}

func (s *Server) directMessageLeadHasInboundSince(ctx context.Context, leadID uuid.UUID, since time.Time) bool {
	var count int64
	_ = s.db.WithContext(ctx).Model(&models.SCRMMessage{}).
		Where("lead_id = ? AND sender_type NOT IN ? AND message_time >= ?", leadID, []string{"user", "terminal"}, since).
		Count(&count).Error
	return count > 0
}

func (s *Server) waitDirectMessageDelay(ctx context.Context, task models.Task, seconds int, action string) bool {
	if seconds <= 0 {
		return true
	}
	s.logTaskBackground(ctx, task, "INFO", action, fmt.Sprintf("私信编排等待 %d 秒", seconds))
	select {
	case <-ctx.Done():
		s.failDirectMessagesTask(ctx, task, "timeout", "监听私信执行超时")
		return false
	case <-time.After(time.Duration(seconds) * time.Second):
		return true
	}
}

func (s *Server) failDirectMessagesTask(ctx context.Context, task models.Task, action string, detail string) {
	s.updateTaskState(ctx, task.ID, "failed", 100, nil)
	s.logTaskBackground(ctx, task, "ERROR", action, detail)
}

func normalizeDirectMessageStepType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "text":
		return "text"
	case "image", "voice", "gif", "forward":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func directMessageLeadTarget(lead models.SCRMLead) string {
	return strings.TrimSpace(lead.UserAccount)
}

func leadDisplayForDirectMessage(lead models.SCRMLead) string {
	return firstNonEmpty(lead.UserNickname, lead.UserAccount, lead.TargetID.String())
}

func renderDirectMessageTemplate(template string, lead models.SCRMLead) string {
	hitAt := ""
	if lead.HitAt != nil {
		hitAt = lead.HitAt.Local().Format("2006-01-02 15:04:05")
	}
	values := map[string]string{
		"昵称":       firstNonEmpty(lead.UserNickname, lead.UserAccount, "朋友"),
		"账号":       directMessageLeadTarget(lead),
		"来源群":      lead.SourceChatName,
		"来源群ID":    lead.SourceChatID,
		"命中词":      lead.TriggerWord,
		"原消息":      lead.TriggerMessage,
		"命中时间":     hitAt,
		"nickname": lead.UserNickname,
		"account":  directMessageLeadTarget(lead),
		"source":   lead.SourceChatName,
		"keyword":  lead.TriggerWord,
		"message":  lead.TriggerMessage,
		"time":     hitAt,
	}
	replacements := make([]string, 0, len(values)*4)
	for key, value := range values {
		replacements = append(replacements, "{"+key+"}", value, "{{"+key+"}}", value)
	}
	return strings.TrimSpace(strings.NewReplacer(replacements...).Replace(template))
}

func directMessageRecordContent(step DirectMessageStep, content string) string {
	switch step.Type {
	case "image":
		return firstNonEmpty(content, "[图片] "+step.MediaAssetID)
	case "voice":
		return "[语音] " + step.MediaAssetID
	case "gif":
		return firstNonEmpty(content, "[GIF] "+step.MediaAssetID)
	case "forward":
		return "[引用消息] " + strings.TrimSpace(step.SourceChatID) + "/" + strings.TrimSpace(step.MessageID)
	default:
		return content
	}
}

func directMessageStepPreview(step DirectMessageStep, content string) string {
	switch step.Type {
	case "image":
		return "图片 " + directMessagePreview(firstNonEmpty(content, step.MediaAssetID))
	case "voice":
		return "语音 " + strings.TrimSpace(step.MediaAssetID)
	case "gif":
		return "GIF " + directMessagePreview(firstNonEmpty(content, step.MediaAssetID))
	case "forward":
		return "引用消息 " + strings.TrimSpace(step.SourceChatID) + "/" + strings.TrimSpace(step.MessageID)
	default:
		return directMessagePreview(content)
	}
}

func directMessagePreview(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= 40 {
		return string(runes)
	}
	return string(runes[:40]) + "..."
}

func directMessageSettingsTenant(task models.Task, leads []models.SCRMLead) uuid.UUID {
	if task.TenantID != uuid.Nil {
		return task.TenantID
	}
	if len(leads) > 0 {
		return leads[0].TenantID
	}
	return uuid.Nil
}

func parseUUIDStrings(values []string, invalidMessage string) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	ids := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("%s", invalidMessage)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func uuidSliceStrings(ids []uuid.UUID) []string {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, id.String())
	}
	return items
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
