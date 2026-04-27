package handlers

import (
	"context"
	"encoding/json"
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

type systemSecuritySettings struct {
	EnforceTenantIsolation bool `json:"enforce_tenant_isolation"`
	RequireAdminApproval   bool `json:"require_admin_approval"`
	MaskSensitiveLogs      bool `json:"mask_sensitive_logs"`
}

type systemFrequencySettings struct {
	MaxConcurrentTasks     int `json:"max_concurrent_tasks"`
	MaxConcurrentOutreach  int `json:"max_concurrent_outreach"`
	WSLogBatchSize         int `json:"ws_log_batch_size"`
	DashboardRefreshSecond int `json:"dashboard_refresh_second"`
}

type systemListenerHealthSettings struct {
	AutoAccountCheckEnabled     bool `json:"auto_account_check_enabled"`
	AccountCheckIntervalMinutes int  `json:"account_check_interval_minutes"`
	AutoProxyCheckEnabled       bool `json:"auto_proxy_check_enabled"`
	ProxyCheckIntervalMinutes   int  `json:"proxy_check_interval_minutes"`
	SilenceAlertMinutes         int  `json:"silence_alert_minutes"`
}

type systemAuditSettings struct {
	LogRetentionDays  int  `json:"log_retention_days"`
	RealtimeLogStream bool `json:"realtime_log_stream"`
	NotifyOnFailure   bool `json:"notify_on_failure"`
}

type systemAdapterSettings struct {
	TelegramSyncEnabled  bool `json:"telegram_sync_enabled"`
	TelegramApplyEnabled bool `json:"telegram_apply_enabled"`
	OutreachDryRun       bool `json:"outreach_dry_run"`
	WorkflowDryRun       bool `json:"workflow_dry_run"`
}

type systemRiskControlSettings struct {
	AutoBypassHighRisk           bool `json:"auto_bypass_high_risk"`
	AutoBypassActiveRestrictions int  `json:"auto_bypass_active_restrictions"`
	AutoBypassFailures24H        int  `json:"auto_bypass_failures_24h"`
	MessageCooldownMinutes       int  `json:"message_cooldown_minutes"`
	MessageJitterMinutes         int  `json:"message_jitter_minutes"`
	JoinDailyLimit               int  `json:"join_daily_limit"`
	JoinIntervalMinutes          int  `json:"join_interval_minutes"`
	JoinJitterMinutes            int  `json:"join_jitter_minutes"`
}

type systemSettingsPayload struct {
	Security       systemSecuritySettings       `json:"security"`
	Frequency      systemFrequencySettings      `json:"frequency"`
	ListenerHealth systemListenerHealthSettings `json:"listener_health"`
	Audit          systemAuditSettings          `json:"audit"`
	Adapter        systemAdapterSettings        `json:"adapter"`
	RiskControl    systemRiskControlSettings    `json:"risk_control"`
}

type systemSettingsResponse struct {
	Security       systemSecuritySettings       `json:"security"`
	Frequency      systemFrequencySettings      `json:"frequency"`
	ListenerHealth systemListenerHealthSettings `json:"listener_health"`
	Audit          systemAuditSettings          `json:"audit"`
	Adapter        systemAdapterSettings        `json:"adapter"`
	RiskControl    systemRiskControlSettings    `json:"risk_control"`
	UpdatedAt      time.Time                    `json:"updated_at"`
}

type systemSettingsHistoryItem struct {
	ID        string         `json:"id"`
	Section   string         `json:"section"`
	Summary   string         `json:"summary"`
	Before    map[string]any `json:"before"`
	After     map[string]any `json:"after"`
	ChangedBy string         `json:"changed_by"`
	CreatedAt time.Time      `json:"created_at"`
}

func defaultSystemSettings() systemSettingsPayload {
	return systemSettingsPayload{
		Security: systemSecuritySettings{
			EnforceTenantIsolation: true,
			RequireAdminApproval:   true,
			MaskSensitiveLogs:      true,
		},
		Frequency: systemFrequencySettings{
			MaxConcurrentTasks:     12,
			MaxConcurrentOutreach:  4,
			WSLogBatchSize:         80,
			DashboardRefreshSecond: 30,
		},
		ListenerHealth: systemListenerHealthSettings{
			AutoAccountCheckEnabled:     true,
			AccountCheckIntervalMinutes: 60,
			AutoProxyCheckEnabled:       true,
			ProxyCheckIntervalMinutes:   60,
			SilenceAlertMinutes:         15,
		},
		Audit: systemAuditSettings{
			LogRetentionDays:  30,
			RealtimeLogStream: true,
			NotifyOnFailure:   true,
		},
		Adapter: systemAdapterSettings{
			TelegramSyncEnabled:  true,
			TelegramApplyEnabled: false,
			OutreachDryRun:       true,
			WorkflowDryRun:       true,
		},
		RiskControl: systemRiskControlSettings{
			AutoBypassHighRisk:           true,
			AutoBypassActiveRestrictions: 3,
			AutoBypassFailures24H:        10,
			MessageCooldownMinutes:       60,
			MessageJitterMinutes:         10,
			JoinDailyLimit:               20,
			JoinIntervalMinutes:          120,
			JoinJitterMinutes:            30,
		},
	}
}

func normalizeSystemSettings(input systemSettingsPayload) systemSettingsPayload {
	defaults := defaultSystemSettings()
	listenerHealth := input.ListenerHealth
	if listenerHealth.AccountCheckIntervalMinutes == 0 && listenerHealth.ProxyCheckIntervalMinutes == 0 && listenerHealth.SilenceAlertMinutes == 0 && !listenerHealth.AutoAccountCheckEnabled && !listenerHealth.AutoProxyCheckEnabled {
		listenerHealth = defaults.ListenerHealth
	}
	if listenerHealth.ProxyCheckIntervalMinutes == 0 && !listenerHealth.AutoProxyCheckEnabled {
		listenerHealth.AutoProxyCheckEnabled = defaults.ListenerHealth.AutoProxyCheckEnabled
		listenerHealth.ProxyCheckIntervalMinutes = defaults.ListenerHealth.ProxyCheckIntervalMinutes
	}

	return systemSettingsPayload{
		Security: systemSecuritySettings{
			EnforceTenantIsolation: input.Security.EnforceTenantIsolation,
			RequireAdminApproval:   input.Security.RequireAdminApproval,
			MaskSensitiveLogs:      input.Security.MaskSensitiveLogs,
		},
		Frequency: systemFrequencySettings{
			MaxConcurrentTasks:     clampSettingsInt(input.Frequency.MaxConcurrentTasks, 1, 64, defaults.Frequency.MaxConcurrentTasks),
			MaxConcurrentOutreach:  clampSettingsInt(input.Frequency.MaxConcurrentOutreach, 1, 32, defaults.Frequency.MaxConcurrentOutreach),
			WSLogBatchSize:         clampSettingsInt(input.Frequency.WSLogBatchSize, 20, 200, defaults.Frequency.WSLogBatchSize),
			DashboardRefreshSecond: clampSettingsInt(input.Frequency.DashboardRefreshSecond, 10, 300, defaults.Frequency.DashboardRefreshSecond),
		},
		ListenerHealth: systemListenerHealthSettings{
			AutoAccountCheckEnabled:     listenerHealth.AutoAccountCheckEnabled,
			AccountCheckIntervalMinutes: clampSettingsInt(listenerHealth.AccountCheckIntervalMinutes, 5, 24*60, defaults.ListenerHealth.AccountCheckIntervalMinutes),
			AutoProxyCheckEnabled:       listenerHealth.AutoProxyCheckEnabled,
			ProxyCheckIntervalMinutes:   clampSettingsInt(listenerHealth.ProxyCheckIntervalMinutes, 5, 24*60, defaults.ListenerHealth.ProxyCheckIntervalMinutes),
			SilenceAlertMinutes:         clampSettingsInt(listenerHealth.SilenceAlertMinutes, 1, 24*60, defaults.ListenerHealth.SilenceAlertMinutes),
		},
		Audit: systemAuditSettings{
			LogRetentionDays:  clampSettingsInt(input.Audit.LogRetentionDays, 7, 365, defaults.Audit.LogRetentionDays),
			RealtimeLogStream: input.Audit.RealtimeLogStream,
			NotifyOnFailure:   input.Audit.NotifyOnFailure,
		},
		Adapter: systemAdapterSettings{
			TelegramSyncEnabled:  input.Adapter.TelegramSyncEnabled,
			TelegramApplyEnabled: input.Adapter.TelegramApplyEnabled,
			OutreachDryRun:       input.Adapter.OutreachDryRun,
			WorkflowDryRun:       input.Adapter.WorkflowDryRun,
		},
		RiskControl: systemRiskControlSettings{
			AutoBypassHighRisk:           input.RiskControl.AutoBypassHighRisk,
			AutoBypassActiveRestrictions: clampSettingsInt(input.RiskControl.AutoBypassActiveRestrictions, 1, 20, defaults.RiskControl.AutoBypassActiveRestrictions),
			AutoBypassFailures24H:        clampSettingsInt(input.RiskControl.AutoBypassFailures24H, 1, 200, defaults.RiskControl.AutoBypassFailures24H),
			MessageCooldownMinutes:       clampSettingsInt(input.RiskControl.MessageCooldownMinutes, 1, 24*60, defaults.RiskControl.MessageCooldownMinutes),
			MessageJitterMinutes:         clampSettingsInt(input.RiskControl.MessageJitterMinutes, 0, 120, defaults.RiskControl.MessageJitterMinutes),
			JoinDailyLimit:               clampSettingsInt(input.RiskControl.JoinDailyLimit, 1, 1000, defaults.RiskControl.JoinDailyLimit),
			JoinIntervalMinutes:          clampSettingsInt(input.RiskControl.JoinIntervalMinutes, 1, 24*60, defaults.RiskControl.JoinIntervalMinutes),
			JoinJitterMinutes:            clampSettingsInt(input.RiskControl.JoinJitterMinutes, 0, 240, defaults.RiskControl.JoinJitterMinutes),
		},
	}
}

func clampSettingsInt(value, minValue, maxValue, fallback int) int {
	if value == 0 {
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

func parseSystemSettings(raw datatypes.JSON) systemSettingsPayload {
	settings := defaultSystemSettings()
	if len(raw) == 0 {
		return settings
	}

	var decoded systemSettingsPayload
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return settings
	}
	return normalizeSystemSettings(decoded)
}

func toSystemSettingsResponse(settings systemSettingsPayload, updatedAt time.Time) systemSettingsResponse {
	return systemSettingsResponse{
		Security:       settings.Security,
		Frequency:      settings.Frequency,
		ListenerHealth: settings.ListenerHealth,
		Audit:          settings.Audit,
		Adapter:        settings.Adapter,
		RiskControl:    settings.RiskControl,
		UpdatedAt:      updatedAt,
	}
}

func (s *Server) ensureSystemSettings(ctx context.Context, tenantID uuid.UUID) (models.SystemSetting, systemSettingsPayload, error) {
	var record models.SystemSetting
	err := s.db.WithContext(ctx).First(&record).Error
	if err == nil {
		return record, parseSystemSettings(record.Payload), nil
	}
	if err != gorm.ErrRecordNotFound {
		return models.SystemSetting{}, systemSettingsPayload{}, err
	}

	settings := defaultSystemSettings()
	payload, marshalErr := json.Marshal(settings)
	if marshalErr != nil {
		return models.SystemSetting{}, systemSettingsPayload{}, marshalErr
	}

	record = models.SystemSetting{
		ID:       uuid.New(),
		TenantID: tenantID,
		Payload:  datatypes.JSON(payload),
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return models.SystemSetting{}, systemSettingsPayload{}, err
	}

	return record, settings, nil
}

func (s *Server) readSystemSettings(ctx context.Context, tenantID uuid.UUID) systemSettingsPayload {
	_, settings, err := s.ensureSystemSettings(ctx, uuid.Nil)
	if err != nil {
		return defaultSystemSettings()
	}
	return settings
}

func (s *Server) GetSettings(c *gin.Context) {
	record, settings, err := s.ensureSystemSettings(c.Request.Context(), uuid.Nil)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取系统设置失败")
		return
	}
	utils.OK(c, toSystemSettingsResponse(settings, record.UpdatedAt))
}

func (s *Server) GetSettingsHistory(c *gin.Context) {
	var rows []models.SystemSettingHistory
	if err := s.db.WithContext(c.Request.Context()).
		Where("tenant_id = ?", uuid.Nil).
		Order("created_at desc").
		Limit(20).
		Find(&rows).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取系统设置历史失败")
		return
	}
	usernames := s.loadSettingsHistoryUsernames(c.Request.Context(), uuid.Nil, rows)
	items := make([]systemSettingsHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, systemSettingsHistoryItem{
			ID:        row.ID.String(),
			Section:   row.Section,
			Summary:   row.Summary,
			Before:    parseJSONObject(row.BeforeValue),
			After:     parseJSONObject(row.AfterValue),
			ChangedBy: settingsHistoryChangedBy(row.ChangedBy, usernames),
			CreatedAt: row.CreatedAt,
		})
	}
	utils.OK(c, items)
}

func (s *Server) UpdateSettings(c *gin.Context) {
	var req systemSettingsPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "系统设置格式不正确")
		return
	}

	record, previousSettings, err := s.ensureSystemSettings(c.Request.Context(), uuid.Nil)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取系统设置失败")
		return
	}

	settings := normalizeSystemSettings(req)
	payload, err := json.Marshal(settings)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "序列化系统设置失败")
		return
	}

	if err := s.db.WithContext(c.Request.Context()).
		Model(&models.SystemSetting{}).
		Where("id = ?", record.ID).
		Updates(map[string]any{"payload": datatypes.JSON(payload)}).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "保存系统设置失败")
		return
	}
	s.logSystemSettingsHistory(c.Request.Context(), uuid.Nil, s.userIDPtr(c), previousSettings, settings)

	record.Payload = datatypes.JSON(payload)
	if err := s.db.WithContext(c.Request.Context()).First(&record, "id = ?", record.ID).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取最新系统设置失败")
		return
	}

	utils.OK(c, toSystemSettingsResponse(settings, record.UpdatedAt))
}

func (s *Server) logSystemSettingsHistory(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, before systemSettingsPayload, after systemSettingsPayload) {
	entries := []struct {
		section string
		summary string
		before  any
		after   any
	}{
		{
			section: "risk_control",
			summary: buildRiskControlChangeSummary(before.RiskControl, after.RiskControl),
			before:  before.RiskControl,
			after:   after.RiskControl,
		},
		{
			section: "listener_health",
			summary: buildListenerHealthChangeSummary(before.ListenerHealth, after.ListenerHealth),
			before:  before.ListenerHealth,
			after:   after.ListenerHealth,
		},
	}
	for _, entry := range entries {
		if entry.summary == "" {
			continue
		}
		beforeJSON, _ := json.Marshal(entry.before)
		afterJSON, _ := json.Marshal(entry.after)
		_ = s.db.WithContext(ctx).Create(&models.SystemSettingHistory{
			ID:          uuid.New(),
			TenantID:    tenantID,
			ChangedBy:   userID,
			Section:     entry.section,
			Summary:     entry.summary,
			BeforeValue: datatypes.JSON(beforeJSON),
			AfterValue:  datatypes.JSON(afterJSON),
		}).Error
	}
}

func buildRiskControlChangeSummary(before systemRiskControlSettings, after systemRiskControlSettings) string {
	if before == after {
		return ""
	}
	switch {
	case !before.AutoBypassHighRisk && after.AutoBypassHighRisk:
		return "开启高风险自动避让"
	case before.AutoBypassHighRisk && !after.AutoBypassHighRisk:
		return "关闭高风险自动避让"
	default:
		return "更新风控避让阈值"
	}
}

func buildListenerHealthChangeSummary(before systemListenerHealthSettings, after systemListenerHealthSettings) string {
	if before == after {
		return ""
	}
	switch {
	case !before.AutoAccountCheckEnabled && after.AutoAccountCheckEnabled:
		return "开启监听账号定时检测"
	case before.AutoAccountCheckEnabled && !after.AutoAccountCheckEnabled:
		return "关闭监听账号定时检测"
	case !before.AutoProxyCheckEnabled && after.AutoProxyCheckEnabled:
		return "开启代理列表定时检测"
	case before.AutoProxyCheckEnabled && !after.AutoProxyCheckEnabled:
		return "关闭代理列表定时检测"
	default:
		return "更新监听健康检测策略"
	}
}

func parseJSONObject(raw datatypes.JSON) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func (s *Server) loadSettingsHistoryUsernames(ctx context.Context, tenantID uuid.UUID, rows []models.SystemSettingHistory) map[uuid.UUID]string {
	ids := make([]uuid.UUID, 0, len(rows))
	seen := make(map[uuid.UUID]struct{}, len(rows))
	for _, row := range rows {
		if row.ChangedBy == nil {
			continue
		}
		if _, ok := seen[*row.ChangedBy]; ok {
			continue
		}
		seen[*row.ChangedBy] = struct{}{}
		ids = append(ids, *row.ChangedBy)
	}
	if len(ids) == 0 {
		return map[uuid.UUID]string{}
	}

	var users []models.User
	if err := s.db.WithContext(ctx).
		Select("id", "username").
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Find(&users).Error; err != nil {
		return map[uuid.UUID]string{}
	}

	result := make(map[uuid.UUID]string, len(users))
	for _, user := range users {
		result[user.ID] = user.Username
	}
	return result
}

func settingsHistoryChangedBy(value *uuid.UUID, usernames map[uuid.UUID]string) string {
	if value == nil {
		return ""
	}
	if username := strings.TrimSpace(usernames[*value]); username != "" {
		return username
	}
	return value.String()
}
