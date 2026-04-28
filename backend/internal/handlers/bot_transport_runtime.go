package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) SetupBotWebhook(c *gin.Context) {
	var input struct {
		WebhookURL string `json:"webhook_url"`
	}
	_ = c.ShouldBindJSON(&input)

	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	if strings.TrimSpace(config.Token) == "" {
		utils.Fail(c, http.StatusBadRequest, "请先填写 Bot Token")
		return
	}
	if strings.TrimSpace(config.WebhookSecret) == "" {
		config.WebhookSecret = randomCode(24)
	}

	webhookURL := strings.TrimSpace(input.WebhookURL)
	if webhookURL == "" {
		webhookURL = strings.TrimSpace(config.WebhookURL)
	}
	if webhookURL == "" {
		utils.Fail(c, http.StatusBadRequest, "请填写公网 Webhook 地址")
		return
	}
	webhookURL = normalizeBotWebhookURL(webhookURL, config.WebhookSecret)
	if err := validateBotWebhookURL(webhookURL); err != nil {
		config.WebhookURL = webhookURL
		config.LastWebhookStatus = "failed"
		config.LastWebhookMessage = err.Error()
		_ = s.db.WithContext(c.Request.Context()).Save(&config).Error
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := probeBotWebhookURL(webhookURL); err != nil {
		now := time.Now()
		config.WebhookURL = webhookURL
		config.LastWebhookAt = &now
		config.LastWebhookStatus = "failed"
		config.LastWebhookMessage = "Webhook 连接不成功：" + err.Error()
		_ = s.db.WithContext(c.Request.Context()).Save(&config).Error
		utils.Fail(c, http.StatusBadRequest, config.LastWebhookMessage)
		return
	}

	message, err := setTelegramWebhook(config.Token, webhookURL)
	now := time.Now()
	config.WebhookURL = webhookURL
	config.Enabled = true
	config.LastWebhookAt = &now
	config.LastWebhookStatus = "success"
	config.LastWebhookMessage = "Webhook 连接成功：" + message
	if err != nil {
		config.LastWebhookStatus = "failed"
		config.LastWebhookMessage = err.Error()
		_ = s.db.WithContext(c.Request.Context()).Save(&config).Error
		utils.Fail(c, http.StatusBadRequest, "设置 Webhook 失败："+err.Error())
		return
	}
	config.UpdatedAt = now
	if err := s.db.WithContext(c.Request.Context()).Save(&config).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "保存 Webhook 状态失败")
		return
	}
	utils.OK(c, gin.H{"status": "success", "connected": true, "webhook_url": webhookURL, "message": config.LastWebhookMessage, "config": config})
}

func validateBotWebhookURL(webhookURL string) error {
	parsed, err := url.Parse(webhookURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("Webhook 地址格式不正确，请填写完整的 HTTPS 公网地址")
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("Telegram Webhook 必须使用 HTTPS 公网地址；当前地址是 %s。没有 HTTPS 域名时，请使用本地轮询模式", parsed.Scheme)
	}
	return nil
}

func probeBotWebhookURL(webhookURL string) error {
	body, _ := json.Marshal(gin.H{})
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("请求地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("无法访问回调地址：%s", err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("回调地址返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) GetBotWebhookStatus(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	if strings.TrimSpace(config.Token) == "" {
		utils.Fail(c, http.StatusBadRequest, "请先填写 Bot Token")
		return
	}
	info, err := getTelegramWebhookInfo(config.Token)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "读取 Webhook 状态失败："+err.Error())
		return
	}
	status := botWebhookConnectionStatus(config, info)
	if statusURL, _ := status["webhook_url"].(string); strings.TrimSpace(statusURL) != "" {
		if err := probeBotWebhookURL(statusURL); err != nil {
			status["connected"] = false
			status["status"] = "failed"
			status["message"] = "Webhook 连接不成功：" + err.Error()
		}
	}
	status["telegram"] = info
	utils.OK(c, status)
}

func botWebhookConnectionStatus(config models.BotConfig, info map[string]any) gin.H {
	result, _ := info["result"].(map[string]any)
	telegramURL, _ := result["url"].(string)
	webhookURL := firstNonEmpty(strings.TrimSpace(telegramURL), strings.TrimSpace(config.WebhookURL))
	pendingCount, _ := result["pending_update_count"].(float64)
	lastError, _ := result["last_error_message"].(string)
	if webhookURL == "" {
		return gin.H{
			"connected":            false,
			"status":               "not_configured",
			"message":              "Webhook 未配置",
			"webhook_url":          "",
			"pending_update_count": int(pendingCount),
		}
	}
	if strings.TrimSpace(lastError) != "" {
		return gin.H{
			"connected":            false,
			"status":               "failed",
			"message":              "Webhook 连接不成功：" + lastError,
			"webhook_url":          webhookURL,
			"pending_update_count": int(pendingCount),
		}
	}
	if strings.TrimSpace(config.WebhookURL) != "" && strings.TrimSpace(telegramURL) != "" && strings.TrimSpace(config.WebhookURL) != strings.TrimSpace(telegramURL) {
		return gin.H{
			"connected":            false,
			"status":               "mismatch",
			"message":              "Webhook 连接不成功：Telegram 当前地址与后台保存地址不一致",
			"webhook_url":          webhookURL,
			"pending_update_count": int(pendingCount),
		}
	}
	return gin.H{
		"connected":            true,
		"status":               "success",
		"message":              "Webhook 连接成功",
		"webhook_url":          webhookURL,
		"pending_update_count": int(pendingCount),
	}
}

func (s *Server) ClearBotWebhook(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	if strings.TrimSpace(config.Token) == "" {
		utils.Fail(c, http.StatusBadRequest, "请先填写 Bot Token")
		return
	}
	message, err := deleteTelegramWebhook(config.Token, true)
	now := time.Now()
	config.WebhookURL = ""
	config.LastWebhookAt = &now
	config.LastWebhookStatus = "success"
	config.LastWebhookMessage = message
	if err != nil {
		config.LastWebhookStatus = "failed"
		config.LastWebhookMessage = err.Error()
		_ = s.db.WithContext(c.Request.Context()).Save(&config).Error
		utils.Fail(c, http.StatusBadRequest, "关闭 Webhook 失败："+err.Error())
		return
	}
	config.UpdatedAt = now
	if err := s.db.WithContext(c.Request.Context()).Save(&config).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "保存 Webhook 状态失败")
		return
	}
	utils.OK(c, gin.H{"status": "cleared", "message": message, "config": config})
}

func (s *Server) GetBotPollingStatus(c *gin.Context) {
	tenantKey := s.tenantID(c).String()
	s.botPollMu.Lock()
	runtime := s.botPollers[tenantKey]
	s.botPollMu.Unlock()
	if runtime == nil {
		utils.OK(c, gin.H{"running": false})
		return
	}
	lastError, _ := runtime.lastError.Load().(string)
	lastMessageAt := ""
	if unix := runtime.lastMessageAtUnix.Load(); unix > 0 {
		lastMessageAt = time.Unix(unix, 0).Format(time.RFC3339)
	}
	utils.OK(c, gin.H{
		"running":         true,
		"started_at":      runtime.startedAt.Format(time.RFC3339),
		"last_update_id":  runtime.lastUpdateID.Load(),
		"handled_count":   runtime.handledCount.Load(),
		"last_error":      lastError,
		"last_message_at": lastMessageAt,
	})
}

func (s *Server) StartBotPolling(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	if strings.TrimSpace(config.Token) == "" {
		utils.Fail(c, http.StatusBadRequest, "请先填写 Bot Token")
		return
	}
	if info, err := getTelegramWebhookInfo(config.Token); err == nil {
		if result, ok := info["result"].(map[string]any); ok {
			if url, _ := result["url"].(string); strings.TrimSpace(url) != "" {
				if _, err := deleteTelegramWebhook(config.Token, false); err != nil {
					utils.Fail(c, http.StatusBadRequest, "启动本地轮询前关闭 Webhook 失败："+err.Error())
					return
				}
				now := time.Now()
				config.LastWebhookAt = &now
				config.LastWebhookStatus = "paused_for_polling"
				config.LastWebhookMessage = "已临时关闭 Webhook 并保留待处理消息，本地轮询接管指令"
			}
		}
	}

	config.Enabled = true
	config.Running = true
	config.UpdatedAt = time.Now()
	_ = s.db.WithContext(c.Request.Context()).Save(&config).Error

	ctx, runtime := s.startBotPollingRuntime(config)
	go s.runBotPolling(ctx, config, runtime)
	utils.OK(c, gin.H{"status": "running", "started_at": runtime.startedAt.Format(time.RFC3339)})
}

func (s *Server) startBotPollingRuntime(config models.BotConfig) (context.Context, *botPollRuntime) {
	tenantKey := config.TenantID.String()
	s.botPollMu.Lock()
	if existing := s.botPollers[tenantKey]; existing != nil {
		existing.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime := &botPollRuntime{
		cancel:    cancel,
		startedAt: time.Now(),
	}
	s.botPollers[tenantKey] = runtime
	s.botPollMu.Unlock()
	return ctx, runtime
}

func (s *Server) resumeBotPollersOnStartup() {
	var configs []models.BotConfig
	if err := s.db.Where("enabled = ? AND running = ?", true, true).Find(&configs).Error; err != nil {
		log.Printf("resume bot polling skipped: load configs failed: %v", err)
		return
	}
	for _, cfg := range configs {
		if strings.TrimSpace(cfg.Token) == "" {
			continue
		}
		if cfg.LastWebhookStatus != "paused_for_polling" && strings.TrimSpace(cfg.WebhookURL) != "" {
			continue
		}
		ctx, runtime := s.startBotPollingRuntime(cfg)
		go s.runBotPolling(ctx, cfg, runtime)
		log.Printf("bot polling resumed on startup tenant=%s started_at=%s", cfg.TenantID, runtime.startedAt.Format(time.RFC3339))
	}
}

func (s *Server) StopBotPolling(c *gin.Context) {
	tenantKey := s.tenantID(c).String()
	s.botPollMu.Lock()
	runtime := s.botPollers[tenantKey]
	if runtime != nil {
		runtime.cancel()
		delete(s.botPollers, tenantKey)
	}
	s.botPollMu.Unlock()
	if config, err := s.ensureBotConfig(c); err == nil {
		config.Running = false
		config.UpdatedAt = time.Now()
		_ = s.db.WithContext(c.Request.Context()).Save(&config).Error
	}
	utils.OK(c, gin.H{"status": "stopped"})
}

func (s *Server) BotWebhook(c *gin.Context) {
	secret := strings.TrimSpace(c.Param("secret"))
	if secret == "" {
		c.Status(http.StatusNotFound)
		return
	}

	var config models.BotConfig
	if err := s.db.WithContext(c.Request.Context()).Where("webhook_secret = ?", secret).First(&config).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	var update telegramUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		log.Printf("bot webhook ignored: invalid update tenant=%s err=%v", config.TenantID, err)
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": "invalid update"})
		return
	}
	if strings.TrimSpace(update.Message.Text) == "" && strings.TrimSpace(update.Message.Document.FileID) == "" && strings.TrimSpace(update.CallbackQuery.Data) == "" {
		log.Printf("bot webhook ignored: empty message tenant=%s update_id=%d", config.TenantID, update.UpdateID)
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": "empty message"})
		return
	}

	chatID := updateChatID(update)
	userID := updateUserID(update)
	if !config.Enabled {
		log.Printf("bot webhook ignored: bot disabled tenant=%s update_id=%d user=%s chat=%s", config.TenantID, update.UpdateID, userID, chatID)
		if strings.TrimSpace(config.Token) != "" && chatID != "0" {
			_ = sendTelegramBotMessage(config.Token, chatID, "Bot 服务还没有启用，请管理员在后台 Bot 配置里启用后再试。")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": "bot disabled"})
		return
	}

	if err := s.handleBotUpdate(c.Request.Context(), config, update); err != nil {
		safeErr := redactTelegramBotError(config.Token, err.Error())
		log.Printf("bot webhook command failed: tenant=%s update_id=%d user=%s chat=%s err=%s", config.TenantID, update.UpdateID, userID, chatID, safeErr)
		_ = sendTelegramBotMessage(config.Token, chatID, "指令处理失败："+safeErr)
	} else {
		log.Printf("bot webhook handled: tenant=%s update_id=%d user=%s chat=%s", config.TenantID, update.UpdateID, userID, chatID)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) runBotPolling(ctx context.Context, config models.BotConfig, runtime *botPollRuntime) {
	defer func() {
		s.botPollMu.Lock()
		if current := s.botPollers[config.TenantID.String()]; current == runtime {
			delete(s.botPollers, config.TenantID.String())
		}
		s.botPollMu.Unlock()
	}()

	offset := int64(0)
	runtimeConfig := config
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if fresh, err := s.loadBotConfigByTenant(ctx, runtimeConfig.TenantID); err == nil {
			runtimeConfig = fresh
		} else {
			runtime.lastError.Store("刷新 Bot 配置失败：" + err.Error())
		}
		if strings.TrimSpace(runtimeConfig.Token) == "" {
			runtime.lastError.Store("Bot Token 为空，轮询暂停等待配置")
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
		}

		updates, err := getTelegramUpdates(runtimeConfig.Token, offset)
		if err != nil {
			runtime.lastError.Store(err.Error())
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
		}
		runtime.lastError.Store("")
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
				runtime.lastUpdateID.Store(update.UpdateID)
			}
			if strings.TrimSpace(update.Message.Text) == "" && strings.TrimSpace(update.Message.Document.FileID) == "" && strings.TrimSpace(update.CallbackQuery.Data) == "" {
				continue
			}
			chatID := updateChatID(update)
			userID := updateUserID(update)
			if err := s.handleBotUpdate(context.Background(), runtimeConfig, update); err != nil {
				safeErr := redactTelegramBotError(runtimeConfig.Token, err.Error())
				runtime.lastError.Store(safeErr)
				log.Printf("bot polling command failed: tenant=%s update_id=%d user=%s chat=%s err=%s", runtimeConfig.TenantID, update.UpdateID, userID, chatID, safeErr)
				if strings.TrimSpace(runtimeConfig.Token) != "" && chatID != "0" {
					_ = sendTelegramBotMessage(runtimeConfig.Token, chatID, "指令处理失败："+safeErr)
				}
			} else {
				log.Printf("bot polling handled: tenant=%s update_id=%d user=%s chat=%s", runtimeConfig.TenantID, update.UpdateID, userID, chatID)
			}
			runtime.handledCount.Add(1)
			runtime.lastMessageAtUnix.Store(time.Now().Unix())
		}
	}
}

func (s *Server) loadBotConfigByTenant(ctx context.Context, tenantID uuid.UUID) (models.BotConfig, error) {
	var config models.BotConfig
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&config).Error
	return config, err
}
