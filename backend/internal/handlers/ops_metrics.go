package handlers

import (
	"net/http"
	"runtime"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) OpsMetrics(c *gin.Context) {
	sqlDB, err := s.db.DB()
	if err != nil {
		utils.Fail(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	stats := sqlDB.Stats()

	var queued, running, failed int64
	_ = s.db.WithContext(c.Request.Context()).Model(&models.Task{}).Where("status = ?", "queued").Count(&queued).Error
	_ = s.db.WithContext(c.Request.Context()).Model(&models.Task{}).Where("status = ?", "running").Count(&running).Error
	_ = s.db.WithContext(c.Request.Context()).Model(&models.Task{}).Where("status IN ?", []string{"failed", "error"}).Count(&failed).Error

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	utils.OK(c, gin.H{
		"db": gin.H{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"wait_count":       stats.WaitCount,
			"wait_duration_ms": stats.WaitDuration.Milliseconds(),
			"max_open":         stats.MaxOpenConnections,
		},
		"runtime": gin.H{
			"goroutines": runtime.NumGoroutine(),
			"memory_mb":  mem.Alloc / 1024 / 1024,
		},
		"tasks": gin.H{
			"queued":  queued,
			"running": running,
			"failed":  failed,
		},
		"websocket": gin.H{
			"log_connections": s.wsConnections.Load(),
		},
	})
}
