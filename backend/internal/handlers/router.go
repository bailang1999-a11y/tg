package handlers

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"codex3/backend/internal/config"
	"codex3/backend/internal/middleware"
	"codex3/backend/internal/services"
	"codex3/backend/internal/taskqueue"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	cfg            config.Config
	db             *gorm.DB
	auth           *services.AuthService
	taskPublisher  *taskqueue.Publisher
	wsConnections  atomic.Int64
	dashboardCache sync.Map
	logStreamCache sync.Map
	listenerMu     sync.Mutex
	listeners      map[string]*scrmListenerRuntime
	botPollMu      sync.Mutex
	botPollers     map[string]*botPollRuntime
}

func NewRouter(cfg config.Config, db *gorm.DB, auth *services.AuthService) *gin.Engine {
	return NewRouterWithTaskQueue(cfg, db, auth, nil)
}

func NewServer(cfg config.Config, db *gorm.DB, auth *services.AuthService, publisher *taskqueue.Publisher) *Server {
	return &Server{
		cfg:           cfg,
		db:            db,
		auth:          auth,
		taskPublisher: publisher,
		listeners:     map[string]*scrmListenerRuntime{},
		botPollers:    map[string]*botPollRuntime{},
	}
}

func NewRouterWithTaskQueue(cfg config.Config, db *gorm.DB, auth *services.AuthService, publisher *taskqueue.Publisher) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	s := NewServer(cfg, db, auth, publisher)
	go s.resumeBotPollersOnStartup()

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), s.inFlightLimit(), s.cors())
	r.GET("/storage/uploads/*filepath", s.PublicUpload)
	r.GET("/health", s.Health)
	r.GET("/ready", s.Ready)

	api := r.Group("/api/v1")
	api.POST("/auth/login", s.Login)
	api.POST("/bot/webhook/:secret", s.BotWebhook)

	protected := api.Group("")
	protected.Use(middleware.Auth(auth))
	protected.GET("/me", s.Me)
	protected.GET("/dashboard", s.Dashboard)
	protected.GET("/ws/logs", s.LogStream)
	protected.GET("/ops/metrics", middleware.RequireAdmin(), s.OpsMetrics)

	protected.GET("/users", middleware.RequireAdmin(), s.ListUsers)
	protected.POST("/users", middleware.RequireAdmin(), s.CreateUser)
	protected.PUT("/users/:id/telegram", middleware.RequireAdmin(), s.BindUserTelegram)
	protected.PUT("/users/:id/status", middleware.RequireAdmin(), s.UpdateUserStatus)
	protected.DELETE("/users/:id", middleware.RequireAdmin(), s.DeleteUser)

	protected.GET("/groups/:resource", s.ListGroups)
	protected.POST("/groups/:resource", s.CreateGroup)
	protected.PUT("/groups/:resource/:id", s.RenameGroup)
	protected.DELETE("/groups/:resource/:id", s.DeleteGroup)

	protected.GET("/terminals", s.ListTerminals)
	protected.GET("/terminals/risk-board", s.ListTerminalRiskBoard)
	protected.POST("/terminals/batch", middleware.RequireAdmin(), s.BatchTerminalOperation)
	protected.POST("/terminals/check", s.CreateCheckTerminalsTask)
	protected.PUT("/terminals/:id/limits", middleware.RequireAdmin(), s.UpdateTerminalLimits)
	protected.PUT("/terminals/:id/cooldown/clear", middleware.RequireAdmin(), s.ClearTerminalCooldown)
	protected.GET("/terminals/:id/risk-stats", s.GetTerminalRiskStats)
	protected.GET("/terminals/:id/restrictions", s.ListTerminalRestrictions)
	protected.POST("/terminals/:id/restrictions/clear", middleware.RequireAdmin(), s.ClearTerminalRestrictions)
	protected.DELETE("/terminals/:id/restrictions/:restriction_id", middleware.RequireAdmin(), s.DeleteTerminalRestriction)
	protected.DELETE("/terminals/:id", s.DeleteTerminal)

	protected.POST("/imports", s.CreateImportTask)
	protected.POST("/imports/session", s.CreateSessionImportTask)
	protected.POST("/imports/tdata", s.CreateTDataImportTask)
	protected.GET("/network-nodes", s.ListNetworkNodes)
	protected.POST("/network-nodes/import", s.ImportNetworkNodes)
	protected.POST("/network-nodes/test", s.CreateNetworkTestTask)
	protected.GET("/targets", s.ListTargets)
	protected.POST("/targets/import", s.ImportTargets)
	protected.POST("/targets/import-terminals", s.ImportTerminalTargets)
	protected.POST("/targets/join", s.CreateJoinTargetsTask)
	protected.POST("/targets/memberships/refresh", middleware.RequireAdmin(), s.CreateRefreshTargetMembershipsTask)
	protected.GET("/targets/:id/memberships", s.ListTargetMemberships)
	protected.GET("/assets", s.ListAssets)
	protected.POST("/assets/upload", s.UploadAssets)
	protected.POST("/assets/workflow-media", s.UploadWorkflowMedia)
	protected.DELETE("/assets/:id", s.DeleteAsset)
	protected.GET("/workflows", s.ListWorkflows)
	protected.POST("/workflows", s.CreateWorkflow)
	protected.POST("/workflows/:id/run", s.RunWorkflow)
	protected.POST("/profiles/modify", s.CreateProfileTask)
	protected.POST("/outreach/jobs", s.CreateOutreachTask)
	protected.POST("/mass-messaging/jobs", s.CreateMassMessagingTask)
	protected.POST("/direct-messages/jobs", s.CreateDirectMessageTask)
	protected.GET("/settings", s.GetSettings)
	protected.GET("/settings/history", middleware.RequireAdmin(), s.GetSettingsHistory)
	protected.PUT("/settings", middleware.RequireAdmin(), s.UpdateSettings)
	protected.GET("/system/version", middleware.RequireAdmin(), s.GetSystemVersion)
	protected.POST("/system/update", middleware.RequireAdmin(), s.StartSystemUpdate)

	protected.GET("/bot/config", middleware.RequireAdmin(), s.GetBotConfig)
	protected.PUT("/bot/config", middleware.RequireAdmin(), s.UpdateBotConfig)
	protected.POST("/bot/test", middleware.RequireAdmin(), s.TestBotConfig)
	protected.POST("/bot/start", middleware.RequireAdmin(), s.StartBotPush)
	protected.POST("/bot/stop", middleware.RequireAdmin(), s.StopBotPush)
	protected.POST("/bot/commands/sync", middleware.RequireAdmin(), s.SyncBotCommands)
	protected.POST("/bot/webhook/setup", middleware.RequireAdmin(), s.SetupBotWebhook)
	protected.POST("/bot/webhook/clear", middleware.RequireAdmin(), s.ClearBotWebhook)
	protected.GET("/bot/webhook/status", middleware.RequireAdmin(), s.GetBotWebhookStatus)
	protected.GET("/bot/polling", middleware.RequireAdmin(), s.GetBotPollingStatus)
	protected.POST("/bot/polling/start", middleware.RequireAdmin(), s.StartBotPolling)
	protected.POST("/bot/polling/stop", middleware.RequireAdmin(), s.StopBotPolling)
	protected.GET("/bot/licenses", middleware.RequireAdmin(), s.ListBotLicenses)
	protected.POST("/bot/licenses", middleware.RequireAdmin(), s.CreateBotLicenses)
	protected.PUT("/bot/licenses/:id/status", middleware.RequireAdmin(), s.UpdateBotLicenseStatus)
	protected.DELETE("/bot/licenses/:id", middleware.RequireAdmin(), s.DeleteBotLicense)
	protected.GET("/bot/subscribers", middleware.RequireAdmin(), s.ListBotSubscribers)
	protected.GET("/bot-users", middleware.RequireAdmin(), s.ListBotUserDashboard)
	protected.GET("/bot-users/:id", middleware.RequireAdmin(), s.GetBotUserDashboard)
	protected.PUT("/bot-users/:id", middleware.RequireAdmin(), s.UpdateBotUserDashboard)
	protected.GET("/listener-admin/overview", middleware.RequireAdmin(), s.GetListenerAdminOverview)
	protected.GET("/listener-admin/accounts", middleware.RequireAdmin(), s.ListListenerAccounts)
	protected.POST("/listener-admin/accounts/import", middleware.RequireAdmin(), s.ImportListenerAccounts)
	protected.POST("/listener-admin/accounts/import-files", middleware.RequireAdmin(), s.ImportListenerAccountsFromFiles)
	protected.POST("/listener-admin/accounts/check", middleware.RequireAdmin(), s.CheckListenerAccounts)
	protected.DELETE("/listener-admin/accounts/abnormal", middleware.RequireAdmin(), s.DeleteAbnormalListenerAccounts)
	protected.DELETE("/listener-admin/accounts/:id", middleware.RequireAdmin(), s.DeleteListenerAccount)
	protected.GET("/listener-admin/targets", middleware.RequireAdmin(), s.ListListenerTargets)
	protected.POST("/listener-admin/targets/refresh", middleware.RequireAdmin(), s.RefreshListenerTargets)
	protected.POST("/listener-admin/targets/import", middleware.RequireAdmin(), s.ImportListenerTargets)
	protected.DELETE("/listener-admin/targets/:id", middleware.RequireAdmin(), s.DeleteListenerTarget)
	protected.GET("/listener-admin/proxies", middleware.RequireAdmin(), s.ListListenerProxies)
	protected.POST("/listener-admin/proxies/check", middleware.RequireAdmin(), s.CheckListenerProxies)
	protected.POST("/listener-admin/proxies/import", middleware.RequireAdmin(), s.ImportListenerProxies)
	protected.POST("/listener-admin/proxies/assign", middleware.RequireAdmin(), s.AssignListenerProxies)
	protected.DELETE("/listener-admin/proxies/:id", middleware.RequireAdmin(), s.DeleteListenerProxy)

	// SCRM Endpoints
	protected.GET("/scrm/rules", s.ListSCRMRules)
	protected.POST("/scrm/rules", s.CreateSCRMRule)
	protected.DELETE("/scrm/rules/:id", s.DeleteSCRMRule)
	protected.GET("/scrm/leads", s.ListSCRMLeads)
	protected.POST("/scrm/leads/:lead_id/blacklist", s.BlacklistSCRMLeadUser)
	protected.GET("/scrm/messages/:lead_id", s.ListSCRMMessages)
	protected.POST("/scrm/messages/:lead_id", s.SendSCRMMessage)
	protected.GET("/scrm/listener", s.GetSCRMListenerStatus)
	protected.POST("/scrm/listener/start", s.StartSCRMListener)
	protected.POST("/scrm/listener/stop", s.StopSCRMListener)
	protected.POST("/scrm/rules/:id/start", s.StartSCRMListenerRule)
	protected.POST("/scrm/rules/:id/pause", s.PauseSCRMListenerRule)

	protected.GET("/tasks", s.ListTasks)
	protected.POST("/tasks/refresh", s.RefreshTasks)
	protected.POST("/tasks", s.CreateTask)
	protected.PUT("/tasks/batch", s.BatchTaskAction)
	protected.DELETE("/tasks/batch", s.BatchDeleteTasks)
	protected.PUT("/tasks/:id/:action", s.UpdateTaskAction)
	protected.DELETE("/tasks/:id", s.DeleteTask)
	protected.GET("/tasks/:id/logs", s.ListTaskLogs)
	protected.GET("/logs", s.ListLogs)
	protected.DELETE("/logs", s.ClearLogs)

	return r
}

func (s *Server) inFlightLimit() gin.HandlerFunc {
	limit := s.cfg.HTTPMaxInFlight
	if limit <= 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	sem := make(chan struct{}, limit)
	return func(c *gin.Context) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			c.Next()
		default:
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "服务繁忙，请稍后重试",
			})
		}
	}
}

func (s *Server) cors() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range s.cfg.CORSOrigins {
		allowed[origin] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if strings.EqualFold(c.Request.Method, http.MethodOptions) {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
