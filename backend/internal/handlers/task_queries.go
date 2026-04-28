package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"codex3/backend/internal/middleware"
	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/datatypes"
)

var listenerRefExtractPattern = regexp.MustCompile(`监听号\s+([^\s：:，,]+)`)
var listenerRefNormalizePattern = regexp.MustCompile(`[^a-z0-9]+`)

func (s *Server) ListTasks(c *gin.Context) {
	var tasks []models.Task
	limit := boundedQueryInt(c, "limit", 200, 1, 500)
	offset := boundedQueryInt(c, "offset", 0, 0, 100000)
	query := s.db.WithContext(c.Request.Context()).Order("created_at desc")
	claims := middleware.CurrentClaims(c)
	if claims == nil || claims.Role != models.RoleAdmin {
		query = query.Where("tenant_id = ?", s.tenantID(c))
		userID := s.userIDPtr(c)
		if userID == nil {
			utils.Fail(c, http.StatusUnauthorized, "未登录")
			return
		}
		ownerCondition, ownerArgs := s.taskOwnerFilter(c.Request.Context(), c, *userID)
		query = query.Where(ownerCondition, ownerArgs...)
	}
	if taskType := c.Query("type"); taskType != "" {
		query = query.Where("type = ?", taskType)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("created_by = ?", userID)
	}
	if botUserID := c.Query("bot_user_id"); botUserID != "" {
		query = query.Where("CAST(payload AS TEXT) LIKE ?", "%"+botUserID+"%")
	}
	if err := query.Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取任务失败")
		return
	}
	if c.Query("type") == "" || c.Query("type") == "bot_dm" {
		var dmTasks []models.BotDMTask
		dmQuery := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", s.tenantID(c)).Order("created_at desc").Limit(limit).Offset(offset)
		if claims == nil || claims.Role != models.RoleAdmin {
			userID := s.userIDPtr(c)
			if userID == nil {
				utils.Fail(c, http.StatusUnauthorized, "未登录")
				return
			}
			dmQuery = dmQuery.Where("subscriber_id IN (?)",
				s.db.WithContext(c.Request.Context()).
					Model(&models.BotSubscriber{}).
					Select("id").
					Where("tenant_id = ? AND user_id = ?", s.tenantID(c), *userID))
		}
		if botUserID := c.Query("bot_user_id"); botUserID != "" {
			dmQuery = dmQuery.Where("subscriber_id = ?", botUserID)
		}
		if userID := c.Query("user_id"); userID != "" {
			dmQuery = dmQuery.Where("subscriber_id IN (?)",
				s.db.WithContext(c.Request.Context()).
					Model(&models.BotSubscriber{}).
					Select("id").
					Where("tenant_id = ? AND user_id = ?", s.tenantID(c), userID))
		}
		if status := c.Query("status"); status != "" {
			switch strings.ToLower(strings.TrimSpace(status)) {
			case "running":
				dmQuery = dmQuery.Where("status IN ?", []string{"active", "running"})
			case "success", "completed", "finished":
				dmQuery = dmQuery.Where("status IN ?", []string{"completed", "success", "finished"})
			case "stopped":
				dmQuery = dmQuery.Where("status IN ?", []string{"stopped", "paused"})
			default:
				dmQuery = dmQuery.Where("status = ?", status)
			}
		}
		if err := dmQuery.Find(&dmTasks).Error; err == nil {
			for _, dmTask := range dmTasks {
				payload, _ := json.Marshal(map[string]any{
					"bot_subscriber_id": dmTask.SubscriberID.String(),
					"account_group":     dmTask.AccountGroupName,
					"messages":          jsonStringSlice(dmTask.Messages),
					"sent_count":        dmTask.SentCount,
					"min_delay_seconds": dmTask.MinDelaySeconds,
					"max_delay_seconds": dmTask.MaxDelaySeconds,
				})
				tasks = append(tasks, models.Task{
					ID:        dmTask.ID,
					TenantID:  dmTask.TenantID,
					Name:      firstNonEmpty(dmTask.Name, "Bot 私信任务"),
					Type:      "bot_dm",
					Status:    dmTask.Status,
					Progress:  progressForBotDMStatus(dmTask.Status),
					Payload:   datatypes.JSON(payload),
					CreatedAt: dmTask.CreatedAt,
					UpdatedAt: dmTask.UpdatedAt,
				})
			}
		}
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	utils.OK(c, s.enrichTasks(c.Request.Context(), tasks))
}

func (s *Server) RefreshTasks(c *gin.Context) {
	claims := middleware.CurrentClaims(c)
	query := s.db.WithContext(c.Request.Context()).Model(&models.Task{})
	if claims == nil || claims.Role != models.RoleAdmin {
		query = query.Where("tenant_id = ?", s.tenantID(c))
	}
	_ = query.Where("type = ? AND status = ? AND updated_at < ?", "scrm_listener", "running", time.Now().Add(-30*time.Second)).Update("updated_at", time.Now()).Error
	utils.OK(c, gin.H{"status": "refreshed", "message": "任务状态已刷新"})
}

func (s *Server) ListTaskLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "任务 ID 无效")
		return
	}
	claims := middleware.CurrentClaims(c)
	if claims == nil || claims.Role != models.RoleAdmin {
		userID := s.userIDPtr(c)
		if userID == nil {
			utils.Fail(c, http.StatusUnauthorized, "未登录")
			return
		}
		ownerCondition, ownerArgs := s.taskOwnerFilter(c.Request.Context(), c, *userID)
		var count int64
		if err := s.db.WithContext(c.Request.Context()).
			Model(&models.Task{}).
			Where("tenant_id = ? AND id = ?", s.tenantID(c), id).
			Where(ownerCondition, ownerArgs...).
			Count(&count).Error; err != nil {
			utils.Fail(c, http.StatusInternalServerError, "读取任务失败")
			return
		}
		if count == 0 {
			utils.Fail(c, http.StatusForbidden, "无权查看该任务日志")
			return
		}
	}
	var logs []models.TaskLog
	limit := boundedQueryInt(c, "limit", 1000, 1, 2000)
	offset := boundedQueryInt(c, "offset", 0, 0, 1000000)
	if err := s.db.WithContext(c.Request.Context()).Where("task_id = ?", id).Order("created_at asc").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取日志失败")
		return
	}
	utils.OK(c, s.enrichTaskLogs(c.Request.Context(), logs))
}

func (s *Server) ListLogs(c *gin.Context) {
	var logs []models.TaskLog
	limit := boundedQueryInt(c, "limit", 200, 1, 500)
	offset := boundedQueryInt(c, "offset", 0, 0, 1000000)
	query := s.db.WithContext(c.Request.Context()).Order("created_at desc")
	taskScope := s.db.WithContext(c.Request.Context()).
		Model(&models.Task{}).
		Select("id").
		Where("tenant_id = ?", s.tenantID(c))
	claims := middleware.CurrentClaims(c)
	if claims == nil || claims.Role != models.RoleAdmin {
		userID := s.userIDPtr(c)
		if userID == nil {
			utils.Fail(c, http.StatusUnauthorized, "未登录")
			return
		}
		ownerCondition, ownerArgs := s.taskOwnerFilter(c.Request.Context(), c, *userID)
		taskScope = taskScope.Where(ownerCondition, ownerArgs...)
	}
	if taskID := c.Query("task_id"); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if level := c.Query("level"); level != "" {
		query = query.Where("level = ?", strings.ToUpper(level))
	}
	if category := c.Query("category"); category != "" {
		query = query.Where("category = ?", category)
	}
	if taskType := c.Query("type"); taskType != "" {
		taskScope = taskScope.Where("type = ?", taskType)
	}
	if userID := c.Query("user_id"); userID != "" {
		taskScope = taskScope.Where("created_by = ?", userID)
	}
	if botUserID := c.Query("bot_user_id"); botUserID != "" {
		taskScope = taskScope.Where("CAST(payload AS TEXT) LIKE ?", "%"+botUserID+"%")
	}
	if c.Query("task_id") == "" {
		query = query.Where("task_id IN (?)", taskScope)
	}
	if err := query.Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取日志失败")
		return
	}
	utils.OK(c, s.enrichTaskLogs(c.Request.Context(), logs))
}

func (s *Server) taskOwnerFilter(ctx context.Context, c *gin.Context, userID uuid.UUID) (string, []any) {
	condition := "(created_by = ?"
	args := []any{userID}
	var subscriberIDs []uuid.UUID
	_ = s.db.WithContext(ctx).
		Model(&models.BotSubscriber{}).
		Where("tenant_id = ? AND user_id = ?", s.tenantID(c), userID).
		Pluck("id", &subscriberIDs).Error
	for _, subscriberID := range subscriberIDs {
		condition += " OR CAST(payload AS TEXT) LIKE ?"
		args = append(args, "%"+subscriberID.String()+"%")
	}
	condition += ")"
	return condition, args
}

func boundedQueryInt(c *gin.Context, key string, fallback int, minValue int, maxValue int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (s *Server) ClearLogs(c *gin.Context) {
	ctx := c.Request.Context()
	claims := middleware.CurrentClaims(c)
	query := s.db.WithContext(ctx).Where("tenant_id = ?", s.tenantID(c))
	if claims == nil || claims.Role != models.RoleAdmin {
		userID := s.userIDPtr(c)
		if userID == nil {
			utils.Fail(c, http.StatusUnauthorized, "未登录")
			return
		}
		query = query.Where("task_id IN (?)", s.db.WithContext(ctx).Model(&models.Task{}).Select("id").Where("tenant_id = ? AND created_by = ?", s.tenantID(c), *userID))
	}
	if err := query.Delete(&models.TaskLog{}).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "清除日志失败")
		return
	}
	utils.OK(c, gin.H{"status": "cleared", "message": "日志已彻底清除"})
}

func (s *Server) enrichTaskLogs(ctx context.Context, logs []models.TaskLog) []taskLogItem {
	if len(logs) == 0 {
		return []taskLogItem{}
	}
	taskIDs := make([]uuid.UUID, 0, len(logs))
	seen := map[uuid.UUID]bool{}
	for _, log := range logs {
		if !seen[log.TaskID] {
			taskIDs = append(taskIDs, log.TaskID)
			seen[log.TaskID] = true
		}
	}
	var tasks []models.Task
	_ = s.db.WithContext(ctx).Where("id IN ?", taskIDs).Find(&tasks).Error
	taskMap := s.enrichLogTasks(ctx, tasks)
	refDisplayMap := s.buildLogTerminalRefDisplayMap(ctx, logs)
	out := make([]taskLogItem, 0, len(logs))
	for _, log := range logs {
		item := taskLogItem{
			TaskLog:    log,
			LevelText:  taskLogLevelText(log.Level),
			ActionText: taskLogActionText(log.Action),
		}
		item.Details = taskLogDetailText(log.Details)
		if task, ok := taskMap[log.TaskID]; ok {
			taskCopy := task
			item.Task = &taskCopy
		}
		if display := resolveLogTerminalDisplay(log, refDisplayMap); display != "" {
			item.TerminalRef = display
			item.Details = rewriteListenerRefInDetail(item.Details, extractListenerRefFromLog(log), display)
		}
		out = append(out, item)
	}
	return out
}

func (s *Server) enrichLogTasks(ctx context.Context, tasks []models.Task) map[uuid.UUID]taskLogTaskItem {
	taskMap := map[uuid.UUID]taskLogTaskItem{}
	if len(tasks) == 0 {
		return taskMap
	}
	userIDs := make([]uuid.UUID, 0)
	botIDs := make([]uuid.UUID, 0)
	seenUsers := map[uuid.UUID]bool{}
	seenBots := map[uuid.UUID]bool{}
	for _, task := range tasks {
		if task.CreatedBy != nil && !seenUsers[*task.CreatedBy] {
			userIDs = append(userIDs, *task.CreatedBy)
			seenUsers[*task.CreatedBy] = true
		}
		if botID, ok := taskBotSubscriberID(task); ok && !seenBots[botID] {
			botIDs = append(botIDs, botID)
			seenBots[botID] = true
		}
	}

	userMap := map[uuid.UUID]models.User{}
	if len(userIDs) > 0 {
		var users []models.User
		_ = s.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error
		for _, user := range users {
			userMap[user.ID] = user
		}
	}

	botMap := map[uuid.UUID]models.BotSubscriber{}
	if len(botIDs) > 0 {
		var subscribers []models.BotSubscriber
		_ = s.db.WithContext(ctx).Where("id IN ?", botIDs).Find(&subscribers).Error
		for _, subscriber := range subscribers {
			botMap[subscriber.ID] = subscriber
		}
	}

	for _, task := range tasks {
		item := taskLogTaskItem{
			ID:        task.ID,
			Name:      task.Name,
			Type:      task.Type,
			Status:    task.Status,
			Progress:  task.Progress,
			Payload:   taskLogPayload(task),
			CreatedAt: task.CreatedAt,
			UpdatedAt: task.UpdatedAt,
		}
		if task.CreatedBy != nil {
			if user, ok := userMap[*task.CreatedBy]; ok {
				item.Creator = &taskUserRef{
					ID:               user.ID.String(),
					Username:         user.Username,
					Role:             user.Role,
					TelegramUserID:   user.TelegramUserID,
					TelegramUsername: user.TelegramUsername,
				}
			}
		}
		if botID, ok := taskBotSubscriberID(task); ok {
			if subscriber, exists := botMap[botID]; exists {
				item.BotUser = &taskBotUserRef{
					ID:             subscriber.ID.String(),
					Nickname:       botSubscriberDisplayName(subscriber),
					Username:       subscriber.Username,
					TelegramUserID: subscriber.TelegramUserID,
					Status:         botSubscriberStatusText(subscriber),
					Plan:           botPlanText(subscriber.Plan),
				}
			}
		}
		taskMap[task.ID] = item
	}
	return taskMap
}

func taskLogPayload(task models.Task) datatypes.JSON {
	if len(task.Payload) == 0 || !json.Valid(task.Payload) {
		return nil
	}
	var payload struct {
		BotSubscriberID string `json:"bot_subscriber_id,omitempty"`
	}
	if err := json.Unmarshal(task.Payload, &payload); err != nil || strings.TrimSpace(payload.BotSubscriberID) == "" {
		return nil
	}
	data, _ := json.Marshal(payload)
	return datatypes.JSON(data)
}

func (s *Server) buildLogTerminalRefDisplayMap(ctx context.Context, logs []models.TaskLog) map[string]string {
	result := map[string]string{}
	if len(logs) == 0 {
		return result
	}
	tenantID := logs[0].TenantID
	candidates := map[string]struct{}{}
	uuidRefs := make([]uuid.UUID, 0)
	seenUUID := map[uuid.UUID]struct{}{}
	addRef := func(value string) {
		ref := strings.TrimSpace(value)
		if ref == "" {
			return
		}
		candidates[ref] = struct{}{}
		if parsed, err := uuid.Parse(ref); err == nil {
			if _, ok := seenUUID[parsed]; !ok {
				uuidRefs = append(uuidRefs, parsed)
				seenUUID[parsed] = struct{}{}
			}
		}
	}
	for _, log := range logs {
		addRef(log.TerminalRef)
		addRef(extractListenerRefFromLog(log))
	}
	if len(candidates) == 0 {
		return result
	}
	refList := make([]string, 0, len(candidates))
	for key := range candidates {
		refList = append(refList, key)
	}
	var terminals []models.Terminal
	queryTerminals := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	switch {
	case len(uuidRefs) > 0:
		queryTerminals = queryTerminals.Where("id IN ? OR nickname IN ? OR phone IN ?", uuidRefs, refList, refList)
	default:
		queryTerminals = queryTerminals.Where("nickname IN ? OR phone IN ?", refList, refList)
	}
	_ = queryTerminals.Find(&terminals).Error
	for _, terminal := range terminals {
		display := strings.TrimSpace(terminal.Nickname)
		if display == "" {
			display = strings.TrimSpace(terminal.Phone)
		}
		if display == "" {
			continue
		}
		mapTerminalRef(result, terminal.ID.String(), display)
		mapTerminalRef(result, terminal.Phone, display)
		mapTerminalRef(result, terminal.Nickname, display)
	}
	var listenerAccounts []models.ListenerAccount
	queryAccounts := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	switch {
	case len(uuidRefs) > 0:
		queryAccounts = queryAccounts.Where("id IN ? OR nickname IN ? OR phone IN ?", uuidRefs, refList, refList)
	default:
		queryAccounts = queryAccounts.Where("nickname IN ? OR phone IN ?", refList, refList)
	}
	_ = queryAccounts.Find(&listenerAccounts).Error
	for _, account := range listenerAccounts {
		display := strings.TrimSpace(account.Nickname)
		if display == "" {
			display = strings.TrimSpace(account.Phone)
		}
		if display == "" {
			continue
		}
		mapTerminalRef(result, account.ID.String(), display)
		mapTerminalRef(result, account.Phone, display)
		mapTerminalRef(result, account.Nickname, display)
	}
	s.applyTerminalRefFuzzyMap(result, candidates, terminals, listenerAccounts)
	return result
}

func (s *Server) applyTerminalRefFuzzyMap(target map[string]string, candidates map[string]struct{}, terminals []models.Terminal, listenerAccounts []models.ListenerAccount) {
	if len(candidates) == 0 {
		return
	}
	type refEntry struct {
		raw      string
		norm     string
		display  string
		priority int
	}
	allRefs := make([]refEntry, 0, len(terminals)*3+len(listenerAccounts)*3)
	appendEntry := func(raw string, display string, priority int) {
		ref := strings.TrimSpace(raw)
		if ref == "" || strings.TrimSpace(display) == "" {
			return
		}
		norm := normalizeListenerRef(ref)
		if norm == "" {
			return
		}
		allRefs = append(allRefs, refEntry{
			raw:      ref,
			norm:     norm,
			display:  strings.TrimSpace(display),
			priority: priority,
		})
	}
	for _, terminal := range terminals {
		display := strings.TrimSpace(terminal.Nickname)
		if display == "" {
			display = strings.TrimSpace(terminal.Phone)
		}
		appendEntry(terminal.ID.String(), display, 3)
		appendEntry(terminal.Phone, display, 2)
		appendEntry(terminal.Nickname, display, 1)
	}
	for _, account := range listenerAccounts {
		display := strings.TrimSpace(account.Nickname)
		if display == "" {
			display = strings.TrimSpace(account.Phone)
		}
		appendEntry(account.ID.String(), display, 3)
		appendEntry(account.Phone, display, 2)
		appendEntry(account.Nickname, display, 1)
	}
	for candidate := range candidates {
		ref := strings.TrimSpace(candidate)
		if ref == "" {
			continue
		}
		if _, exists := target[ref]; exists {
			continue
		}
		norm := normalizeListenerRef(ref)
		if norm == "" || len(norm) < 6 {
			continue
		}
		bestScore := 0
		bestDisplay := ""
		for _, entry := range allRefs {
			if entry.norm == "" {
				continue
			}
			score := 0
			switch {
			case norm == entry.norm:
				score = 100 + entry.priority
			case strings.Contains(norm, entry.norm):
				score = 80 + entry.priority
			case strings.Contains(entry.norm, norm):
				score = 60 + entry.priority
			}
			if score > bestScore {
				bestScore = score
				bestDisplay = entry.display
			}
		}
		if bestDisplay != "" {
			target[ref] = fmt.Sprintf("%s · %s", ref, bestDisplay)
		}
	}
}

func mapTerminalRef(target map[string]string, rawRef string, nickname string) {
	ref := strings.TrimSpace(rawRef)
	if ref == "" || nickname == "" {
		return
	}
	target[ref] = fmt.Sprintf("%s · %s", ref, nickname)
	normalized := normalizeListenerRef(ref)
	if normalized != "" {
		target[normalized] = fmt.Sprintf("%s · %s", ref, nickname)
	}
}

func extractListenerRefFromLog(log models.TaskLog) string {
	if strings.TrimSpace(log.TerminalRef) != "" {
		return strings.TrimSpace(log.TerminalRef)
	}
	matches := listenerRefExtractPattern.FindStringSubmatch(strings.TrimSpace(log.Details))
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func resolveLogTerminalDisplay(log models.TaskLog, refDisplayMap map[string]string) string {
	ref := extractListenerRefFromLog(log)
	if ref == "" {
		return ""
	}
	if display, ok := refDisplayMap[ref]; ok {
		return display
	}
	normalized := normalizeListenerRef(ref)
	if normalized != "" {
		if display, ok := refDisplayMap[normalized]; ok {
			return display
		}
	}
	return ""
}

func rewriteListenerRefInDetail(detail string, rawRef string, display string) string {
	ref := strings.TrimSpace(rawRef)
	if ref == "" || strings.TrimSpace(display) == "" {
		return detail
	}
	pattern := regexp.MustCompile(`监听号\s+` + regexp.QuoteMeta(ref))
	return pattern.ReplaceAllString(detail, "监听号 "+display)
}

func normalizeListenerRef(value string) string {
	ref := strings.ToLower(strings.TrimSpace(value))
	if ref == "" {
		return ""
	}
	return listenerRefNormalizePattern.ReplaceAllString(ref, "")
}

func (s *Server) enrichTasks(ctx context.Context, tasks []models.Task) []taskListItem {
	if len(tasks) == 0 {
		return []taskListItem{}
	}
	userIDs := make([]uuid.UUID, 0)
	botIDs := make([]uuid.UUID, 0)
	seenUsers := map[uuid.UUID]bool{}
	seenBots := map[uuid.UUID]bool{}
	for _, task := range tasks {
		if task.CreatedBy != nil && !seenUsers[*task.CreatedBy] {
			userIDs = append(userIDs, *task.CreatedBy)
			seenUsers[*task.CreatedBy] = true
		}
		if botID, ok := taskBotSubscriberID(task); ok && !seenBots[botID] {
			botIDs = append(botIDs, botID)
			seenBots[botID] = true
		}
	}

	userMap := map[uuid.UUID]models.User{}
	if len(userIDs) > 0 {
		var users []models.User
		_ = s.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error
		for _, user := range users {
			userMap[user.ID] = user
		}
	}

	botMap := map[uuid.UUID]models.BotSubscriber{}
	dmMap := map[uuid.UUID][]models.BotDMTask{}
	if len(botIDs) > 0 {
		var subscribers []models.BotSubscriber
		_ = s.db.WithContext(ctx).Where("id IN ?", botIDs).Find(&subscribers).Error
		for _, subscriber := range subscribers {
			botMap[subscriber.ID] = subscriber
		}
		var dmTasks []models.BotDMTask
		_ = s.db.WithContext(ctx).Where("subscriber_id IN ? AND status IN ?", botIDs, []string{"active", "running", "queued"}).Order("created_at desc").Find(&dmTasks).Error
		for _, dmTask := range dmTasks {
			dmMap[dmTask.SubscriberID] = append(dmMap[dmTask.SubscriberID], dmTask)
		}
	}

	out := make([]taskListItem, 0, len(tasks))
	for _, task := range tasks {
		item := taskListItem{
			Task:     task,
			Settings: taskSettings(task),
		}
		if task.CreatedBy != nil {
			if user, ok := userMap[*task.CreatedBy]; ok {
				item.Creator = &taskUserRef{
					ID:               user.ID.String(),
					Username:         user.Username,
					Role:             user.Role,
					TelegramUserID:   user.TelegramUserID,
					TelegramUsername: user.TelegramUsername,
				}
			}
		}
		if botID, ok := taskBotSubscriberID(task); ok {
			if subscriber, exists := botMap[botID]; exists {
				item.BotUser = &taskBotUserRef{
					ID:             subscriber.ID.String(),
					Nickname:       botSubscriberDisplayName(subscriber),
					Username:       subscriber.Username,
					TelegramUserID: subscriber.TelegramUserID,
					Status:         botSubscriberStatusText(subscriber),
					Plan:           botPlanText(subscriber.Plan),
				}
				item.BotDMTasks = dmMap[subscriber.ID]
				item.BotDMSettings = botDMTaskSettings(dmMap[subscriber.ID])
			}
		}
		out = append(out, item)
	}
	return out
}

var logUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const logStreamCacheTTL = 2 * time.Second

type logStreamCacheEntry struct {
	expiresAt time.Time
	payload   gin.H
}

func (s *Server) LogStream(c *gin.Context) {
	conn, err := logUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	s.wsConnections.Add(1)
	defer s.wsConnections.Add(-1)

	settings := s.readSystemSettings(c.Request.Context(), s.tenantID(c))
	limit := settings.Frequency.WSLogBatchSize
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	cacheKey := s.logStreamCacheKey(c)
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if cached, ok := s.loadLogStreamCache(cacheKey); ok {
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteJSON(cached); err != nil {
					return
				}
				continue
			}
			var logs []models.TaskLog
			query := s.db.WithContext(c.Request.Context()).Order("created_at desc").Limit(limit)
			taskScope := s.db.WithContext(c.Request.Context()).
				Model(&models.Task{}).
				Select("id").
				Where("tenant_id = ?", s.tenantID(c))
			useTaskScope := false
			claims := middleware.CurrentClaims(c)
			if claims == nil || claims.Role != models.RoleAdmin {
				userID := s.userIDPtr(c)
				if userID == nil {
					_ = conn.WriteJSON(gin.H{"error": "未登录"})
					return
				}
				ownerCondition, ownerArgs := s.taskOwnerFilter(c.Request.Context(), c, *userID)
				taskScope = taskScope.Where(ownerCondition, ownerArgs...)
				useTaskScope = true
			}
			if taskID := c.Query("task_id"); taskID != "" {
				query = query.Where("task_id = ?", taskID)
			}
			if level := c.Query("level"); level != "" {
				query = query.Where("level = ?", strings.ToUpper(level))
			}
			if category := c.Query("category"); category != "" {
				query = query.Where("category = ?", category)
			}
			if taskType := c.Query("type"); taskType != "" {
				taskScope = taskScope.Where("type = ?", taskType)
				useTaskScope = true
			}
			if userID := c.Query("user_id"); userID != "" {
				taskScope = taskScope.Where("created_by = ?", userID)
				useTaskScope = true
			}
			if botUserID := c.Query("bot_user_id"); botUserID != "" {
				taskScope = taskScope.Where("CAST(payload AS TEXT) LIKE ?", "%"+botUserID+"%")
				useTaskScope = true
			}
			if useTaskScope {
				query = query.Where("task_id IN (?)", taskScope)
			}
			if err := query.Find(&logs).Error; err != nil {
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_ = conn.WriteJSON(gin.H{"error": "读取日志失败"})
				return
			}
			payload := gin.H{"type": "logs", "data": s.enrichTaskLogs(c.Request.Context(), logs)}
			s.storeLogStreamCache(cacheKey, payload)
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteJSON(payload); err != nil {
				return
			}
		}
	}
}

func (s *Server) logStreamCacheKey(c *gin.Context) string {
	claims := middleware.CurrentClaims(c)
	userKey := "anonymous"
	if claims != nil {
		userKey = claims.UserID.String() + ":" + claims.Role
	}
	return s.tenantID(c).String() + ":" + userKey + ":" + c.Request.URL.RawQuery
}

func (s *Server) loadLogStreamCache(key string) (gin.H, bool) {
	value, ok := s.logStreamCache.Load(key)
	if !ok {
		return nil, false
	}
	entry, ok := value.(logStreamCacheEntry)
	if !ok || time.Now().After(entry.expiresAt) {
		s.logStreamCache.Delete(key)
		return nil, false
	}
	return entry.payload, true
}

func (s *Server) storeLogStreamCache(key string, payload gin.H) {
	s.logStreamCache.Store(key, logStreamCacheEntry{
		expiresAt: time.Now().Add(logStreamCacheTTL),
		payload:   payload,
	})
}
