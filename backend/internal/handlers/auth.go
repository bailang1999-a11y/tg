package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codex3/backend/internal/middleware"
	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Server) Health(c *gin.Context) {
	utils.OK(c, gin.H{"status": "ok"})
}

func (s *Server) Ready(c *gin.Context) {
	sqlDB, err := s.db.DB()
	if err != nil {
		utils.Fail(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		utils.Fail(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	utils.OK(c, gin.H{"status": "ready"})
}

func (s *Server) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	user, token, err := s.auth.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		s.logLoginAction(c, strings.TrimSpace(req.Username), nil, false, err.Error())
		utils.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	s.logLoginAction(c, user.Username, user, true, "登录成功")
	utils.OK(c, gin.H{"token": token, "user": user})
}

func (s *Server) logLoginAction(c *gin.Context, username string, user *models.User, success bool, result string) {
	tenantID := uuid.Nil
	var createdBy *uuid.UUID
	if user != nil {
		tenantID = user.TenantID
		createdBy = &user.ID
	} else if username != "" {
		var existing models.User
		if err := s.db.WithContext(c.Request.Context()).Where("username = ?", username).First(&existing).Error; err == nil {
			tenantID = existing.TenantID
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return
		}
	}
	status := "success"
	level := "INFO"
	if !success {
		status = "failed"
		level = "WARN"
	}
	payload := datatypes.JSON([]byte(fmt.Sprintf(`{"username":"%s","ip":"%s","success":%t}`, sanitizeAuditJSONText(username), c.ClientIP(), success)))
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      "操作日志-登录",
		Type:      "audit_action",
		Status:    status,
		Progress:  100,
		Payload:   payload,
		CreatedBy: createdBy,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		return
	}
	detail := fmt.Sprintf("操作：登录；接口：POST /api/v1/auth/login；用户：%s；来源 IP：%s；结果：%s", firstNonEmpty(username, "未知用户"), c.ClientIP(), result)
	_ = s.createTaskLog(c.Request.Context(), task, level, "audit_action", detail, "", "")
}

func sanitizeAuditJSONText(value string) string {
	data, _ := json.Marshal(value)
	return strings.Trim(string(data), `"`)
}

func (s *Server) Me(c *gin.Context) {
	claims := middleware.CurrentClaims(c)
	if claims == nil {
		utils.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var user models.User
	if err := s.db.WithContext(c.Request.Context()).First(&user, "id = ?", claims.UserID).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	utils.OK(c, user)
}
