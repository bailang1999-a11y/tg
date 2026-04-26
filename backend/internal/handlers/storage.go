package handlers

import (
	"net/http"
	"path/filepath"
	"strings"

	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var publicUploadCategories = map[string]bool{
	"assets":               true,
	"avatars":              true,
	"workflow-media":       true,
	"terminal-avatars":     true,
	"terminal-avatar-sync": true,
}

func (s *Server) PublicUpload(c *gin.Context) {
	rawPath := strings.TrimPrefix(c.Param("filepath"), "/")
	clean := filepath.Clean(rawPath)
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		utils.Fail(c, http.StatusNotFound, "文件不存在")
		return
	}

	parts := strings.Split(filepath.ToSlash(clean), "/")
	if len(parts) < 3 || !publicUploadCategories[parts[1]] {
		utils.Fail(c, http.StatusNotFound, "文件不存在")
		return
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		utils.Fail(c, http.StatusNotFound, "文件不存在")
		return
	}

	c.File(filepath.Join("storage", "uploads", clean))
}
