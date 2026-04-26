package handlers

import (
	"context"
	"time"

	"codex3/backend/internal/models"

	"github.com/google/uuid"
)

func (s *Server) claimTaskRun(ctx context.Context, taskID uuid.UUID, taskTypes ...string) (bool, func()) {
	runID := uuid.NewString()
	now := time.Now()
	staleAfter := s.cfg.TaskRunLockStaleAfter
	if staleAfter <= 0 {
		staleAfter = 26 * time.Hour
	}
	cutoff := now.Add(-staleAfter)
	query := s.db.WithContext(ctx).
		Model(&models.Task{}).
		Where("id = ?", taskID).
		Where("status IN ?", []string{"pending", "queued", "retrying", "running"}).
		Where("(run_id = '' OR run_id IS NULL OR run_locked_at IS NULL OR run_locked_at < ?)", cutoff)
	if len(taskTypes) > 0 {
		query = query.Where("type IN ?", taskTypes)
	}
	result := query.Updates(map[string]any{
		"run_id":        runID,
		"run_locked_at": now,
		"status":        "running",
		"updated_at":    now,
	})
	if result.Error != nil || result.RowsAffected == 0 {
		return false, func() {}
	}
	release := func() {
		_ = s.db.WithContext(context.Background()).
			Model(&models.Task{}).
			Where("id = ? AND run_id = ?", taskID, runID).
			Updates(map[string]any{
				"run_id":        "",
				"run_locked_at": nil,
				"updated_at":    time.Now(),
			}).Error
	}
	return true, release
}
