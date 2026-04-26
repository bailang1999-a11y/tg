package handlers

import (
	"net/http"

	"codex3/backend/internal/middleware"
	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) isAdmin(c *gin.Context) bool {
	claims := middleware.CurrentClaims(c)
	return claims != nil && claims.Role == models.RoleAdmin
}

func (s *Server) applyTenantAccess(c *gin.Context, query *gorm.DB) *gorm.DB {
	if s.isAdmin(c) {
		return query
	}
	return query.Where("tenant_id = ?", s.tenantID(c))
}

func (s *Server) tenantFilterID(c *gin.Context) uuid.UUID {
	if s.isAdmin(c) {
		return uuid.Nil
	}
	return s.tenantID(c)
}

func (s *Server) requireAdminForMutation(c *gin.Context, message string) bool {
	if s.isAdmin(c) {
		return true
	}
	if message == "" {
		message = "权限不足"
	}
	utils.Fail(c, http.StatusForbidden, message)
	c.Abort()
	return false
}
