package handlers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var allowedAvatarExtensions = map[string]bool{
	".jpeg": true,
	".jepg": true,
	".jpg":  true,
	".png":  true,
	".gif":  true,
}

type assetUploadSummary struct {
	Success   int                     `json:"success"`
	Failed    int                     `json:"failed"`
	Duplicate int                     `json:"duplicate"`
	Skipped   int                     `json:"skipped"`
	GroupID   *uuid.UUID              `json:"group_id,omitempty"`
	GroupName string                  `json:"group_name,omitempty"`
	Items     []assetUploadResultItem `json:"items"`
}

type assetUploadResultItem struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	URL    string `json:"url,omitempty"`
}

var workflowMediaMIMEs = map[string]string{
	".jpeg": "image/jpeg",
	".jepg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".wav":  "audio/wav",
	".webm": "audio/webm",
}

type avatarGroupResponse struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	ResourceType string    `json:"resource_type"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	AssetCount   int64     `json:"asset_count"`
}

func (s *Server) ListAvatarGroups(c *gin.Context) {
	var groups []models.Group
	groupQuery := s.db.WithContext(c.Request.Context()).Where("resource_type = ?", "avatar")
	groupQuery = s.applyTenantAccess(c, groupQuery)
	if err := groupQuery.Order("created_at desc").Find(&groups).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取头像分组失败")
		return
	}

	type countRow struct {
		GroupID uuid.UUID `gorm:"column:group_id"`
		Count   int64     `gorm:"column:count"`
	}
	var rows []countRow
	countQuery := s.db.WithContext(c.Request.Context()).Model(&models.Asset{}).
		Select("group_id, count(*) as count").
		Where("group_id IS NOT NULL AND mime_type IN ?", []string{"image/jpeg", "image/png", "image/gif"})
	countQuery = s.applyTenantAccess(c, countQuery)
	if err := countQuery.Group("group_id").Scan(&rows).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "统计头像数量失败")
		return
	}
	counts := make(map[uuid.UUID]int64, len(rows))
	for _, row := range rows {
		counts[row.GroupID] = row.Count
	}
	var total int64
	totalQuery := s.db.WithContext(c.Request.Context()).Model(&models.Asset{}).
		Where("mime_type IN ?", []string{"image/jpeg", "image/png", "image/gif"})
	totalQuery = s.applyTenantAccess(c, totalQuery)
	if err := totalQuery.Count(&total).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "统计头像总数失败")
		return
	}

	out := make([]avatarGroupResponse, 0, len(groups)+1)
	out = append(out, avatarGroupResponse{
		ID:           uuid.Nil,
		TenantID:     s.tenantID(c),
		ResourceType: "avatar",
		Name:         "全部分组",
		Description:  "全部头像分组图片",
		AssetCount:   total,
	})
	for _, group := range groups {
		out = append(out, avatarGroupResponse{
			ID:           group.ID,
			TenantID:     group.TenantID,
			ResourceType: group.ResourceType,
			Name:         group.Name,
			Description:  group.Description,
			AssetCount:   counts[group.ID],
		})
	}
	utils.OK(c, out)
}

func (s *Server) ListAssets(c *gin.Context) {
	var items []models.Asset
	query := s.db.WithContext(c.Request.Context()).Order("created_at desc")
	query = s.applyTenantAccess(c, query)
	if groupID := c.Query("group_id"); groupID != "" {
		parsed, err := uuid.Parse(groupID)
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "分组 ID 无效")
			return
		}
		query = query.Where("group_id = ?", parsed)
	}
	if err := query.Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取素材失败")
		return
	}
	utils.OK(c, items)
}

func (s *Server) UploadAssets(c *gin.Context) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		utils.Fail(c, http.StatusBadRequest, "请先选择要上传的头像图片")
		return
	}
	if err := c.Request.ParseMultipartForm(128 << 20); err != nil {
		utils.Fail(c, http.StatusBadRequest, "解析上传图片失败")
		return
	}
	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		utils.Fail(c, http.StatusBadRequest, "请至少选择一张头像图片")
		return
	}

	groupID, groupName, err := s.resolveAvatarGroup(c, strings.TrimSpace(c.PostForm("group_id")), strings.TrimSpace(c.PostForm("new_group_name")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	summary := assetUploadSummary{
		GroupID:   groupID,
		GroupName: groupName,
		Items:     []assetUploadResultItem{},
	}
	for _, header := range files {
		data, err := readMultipartFile(header)
		if err != nil {
			summary.Failed++
			summary.Items = append(summary.Items, assetUploadResultItem{Name: header.Filename, Status: "failed", Reason: "读取文件失败"})
			continue
		}
		if strings.EqualFold(filepath.Ext(header.Filename), ".zip") {
			candidates, err := expandZip(header.Filename, data)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, assetUploadResultItem{Name: header.Filename, Status: "failed", Reason: "解压 zip 失败"})
				continue
			}
			for _, candidate := range candidates {
				if err := s.importAvatarAssetData(c, groupID, candidate.Name, candidate.Data, true, &summary); err != nil {
					utils.Fail(c, http.StatusInternalServerError, "导入头像素材失败")
					return
				}
			}
			continue
		}
		if err := s.importAvatarAssetData(c, groupID, header.Filename, data, false, &summary); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "导入头像素材失败")
			return
		}
	}

	utils.Created(c, summary)
}

func (s *Server) UploadWorkflowMedia(c *gin.Context) {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		utils.Fail(c, http.StatusBadRequest, "请先选择要上传的媒体文件")
		return
	}
	if err := c.Request.ParseMultipartForm(128 << 20); err != nil {
		utils.Fail(c, http.StatusBadRequest, "解析上传媒体失败")
		return
	}
	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		utils.Fail(c, http.StatusBadRequest, "请至少选择一个媒体文件")
		return
	}

	summary := assetUploadSummary{Items: []assetUploadResultItem{}}
	for _, header := range files {
		data, err := readMultipartFile(header)
		if err != nil {
			summary.Failed++
			summary.Items = append(summary.Items, assetUploadResultItem{Name: header.Filename, Status: "failed", Reason: "读取文件失败"})
			continue
		}
		if err := s.importWorkflowMediaAssetData(c, header.Filename, data, &summary); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "导入工作流媒体失败")
			return
		}
	}

	utils.Created(c, summary)
}

func (s *Server) importWorkflowMediaAssetData(c *gin.Context, name string, data []byte, summary *assetUploadSummary) error {
	ext := strings.ToLower(filepath.Ext(name))
	mimeType, ok := workflowMediaMIMEs[ext]
	if !ok {
		summary.Failed++
		summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "failed", Reason: "仅支持 jpg/png/gif/mp3/m4a/aac/ogg/wav/webm"})
		return nil
	}
	if detected := http.DetectContentType(data); strings.HasPrefix(detected, "image/") || strings.HasPrefix(detected, "audio/") {
		mimeType = detected
	}

	hash := md5.Sum(data)
	md5Value := hex.EncodeToString(hash[:])
	var existing models.Asset
	query := s.db.WithContext(c.Request.Context()).Where("md5 = ?", md5Value)
	query = s.applyTenantAccess(c, query)
	if err := query.First(&existing).Error; err == nil {
		summary.Duplicate++
		summary.Items = append(summary.Items, assetUploadResultItem{
			ID:     existing.ID.String(),
			Name:   name,
			Status: "duplicate",
			Reason: "媒体 MD5 已存在，已复用素材",
			URL:    existing.URL,
		})
		return nil
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	path, err := saveUploadedBytes(s.tenantID(c), "workflow-media", name, data)
	if err != nil {
		summary.Failed++
		summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "failed", Reason: "保存媒体失败"})
		return nil
	}

	asset := models.Asset{
		ID:       uuid.New(),
		TenantID: s.tenantID(c),
		Name:     filepath.Base(name),
		MimeType: mimeType,
		MD5:      md5Value,
		URL:      "/" + filepath.ToSlash(path),
		FilePath: path,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&asset).Error; err != nil {
		summary.Failed++
		summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "failed", Reason: "写入素材库失败"})
		_ = removeStoredAssetFile(path)
		return nil
	}

	summary.Success++
	summary.Items = append(summary.Items, assetUploadResultItem{ID: asset.ID.String(), Name: name, Status: "success", URL: asset.URL})
	return nil
}

func (s *Server) importAvatarAssetData(c *gin.Context, groupID *uuid.UUID, name string, data []byte, skipNonImage bool, summary *assetUploadSummary) error {
	mimeType := http.DetectContentType(data)
	if !isSupportedAvatarImage(name, mimeType) {
		if skipNonImage {
			summary.Skipped++
			summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "skipped", Reason: "非图片文件已跳过"})
		} else {
			summary.Failed++
			summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "failed", Reason: "仅支持 jpeg/jpg/png/gif 图片或 zip 压缩包"})
		}
		return nil
	}

	hash := md5.Sum(data)
	md5Value := hex.EncodeToString(hash[:])
	var existing int64
	query := s.db.WithContext(c.Request.Context()).Model(&models.Asset{}).Where("md5 = ?", md5Value)
	query = s.applyTenantAccess(c, query)
	if err := query.Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		summary.Duplicate++
		summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "duplicate", Reason: "图片 MD5 已存在"})
		return nil
	}

	path, err := saveUploadedBytes(s.tenantID(c), "avatars", name, data)
	if err != nil {
		summary.Failed++
		summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "failed", Reason: "保存图片失败"})
		return nil
	}
	asset := models.Asset{
		ID:       uuid.New(),
		TenantID: s.tenantID(c),
		GroupID:  groupID,
		Name:     filepath.Base(name),
		MimeType: mimeType,
		MD5:      md5Value,
		URL:      "/" + filepath.ToSlash(path),
		FilePath: path,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&asset).Error; err != nil {
		summary.Failed++
		summary.Items = append(summary.Items, assetUploadResultItem{Name: name, Status: "failed", Reason: "写入素材库失败"})
		_ = removeStoredAssetFile(path)
		return nil
	}
	summary.Success++
	summary.Items = append(summary.Items, assetUploadResultItem{ID: asset.ID.String(), Name: name, Status: "success", URL: asset.URL})
	return nil
}

func isSupportedAvatarImage(name string, mimeType string) bool {
	if !allowedAvatarExtensions[strings.ToLower(filepath.Ext(name))] {
		return false
	}
	switch mimeType {
	case "image/jpeg", "image/png", "image/gif":
		return true
	default:
		return false
	}
}

func (s *Server) DeleteAsset(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "素材 ID 无效")
		return
	}
	var asset models.Asset
	query := s.db.WithContext(c.Request.Context()).Where("id = ?", id)
	query = s.applyTenantAccess(c, query)
	if err := query.First(&asset).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "素材不存在")
		return
	}
	deleteQuery := s.db.WithContext(c.Request.Context()).Where("id = ?", id)
	deleteQuery = s.applyTenantAccess(c, deleteQuery)
	if err := deleteQuery.Delete(&models.Asset{}).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除素材失败")
		return
	}
	if asset.FilePath != "" {
		_ = removeStoredAssetFile(asset.FilePath)
	}
	utils.OK(c, gin.H{"deleted": id})
}

func (s *Server) DeleteAvatarGroup(c *gin.Context, id uuid.UUID) {
	var group models.Group
	groupQuery := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", id, "avatar")
	groupQuery = s.applyTenantAccess(c, groupQuery)
	if err := groupQuery.First(&group).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "头像分组不存在")
		return
	}

	var assets []models.Asset
	assetQuery := s.db.WithContext(c.Request.Context()).Where("group_id = ?", id)
	assetQuery = s.applyTenantAccess(c, assetQuery)
	if err := assetQuery.Find(&assets).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取头像分组素材失败")
		return
	}

	if err := s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		assetDelete := tx.Where("group_id = ?", id)
		assetDelete = s.applyTenantAccess(c, assetDelete)
		if err := assetDelete.Delete(&models.Asset{}).Error; err != nil {
			return err
		}
		groupDelete := tx.Where("id = ? AND resource_type = ?", id, "avatar")
		groupDelete = s.applyTenantAccess(c, groupDelete)
		return groupDelete.Delete(&models.Group{}).Error
	}); err != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除头像分组失败")
		return
	}

	fileErrors := 0
	for _, asset := range assets {
		if asset.FilePath != "" {
			if err := removeStoredAssetFile(asset.FilePath); err != nil {
				fileErrors++
			}
		}
	}
	if fileErrors > 0 {
		utils.Fail(c, http.StatusInternalServerError, "头像分组已删除，部分文件或缓存清理失败")
		return
	}
	utils.OK(c, gin.H{"deleted": id, "assets": len(assets)})
}

func (s *Server) resolveAvatarGroup(c *gin.Context, groupIDText string, newGroupName string) (*uuid.UUID, string, error) {
	if groupIDText != "" && newGroupName != "" {
		return nil, "", fmt.Errorf("请选择已有头像分组或填写新分组，不能同时使用")
	}
	if newGroupName != "" {
		group := models.Group{
			ID:           uuid.New(),
			TenantID:     s.tenantID(c),
			ResourceType: "avatar",
			Name:         newGroupName,
		}
		if err := s.db.WithContext(c.Request.Context()).Create(&group).Error; err != nil {
			return nil, "", fmt.Errorf("创建头像分组失败")
		}
		return &group.ID, group.Name, nil
	}
	if groupIDText == "" {
		return nil, "全部分组", nil
	}
	parsed, err := uuid.Parse(groupIDText)
	if err != nil {
		return nil, "", fmt.Errorf("分组 ID 无效")
	}
	var group models.Group
	query := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", parsed, "avatar")
	query = s.applyTenantAccess(c, query)
	if err := query.First(&group).Error; err != nil {
		return nil, "", fmt.Errorf("头像分组不存在")
	}
	return &group.ID, group.Name, nil
}

func removeStoredAssetFile(path string) error {
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") || !strings.HasPrefix(filepath.ToSlash(clean), "storage/uploads/") {
		return nil
	}
	var firstErr error
	removeOne := func(candidate string) {
		if err := os.Remove(candidate); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	removeOne(clean)
	for _, candidate := range assetCacheCandidates(clean) {
		removeOne(candidate)
	}
	_ = os.Remove(filepath.Join(filepath.Dir(clean), ".cache"))
	_ = os.Remove(filepath.Join(filepath.Dir(clean), "cache"))
	_ = os.Remove(filepath.Join(filepath.Dir(clean), "thumbs"))
	return firstErr
}

func assetCacheCandidates(path string) []string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	candidates := []string{
		path + ".cache",
		path + ".thumb",
		path + ".tmp",
	}
	for _, cacheDir := range []string{".cache", "cache", "thumbs"} {
		matches, _ := filepath.Glob(filepath.Join(dir, cacheDir, base+"*"))
		candidates = append(candidates, matches...)
	}
	return candidates
}
