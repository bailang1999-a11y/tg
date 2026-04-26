package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type taskActionRequest struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"`
}

func (s *Server) CreateTask(c *gin.Context) {
	var req struct {
		Name    string         `json:"name" binding:"required"`
		Type    string         `json:"type" binding:"required"`
		Payload datatypes.JSON `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "任务名称和类型不能为空")
		return
	}
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      req.Name,
		Type:      req.Type,
		Status:    "pending",
		Progress:  0,
		Payload:   req.Payload,
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "创建任务失败")
		return
	}
	utils.Created(c, task)
}

func (s *Server) UpdateTaskAction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "任务 ID 无效")
		return
	}
	action := c.Param("action")
	nextStatus := map[string]string{
		"start":   "running",
		"pause":   "paused",
		"resume":  "running",
		"stop":    "stopped",
		"force":   "stopped",
		"restart": "queued",
	}[action]
	if nextStatus == "" {
		utils.Fail(c, http.StatusBadRequest, "不支持的任务操作")
		return
	}
	result := s.db.WithContext(c.Request.Context()).Model(&models.Task{}).Where("id = ?", id).Updates(map[string]any{
		"status":   nextStatus,
		"progress": progressForStatus(nextStatus),
	})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "更新任务失败")
		return
	}
	if result.RowsAffected == 0 {
		dmStatus := map[string]string{"start": "active", "pause": "paused", "resume": "active", "stop": "completed", "force": "completed", "restart": "queued"}[action]
		if dmStatus == "" {
			utils.Fail(c, http.StatusBadRequest, "不支持的任务操作")
			return
		}
		ctx := c.Request.Context()
		var dmTask models.BotDMTask
		if err := s.db.WithContext(ctx).Where("id = ?", id).First(&dmTask).Error; err != nil {
			utils.Fail(c, http.StatusNotFound, "Bot 私信任务不存在")
			return
		}
		updates := map[string]any{
			"status":     dmStatus,
			"updated_at": time.Now(),
		}
		if dmStatus == "completed" {
			now := time.Now()
			updates["ended_at"] = &now
		}
		if err := s.db.WithContext(ctx).Model(&models.BotDMTask{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			utils.Fail(c, http.StatusInternalServerError, "更新 Bot 私信任务失败")
			return
		}
		s.applyBotDMTaskLifecycleAction(ctx, dmTask, action)
		utils.OK(c, gin.H{"id": id, "status": dmStatus})
		return
	}
	s.applyTaskLifecycleAction(c.Request.Context(), id, action)
	_ = s.db.WithContext(c.Request.Context()).Create(&models.TaskLog{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		TaskID:    id,
		Level:     "INFO",
		Category:  "task",
		Action:    action,
		Details:   "任务状态已切换为 " + nextStatus,
		TraceID:   uuid.NewString(),
		CreatedAt: time.Now(),
	}).Error
	if action == "start" || action == "restart" {
		s.dispatchRunnableTask(c.Request.Context(), id)
	}
	utils.OK(c, gin.H{"id": id, "status": nextStatus})
}

func (s *Server) applyBotDMTaskLifecycleAction(ctx context.Context, dmTask models.BotDMTask, action string) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "pause":
		_ = s.db.WithContext(ctx).
			Model(&models.BotDMTask{}).
			Where("tenant_id = ? AND subscriber_id = ? AND status IN ?", dmTask.TenantID, dmTask.SubscriberID, []string{"active", "running", "queued"}).
			Updates(map[string]any{"status": "paused", "updated_at": time.Now()}).Error
	case "stop", "force":
		now := time.Now()
		_ = s.db.WithContext(ctx).
			Model(&models.BotDMTask{}).
			Where("tenant_id = ? AND subscriber_id = ? AND status IN ?", dmTask.TenantID, dmTask.SubscriberID, []string{"active", "running", "queued", "paused"}).
			Updates(map[string]any{"status": "completed", "ended_at": &now, "updated_at": now}).Error
	}
}

func (s *Server) applyTaskLifecycleAction(ctx context.Context, taskID uuid.UUID, action string) {
	if action != "stop" && action != "force" && action != "pause" {
		return
	}
	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
		return
	}
	if task.Type == "scrm_listener" {
		s.stopSCRMRuntimeByTask(task, action)
	}
	subscriberID := botTaskSubscriberID(task)
	if subscriberID != uuid.Nil {
		var subscriber models.BotSubscriber
		if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", task.TenantID, subscriberID).First(&subscriber).Error; err == nil {
			switch action {
			case "pause":
				s.stopBotSubscriberProcesses(ctx, task.TenantID, subscriber, "paused", "paused", "后台任务中心暂停任务，已停止该 Bot 用户监听与私信进程")
			case "stop", "force":
				s.stopBotSubscriberProcesses(ctx, task.TenantID, subscriber, "completed", "stopped", "后台任务中心停止任务，已停止该 Bot 用户监听与私信进程")
			}
		}
	}
}

func (s *Server) stopSCRMRuntimeByTask(task models.Task, action string) {
	s.listenerMu.Lock()
	defer s.listenerMu.Unlock()
	for key, runtime := range s.listeners {
		if runtime == nil || runtime.task.ID != task.ID {
			continue
		}
		runtime.stopping.Store(true)
		runtime.cancel()
		delete(s.listeners, key)
		return
	}
}

func parseTaskIDs(raw []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(raw))
	seen := map[uuid.UUID]struct{}{}
	for _, item := range raw {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("任务 ID 无效：%s", value)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("请至少选择 1 个任务")
	}
	return ids, nil
}

func (s *Server) BatchTaskAction(c *gin.Context) {
	var req taskActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		utils.Fail(c, http.StatusBadRequest, "操作不能为空")
		return
	}
	valid := map[string]bool{"start": true, "pause": true, "resume": true, "stop": true, "force": true, "restart": true}
	if !valid[action] {
		utils.Fail(c, http.StatusBadRequest, "不支持的任务操作")
		return
	}
	ids, err := parseTaskIDs(req.IDs)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	results := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		status, itemErr := s.doSingleTaskAction(ctx, id, action)
		if itemErr != nil {
			results = append(results, gin.H{"id": id.String(), "ok": false, "error": itemErr.Error()})
			continue
		}
		results = append(results, gin.H{"id": id.String(), "ok": true, "status": status})
	}
	utils.OK(c, gin.H{"action": action, "results": results})
}

func (s *Server) doSingleTaskAction(ctx context.Context, id uuid.UUID, action string) (string, error) {
	nextStatus := map[string]string{
		"start":   "running",
		"pause":   "paused",
		"resume":  "running",
		"stop":    "stopped",
		"force":   "stopped",
		"restart": "queued",
	}[action]
	if nextStatus == "" {
		return "", fmt.Errorf("不支持的任务操作")
	}

	result := s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", id).Updates(map[string]any{
		"status":   nextStatus,
		"progress": progressForStatus(nextStatus),
	})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected > 0 {
		s.applyTaskLifecycleAction(ctx, id, action)
		var tenantTask models.Task
		_ = s.db.WithContext(ctx).Select("tenant_id").Where("id = ?", id).First(&tenantTask).Error
		_ = s.db.WithContext(ctx).Create(&models.TaskLog{
			ID:        uuid.New(),
			TenantID:  tenantTask.TenantID,
			TaskID:    id,
			Level:     "INFO",
			Category:  "task",
			Action:    action,
			Details:   "任务状态已切换为 " + nextStatus,
			TraceID:   uuid.NewString(),
			CreatedAt: time.Now(),
		}).Error
		if action == "start" || action == "restart" {
			s.dispatchRunnableTask(ctx, id)
		}
		return nextStatus, nil
	}

	dmStatus := map[string]string{"start": "active", "pause": "paused", "resume": "active", "stop": "completed", "force": "completed", "restart": "queued"}[action]
	if dmStatus == "" {
		return "", fmt.Errorf("不支持的任务操作")
	}
	var dmTask models.BotDMTask
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&dmTask).Error; err != nil {
		return "", fmt.Errorf("任务不存在")
	}
	updates := map[string]any{
		"status":     dmStatus,
		"updated_at": time.Now(),
	}
	if dmStatus == "completed" {
		now := time.Now()
		updates["ended_at"] = &now
	}
	if err := s.db.WithContext(ctx).Model(&models.BotDMTask{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return "", err
	}
	s.applyBotDMTaskLifecycleAction(ctx, dmTask, action)
	return dmStatus, nil
}

func (s *Server) BatchDeleteTasks(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	ids, err := parseTaskIDs(req.IDs)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := c.Request.Context()
	tenantID := s.tenantID(c)
	results := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		if delErr := s.deleteTaskByID(ctx, tenantID, id); delErr != nil {
			results = append(results, gin.H{"id": id.String(), "ok": false, "error": delErr.Error()})
			continue
		}
		results = append(results, gin.H{"id": id.String(), "ok": true})
	}
	utils.OK(c, gin.H{"results": results})
}

func (s *Server) deleteTaskByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	var task models.Task
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			var dmTask models.BotDMTask
			if dmErr := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&dmTask).Error; dmErr == nil {
				if strings.EqualFold(dmTask.Status, "active") || strings.EqualFold(dmTask.Status, "running") || strings.EqualFold(dmTask.Status, "queued") {
					return fmt.Errorf("执行中的 Bot 私信任务请先停止")
				}
				return s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&models.BotDMTask{}).Error
			}
			return fmt.Errorf("任务不存在")
		}
		return err
	}
	if strings.EqualFold(task.Status, "running") || strings.EqualFold(task.Status, "queued") {
		return fmt.Errorf("执行中的任务暂不支持删除")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND task_id = ?", tenantID, id).Delete(&models.TaskLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&models.Task{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func progressForStatus(status string) int {
	switch status {
	case "queued", "pending":
		return 0
	case "success":
		return 100
	default:
		return 1
	}
}

func progressForBotDMStatus(status string) int {
	switch strings.ToLower(status) {
	case "completed", "success", "finished":
		return 100
	case "active", "running":
		return 50
	case "queued", "pending":
		return 5
	default:
		return 1
	}
}

func (s *Server) DeleteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "任务 ID 无效")
		return
	}

	if err := s.deleteTaskByID(c.Request.Context(), s.tenantID(c), id); err != nil {
		if strings.Contains(err.Error(), "不存在") {
			utils.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "执行中") {
			utils.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		utils.Fail(c, http.StatusInternalServerError, "删除任务失败："+err.Error())
		return
	}

	utils.OK(c, gin.H{"deleted": id.String()})
}

func (s *Server) updateTaskState(ctx context.Context, taskID uuid.UUID, status string, progress int, summary datatypes.JSON) {
	updates := map[string]any{
		"status":   status,
		"progress": progress,
	}
	if len(summary) > 0 {
		updates["summary"] = summary
	}
	_ = s.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).Updates(updates).Error
}

func (s *Server) dispatchRunnableTask(ctx context.Context, taskID uuid.UUID) {
	var task models.Task
	if err := s.db.WithContext(ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
		return
	}
	switch task.Type {
	case "mass_messaging":
		if !s.enqueueTask(ctx, task, "run") {
			go s.runMassMessagingTask(task.ID)
		}
	case "direct_messages":
		if !s.enqueueTask(ctx, task, "run") {
			go s.runDirectMessagesTask(task.ID)
		}
	case "join_targets":
		if !s.enqueueTask(ctx, task, "run") {
			go s.RunJoinTargetsTask(task.ID)
		}
	case "account_status_check":
		if !s.enqueueTask(ctx, task, "run") {
			go s.RunCheckTerminalsTask(task.ID)
		}
	case "target_membership_refresh":
		if !s.enqueueTask(ctx, task, "run") {
			go s.RunRefreshTargetMembershipsTask(task.ID)
		}
	case "profile_modification":
		if !s.enqueueTask(ctx, task, "run") {
			go s.RunProfileModificationTask(task.ID)
		}
	case "import_validation", "import_session", "import_tdata":
		if !s.enqueueTask(ctx, task, "run") {
			go s.RunImportTask(task.ID)
		}
	}
}

func (s *Server) logTaskBackground(ctx context.Context, task models.Task, level, action, detail string) {
	_ = s.db.WithContext(ctx).Create(&models.TaskLog{
		ID:        uuid.New(),
		TenantID:  task.TenantID,
		TaskID:    task.ID,
		Level:     level,
		Category:  task.Type,
		Action:    action,
		Details:   detail,
		TraceID:   uuid.NewString(),
		CreatedAt: time.Now(),
	}).Error
}
