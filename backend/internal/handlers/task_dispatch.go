package handlers

import (
	"context"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/internal/taskqueue"
)

func (s *Server) enqueueTask(ctx context.Context, task models.Task, action string) bool {
	if s.taskPublisher == nil {
		return false
	}
	if action == "" {
		action = "run"
	}
	if err := s.taskPublisher.PublishTask(ctx, taskqueue.TaskMessage{
		TaskID:    task.ID,
		TenantID:  task.TenantID,
		Type:      task.Type,
		Action:    action,
		CreatedAt: time.Now(),
	}); err != nil {
		s.logTaskBackground(ctx, task, "WARN", "queue", "任务队列不可用，切换为本地执行："+err.Error())
		return false
	}
	s.logTaskBackground(ctx, task, "INFO", "queue", "任务已投递到工作队列")
	return true
}
