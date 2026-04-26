package handlers

import (
	"context"
	"net/http"
	"time"

	"codex3/backend/internal/middleware"
	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
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
		utils.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	utils.OK(c, gin.H{"token": token, "user": user})
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
