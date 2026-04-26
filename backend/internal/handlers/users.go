package handlers

import (
	"net/http"
	"strings"
	"time"

	"codex3/backend/internal/middleware"
	"codex3/backend/internal/models"
	"codex3/backend/internal/services"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) ListUsers(c *gin.Context) {
	var users []models.User
	if err := s.db.WithContext(c.Request.Context()).Order("created_at desc").Find(&users).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取用户失败")
		return
	}
	utils.OK(c, users)
}

func (s *Server) CreateUser(c *gin.Context) {
	var req struct {
		Username       string `json:"username" binding:"required"`
		Password       string `json:"password" binding:"required,min=8"`
		Email          string `json:"email"`
		Role           string `json:"role"`
		TelegramUserID string `json:"telegram_user_id"`
		TrialDays      int    `json:"trial_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "用户信息不完整，密码至少 8 位")
		return
	}
	if req.Role == "" {
		req.Role = models.RoleUser
	}
	if req.Role != models.RoleAdmin && req.Role != models.RoleUser {
		utils.Fail(c, http.StatusBadRequest, "角色必须是 admin 或 user")
		return
	}

	id := uuid.New()
	tenantID := id
	if req.Role == models.RoleAdmin {
		tenantID = uuid.Nil
	}
	hash, err := services.HashPassword(req.Password)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "密码加密失败")
		return
	}

	claims := middleware.CurrentClaims(c)
	var createdBy *uuid.UUID
	if claims != nil {
		createdBy = &claims.UserID
	}

	user := models.User{
		ID:           id,
		TenantID:     tenantID,
		Username:     req.Username,
		PasswordHash: hash,
		Email:        req.Email,
		Role:         req.Role,
		Status:       models.StatusActive,
		CreatedBy:    createdBy,
	}
	if req.Role == models.RoleUser {
		user.TelegramUserID = strings.TrimSpace(req.TelegramUserID)
		if req.TrialDays > 0 {
			trialEnds := time.Now().Add(time.Duration(req.TrialDays) * 24 * time.Hour)
			user.TrialEndsAt = &trialEnds
		}
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&user).Error; err != nil {
		utils.Fail(c, http.StatusConflict, "用户名已存在或创建失败")
		return
	}
	if user.TelegramUserID != "" {
		_ = s.bindTelegramUserToWebUser(c, user)
	}
	utils.Created(c, user)
}

func (s *Server) BindUserTelegram(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	var req struct {
		TelegramUserID string `json:"telegram_user_id"`
		TrialDays      int    `json:"trial_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "绑定参数无效")
		return
	}
	var user models.User
	if err := s.db.WithContext(c.Request.Context()).First(&user, "id = ?", id).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	user.TelegramUserID = strings.TrimSpace(req.TelegramUserID)
	if req.TrialDays > 0 {
		trialEnds := time.Now().Add(time.Duration(req.TrialDays) * 24 * time.Hour)
		user.TrialEndsAt = &trialEnds
	}
	user.UpdatedAt = time.Now()
	if err := s.db.WithContext(c.Request.Context()).Save(&user).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "保存绑定失败")
		return
	}
	if user.TelegramUserID != "" {
		_ = s.bindTelegramUserToWebUser(c, user)
	}
	utils.OK(c, user)
}

func (s *Server) bindTelegramUserToWebUser(c *gin.Context, user models.User) error {
	now := time.Now()
	var subscriber models.BotSubscriber
	err := s.db.WithContext(c.Request.Context()).
		Where("telegram_user_id = ?", user.TelegramUserID).
		First(&subscriber).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	updates := map[string]any{
		"user_id":    user.ID,
		"updated_at": now,
	}
	if user.TrialEndsAt != nil {
		updates["trial_ends_at"] = user.TrialEndsAt
		updates["status"] = "active"
		updates["plan"] = "trial"
	}
	return s.db.WithContext(c.Request.Context()).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(updates).Error
}

func (s *Server) UpdateUserStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入状态")
		return
	}
	if req.Status != models.StatusActive && req.Status != models.StatusDisabled {
		utils.Fail(c, http.StatusBadRequest, "状态必须是 active 或 disabled")
		return
	}
	if err := s.db.WithContext(c.Request.Context()).Model(&models.User{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "更新用户状态失败")
		return
	}
	utils.OK(c, gin.H{"id": id, "status": req.Status})
}

func (s *Server) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	var user models.User
	if err := s.db.WithContext(c.Request.Context()).First(&user, "id = ?", id).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role == models.RoleAdmin {
		utils.Fail(c, http.StatusBadRequest, "不允许删除管理员用户")
		return
	}

	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		tenantID := user.TenantID
		for _, model := range []any{
			&models.TaskLog{},
			&models.Task{},
			&models.Workflow{},
			&models.Asset{},
			&models.Target{},
			&models.NetworkNode{},
			&models.Terminal{},
			&models.Group{},
		} {
			if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(model).Error; err != nil {
				return err
			}
		}
		return tx.Unscoped().Delete(&models.User{}, "id = ?", user.ID).Error
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除用户及租户数据失败")
		return
	}
	utils.OK(c, gin.H{"deleted": id})
}
