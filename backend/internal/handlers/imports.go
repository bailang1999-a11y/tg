package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const maxImportFileBytes = 50 << 20

type importMode string

const (
	importModeMixed   importMode = "mixed"
	importModeSession importMode = "session"
	importModeTData   importMode = "tdata"
)

type importSummary struct {
	TaskID    uuid.UUID          `json:"task_id"`
	Success   int                `json:"success"`
	Failed    int                `json:"failed"`
	Duplicate int                `json:"duplicate"`
	Skipped   int                `json:"skipped"`
	Terminals int                `json:"terminals"`
	Assets    int                `json:"assets"`
	Failures  map[string]int     `json:"failures"`
	Stages    []importStage      `json:"stages"`
	Items     []importResultItem `json:"items"`
}

type importStage struct {
	Key     string         `json:"key"`
	Label   string         `json:"label"`
	Status  string         `json:"status"`
	Current int            `json:"current"`
	Total   int            `json:"total"`
	Percent int            `json:"percent"`
	Detail  string         `json:"detail"`
	Metrics map[string]int `json:"metrics,omitempty"`
}

type importResultItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type importCandidate struct {
	Name string
	Data []byte
}

type importUnit struct {
	Name       string
	Data       []byte
	AccessType string
	SourceSize int
}

type importBuildStats struct {
	SessionUnits int
	TDataUnits   int
	AssetUnits   int
	SkippedMerge int
}

type importMergeSkip struct {
	Unit   importUnit
	Reason string
}

type importTaskPayload struct {
	Mode    importMode         `json:"mode"`
	GroupID string             `json:"group_id,omitempty"`
	Files   []stagedImportFile `json:"files"`
}

type stagedImportFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func (s *Server) CreateImportTask(c *gin.Context) {
	s.createImportTask(c, importModeMixed)
}

func (s *Server) CreateSessionImportTask(c *gin.Context) {
	s.createImportTask(c, importModeSession)
}

func (s *Server) CreateTDataImportTask(c *gin.Context) {
	s.createImportTask(c, importModeTData)
}

func (s *Server) createImportTask(c *gin.Context, mode importMode) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		utils.Fail(c, http.StatusBadRequest, "请先选择要导入的文件")
		return
	}

	if err := c.Request.ParseMultipartForm(128 << 20); err != nil {
		utils.Fail(c, http.StatusBadRequest, "解析上传文件失败")
		return
	}
	form := c.Request.MultipartForm
	files := form.File["files"]
	if len(files) == 0 {
		utils.Fail(c, http.StatusBadRequest, "请至少选择一个文件")
		return
	}

	groupIDText := strings.TrimSpace(c.PostForm("group_id"))
	if value := groupIDText; value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "分组 ID 无效")
			return
		}
		_ = parsed
	}

	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      mode.taskName(),
		Type:      mode.taskType(),
		Status:    "queued",
		Progress:  0,
		CreatedBy: s.userIDPtr(c),
	}
	stagedFiles, err := stageImportFiles(task.TenantID, task.ID, files)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "保存上传文件失败："+err.Error())
		return
	}
	payloadBytes, _ := json.Marshal(importTaskPayload{
		Mode:    mode,
		GroupID: groupIDText,
		Files:   stagedFiles,
	})
	task.Payload = datatypes.JSON(payloadBytes)

	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		_ = removeImportTaskStage(task.TenantID, task.ID)
		utils.Fail(c, http.StatusInternalServerError, "创建导入任务失败")
		return
	}

	s.logTask(c, task, "INFO", "created", fmt.Sprintf("导入任务已创建：收到 %d 个上传项，等待执行器消费", len(files)))
	if !s.enqueueTask(c.Request.Context(), task, "run") {
		go s.RunImportTask(task.ID)
	}
	utils.Created(c, gin.H{"task": task})
}

func (m importMode) taskName() string {
	switch m {
	case importModeSession:
		return "Session 导入任务"
	case importModeTData:
		return "TData 导入任务"
	default:
		return "导入中心任务"
	}
}

func (m importMode) taskType() string {
	switch m {
	case importModeSession:
		return "import_session"
	case importModeTData:
		return "import_tdata"
	default:
		return "import_validation"
	}
}

func (s *Server) RunImportTask(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	claimed, release := s.claimTaskRun(ctx, taskID, "import_validation", "import_session", "import_tdata")
	if !claimed {
		return
	}
	defer release()

	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ? AND type IN ?", taskID, []string{"import_validation", "import_session", "import_tdata"}).First(&task).Error; err != nil {
		return
	}
	defer removeImportTaskStage(task.TenantID, task.ID)

	var payload importTaskPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		s.failImportTask(ctx, task, "导入任务参数解析失败："+err.Error())
		return
	}
	mode := payload.Mode
	if mode == "" {
		mode = importModeMixed
	}
	var groupID *uuid.UUID
	if strings.TrimSpace(payload.GroupID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(payload.GroupID))
		if err != nil {
			s.failImportTask(ctx, task, "分组 ID 无效")
			return
		}
		groupID = &parsed
	}

	s.updateTaskState(ctx, task.ID, "running", 5, nil)
	summary := importSummary{
		TaskID:   task.ID,
		Failures: map[string]int{},
		Items:    []importResultItem{},
	}
	s.logTaskBackground(ctx, task, "INFO", "scan", fmt.Sprintf("文件扫描中：收到 %d 个上传项", len(payload.Files)))

	candidates, skipped := s.expandStagedImportFiles(ctx, task, payload.Files, &summary)
	summary.Skipped += skipped
	units, buildStats := s.buildImportUnits(ctx, task, candidates, mode, &summary)
	s.updateTaskProgressContext(ctx, task.ID, 20)
	s.logTaskBackground(ctx, task, "INFO", "format_check", fmt.Sprintf("格式检查中：展开得到 %d 个候选文件，识别 %d 个导入项", len(candidates), len(units)))

	s.updateTaskProgressContext(ctx, task.ID, 40)
	s.logTaskBackground(ctx, task, "INFO", "duplicate_check", "重复检测中：基于终端哈希/手机号与图片 MD5 比对")

	for _, unit := range units {
		s.importOneUnit(ctx, task, groupID, unit, &summary)
	}

	s.updateTaskProgressContext(ctx, task.ID, 78)
	s.logTaskBackground(ctx, task, "INFO", "persist", fmt.Sprintf("入库中：终端 %d 个，素材 %d 个", summary.Terminals, summary.Assets))

	s.updateTaskProgressContext(ctx, task.ID, 92)
	s.logTaskBackground(ctx, task, "INFO", "status_check", "状态校验中：已完成本地入库校验，外部账号状态等待授权适配器执行")

	summary.Stages = buildImportStages(len(payload.Files), len(candidates), len(units), buildStats, summary)

	status := "success"
	if summary.Success == 0 && summary.Failed > 0 {
		status = "failed"
	}
	summaryBytes, _ := json.Marshal(summary)
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":   status,
		"progress": 100,
		"summary":  datatypes.JSON(summaryBytes),
	}).Error
	s.logTaskBackground(ctx, task, "INFO", "summary", fmt.Sprintf("完成汇总：成功 %d，失败 %d，重复 %d，跳过 %d", summary.Success, summary.Failed, summary.Duplicate, summary.Skipped))
}

func (s *Server) failImportTask(ctx context.Context, task models.Task, reason string) {
	summaryBytes, _ := json.Marshal(importSummary{
		TaskID:   task.ID,
		Failures: map[string]int{reason: 1},
		Items:    []importResultItem{},
	})
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":   "failed",
		"progress": 100,
		"summary":  datatypes.JSON(summaryBytes),
	}).Error
	s.logTaskBackground(ctx, task, "ERROR", "failed", reason)
}

func stageImportFiles(tenantID, taskID uuid.UUID, files []*multipart.FileHeader) ([]stagedImportFile, error) {
	base := importTaskStageDir(tenantID, taskID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, err
	}
	staged := make([]stagedImportFile, 0, len(files))
	for index, header := range files {
		data, err := readMultipartFile(header)
		if err != nil {
			return nil, err
		}
		name := header.Filename
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("upload-%d", index+1)
		}
		path := filepath.Join(base, fmt.Sprintf("%03d-%s", index+1, sanitizeFilename(filepath.Base(name))))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return nil, err
		}
		staged = append(staged, stagedImportFile{Name: name, Path: path, Size: int64(len(data))})
	}
	return staged, nil
}

func importTaskStageDir(tenantID, taskID uuid.UUID) string {
	return filepath.Join("storage", "uploads", tenantID.String(), "import-jobs", taskID.String())
}

func removeImportTaskStage(tenantID, taskID uuid.UUID) error {
	return os.RemoveAll(importTaskStageDir(tenantID, taskID))
}

func (s *Server) expandStagedImportFiles(ctx context.Context, task models.Task, files []stagedImportFile, summary *importSummary) ([]importCandidate, int) {
	candidates := make([]importCandidate, 0, len(files))
	skipped := 0
	for _, staged := range files {
		data, err := readStoredImportFile(staged.Path)
		if err != nil {
			s.addImportFailure(summary, staged.Name, "unknown", "failed", err.Error())
			s.logTaskBackground(ctx, task, "ERROR", "scan", fmt.Sprintf("%s 读取失败：%s", staged.Name, err.Error()))
			continue
		}
		ext := strings.ToLower(filepath.Ext(staged.Name))
		switch ext {
		case ".zip":
			expanded, err := expandZip(staged.Name, data)
			if err != nil {
				s.addImportFailure(summary, staged.Name, "zip", "failed", err.Error())
				s.logTaskBackground(ctx, task, "ERROR", "scan", fmt.Sprintf("%s 解压失败：%s", staged.Name, err.Error()))
				continue
			}
			candidates = append(candidates, expanded...)
		case ".rar", ".7z":
			skipped++
			s.addImportFailure(summary, staged.Name, "archive", "skipped", "当前内置导入器暂不解压 rar/7z，请先转为 zip 或直接上传文件夹")
		default:
			candidates = append(candidates, importCandidate{Name: staged.Name, Data: data})
		}
	}
	return candidates, skipped
}

func readStoredImportFile(path string) ([]byte, error) {
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") || !strings.HasPrefix(filepath.ToSlash(clean), "storage/uploads/") {
		return nil, errors.New("导入暂存路径无效")
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}
	if len(data) > maxImportFileBytes {
		return nil, errors.New("单个文件超过 50MB 限制")
	}
	return data, nil
}

func (s *Server) expandImportFiles(c *gin.Context, task models.Task, files []*multipart.FileHeader, summary *importSummary) ([]importCandidate, int) {
	candidates := make([]importCandidate, 0, len(files))
	skipped := 0
	for _, header := range files {
		data, err := readMultipartFile(header)
		if err != nil {
			s.addImportFailure(summary, header.Filename, "unknown", "failed", err.Error())
			s.logTask(c, task, "ERROR", "scan", fmt.Sprintf("%s 读取失败：%s", header.Filename, err.Error()))
			continue
		}

		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".zip":
			expanded, err := expandZip(header.Filename, data)
			if err != nil {
				s.addImportFailure(summary, header.Filename, "zip", "failed", err.Error())
				s.logTask(c, task, "ERROR", "scan", fmt.Sprintf("%s 解压失败：%s", header.Filename, err.Error()))
				continue
			}
			candidates = append(candidates, expanded...)
		case ".rar", ".7z":
			skipped++
			s.addImportFailure(summary, header.Filename, "archive", "skipped", "当前内置导入器暂不解压 rar/7z，请先转为 zip 或直接上传文件夹")
		default:
			candidates = append(candidates, importCandidate{Name: header.Filename, Data: data})
		}
	}
	return candidates, skipped
}

func (s *Server) buildImportUnits(ctx context.Context, task models.Task, candidates []importCandidate, mode importMode, summary *importSummary) ([]importUnit, importBuildStats) {
	units := make([]importUnit, 0, len(candidates))
	sessionUnits := make([]importUnit, 0)
	assetUnits := make([]importUnit, 0)
	tdataGroups := map[string][]importCandidate{}
	tdataKeys := make([]string, 0)
	stats := importBuildStats{}

	addTData := func(candidate importCandidate) {
		key, ok := tdataGroupKey(candidate.Name)
		if !ok {
			s.skipImportCandidate(ctx, task, summary, candidate.Name, "data", "没有找到 tdata 目录，请选择 tdata 文件夹或包含 tdata 的 zip")
			return
		}
		if isArchiveRootTDataKey(key) {
			s.skipImportCandidate(ctx, task, summary, candidate.Name, "data", "zip 根目录 tdata 不作为账号，请按账号子文件夹归档")
			return
		}
		if _, exists := tdataGroups[key]; !exists {
			tdataKeys = append(tdataKeys, key)
		}
		tdataGroups[key] = append(tdataGroups[key], candidate)
	}

	for _, candidate := range candidates {
		fileType := detectImportType(candidate.Name, candidate.Data)
		switch mode {
		case importModeSession:
			if fileType == "session" {
				units = append(units, importUnit{Name: candidate.Name, Data: candidate.Data, AccessType: "session", SourceSize: 1})
				stats.SessionUnits++
			} else {
				s.skipImportCandidate(ctx, task, summary, candidate.Name, fileType, "Session 入口只接受 .session 文件或包含 .session 的 zip")
			}
		case importModeTData:
			if fileType == "data" {
				addTData(candidate)
			} else {
				s.skipImportCandidate(ctx, task, summary, candidate.Name, fileType, "TData 入口只接受 tdata 文件夹或包含 tdata 的 zip")
			}
		default:
			switch fileType {
			case "session":
				sessionUnits = append(sessionUnits, importUnit{Name: candidate.Name, Data: candidate.Data, AccessType: "session", SourceSize: 1})
			case "data":
				addTData(candidate)
			case "image":
				assetUnits = append(assetUnits, importUnit{Name: candidate.Name, Data: candidate.Data, AccessType: "image", SourceSize: 1})
			default:
				s.skipImportCandidate(ctx, task, summary, candidate.Name, "unknown", "不支持的文件类型")
			}
		}
	}

	sort.Strings(tdataKeys)
	tdataUnits := make([]importUnit, 0, len(tdataKeys))
	for _, key := range tdataKeys {
		data, err := archiveTDataGroup(key, tdataGroups[key])
		if err != nil {
			s.addImportFailure(summary, key, "data", "failed", err.Error())
			s.logTaskBackground(ctx, task, "ERROR", "format_check", fmt.Sprintf("%s 聚合失败：%s", key, err.Error()))
			continue
		}
		unit := importUnit{Name: key + ".zip", Data: data, AccessType: "data", SourceSize: len(tdataGroups[key])}
		if mode == importModeMixed {
			tdataUnits = append(tdataUnits, unit)
		} else {
			units = append(units, unit)
			stats.TDataUnits++
		}
	}

	if mode == importModeMixed {
		mergedAccountUnits, skippedSessions := mergeMixedAccountUnits(sessionUnits, tdataUnits)
		for _, unit := range mergedAccountUnits {
			units = append(units, unit)
			if unit.AccessType == "data" {
				stats.TDataUnits++
				continue
			}
			stats.SessionUnits++
		}
		for _, skipped := range skippedSessions {
			stats.SkippedMerge++
			s.skipImportCandidate(ctx, task, summary, skipped.Unit.Name, skipped.Unit.AccessType, skipped.Reason)
		}
		for _, unit := range assetUnits {
			units = append(units, unit)
			stats.AssetUnits++
		}
	}

	return units, stats
}

func mergeMixedAccountUnits(sessionUnits, tdataUnits []importUnit) ([]importUnit, []importMergeSkip) {
	merged := make([]importUnit, 0, len(sessionUnits)+len(tdataUnits))
	skipped := make([]importMergeSkip, 0)
	tdataPhones := make(map[string]bool, len(tdataUnits))

	for _, unit := range tdataUnits {
		if phone := normalizeTerminalPhone(extractPhone(unit.Name)); phone != "" {
			tdataPhones[phone] = true
		}
		merged = append(merged, unit)
	}

	for _, unit := range sessionUnits {
		phone := normalizeTerminalPhone(extractPhone(unit.Name))
		if phone != "" && tdataPhones[phone] {
			skipped = append(skipped, importMergeSkip{
				Unit:   unit,
				Reason: "同账号已识别 TData，Session 已合并跳过",
			})
			continue
		}
		merged = append(merged, unit)
	}

	return merged, skipped
}

func (s *Server) skipImportCandidate(ctx context.Context, task models.Task, summary *importSummary, name, itemType, reason string) {
	summary.Skipped++
	s.addImportFailure(summary, name, itemType, "skipped", reason)
	s.logTaskBackground(ctx, task, "WARN", "format_check", fmt.Sprintf("%s 已跳过：%s", name, reason))
}

func (s *Server) importOneUnit(ctx context.Context, task models.Task, groupID *uuid.UUID, unit importUnit, summary *importSummary) {
	if unit.AccessType == "image" {
		if err := s.importAsset(ctx, task, groupID, unit, summary); err != nil {
			s.addImportFailure(summary, unit.Name, unit.AccessType, "failed", err.Error())
			s.logTaskBackground(ctx, task, "ERROR", "persist", fmt.Sprintf("%s 入库失败：%s", unit.Name, err.Error()))
		}
		return
	}

	if err := s.importTerminal(ctx, task, groupID, unit, summary); err != nil {
		s.addImportFailure(summary, unit.Name, unit.AccessType, "failed", err.Error())
		s.logTaskBackground(ctx, task, "ERROR", "persist", fmt.Sprintf("%s 入库失败：%s", unit.Name, err.Error()))
	}
}

func (s *Server) importTerminal(ctx context.Context, task models.Task, groupID *uuid.UUID, unit importUnit, summary *importSummary) error {
	hash := sha256.Sum256(unit.Data)
	sessionHash := hex.EncodeToString(hash[:])
	phone := normalizeTerminalPhone(extractPhone(unit.Name))

	reason, err := s.terminalDuplicateReason(ctx, task.TenantID, sessionHash, phone)
	if err != nil {
		return err
	}
	if reason != "" {
		summary.Duplicate++
		summary.Items = append(summary.Items, importResultItem{Name: unit.Name, Type: unit.AccessType, Status: "duplicate", Reason: reason})
		s.logTaskBackground(ctx, task, "WARN", "duplicate_check", fmt.Sprintf("%s 重复，已跳过：%s", unit.Name, reason))
		return nil
	}

	path, err := saveUploadedBytes(task.TenantID, "terminals", unit.Name, unit.Data)
	if err != nil {
		return err
	}

	nickname := cleanBaseName(unit.Name)
	if phone != "" {
		nickname = phone
	}
	_, originCountry, originFlag := syncTerminalPhoneIdentity(phone, "", "")
	terminal := models.Terminal{
		ID:            uuid.New(),
		TenantID:      task.TenantID,
		Phone:         phone,
		Nickname:      nickname,
		Status:        "offline",
		AccessType:    unit.AccessType,
		OriginCountry: originCountry,
		OriginFlag:    originFlag,
		ExitCountry:   "未绑定",
		GroupID:       groupID,
		RiskStatus:    "正常",
		BanStatus:     "正常",
		FilePath:      path,
		SessionHash:   sessionHash,
	}
	if err := s.db.WithContext(ctx).Create(&terminal).Error; err != nil {
		return err
	}

	summary.Success++
	summary.Terminals++
	summary.Items = append(summary.Items, importResultItem{Name: unit.Name, Type: unit.AccessType, Status: "success"})
	detail := fmt.Sprintf("%s 已导入为终端 %s", unit.Name, terminal.ID.String())
	if unit.SourceSize > 1 {
		detail = fmt.Sprintf("%s 已聚合 %d 个 tdata 文件并导入为终端 %s", unit.Name, unit.SourceSize, terminal.ID.String())
	}
	s.logTaskBackground(ctx, task, "INFO", "persist", detail)
	return nil
}

func (s *Server) terminalDuplicateReason(ctx context.Context, tenantID uuid.UUID, sessionHash string, phone string) (string, error) {
	var hashExisting int64
	query := s.db.WithContext(ctx).Model(&models.Terminal{}).Where("session_hash = ?", sessionHash)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Count(&hashExisting).Error; err != nil {
		return "", err
	}
	if hashExisting > 0 {
		return "Session 哈希已存在", nil
	}
	if phone == "" {
		return "", nil
	}
	var phoneExisting int64
	query = s.db.WithContext(ctx).Model(&models.Terminal{}).Where("regexp_replace(phone, '[^0-9]', '', 'g') = ?", normalizeTerminalPhone(phone))
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Count(&phoneExisting).Error; err != nil {
		return "", err
	}
	if phoneExisting > 0 {
		return "手机号已存在", nil
	}
	return "", nil
}

func (s *Server) importAsset(ctx context.Context, task models.Task, groupID *uuid.UUID, unit importUnit, summary *importSummary) error {
	hash := md5.Sum(unit.Data)
	md5Value := hex.EncodeToString(hash[:])

	var existing int64
	query := s.db.WithContext(ctx).Model(&models.Asset{}).Where("md5 = ?", md5Value)
	if task.TenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", task.TenantID)
	}
	if err := query.Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		summary.Duplicate++
		summary.Items = append(summary.Items, importResultItem{Name: unit.Name, Type: "image", Status: "duplicate", Reason: "图片 MD5 已存在"})
		s.logTaskBackground(ctx, task, "WARN", "duplicate_check", fmt.Sprintf("%s 图片重复，已跳过", unit.Name))
		return nil
	}

	path, err := saveUploadedBytes(task.TenantID, "assets", unit.Name, unit.Data)
	if err != nil {
		return err
	}
	asset := models.Asset{
		ID:       uuid.New(),
		TenantID: task.TenantID,
		GroupID:  groupID,
		Name:     filepath.Base(unit.Name),
		MimeType: http.DetectContentType(unit.Data),
		MD5:      md5Value,
		URL:      "/" + filepath.ToSlash(path),
		FilePath: path,
	}
	if err := s.db.WithContext(ctx).Create(&asset).Error; err != nil {
		return err
	}

	summary.Success++
	summary.Assets++
	summary.Items = append(summary.Items, importResultItem{Name: unit.Name, Type: "image", Status: "success"})
	s.logTaskBackground(ctx, task, "INFO", "persist", fmt.Sprintf("%s 已导入素材池", unit.Name))
	return nil
}

func (s *Server) addImportFailure(summary *importSummary, name, itemType, status, reason string) {
	if status == "failed" {
		summary.Failed++
	}
	if summary.Failures == nil {
		summary.Failures = map[string]int{}
	}
	summary.Failures[reason]++
	summary.Items = append(summary.Items, importResultItem{Name: name, Type: itemType, Status: status, Reason: reason})
}

func buildImportStages(uploadedFiles, candidates, units int, stats importBuildStats, summary importSummary) []importStage {
	processedUnits := summary.Success + summary.Failed + summary.Duplicate
	finalTotal := summary.Success + summary.Failed + summary.Duplicate + summary.Skipped
	if finalTotal == 0 {
		finalTotal = candidates
	}

	return []importStage{
		newImportStage(
			"scan",
			"文件扫描",
			uploadedFiles,
			uploadedFiles,
			fmt.Sprintf("收到 %d 个上传项，展开得到 %d 个候选文件", uploadedFiles, candidates),
			map[string]int{
				"上传项":  uploadedFiles,
				"候选文件": candidates,
			},
		),
		newImportStage(
			"format_check",
			"格式检查",
			candidates,
			candidates,
			fmt.Sprintf("识别 Session %d 个、TData %d 个、素材 %d 个", stats.SessionUnits, stats.TDataUnits, stats.AssetUnits),
			map[string]int{
				"Session": stats.SessionUnits,
				"TData":   stats.TDataUnits,
				"素材":      stats.AssetUnits,
				"跳过":      summary.Skipped,
			},
		),
		newImportStage(
			"account_merge",
			"账号聚合",
			stats.SessionUnits+stats.TDataUnits,
			stats.SessionUnits+stats.TDataUnits+stats.SkippedMerge,
			fmt.Sprintf("按手机号/数字文件夹聚合出 %d 个账号导入项", stats.SessionUnits+stats.TDataUnits),
			map[string]int{
				"账号导入项": stats.SessionUnits + stats.TDataUnits,
				"合并跳过":  stats.SkippedMerge,
			},
		),
		newImportStage(
			"duplicate_check",
			"重复检测",
			units,
			units,
			fmt.Sprintf("检测 %d 个导入项，重复 %d 个", units, summary.Duplicate),
			map[string]int{
				"检测项": units,
				"重复":  summary.Duplicate,
			},
		),
		newImportStage(
			"persist",
			"入库",
			processedUnits,
			units,
			fmt.Sprintf("成功 %d 个，失败 %d 个，重复 %d 个", summary.Success, summary.Failed, summary.Duplicate),
			map[string]int{
				"成功": summary.Success,
				"失败": summary.Failed,
				"重复": summary.Duplicate,
			},
		),
		newImportStage(
			"status_check",
			"状态校验",
			summary.Terminals,
			summary.Terminals,
			fmt.Sprintf("已完成 %d 个终端的本地入库校验", summary.Terminals),
			map[string]int{
				"终端": summary.Terminals,
			},
		),
		newImportStage(
			"summary",
			"完成汇总",
			finalTotal,
			finalTotal,
			fmt.Sprintf("成功 %d，失败 %d，重复 %d，跳过 %d", summary.Success, summary.Failed, summary.Duplicate, summary.Skipped),
			map[string]int{
				"成功": summary.Success,
				"失败": summary.Failed,
				"重复": summary.Duplicate,
				"跳过": summary.Skipped,
			},
		),
	}
}

func newImportStage(key, label string, current, total int, detail string, metrics map[string]int) importStage {
	percent := 100
	status := "success"
	if total > 0 {
		if current < 0 {
			current = 0
		}
		if current > total {
			current = total
		}
		percent = int(float64(current) / float64(total) * 100)
		if percent < 100 {
			status = "running"
		}
	} else {
		current = 0
		total = 0
	}
	return importStage{
		Key:     key,
		Label:   label,
		Status:  status,
		Current: current,
		Total:   total,
		Percent: percent,
		Detail:  detail,
		Metrics: metrics,
	}
}

func (s *Server) logTask(c *gin.Context, task models.Task, level, action, detail string) {
	_ = s.db.WithContext(c.Request.Context()).Create(&models.TaskLog{
		ID:        uuid.New(),
		TenantID:  task.TenantID,
		TaskID:    task.ID,
		Level:     level,
		Category:  task.Type,
		Action:    action,
		Details:   detail,
		TraceID:   uuid.NewString(),
		CreatedAt: time.Now(),
	}).Error
}

func (s *Server) updateTaskProgress(c *gin.Context, taskID uuid.UUID, progress int) {
	s.updateTaskProgressContext(c.Request.Context(), taskID, progress)
}

func (s *Server) updateTaskProgressContext(ctx context.Context, taskID uuid.UUID, progress int) {
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).Update("progress", progress).Error
}

func readMultipartFile(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImportFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImportFileBytes {
		return nil, errors.New("单个文件超过 50MB 限制")
	}
	return data, nil
}

func expandZip(name string, data []byte) ([]importCandidate, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	out := make([]importCandidate, 0, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.Name, err)
		}
		content, err := io.ReadAll(io.LimitReader(src, maxImportFileBytes+1))
		_ = src.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.Name, err)
		}
		if len(content) > maxImportFileBytes {
			return nil, fmt.Errorf("%s 超过 50MB 限制", file.Name)
		}
		out = append(out, importCandidate{Name: filepath.Join(name, file.Name), Data: content})
	}
	return out, nil
}

func tdataGroupKey(name string) (string, bool) {
	normalized := filepath.ToSlash(name)
	parts := strings.Split(normalized, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "tdata") {
			return strings.Join(parts[:i+1], "/"), true
		}
	}
	if strings.EqualFold(filepath.Ext(normalized), ".data") {
		return strings.TrimSuffix(normalized, filepath.Ext(normalized)), true
	}
	for i := len(parts) - 2; i >= 0; i-- {
		part := strings.TrimSuffix(parts[i], filepath.Ext(parts[i]))
		if isArchivePathSegment(parts[i]) {
			continue
		}
		if numericToken(part) != "" && looksLikeTDataAccountPath(parts[i+1:]) {
			return strings.Join(parts[:i+1], "/"), true
		}
	}
	return "", false
}

func archiveTDataGroup(root string, candidates []importCandidate) ([]byte, error) {
	if len(candidates) == 0 {
		return nil, errors.New("tdata 分组为空")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, candidate := range candidates {
		rel := strings.TrimPrefix(filepath.ToSlash(candidate.Name), filepath.ToSlash(root)+"/")
		if rel == "" || rel == filepath.ToSlash(candidate.Name) {
			rel = filepath.Base(candidate.Name)
		}
		file, err := writer.Create(rel)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err := file.Write(candidate.Data); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func detectImportType(name string, data []byte) string {
	lower := strings.ToLower(filepath.ToSlash(name))
	ext := strings.ToLower(filepath.Ext(lower))
	switch ext {
	case ".session":
		return "session"
	case ".data":
		return "data"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return "image"
	}
	if strings.Contains(lower, "/tdata/") || strings.Contains(lower, "/data/") || strings.Contains(lower, "tdata") {
		return "data"
	}
	if _, ok := tdataGroupKey(name); ok {
		return "data"
	}
	contentType := http.DetectContentType(data)
	if strings.HasPrefix(contentType, "image/") {
		return "image"
	}
	return "unknown"
}

func looksLikeTDataAccountPath(parts []string) bool {
	for _, part := range parts {
		base := strings.ToLower(strings.TrimSpace(part))
		ext := strings.ToLower(filepath.Ext(base))
		switch {
		case base == "key_data":
			return true
		case ext == ".data":
			return true
		case ext == "" && base != "":
			return true
		case strings.HasPrefix(base, "map") && ext == "":
			return true
		}
	}
	return false
}

func isArchivePathSegment(part string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(part))) {
	case ".zip", ".rar", ".7z":
		return true
	default:
		return false
	}
}

func saveUploadedBytes(tenantID uuid.UUID, category, name string, data []byte) (string, error) {
	base := filepath.Join("storage", "uploads", tenantID.String(), category)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	filename := fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), hex.EncodeToString(hash[:8]), sanitizeFilename(filepath.Base(name)))
	path := filepath.Join(base, filename)
	return path, os.WriteFile(path, data, 0o600)
}

var phoneRegex = regexp.MustCompile(`\d{5,}`)

func extractPhone(name string) string {
	normalized := filepath.ToSlash(name)
	parts := strings.Split(normalized, "/")
	if len(parts) == 0 {
		return ""
	}

	base := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
	if phone := numericToken(base); phone != "" {
		return phone
	}

	// Folder imports and zip entries often look like:
	// root.zip/14452719040/tdata/key_datas
	// Prefer the nearest account folder over digits in the archive name.
	for i := len(parts) - 2; i >= 0; i-- {
		if isArchivePathSegment(parts[i]) {
			continue
		}
		part := strings.TrimSuffix(parts[i], filepath.Ext(parts[i]))
		if phone := numericToken(part); phone != "" {
			return phone
		}
	}
	return ""
}

func numericToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	matches := phoneRegex.FindAllString(value, -1)
	if len(matches) == 0 {
		return ""
	}
	if len(matches) > 1 && strings.Contains(value, "+") {
		prefix := ""
		for _, candidate := range matches[:len(matches)-1] {
			if len(candidate) >= 1 && len(candidate) <= 4 {
				prefix = candidate
				break
			}
		}
		last := matches[len(matches)-1]
		if prefix != "" {
			if strings.HasPrefix(last, prefix) {
				return last
			}
			return prefix + last
		}
	}
	// If a segment contains several numbers, the last one is usually the
	// account marker, e.g. "+1_美国_(2)_14452719040".
	return matches[len(matches)-1]
}

func cleanBaseName(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	base = strings.TrimSuffix(base, ext)
	base = strings.TrimSpace(base)
	if base == "" {
		return "未命名终端"
	}
	return base
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "upload.bin"
	}
	return name
}
