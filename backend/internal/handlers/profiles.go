package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type profileModifyRequest struct {
	Scope           string   `json:"scope" binding:"required"`
	TerminalID      string   `json:"terminal_id"`
	TerminalGroupID string   `json:"terminal_group_id"`
	Nicknames       []string `json:"nicknames"`
	Bios            []string `json:"bios"`
	Homepages       []string `json:"homepages"`
	AvatarAssetIDs  []string `json:"avatar_asset_ids"`
}

type profileModifySummary struct {
	TaskID              uuid.UUID                     `json:"task_id"`
	TerminalCount       int                           `json:"terminal_count"`
	Fields              []string                      `json:"fields"`
	Assignments         []profileModifyAssignment     `json:"assignments"`
	Counts              map[string]int                `json:"counts"`
	Target              profileModifyTargetSnapshot   `json:"target"`
	Status              string                        `json:"status"`
	AppliedCount        int                           `json:"applied_count"`
	PartialCount        int                           `json:"partial_count"`
	FailedCount         int                           `json:"failed_count"`
	RequestedFieldCount int                           `json:"requested_field_count"`
	AppliedFieldCount   int                           `json:"applied_field_count"`
	FailedFieldCount    int                           `json:"failed_field_count"`
	FieldAppliedCount   map[string]int                `json:"field_applied_count"`
	FieldFailedCount    map[string]int                `json:"field_failed_count"`
	FailureCategories   map[string]int                `json:"failure_categories"`
	Results             []profileModifyTerminalResult `json:"results"`
	PendingRefresh      bool                          `json:"pending_refresh"`
}

type profileModifyTargetSnapshot struct {
	Scope           string     `json:"scope"`
	TerminalID      *uuid.UUID `json:"terminal_id,omitempty"`
	TerminalGroupID *uuid.UUID `json:"terminal_group_id,omitempty"`
}

type profileModifyAssignment struct {
	TerminalID     uuid.UUID `json:"terminal_id"`
	Phone          string    `json:"phone,omitempty"`
	Nickname       string    `json:"nickname,omitempty"`
	Bio            string    `json:"bio,omitempty"`
	Homepage       string    `json:"homepage,omitempty"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	AvatarFilePath string    `json:"-"`
}

type profileModifyTerminalResult struct {
	TerminalID      uuid.UUID         `json:"terminal_id"`
	Phone           string            `json:"phone,omitempty"`
	Status          string            `json:"status"`
	Message         string            `json:"message,omitempty"`
	AppliedFields   []string          `json:"applied_fields,omitempty"`
	FailedFields    map[string]string `json:"failed_fields,omitempty"`
	FailureCategory string            `json:"failure_category,omitempty"`
}

func (s *Server) CreateProfileTask(c *gin.Context) {
	var req profileModifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请选择终端范围并填写资料")
		return
	}
	req.Nicknames = cleanProfileValues(req.Nicknames)
	req.Bios = cleanProfileValues(req.Bios)
	req.Homepages = cleanProfileValues(req.Homepages)
	req.AvatarAssetIDs = cleanProfileValues(req.AvatarAssetIDs)

	terminals, target, err := s.resolveProfileTerminals(c, req)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	avatarAssets, err := s.resolveAvatarAssets(c, req.AvatarAssetIDs)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	fields := profileFields(req, avatarAssets)
	if len(fields) == 0 {
		utils.Fail(c, http.StatusBadRequest, "请至少填写昵称、个性签名、个人频道或选择头像")
		return
	}

	taskID := uuid.New()
	payloadBytes, _ := json.Marshal(req)

	task := models.Task{
		ID:        taskID,
		TenantID:  s.tenantID(c),
		Name:      "资料修改任务",
		Type:      "profile_modification",
		Status:    "queued",
		Progress:  0,
		Payload:   datatypes.JSON(payloadBytes),
		CreatedBy: s.userIDPtr(c),
	}
	if target.TerminalGroupID != nil {
		task.TerminalGroupID = target.TerminalGroupID
	}

	err = s.db.WithContext(c.Request.Context()).Create(&task).Error
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建资料修改任务失败")
		return
	}

	s.logTask(c, task, "INFO", "created", fmt.Sprintf("资料修改任务已创建：%d 个终端，%d 个资料字段，等待执行器消费", len(terminals), len(fields)))
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.RunProfileModificationTask(task.ID)
	}
	utils.Created(c, gin.H{"task": task})
}

func (s *Server) RunProfileModificationTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "profile_modification")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type = ?", taskID, "profile_modification").First(&task).Error; err != nil {
		return
	}
	var req profileModifyRequest
	if err := json.Unmarshal(task.Payload, &req); err != nil {
		s.failProfileModificationTask(ctx, task, "资料修改任务参数解析失败："+err.Error())
		return
	}
	req.Nicknames = cleanProfileValues(req.Nicknames)
	req.Bios = cleanProfileValues(req.Bios)
	req.Homepages = cleanProfileValues(req.Homepages)
	req.AvatarAssetIDs = cleanProfileValues(req.AvatarAssetIDs)

	terminals, target, err := s.loadProfileTerminalsFromRequest(ctx, task.TenantID, req)
	if err != nil {
		s.failProfileModificationTask(ctx, task, err.Error())
		return
	}
	avatarAssets, err := s.loadAvatarAssetsFromIDs(ctx, task.TenantID, req.AvatarAssetIDs)
	if err != nil {
		s.failProfileModificationTask(ctx, task, err.Error())
		return
	}
	fields := profileFields(req, avatarAssets)
	if len(fields) == 0 {
		s.failProfileModificationTask(ctx, task, "请至少填写昵称、个性签名、个人频道或选择头像")
		return
	}
	assignments := buildProfileAssignments(terminals, req, avatarAssets)
	summary := profileModifySummary{
		TaskID:        task.ID,
		TerminalCount: len(assignments),
		Fields:        fields,
		Assignments:   assignments,
		Counts: map[string]int{
			"nicknames": len(req.Nicknames),
			"bios":      len(req.Bios),
			"homepages": len(req.Homepages),
			"avatars":   len(avatarAssets),
		},
		Target:            target,
		FieldAppliedCount: map[string]int{},
		FieldFailedCount:  map[string]int{},
		FailureCategories: map[string]int{},
		Results:           make([]profileModifyTerminalResult, 0, len(assignments)),
		PendingRefresh:    true,
	}

	s.updateTaskState(ctx, task.ID, "running", 10, nil)
	applicator := telegram_client.NewApplicator(s.cfg)
	s.logTaskBackground(ctx, task, "INFO", "start", "开始执行资料修改，终端列表不会自动刷新，请手动点击真实刷新")
	for _, assignment := range assignments {
		if assignment.isEmpty() {
			s.logTaskBackground(ctx, task, "WARN", "profile_skipped", assignment.terminalRef()+" 未填写任何资料，已跳过")
			continue
		}

		terminal := findTerminalByID(terminals, assignment.TerminalID)
		if terminal == nil {
			terminalResult := assignment.failedResult("对应终端不存在")
			summary.recordResult(terminalResult)
			s.logTaskBackground(ctx, task, "ERROR", "profile_failed", assignment.detail()+"，执行失败："+formatProfileFailedFields(terminalResult.FailedFields))
			continue
		}

		request := telegram_client.ApplyRequest{
			FilePath:   terminal.FilePath,
			AccessType: terminal.AccessType,
			Nickname:   assignment.Nickname,
			Bio:        assignment.Bio,
			Homepage:   assignment.Homepage,
			AvatarPath: assignment.AvatarFilePath,
		}
		applyResult, applyErr := applicator.Apply(ctx, request)
		terminalResult := assignment.buildApplyResult(applyResult, applyErr)
		if profileResultIndicatesFrozen(terminalResult) {
			s.markTerminalProfileRestricted(ctx, assignment.TerminalID)
		}
		summary.recordResult(terminalResult)

		switch terminalResult.Status {
		case "success":
			s.logTaskBackground(ctx, task, "INFO", "profile_applied", assignment.detail()+"，成功："+formatProfileFieldLabels(terminalResult.AppliedFields)+"，等待手动刷新回拉真实资料")
		case "partial_success":
			s.logTaskBackground(ctx, task, "WARN", "profile_partial", assignment.detail()+"，成功："+formatProfileFieldLabels(terminalResult.AppliedFields)+"；失败："+formatProfileFailedFields(terminalResult.FailedFields)+"。等待手动刷新回拉真实资料")
		default:
			s.logTaskBackground(ctx, task, "ERROR", "profile_failed", assignment.detail()+"，执行失败："+formatProfileFailedFields(terminalResult.FailedFields))
		}
	}

	taskStatus := summary.resolveStatus()
	summary.Status = taskStatus
	summaryBytes, _ := json.Marshal(summary)
	if err := s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":   taskStatus,
		"progress": 100,
		"summary":  datatypes.JSON(summaryBytes),
	}).Error; err != nil {
		return
	}
}

func (s *Server) failProfileModificationTask(ctx context.Context, task models.Task, reason string) {
	summaryBytes, _ := json.Marshal(profileModifySummary{
		TaskID:            task.ID,
		Status:            "failed",
		FieldAppliedCount: map[string]int{},
		FieldFailedCount:  map[string]int{},
		FailureCategories: map[string]int{"unknown": 1},
		Results:           []profileModifyTerminalResult{},
	})
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":   "failed",
		"progress": 100,
		"summary":  datatypes.JSON(summaryBytes),
	}).Error
	s.logTaskBackground(ctx, task, "ERROR", "failed", reason)
}

func (s *Server) resolveProfileTerminals(c *gin.Context, req profileModifyRequest) ([]models.Terminal, profileModifyTargetSnapshot, error) {
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	target := profileModifyTargetSnapshot{Scope: scope}
	query := s.db.WithContext(c.Request.Context()).Order("created_at asc")
	switch scope {
	case "all":
	case "terminal":
		terminalID, err := uuid.Parse(strings.TrimSpace(req.TerminalID))
		if err != nil {
			return nil, target, fmt.Errorf("终端 ID 无效")
		}
		target.TerminalID = &terminalID
		query = query.Where("id = ?", terminalID)
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TerminalGroupID))
		if err != nil {
			return nil, target, fmt.Errorf("终端分组 ID 无效")
		}
		var group models.Group
		if err := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", groupID, "terminal").First(&group).Error; err != nil {
			return nil, target, fmt.Errorf("终端分组不存在")
		}
		target.TerminalGroupID = &groupID
		query = query.Where("group_id = ?", groupID)
	default:
		return nil, target, fmt.Errorf("请选择单个终端、终端分组或全部终端")
	}

	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		return nil, target, err
	}
	if len(terminals) == 0 {
		return nil, target, fmt.Errorf("当前范围内没有可修改的终端")
	}
	return terminals, target, nil
}

func (s *Server) loadProfileTerminalsFromRequest(ctx context.Context, tenantID uuid.UUID, req profileModifyRequest) ([]models.Terminal, profileModifyTargetSnapshot, error) {
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	target := profileModifyTargetSnapshot{Scope: scope}
	query := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at asc")
	switch scope {
	case "all":
	case "terminal":
		terminalID, err := uuid.Parse(strings.TrimSpace(req.TerminalID))
		if err != nil {
			return nil, target, fmt.Errorf("终端 ID 无效")
		}
		target.TerminalID = &terminalID
		query = query.Where("id = ?", terminalID)
	case "group":
		groupID, err := uuid.Parse(strings.TrimSpace(req.TerminalGroupID))
		if err != nil {
			return nil, target, fmt.Errorf("终端分组 ID 无效")
		}
		target.TerminalGroupID = &groupID
		query = query.Where("group_id = ?", groupID)
	default:
		return nil, target, fmt.Errorf("请选择单个终端、终端分组或全部终端")
	}

	var terminals []models.Terminal
	if err := query.Find(&terminals).Error; err != nil {
		return nil, target, err
	}
	if len(terminals) == 0 {
		return nil, target, fmt.Errorf("当前范围内没有可修改的终端")
	}
	return terminals, target, nil
}

func (s *Server) resolveAvatarAssets(c *gin.Context, assetIDTexts []string) ([]models.Asset, error) {
	if len(assetIDTexts) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(assetIDTexts))
	for _, text := range assetIDTexts {
		id, err := uuid.Parse(text)
		if err != nil {
			return nil, fmt.Errorf("头像素材 ID 无效")
		}
		ids = append(ids, id)
	}

	var assets []models.Asset
	if err := s.db.WithContext(c.Request.Context()).Where("id IN ?", ids).Find(&assets).Error; err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]models.Asset, len(assets))
	for _, asset := range assets {
		byID[asset.ID] = asset
	}

	ordered := make([]models.Asset, 0, len(ids))
	for _, id := range ids {
		asset, ok := byID[id]
		if !ok || !strings.HasPrefix(asset.MimeType, "image/") {
			return nil, fmt.Errorf("头像素材不存在或不是图片")
		}
		ordered = append(ordered, asset)
	}
	return ordered, nil
}

func (s *Server) loadAvatarAssetsFromIDs(ctx context.Context, tenantID uuid.UUID, assetIDTexts []string) ([]models.Asset, error) {
	if len(assetIDTexts) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(assetIDTexts))
	for _, text := range assetIDTexts {
		id, err := uuid.Parse(text)
		if err != nil {
			return nil, fmt.Errorf("头像素材 ID 无效")
		}
		ids = append(ids, id)
	}

	var assets []models.Asset
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&assets).Error; err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]models.Asset, len(assets))
	for _, asset := range assets {
		byID[asset.ID] = asset
	}

	ordered := make([]models.Asset, 0, len(ids))
	for _, id := range ids {
		asset, ok := byID[id]
		if !ok || !strings.HasPrefix(asset.MimeType, "image/") {
			return nil, fmt.Errorf("头像素材不存在或不是图片")
		}
		ordered = append(ordered, asset)
	}
	return ordered, nil
}

func profileFields(req profileModifyRequest, avatarAssets []models.Asset) []string {
	fields := []string{}
	if len(req.Nicknames) > 0 {
		fields = append(fields, "nickname")
	}
	if len(req.Bios) > 0 {
		fields = append(fields, "bio")
	}
	if len(req.Homepages) > 0 {
		fields = append(fields, "homepage")
	}
	if len(avatarAssets) > 0 {
		fields = append(fields, "avatar")
	}
	return fields
}

func buildProfileAssignments(terminals []models.Terminal, req profileModifyRequest, avatarAssets []models.Asset) []profileModifyAssignment {
	avatarURLs := make([]string, 0, len(avatarAssets))
	avatarPaths := make([]string, 0, len(avatarAssets))
	for _, asset := range avatarAssets {
		avatarURLs = append(avatarURLs, asset.URL)
		avatarPaths = append(avatarPaths, asset.FilePath)
	}

	assignments := make([]profileModifyAssignment, 0, len(terminals))
	total := len(terminals)
	for index, terminal := range terminals {
		assignments = append(assignments, profileModifyAssignment{
			TerminalID:     terminal.ID,
			Phone:          terminal.Phone,
			Nickname:       pickDistributedValue(req.Nicknames, index, total),
			Bio:            pickDistributedValue(req.Bios, index, total),
			Homepage:       pickDistributedValue(req.Homepages, index, total),
			AvatarURL:      pickDistributedValue(avatarURLs, index, total),
			AvatarFilePath: pickDistributedValue(avatarPaths, index, total),
		})
	}
	return assignments
}

func (a profileModifyAssignment) terminalRef() string {
	if a.Phone != "" {
		return a.Phone
	}
	return a.TerminalID.String()
}

func (a profileModifyAssignment) detail() string {
	parts := []string{}
	if a.Nickname != "" {
		parts = append(parts, "昵称")
	}
	if a.Bio != "" {
		parts = append(parts, "个性签名")
	}
	if a.Homepage != "" {
		parts = append(parts, "个人频道")
	}
	if a.AvatarURL != "" {
		parts = append(parts, "头像")
	}
	return fmt.Sprintf("%s 已分配：%s", a.terminalRef(), strings.Join(parts, "、"))
}

func (a profileModifyAssignment) isEmpty() bool {
	return a.Nickname == "" && a.Bio == "" && a.Homepage == "" && a.AvatarFilePath == ""
}

func (a profileModifyAssignment) requestedFields() []string {
	fields := make([]string, 0, 4)
	if a.Nickname != "" {
		fields = append(fields, "nickname")
	}
	if a.Bio != "" {
		fields = append(fields, "bio")
	}
	if a.Homepage != "" {
		fields = append(fields, "homepage")
	}
	if a.AvatarFilePath != "" {
		fields = append(fields, "avatar")
	}
	return fields
}

func (a profileModifyAssignment) terminalResult(status string, message string, appliedFields []string, failedFields map[string]string) profileModifyTerminalResult {
	return profileModifyTerminalResult{
		TerminalID:      a.TerminalID,
		Phone:           a.Phone,
		Status:          status,
		Message:         strings.TrimSpace(message),
		AppliedFields:   appliedFields,
		FailedFields:    failedFields,
		FailureCategory: profileFailureCategory(message, failedFields),
	}
}

func (a profileModifyAssignment) failedResult(reason string) profileModifyTerminalResult {
	failedFields := make(map[string]string, len(a.requestedFields()))
	for _, field := range a.requestedFields() {
		failedFields[field] = strings.TrimSpace(reason)
	}
	return a.terminalResult("failed", reason, nil, failedFields)
}

func (a profileModifyAssignment) buildApplyResult(applyResult telegram_client.ApplyResult, applyErr error) profileModifyTerminalResult {
	requestedFields := a.requestedFields()
	if len(requestedFields) == 0 {
		return a.terminalResult("skipped", "未分配资料项", nil, nil)
	}

	message := strings.TrimSpace(applyResult.Reason)
	if message == "" && applyErr != nil {
		message = strings.TrimSpace(applyErr.Error())
	}

	appliedFields := make([]string, 0, len(requestedFields))
	failedFields := make(map[string]string)
	hasStructuredFields := len(applyResult.Fields) > 0

	for _, field := range requestedFields {
		fieldResult, ok := applyResult.Fields[field]
		switch {
		case hasStructuredFields && ok && fieldResult.Requested && fieldResult.OK:
			appliedFields = append(appliedFields, field)
		case hasStructuredFields && ok && fieldResult.Requested:
			failedFields[field] = profileFailureReason(fieldResult.Reason, message, applyErr)
		case !hasStructuredFields && applyErr == nil && applyResult.OK:
			appliedFields = append(appliedFields, field)
		default:
			failedFields[field] = profileFailureReason("", message, applyErr)
		}
	}

	switch {
	case len(appliedFields) > 0 && len(failedFields) == 0:
		return a.terminalResult("success", message, appliedFields, nil)
	case len(appliedFields) > 0:
		return a.terminalResult("partial_success", message, appliedFields, failedFields)
	default:
		return a.terminalResult("failed", message, nil, failedFields)
	}
}

func (s *profileModifySummary) recordResult(result profileModifyTerminalResult) {
	if result.Status == "skipped" {
		return
	}
	s.Results = append(s.Results, result)
	switch result.Status {
	case "success":
		s.AppliedCount++
	case "partial_success":
		s.PartialCount++
	default:
		s.FailedCount++
	}

	s.AppliedFieldCount += len(result.AppliedFields)
	s.FailedFieldCount += len(result.FailedFields)
	s.RequestedFieldCount += len(result.AppliedFields) + len(result.FailedFields)

	for _, field := range result.AppliedFields {
		s.FieldAppliedCount[field]++
	}
	for field := range result.FailedFields {
		s.FieldFailedCount[field]++
	}
	if len(result.FailedFields) > 0 {
		category := strings.TrimSpace(result.FailureCategory)
		if category == "" {
			category = profileFailureCategory(result.Message, result.FailedFields)
		}
		s.FailureCategories[category] += len(result.FailedFields)
	}
}

func (s profileModifySummary) resolveStatus() string {
	switch {
	case s.AppliedCount == 0 && s.PartialCount == 0:
		return "failed"
	case s.FailedCount > 0 || s.PartialCount > 0:
		return "partial_success"
	default:
		return "success"
	}
}

func cleanProfileValues(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item != "" {
			cleaned = append(cleaned, item)
		}
	}
	return cleaned
}

func pickDistributedValue(values []string, index int, total int) string {
	if len(values) == 0 {
		return ""
	}
	if total <= 1 {
		return values[0]
	}
	slot := index * len(values) / total
	if slot >= len(values) {
		slot = len(values) - 1
	}
	return values[slot]
}

func findTerminalByID(terminals []models.Terminal, terminalID uuid.UUID) *models.Terminal {
	for index := range terminals {
		if terminals[index].ID == terminalID {
			return &terminals[index]
		}
	}
	return nil
}

func formatProfileFieldLabels(fields []string) string {
	if len(fields) == 0 {
		return "无"
	}
	labels := make([]string, 0, len(fields))
	for _, field := range []string{"nickname", "bio", "homepage", "avatar"} {
		for _, item := range fields {
			if item == field {
				labels = append(labels, profileFieldLabel(item))
				break
			}
		}
	}
	if len(labels) == 0 {
		return "无"
	}
	return strings.Join(labels, "、")
}

func formatProfileFailedFields(failedFields map[string]string) string {
	if len(failedFields) == 0 {
		return "无"
	}
	parts := make([]string, 0, len(failedFields))
	for _, field := range []string{"nickname", "bio", "homepage", "avatar"} {
		reason, ok := failedFields[field]
		if !ok {
			continue
		}
		text := strings.TrimSpace(reason)
		if text == "" {
			text = "执行失败"
		}
		parts = append(parts, fmt.Sprintf("%s（%s）", profileFieldLabel(field), text))
	}
	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, "、")
}

func profileFailureReason(fieldReason string, message string, applyErr error) string {
	if text := strings.TrimSpace(fieldReason); text != "" {
		return text
	}
	if text := strings.TrimSpace(message); text != "" {
		return text
	}
	if applyErr != nil {
		if text := strings.TrimSpace(applyErr.Error()); text != "" {
			return text
		}
	}
	return "执行失败"
}

func profileResultIndicatesFrozen(result profileModifyTerminalResult) bool {
	if telegram_client.IsFrozenAccountReason(result.Message) {
		return true
	}
	for _, reason := range result.FailedFields {
		if telegram_client.IsFrozenAccountReason(reason) {
			return true
		}
	}
	return false
}

func profileFailureCategory(message string, failedFields map[string]string) string {
	reasons := []string{message}
	for _, reason := range failedFields {
		reasons = append(reasons, reason)
	}
	joined := strings.ToLower(strings.Join(reasons, " "))
	switch {
	case strings.Contains(joined, "账号已被冻结"),
		strings.Contains(joined, "frozen"),
		strings.Contains(joined, "not available for frozen accounts"):
		return "frozen"
	case strings.Contains(joined, "会话文件正在被占用"),
		strings.Contains(joined, "database is locked"),
		strings.Contains(joined, "locked"):
		return "session_locked"
	case strings.Contains(joined, "格式"),
		strings.Contains(joined, "format"),
		strings.Contains(joined, "invalid"),
		strings.Contains(joined, "无效"):
		return "format"
	case strings.Contains(joined, "占用"),
		strings.Contains(joined, "occupied"),
		strings.Contains(joined, "taken"),
		strings.Contains(joined, "username"):
		return "occupied"
	case strings.Contains(joined, "授权"),
		strings.Contains(joined, "登录"),
		strings.Contains(joined, "unauthorized"),
		strings.Contains(joined, "auth"),
		strings.Contains(joined, "login"):
		return "auth"
	default:
		return "unknown"
	}
}

func (s *Server) markTerminalProfileRestricted(ctx context.Context, terminalID uuid.UUID) {
	_ = s.db.WithContext(ctx).Model(&models.Terminal{}).Where("id = ?", terminalID).Updates(map[string]any{
		"risk_status": "资料受限",
		"ban_status":  "已冻结",
	}).Error
}

func profileFieldLabel(field string) string {
	switch field {
	case "nickname":
		return "昵称"
	case "bio":
		return "个性签名"
	case "homepage":
		return "个人频道"
	case "avatar":
		return "头像"
	default:
		return field
	}
}
