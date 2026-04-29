package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) ImportListenerAccounts(c *gin.Context) {
	var req struct {
		Content      string `json:"content" binding:"required"`
		GroupID      string `json:"group_id"`
		NewGroupName string `json:"new_group_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入监听账号")
		return
	}
	groupID, groupName, err := s.resolveListenerGroup(c, "listener_account", strings.TrimSpace(req.GroupID), listenerDefaultGroupName(req.GroupID, req.NewGroupName, "监听号"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	lines := strings.Split(req.Content, "\n")
	summary, err := s.importListenerAccountLines(c, lines, groupID, groupName)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "导入监听账号失败")
		return
	}
	utils.Created(c, summary)
}

func (s *Server) importListenerAccountLines(c *gin.Context, lines []string, groupID *uuid.UUID, groupName string) (listenerImportSummary, error) {
	summary := listenerImportSummary{GroupID: groupID, GroupName: groupName, Items: []listenerImportResult{}}
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for _, raw := range lines {
			line := strings.TrimSpace(raw)
			if line == "" {
				summary.Skipped++
				continue
			}
			if isZipName(line) {
				summary.Skipped++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Status: "skipped", Reason: "zip 只是容器，不作为账号入库"})
				continue
			}
			phone, nickname := parseListenerAccountLine(line)
			if phone == "" && nickname == "" {
				summary.Failed++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Status: "failed", Reason: "账号格式无效"})
				continue
			}
			var existing int64
			if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerAccount{}).Where("tenant_id = ? AND phone = ?", s.tenantID(c), phone).Count(&existing).Error; err != nil {
				return err
			}
			if phone != "" && existing > 0 {
				summary.Duplicate++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: phone, Status: "duplicate", Reason: "监听账号已存在"})
				continue
			}
			if nickname == "" {
				nickname = phone
			}
			item := models.ListenerAccount{ID: uuid.New(), TenantID: s.tenantID(c), GroupID: groupID, Phone: phone, Nickname: nickname, Status: "unchecked", RiskStatus: "unknown", AccessType: "tdata"}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			summary.CreatedIDs = append(summary.CreatedIDs, item.ID)
			summary.Success++
			summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: firstNonEmpty(phone, nickname), Status: "success"})
		}
		return nil
	})
	if err != nil {
		return summary, err
	}
	s.autoAssignImportedListenerAccounts(c, &summary)
	return summary, nil
}

func (s *Server) importListenerAccountUnits(c *gin.Context, units []importUnit, groupID *uuid.UUID, groupName string) (listenerImportSummary, error) {
	summary := listenerImportSummary{GroupID: groupID, GroupName: groupName, Items: []listenerImportResult{}}
	tenantID := s.tenantID(c)
	err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for _, unit := range units {
			hash := sha256.Sum256(unit.Data)
			sessionHash := hex.EncodeToString(hash[:])
			phone := listenerAccountPhoneFromUnitName(unit.Name)
			normalizedDigits := normalizeTerminalPhone(phone)
			if normalizedDigits == "" {
				phone, _ = parseListenerAccountLine(cleanBaseName(unit.Name))
				normalizedDigits = normalizeTerminalPhone(phone)
			}
			if normalizedDigits == "" {
				summary.Failed++
				summary.Items = append(summary.Items, listenerImportResult{Line: unit.Name, Status: "failed", Reason: "账号目录未识别到手机号"})
				continue
			}

			var existing int64
			if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerAccount{}).
				Where("tenant_id = ? AND (session_hash = ? OR regexp_replace(phone, '[^0-9]', '', 'g') = ?)", tenantID, sessionHash, normalizedDigits).
				Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				summary.Duplicate++
				summary.Items = append(summary.Items, listenerImportResult{Line: unit.Name, Identifier: normalizedDigits, Status: "duplicate", Reason: "监听账号已存在"})
				continue
			}

			path, err := saveUploadedBytes(tenantID, "listener-accounts", unit.Name, unit.Data)
			if err != nil {
				return err
			}
			normalizedPhone, _, _ := syncTerminalPhoneIdentity(phone, "", "")
			if normalizedPhone == "" {
				normalizedPhone = normalizedDigits
			}
			nickname := cleanBaseName(unit.Name)
			if nickname == "" || strings.EqualFold(nickname, "tdata") {
				nickname = normalizedPhone
			}
			item := models.ListenerAccount{
				ID:          uuid.New(),
				TenantID:    tenantID,
				GroupID:     groupID,
				Phone:       normalizedPhone,
				Nickname:    nickname,
				Status:      "unchecked",
				RiskStatus:  "待检测",
				AccessType:  unit.AccessType,
				FilePath:    path,
				SessionHash: sessionHash,
			}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			summary.CreatedIDs = append(summary.CreatedIDs, item.ID)
			summary.Success++
			summary.Items = append(summary.Items, listenerImportResult{Line: unit.Name, Identifier: normalizedPhone, Status: "success"})
		}
		return nil
	})
	if err != nil {
		return summary, err
	}
	s.autoAssignImportedListenerAccounts(c, &summary)
	return summary, nil
}

func (s *Server) autoAssignImportedListenerAccounts(c *gin.Context, summary *listenerImportSummary) {
	if summary == nil || len(summary.CreatedIDs) == 0 {
		return
	}
	assign, err := s.assignListenerProxiesToAccounts(c, nil, "", summary.CreatedIDs, true)
	if err != nil {
		summary.AssignmentError = err.Error()
		return
	}
	summary.Assignment = &assign
}

func listenerAccountPhoneFromUnitName(name string) string {
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if isArchivePathSegment(parts[i]) {
			continue
		}
		part := strings.TrimSuffix(parts[i], filepath.Ext(parts[i]))
		if phone, _ := parseStructuredAccountName(part); phone != "" {
			return phone
		}
	}
	return normalizeTerminalPhone(extractPhone(name))
}

func (s *Server) buildListenerAccountUploadUnits(form *multipart.Form) (listenerAccountUploadBuild, error) {
	files := form.File["files"]
	if len(files) == 0 {
		return listenerAccountUploadBuild{}, fmt.Errorf("请选择监听账号文件、文件夹或 zip")
	}
	paths := form.Value["paths"]
	candidates := make([]importCandidate, 0, len(files))
	build := listenerAccountUploadBuild{Items: []listenerImportResult{}}
	for index, header := range files {
		name := header.Filename
		if index < len(paths) && strings.TrimSpace(paths[index]) != "" {
			name = strings.TrimSpace(paths[index])
		}
		data, err := readMultipartFile(header)
		if err != nil {
			build.Failed++
			build.Items = append(build.Items, listenerImportResult{Line: name, Status: "failed", Reason: err.Error()})
			continue
		}
		switch strings.ToLower(filepath.Ext(name)) {
		case ".zip":
			expanded, err := expandZip(name, data)
			if err != nil {
				build.Failed++
				build.Items = append(build.Items, listenerImportResult{Line: name, Status: "failed", Reason: err.Error()})
				continue
			}
			candidates = append(candidates, expanded...)
		case ".rar", ".7z":
			build.Skipped++
			build.Items = append(build.Items, listenerImportResult{Line: name, Status: "skipped", Reason: "暂不支持 rar/7z，请先转为 zip"})
		default:
			candidates = append(candidates, importCandidate{Name: name, Data: data})
		}
	}

	sessionUnits := []importUnit{}
	tdataGroups := map[string][]importCandidate{}
	tdataKeys := []string{}
	for _, candidate := range candidates {
		switch detectImportType(candidate.Name, candidate.Data) {
		case "session":
			sessionUnits = append(sessionUnits, importUnit{Name: candidate.Name, Data: candidate.Data, AccessType: "session", SourceSize: 1})
		case "data":
			key, ok := tdataGroupKey(candidate.Name)
			if !ok {
				build.Skipped++
				build.Items = append(build.Items, listenerImportResult{Line: candidate.Name, Status: "skipped", Reason: "没有找到 tdata 目录"})
				continue
			}
			if isArchiveRootTDataKey(key) {
				build.Skipped++
				build.Items = append(build.Items, listenerImportResult{Line: candidate.Name, Status: "skipped", Reason: "zip 根目录 tdata 不作为账号，请按账号子文件夹归档"})
				continue
			}
			if _, exists := tdataGroups[key]; !exists {
				tdataKeys = append(tdataKeys, key)
			}
			tdataGroups[key] = append(tdataGroups[key], candidate)
		default:
			build.Skipped++
			build.Items = append(build.Items, listenerImportResult{Line: candidate.Name, Status: "skipped", Reason: "不是监听账号文件"})
		}
	}

	sort.Strings(tdataKeys)
	tdataUnits := make([]importUnit, 0, len(tdataKeys))
	for _, key := range tdataKeys {
		data, err := archiveTDataGroup(key, tdataGroups[key])
		if err != nil {
			build.Failed++
			build.Items = append(build.Items, listenerImportResult{Line: key, Status: "failed", Reason: err.Error()})
			continue
		}
		tdataUnits = append(tdataUnits, importUnit{Name: key + ".zip", Data: data, AccessType: "data", SourceSize: len(tdataGroups[key])})
	}
	units, skipped := mergeListenerAccountUnits(sessionUnits, tdataUnits)
	for _, item := range skipped {
		build.Skipped++
		build.Items = append(build.Items, listenerImportResult{Line: item.Unit.Name, Status: "skipped", Reason: item.Reason})
	}
	build.Units = units
	return build, nil
}

func mergeListenerAccountUnits(sessionUnits, tdataUnits []importUnit) ([]importUnit, []importMergeSkip) {
	sessionByPhone := map[string]importUnit{}
	for _, unit := range sessionUnits {
		if phone := normalizeTerminalPhone(extractPhone(unit.Name)); phone != "" {
			sessionByPhone[phone] = unit
		}
	}

	merged := make([]importUnit, 0, len(sessionUnits)+len(tdataUnits))
	skipped := make([]importMergeSkip, 0)
	tdataPhones := make(map[string]bool, len(tdataUnits))
	sessionPreferred := map[string]bool{}

	for _, unit := range tdataUnits {
		phone := normalizeTerminalPhone(extractPhone(unit.Name))
		if phone != "" {
			tdataPhones[phone] = true
		}
		if phone != "" && isLightweightTDataUnit(unit) {
			if session, ok := sessionByPhone[phone]; ok {
				merged = append(merged, session)
				sessionPreferred[phone] = true
				skipped = append(skipped, importMergeSkip{
					Unit:   unit,
					Reason: "同账号同时包含轻量 TData 和 Session，已优先使用 Session",
				})
				continue
			}
		}
		merged = append(merged, unit)
	}

	for _, unit := range sessionUnits {
		phone := normalizeTerminalPhone(extractPhone(unit.Name))
		if phone != "" && sessionPreferred[phone] {
			continue
		}
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

func isLightweightTDataUnit(unit importUnit) bool {
	return unit.AccessType == "data" && unit.SourceSize > 0 && unit.SourceSize <= 6 && len(unit.Data) < 128*1024
}

func isArchiveRootTDataKey(key string) bool {
	parts := strings.Split(filepath.ToSlash(strings.TrimSpace(key)), "/")
	if len(parts) != 2 {
		return false
	}
	return strings.EqualFold(filepath.Ext(parts[0]), ".zip") && strings.EqualFold(parts[1], "tdata")
}

func extractListenerAccountNamesFromMultipart(form *multipart.Form) ([]string, error) {
	files := form.File["files"]
	if len(files) == 0 {
		return nil, fmt.Errorf("请选择监听账号文件、文件夹或 zip")
	}
	paths := form.Value["paths"]
	allPaths := make([]string, 0, len(files))
	textLines := []string{}
	for index, header := range files {
		path := header.Filename
		if index < len(paths) && strings.TrimSpace(paths[index]) != "" {
			path = strings.TrimSpace(paths[index])
		}
		if isZipName(path) {
			zipPaths, err := zipEntryPaths(header)
			if err != nil {
				return nil, err
			}
			allPaths = append(allPaths, zipPaths...)
			continue
		}
		if isTextAccountList(path) && len(cleanImportPathParts(path)) <= 1 {
			lines, err := readMultipartTextLines(header)
			if err == nil {
				textLines = append(textLines, lines...)
			}
			continue
		}
		allPaths = append(allPaths, path)
	}
	names := accountNamesFromPaths(allPaths)
	names = append(names, textLines...)
	return uniqueCleanAccountNames(names), nil
}

func zipEntryPaths(header *multipart.FileHeader) ([]string, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("读取 zip 失败")
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取 zip 失败")
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("zip 文件格式无效")
	}
	paths := make([]string, 0, len(reader.File))
	for _, item := range reader.File {
		if item.FileInfo().IsDir() {
			continue
		}
		paths = append(paths, item.Name)
	}
	return paths, nil
}

func readMultipartTextLines(header *multipart.FileHeader) ([]string, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out, nil
}

func accountNamesFromPaths(paths []string) []string {
	rows := make([][]string, 0, len(paths))
	firstSegments := map[string]bool{}
	secondSegments := map[string]bool{}
	for _, raw := range paths {
		parts := cleanImportPathParts(raw)
		if len(parts) == 0 {
			continue
		}
		rows = append(rows, parts)
		firstSegments[parts[0]] = true
		if len(parts) > 1 {
			secondSegments[parts[1]] = true
		}
	}
	if len(rows) == 0 {
		return nil
	}
	level := 0
	if len(firstSegments) == 1 && len(secondSegments) > 1 {
		level = 1
	}
	names := make([]string, 0, len(rows))
	for _, parts := range rows {
		if level < len(parts) {
			names = append(names, parts[level])
		}
	}
	return names
}

func cleanImportPathParts(path string) []string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	rawParts := strings.Split(path, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == "__MACOSX" || strings.HasPrefix(part, ".") {
			continue
		}
		if strings.EqualFold(part, "thumbs.db") {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func uniqueCleanAccountNames(names []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		cleaned := cleanupListenerAccountName(name)
		if cleaned == "" || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		out = append(out, cleaned)
	}
	return out
}

func cleanupListenerAccountName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, filepath.Ext(value))
	value = strings.Trim(value, " \t\r\n/")
	return value
}

func isZipName(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".zip")
}

func isTextAccountList(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".csv":
		return true
	default:
		return false
	}
}

func firstFormValue(form *multipart.Form, key string) string {
	values := form.Value[key]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func listenerDefaultGroupName(groupIDText string, newGroupName string, fallback string) string {
	newGroupName = strings.TrimSpace(newGroupName)
	if strings.TrimSpace(groupIDText) != "" {
		return newGroupName
	}
	return firstNonEmpty(newGroupName, fallback)
}

func (s *Server) ImportListenerAccountsFromFiles(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(128 << 20); err != nil {
		utils.Fail(c, http.StatusBadRequest, "读取上传文件失败")
		return
	}
	form := c.Request.MultipartForm
	if form == nil {
		utils.Fail(c, http.StatusBadRequest, "请选择监听账号文件、文件夹或 zip")
		return
	}
	groupIDText := firstFormValue(form, "group_id")
	newGroupName := listenerDefaultGroupName(groupIDText, firstFormValue(form, "new_group_name"), "监听号")
	groupID, groupName, err := s.resolveListenerGroup(c, "listener_account", strings.TrimSpace(groupIDText), newGroupName)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	build, err := s.buildListenerAccountUploadUnits(form)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(build.Units) == 0 {
		utils.Fail(c, http.StatusBadRequest, "没有识别到监听账号")
		return
	}
	summary, err := s.importListenerAccountUnits(c, build.Units, groupID, groupName)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "导入监听账号失败")
		return
	}
	summary.Failed += build.Failed
	summary.Skipped += build.Skipped
	summary.Items = append(build.Items, summary.Items...)
	utils.Created(c, summary)
}
