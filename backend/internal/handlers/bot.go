package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Server) GetBotConfig(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	utils.OK(c, config)
}

func (s *Server) UpdateBotConfig(c *gin.Context) {
	var input botConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Bot 配置参数无效")
		return
	}

	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	before := config

	trialHours := input.TrialHours
	if trialHours <= 0 {
		trialHours = 5
	}
	features := normalizeBotFeatures(input.TrialFeatures)
	commands := ensureRequiredBotPolicyCommands(input.EnabledCommands)
	commandLabels := normalizeBotCommandLabels(input.CommandLabels)
	keywords := collectSCRMKeywords(input.DefaultKeywords, strings.Join(input.DefaultKeywords, "\n"))
	featuresJSON, _ := json.Marshal(features)
	commandsJSON, _ := json.Marshal(commands)
	commandLabelsJSON, _ := json.Marshal(commandLabels)
	keywordsJSON, _ := json.Marshal(keywords)
	buttonLabels := normalizeBotButtonLabels(input.ButtonLabels)
	replyTemplates := normalizeBotReplyTemplates(input.ReplyTemplates)
	defaultDMMessages := normalizeBotDMMessages(input.DefaultDMMessages)
	buttonLabelsJSON, _ := json.Marshal(buttonLabels)
	replyTemplatesJSON, _ := json.Marshal(replyTemplates)
	defaultDMMessagesJSON, _ := json.Marshal(defaultDMMessages)
	minDelay, maxDelay := normalizeBotDMDelay(input.DMMinDelaySeconds, input.DMMaxDelaySeconds)
	maxMessages := normalizeBotDMMaxMessages(input.DMMaxMessages)
	privateTerminalIDs, err := s.resolveSCRMMonitorTerminalIDs(c, input.PrivateTerminalIDs)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	privateTerminalJSON, _ := json.Marshal(privateTerminalIDs)

	config.Name = strings.TrimSpace(input.Name)
	config.Token = strings.TrimSpace(input.Token)
	config.PushChatID = strings.TrimSpace(input.PushChatID)
	config.AdminChatID = strings.TrimSpace(input.AdminChatID)
	config.AdminContact = strings.TrimSpace(input.AdminContact)
	config.Enabled = input.Enabled
	config.ForceJoinEnabled = input.ForceJoinEnabled
	config.ForceJoinURL = strings.TrimSpace(input.ForceJoinURL)
	config.ForceJoinHandle = normalizeTelegramPublicHandle(input.ForceJoinURL)
	config.TrialEnabled = input.TrialEnabled
	config.TrialHours = trialHours
	config.TrialFeatures = datatypes.JSON(featuresJSON)
	config.EnabledCommands = datatypes.JSON(commandsJSON)
	config.CommandLabels = datatypes.JSON(commandLabelsJSON)
	config.DefaultKeywords = datatypes.JSON(keywordsJSON)
	config.DefaultKeywordLimit = maxInt(input.DefaultKeywordLimit, 20)
	config.DefaultMatchMode = normalizeSCRMMatchMode(input.DefaultMatchMode)
	config.PrivateTerminalIDs = datatypes.JSON(privateTerminalJSON)
	config.WelcomeTitle = strings.TrimSpace(input.WelcomeTitle)
	config.ServiceOverview = strings.TrimSpace(input.ServiceOverview)
	config.QuickStartText = strings.TrimSpace(input.QuickStartText)
	config.FAQText = strings.TrimSpace(input.FAQText)
	config.SupportText = strings.TrimSpace(input.SupportText)
	config.MenuInfoLabel = normalizeBotMenuLabel(input.MenuInfoLabel, "📋 我的信息")
	config.MenuSettingsLabel = normalizeBotMenuLabel(input.MenuSettingsLabel, "⚙️ 设置中心")
	config.MenuFAQLabel = normalizeBotMenuLabel(input.MenuFAQLabel, "❓ 常见问题")
	config.MenuSupportLabel = normalizeBotMenuLabel(input.MenuSupportLabel, "💬 在线客服")
	config.MenuPlaceholder = firstNonEmpty(strings.TrimSpace(input.MenuPlaceholder), "选择功能或输入命令...")
	config.ButtonLabels = datatypes.JSON(buttonLabelsJSON)
	config.ReplyTemplates = datatypes.JSON(replyTemplatesJSON)
	config.DefaultDMMessages = datatypes.JSON(defaultDMMessagesJSON)
	config.DMMinDelaySeconds = minDelay
	config.DMMaxDelaySeconds = maxDelay
	config.DMMaxMessages = maxMessages
	if strings.TrimSpace(input.WebhookURL) != "" {
		config.WebhookURL = strings.TrimSpace(input.WebhookURL)
	}
	if strings.TrimSpace(config.WebhookSecret) == "" {
		config.WebhookSecret = randomCode(24)
	}
	config.UpdatedAt = time.Now()

	if err := s.db.WithContext(c.Request.Context()).Save(&config).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "保存 Bot 配置失败")
		return
	}
	s.logBotConfigUpdate(c, before, config)
	utils.OK(c, config)
}

func (s *Server) logBotConfigUpdate(c *gin.Context, before models.BotConfig, after models.BotConfig) {
	now := time.Now()
	changes := botConfigChangeSummary(before, after)
	detail := "Bot 配置已保存"
	if changes != "" {
		detail += "：" + changes
	}
	payload, _ := json.Marshal(map[string]any{
		"bot_config_id": after.ID.String(),
		"changes":       changes,
	})
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  after.TenantID,
		Name:      "Bot 配置修改",
		Type:      "bot_config",
		Status:    "completed",
		Progress:  100,
		Payload:   datatypes.JSON(payload),
		CreatedBy: s.userIDPtr(c),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		return
	}
	_ = s.db.WithContext(c.Request.Context()).Create(&models.TaskLog{
		ID:        uuid.New(),
		TenantID:  after.TenantID,
		TaskID:    task.ID,
		Level:     "INFO",
		Category:  "bot_config",
		Action:    "update_bot_config",
		Details:   detail,
		CreatedAt: now,
	}).Error
}

func botConfigChangeSummary(before models.BotConfig, after models.BotConfig) string {
	var changes []string
	add := func(label string, oldValue string, newValue string) {
		if strings.TrimSpace(oldValue) != strings.TrimSpace(newValue) {
			changes = append(changes, label)
		}
	}
	addBool := func(label string, oldValue bool, newValue bool) {
		if oldValue != newValue {
			changes = append(changes, label)
		}
	}
	addInt := func(label string, oldValue int, newValue int) {
		if oldValue != newValue {
			changes = append(changes, label)
		}
	}
	addJSON := func(label string, oldValue datatypes.JSON, newValue datatypes.JSON) {
		if string(oldValue) != string(newValue) {
			changes = append(changes, label)
		}
	}
	add("Bot 名称", before.Name, after.Name)
	if strings.TrimSpace(before.Token) != strings.TrimSpace(after.Token) {
		changes = append(changes, "Bot Token")
	}
	add("推送 Chat ID", before.PushChatID, after.PushChatID)
	add("管理员 Chat ID", before.AdminChatID, after.AdminChatID)
	add("客服联系方式", before.AdminContact, after.AdminContact)
	addBool("启用状态", before.Enabled, after.Enabled)
	addBool("强制进群", before.ForceJoinEnabled, after.ForceJoinEnabled)
	add("强制进群链接", before.ForceJoinURL, after.ForceJoinURL)
	addBool("试用开关", before.TrialEnabled, after.TrialEnabled)
	addInt("试用时长", before.TrialHours, after.TrialHours)
	addJSON("试用功能", before.TrialFeatures, after.TrialFeatures)
	addJSON("启用指令", before.EnabledCommands, after.EnabledCommands)
	addJSON("指令文案", before.CommandLabels, after.CommandLabels)
	addJSON("默认关键词", before.DefaultKeywords, after.DefaultKeywords)
	addInt("默认关键词上限", before.DefaultKeywordLimit, after.DefaultKeywordLimit)
	add("默认匹配模式", before.DefaultMatchMode, after.DefaultMatchMode)
	addJSON("私信账号", before.PrivateTerminalIDs, after.PrivateTerminalIDs)
	add("欢迎标题", before.WelcomeTitle, after.WelcomeTitle)
	add("服务概述", before.ServiceOverview, after.ServiceOverview)
	add("快速开始", before.QuickStartText, after.QuickStartText)
	add("FAQ", before.FAQText, after.FAQText)
	add("客服文案", before.SupportText, after.SupportText)
	add("我的信息菜单", before.MenuInfoLabel, after.MenuInfoLabel)
	add("设置中心菜单", before.MenuSettingsLabel, after.MenuSettingsLabel)
	add("FAQ 菜单", before.MenuFAQLabel, after.MenuFAQLabel)
	add("客服菜单", before.MenuSupportLabel, after.MenuSupportLabel)
	add("输入框提示", before.MenuPlaceholder, after.MenuPlaceholder)
	addJSON("按钮文案", before.ButtonLabels, after.ButtonLabels)
	addJSON("回复模板", before.ReplyTemplates, after.ReplyTemplates)
	addJSON("默认私信编排", before.DefaultDMMessages, after.DefaultDMMessages)
	addInt("私信最小延迟", before.DMMinDelaySeconds, after.DMMinDelaySeconds)
	addInt("私信最大延迟", before.DMMaxDelaySeconds, after.DMMaxDelaySeconds)
	addInt("私信条数上限", before.DMMaxMessages, after.DMMaxMessages)
	add("Webhook 地址", before.WebhookURL, after.WebhookURL)
	if len(changes) == 0 {
		return "未检测到字段变化"
	}
	if len(changes) > 8 {
		return strings.Join(changes[:8], "、") + fmt.Sprintf(" 等 %d 项", len(changes))
	}
	return strings.Join(changes, "、")
}

func (s *Server) TestBotConfig(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	if strings.TrimSpace(config.Token) == "" {
		utils.Fail(c, http.StatusBadRequest, "请先填写 Bot Token")
		return
	}

	username, message, err := testTelegramBot(config.Token, config.PushChatID)
	now := time.Now()
	status := "success"
	if err != nil {
		status = "failed"
		message = err.Error()
	}
	config.Username = username
	config.LastTestStatus = status
	config.LastTestMessage = message
	config.LastTestAt = &now
	config.UpdatedAt = now
	_ = s.db.WithContext(c.Request.Context()).Save(&config).Error

	if err != nil {
		utils.Fail(c, http.StatusBadRequest, message)
		return
	}
	utils.OK(c, gin.H{"status": status, "username": username, "message": message, "config": config})
}

func (s *Server) StartBotPush(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	if strings.TrimSpace(config.Token) == "" {
		utils.Fail(c, http.StatusBadRequest, "请先配置 Bot Token")
		return
	}
	if config.LastTestStatus != "success" {
		utils.Fail(c, http.StatusBadRequest, "请先测试 Bot 连接成功后再启动")
		return
	}
	config.Enabled = true
	config.Running = true
	config.UpdatedAt = time.Now()
	if err := s.db.WithContext(c.Request.Context()).Save(&config).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "启动 Bot 推送失败")
		return
	}
	utils.OK(c, gin.H{"status": "running", "config": config})
}

func (s *Server) StopBotPush(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	config.Running = false
	config.UpdatedAt = time.Now()
	if err := s.db.WithContext(c.Request.Context()).Save(&config).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "停止 Bot 推送失败")
		return
	}
	utils.OK(c, gin.H{"status": "stopped", "config": config})
}

func (s *Server) ListBotLicenses(c *gin.Context) {
	var licenses []models.BotLicense
	if err := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", s.tenantID(c)).Order("created_at desc").Limit(500).Find(&licenses).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取卡密失败")
		return
	}
	utils.OK(c, licenses)
}

func (s *Server) CreateBotLicenses(c *gin.Context) {
	var input struct {
		Count        int `json:"count"`
		DurationHour int `json:"duration_hour"`
		MaxBind      int `json:"max_bind"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Fail(c, http.StatusBadRequest, "卡密参数无效")
		return
	}
	if input.Count <= 0 {
		input.Count = 1
	}
	if input.Count > 200 {
		input.Count = 200
	}
	if input.DurationHour <= 0 {
		input.DurationHour = 24 * 30
	}
	if input.MaxBind <= 0 {
		input.MaxBind = 1
	}

	now := time.Now()
	licenses := make([]models.BotLicense, 0, input.Count)
	for i := 0; i < input.Count; i++ {
		licenses = append(licenses, models.BotLicense{
			ID:           uuid.New(),
			TenantID:     s.tenantID(c),
			Code:         "C3-" + randomCode(12),
			Status:       "unused",
			DurationHour: input.DurationHour,
			MaxBind:      input.MaxBind,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&licenses).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "生成卡密失败")
		return
	}
	utils.Created(c, licenses)
}

func (s *Server) UpdateBotLicenseStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "卡密 ID 无效")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.Fail(c, http.StatusBadRequest, "状态无效")
		return
	}
	status := strings.TrimSpace(input.Status)
	if status != "unused" && status != "disabled" && status != "used" && status != "expired" {
		utils.Fail(c, http.StatusBadRequest, "不支持的卡密状态")
		return
	}
	var license models.BotLicense
	if err := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).First(&license).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "卡密不存在")
		return
	}
	license.Status = status
	license.UpdatedAt = time.Now()
	if err := s.db.WithContext(c.Request.Context()).Save(&license).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "更新卡密失败")
		return
	}
	utils.OK(c, license)
}

func (s *Server) DeleteBotLicense(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "卡密 ID 无效")
		return
	}
	result := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).Delete(&models.BotLicense{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除卡密失败")
		return
	}
	if result.RowsAffected == 0 {
		utils.Fail(c, http.StatusNotFound, "卡密不存在")
		return
	}
	utils.OK(c, gin.H{"deleted": id.String()})
}

func (s *Server) ListBotSubscribers(c *gin.Context) {
	var users []models.BotSubscriber
	if err := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", s.tenantID(c)).Order("updated_at desc").Limit(500).Find(&users).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 用户失败")
		return
	}
	utils.OK(c, users)
}

func (s *Server) SyncBotCommands(c *gin.Context) {
	config, err := s.ensureBotConfig(c)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取 Bot 配置失败")
		return
	}
	if strings.TrimSpace(config.Token) == "" {
		utils.Fail(c, http.StatusBadRequest, "请先填写 Bot Token")
		return
	}
	commands := selectedBotCommands(config)
	if err := setTelegramBotCommands(config.Token, commands); err != nil {
		utils.Fail(c, http.StatusBadRequest, "同步指令失败："+err.Error())
		return
	}
	utils.OK(c, gin.H{"status": "synced", "commands": commands})
}

func (s *Server) ensureBotConfig(c *gin.Context) (models.BotConfig, error) {
	tenantID := s.tenantID(c)
	var config models.BotConfig
	err := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).First(&config).Error
	if err == nil {
		needsSave := false
		if strings.TrimSpace(config.WebhookSecret) == "" {
			config.WebhookSecret = randomCode(24)
			needsSave = true
		}
		if len(config.EnabledCommands) == 0 {
			commandsJSON, _ := json.Marshal(defaultBotCommandKeys())
			config.EnabledCommands = datatypes.JSON(commandsJSON)
			if config.TrialHours > 0 {
				config.TrialEnabled = true
			}
			needsSave = true
		} else {
			commands := ensureRequiredBotPolicyCommands(botEnabledCommandKeys(config))
			commandsJSON, _ := json.Marshal(commands)
			normalized := datatypes.JSON(commandsJSON)
			if string(config.EnabledCommands) != string(normalized) {
				config.EnabledCommands = normalized
				needsSave = true
			}
		}
		if len(config.CommandLabels) == 0 {
			raw, _ := json.Marshal(defaultBotCommandLabels())
			config.CommandLabels = datatypes.JSON(raw)
			needsSave = true
		}
		if len(config.ButtonLabels) == 0 {
			raw, _ := json.Marshal(defaultBotButtonLabels())
			config.ButtonLabels = datatypes.JSON(raw)
			needsSave = true
		}
		if len(config.ReplyTemplates) == 0 {
			raw, _ := json.Marshal(defaultBotReplyTemplates())
			config.ReplyTemplates = datatypes.JSON(raw)
			needsSave = true
		}
		if len(config.DefaultDMMessages) == 0 {
			raw, _ := json.Marshal([]string{})
			config.DefaultDMMessages = datatypes.JSON(raw)
			needsSave = true
		}
		if config.DMMinDelaySeconds <= 0 || config.DMMaxDelaySeconds <= 0 {
			config.DMMinDelaySeconds = 4
			config.DMMaxDelaySeconds = 8
			needsSave = true
		}
		if config.DMMaxMessages <= 0 {
			config.DMMaxMessages = 3
			needsSave = true
		}
		if needsSave {
			config.UpdatedAt = time.Now()
			_ = s.db.WithContext(c.Request.Context()).Save(&config).Error
		}
		return config, nil
	}
	if err != gorm.ErrRecordNotFound {
		return models.BotConfig{}, err
	}
	now := time.Now()
	featuresJSON, _ := json.Marshal([]string{"keyword_monitor"})
	keywordsJSON, _ := json.Marshal([]string{})
	privateTerminalJSON, _ := json.Marshal([]string{})
	commandsJSON, _ := json.Marshal(defaultBotCommandKeys())
	commandLabelsJSON, _ := json.Marshal(defaultBotCommandLabels())
	buttonLabelsJSON, _ := json.Marshal(defaultBotButtonLabels())
	replyTemplatesJSON, _ := json.Marshal(defaultBotReplyTemplates())
	defaultDMMessagesJSON, _ := json.Marshal([]string{})
	config = models.BotConfig{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		Name:                "Codex3 Bot",
		AdminContact:        "",
		ForceJoinEnabled:    false,
		TrialEnabled:        true,
		TrialHours:          5,
		TrialFeatures:       datatypes.JSON(featuresJSON),
		EnabledCommands:     datatypes.JSON(commandsJSON),
		CommandLabels:       datatypes.JSON(commandLabelsJSON),
		DefaultKeywords:     datatypes.JSON(keywordsJSON),
		DefaultKeywordLimit: 20,
		DefaultMatchMode:    "fuzzy",
		PrivateTerminalIDs:  datatypes.JSON(privateTerminalJSON),
		WelcomeTitle:        "欢迎使用 Codex3 监听机器人",
		ServiceOverview:     "监听目标群组中的关键词命中，并把线索实时汇聚到你的收件箱。",
		QuickStartText:      "点击下方菜单进入设置中心，或发送 /keywords 查看当前关键词。",
		FAQText:             defaultBotFAQText(),
		SupportText:         "",
		MenuInfoLabel:       "📋 我的信息",
		MenuSettingsLabel:   "⚙️ 设置中心",
		MenuFAQLabel:        "❓ 常见问题",
		MenuSupportLabel:    "💬 在线客服",
		MenuPlaceholder:     "选择功能或输入命令...",
		ButtonLabels:        datatypes.JSON(buttonLabelsJSON),
		ReplyTemplates:      datatypes.JSON(replyTemplatesJSON),
		DefaultDMMessages:   datatypes.JSON(defaultDMMessagesJSON),
		DMMinDelaySeconds:   4,
		DMMaxDelaySeconds:   8,
		DMMaxMessages:       3,
		WebhookSecret:       randomCode(24),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&config).Error; err != nil {
		return models.BotConfig{}, err
	}
	return config, nil
}

func (s *Server) applyBotReferral(ctx context.Context, config models.BotConfig, invitee *models.BotSubscriber, code string) error {
	code = strings.TrimSpace(code)
	if invitee == nil || code == "" || invitee.TelegramUserID == code || invitee.InvitedByID != nil {
		return nil
	}
	var inviter models.BotSubscriber
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND invite_code = ?", config.TenantID, code).First(&inviter).Error; err != nil {
		if err := s.db.WithContext(ctx).Where("tenant_id = ? AND telegram_user_id = ?", config.TenantID, code).First(&inviter).Error; err != nil {
			return nil
		}
	}
	if inviter.ID == invitee.ID {
		return nil
	}
	var existing int64
	if err := s.db.WithContext(ctx).Model(&models.BotReferral{}).Where("invitee_id = ?", invitee.ID).Count(&existing).Error; err != nil || existing > 0 {
		return err
	}
	now := time.Now()
	reward := 2
	referral := models.BotReferral{
		ID:          uuid.New(),
		TenantID:    config.TenantID,
		InviterID:   inviter.ID,
		InviteeID:   invitee.ID,
		RewardHours: reward,
		CreatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(&referral).Error; err != nil {
		return err
	}
	base := now
	if inviter.TrialEndsAt != nil && inviter.TrialEndsAt.After(now) {
		base = *inviter.TrialEndsAt
	}
	newEnds := base.Add(time.Duration(reward) * time.Hour)
	_ = s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", inviter.ID).Updates(map[string]any{
		"trial_ends_at": &newEnds,
		"status":        "active",
		"updated_at":    now,
	}).Error
	invitee.InvitedByID = &inviter.ID
	_ = s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", invitee.ID).Updates(map[string]any{"invited_by_id": inviter.ID, "updated_at": now}).Error
	if strings.TrimSpace(config.Token) != "" {
		_ = sendTelegramBotMessage(config.Token, strconv.FormatInt(mustParseInt64(inviter.TelegramUserID), 10), "邀请奖励已到账：试用时间增加 2 小时。")
	}
	return nil
}

func (s *Server) handleBotDocument(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, update telegramUpdate) error {
	chatID := updateChatID(update)
	state, ok := s.loadBotConversationState(ctx, subscriber.ID)
	if !ok || state.State != "await_account_upload" {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "请先在设置中心点击“上传私信账号”，选择账号分组后再上传文件。", botBackToSettingsKeyboard())
	}
	payload := map[string]string{}
	_ = json.Unmarshal(state.Payload, &payload)
	groupID, _ := uuid.Parse(payload["group_id"])
	data, name, err := downloadTelegramDocument(config.Token, update.Message.Document.FileID, update.Message.Document.FileName)
	if err != nil {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "文件下载失败："+err.Error(), botBackToGroupPickerKeyboard("upload"))
	}
	summary, err := s.importBotPrivateAccounts(ctx, config, subscriber, groupID, name, data)
	if err != nil {
		return err
	}
	s.clearBotConversationState(ctx, subscriber.ID)
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, fmt.Sprintf("账号包解析完成。\n\n文件：%s\n扫描账号：%d\n成功：%d\n重复：%d\n失败：%d\n\n账号状态和风控状态已写入后台，后续可在 Web 看板查看。", name, summary.Total, summary.Success, summary.Duplicate, summary.Failed), botBackToSettingsKeyboard())
}

type botPrivateImportSummary struct {
	Total     int
	Success   int
	Duplicate int
	Failed    int
	Items     []gin.H
}

func (s *Server) importBotPrivateAccounts(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, groupID uuid.UUID, fileName string, data []byte) (botPrivateImportSummary, error) {
	if groupID == uuid.Nil {
		group, err := s.ensureBotDefaultAccountGroup(ctx, config, subscriber)
		if err != nil {
			return botPrivateImportSummary{}, err
		}
		groupID = group.ID
	}
	candidates := []importCandidate{{Name: fileName, Data: data}}
	if strings.EqualFold(filepath.Ext(fileName), ".zip") {
		expanded, err := expandZip(fileName, data)
		if err != nil {
			return botPrivateImportSummary{}, err
		}
		candidates = expanded
	}
	units := s.detectBotPrivateImportUnits(candidates)
	summary := botPrivateImportSummary{Total: len(units), Items: []gin.H{}}
	now := time.Now()
	for _, unit := range units {
		hash := sha256.Sum256(unit.Data)
		sessionHash := hex.EncodeToString(hash[:])
		phone := normalizeTerminalPhone(extractPhone(unit.Name))
		var existing int64
		query := s.db.WithContext(ctx).Model(&models.BotPrivateAccount{}).Where("tenant_id = ? AND subscriber_id = ? AND session_hash = ?", config.TenantID, subscriber.ID, sessionHash)
		if err := query.Count(&existing).Error; err != nil {
			return summary, err
		}
		if existing == 0 && phone != "" {
			if err := s.db.WithContext(ctx).Model(&models.BotPrivateAccount{}).Where("tenant_id = ? AND subscriber_id = ? AND regexp_replace(phone, '[^0-9]', '', 'g') = ?", config.TenantID, subscriber.ID, phone).Count(&existing).Error; err != nil {
				return summary, err
			}
		}
		if existing > 0 {
			summary.Duplicate++
			summary.Items = append(summary.Items, gin.H{"name": unit.Name, "status": "duplicate"})
			continue
		}
		path, err := saveUploadedBytes(config.TenantID, "bot_private_accounts", unit.Name, unit.Data)
		if err != nil {
			summary.Failed++
			summary.Items = append(summary.Items, gin.H{"name": unit.Name, "status": "failed", "reason": err.Error()})
			continue
		}
		account := models.BotPrivateAccount{
			ID:           uuid.New(),
			TenantID:     config.TenantID,
			SubscriberID: subscriber.ID,
			GroupID:      &groupID,
			Phone:        phone,
			Nickname:     firstNonEmpty(phone, cleanBaseName(unit.Name)),
			Status:       "待检测",
			RiskStatus:   "待检测",
			AccessType:   unit.AccessType,
			FilePath:     path,
			SessionHash:  sessionHash,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.db.WithContext(ctx).Create(&account).Error; err != nil {
			summary.Failed++
			summary.Items = append(summary.Items, gin.H{"name": unit.Name, "status": "failed", "reason": err.Error()})
			continue
		}
		summary.Success++
		summary.Items = append(summary.Items, gin.H{"name": unit.Name, "status": "success", "phone": phone})
	}
	raw, _ := json.Marshal(summary.Items)
	upload := models.BotPrivateUpload{
		ID:           uuid.New(),
		TenantID:     config.TenantID,
		SubscriberID: subscriber.ID,
		GroupID:      &groupID,
		FileName:     fileName,
		Total:        summary.Total,
		Success:      summary.Success,
		Duplicate:    summary.Duplicate,
		Failed:       summary.Failed,
		Summary:      datatypes.JSON(raw),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = s.db.WithContext(ctx).Create(&upload).Error
	return summary, nil
}

func (s *Server) detectBotPrivateImportUnits(candidates []importCandidate) []importUnit {
	sessionUnits := []importUnit{}
	tdataGroups := map[string][]importCandidate{}
	keys := []string{}
	for _, candidate := range candidates {
		fileType := detectImportType(candidate.Name, candidate.Data)
		switch fileType {
		case "session":
			sessionUnits = append(sessionUnits, importUnit{Name: candidate.Name, Data: candidate.Data, AccessType: "session", SourceSize: 1})
		case "data":
			key, ok := tdataGroupKey(candidate.Name)
			if !ok {
				key = strings.TrimSuffix(candidate.Name, filepath.Ext(candidate.Name))
			}
			if _, exists := tdataGroups[key]; !exists {
				keys = append(keys, key)
			}
			tdataGroups[key] = append(tdataGroups[key], candidate)
		}
	}
	tdataUnits := []importUnit{}
	for _, key := range keys {
		data, err := archiveTDataGroup(key, tdataGroups[key])
		if err != nil {
			continue
		}
		tdataUnits = append(tdataUnits, importUnit{Name: key + ".zip", Data: data, AccessType: "data", SourceSize: len(tdataGroups[key])})
	}
	merged, _ := mergeMixedAccountUnits(sessionUnits, tdataUnits)
	return merged
}

func normalizeBotPushChatInput(raw string, subscriber models.BotSubscriber) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	lower := strings.ToLower(value)
	switch lower {
	case "个人", "私聊", "我", "默认", "me", "private", "default":
		return "", true
	}
	for _, prefix := range []string{"https://t.me/", "http://t.me/", "t.me/"} {
		if strings.HasPrefix(lower, prefix) {
			slug := strings.Trim(strings.TrimSpace(value[len(prefix):]), "/")
			if slug == "" {
				return "", false
			}
			if strings.HasPrefix(slug, "+") || strings.HasPrefix(slug, "joinchat/") || strings.HasPrefix(slug, "c/") {
				return value, true
			}
			if strings.Contains(slug, "/") {
				slug = strings.Split(slug, "/")[0]
			}
			return "@" + strings.TrimPrefix(slug, "@"), true
		}
	}
	if value == subscriber.TelegramUserID {
		return "", true
	}
	return value, true
}

func (s *Server) setBotSubscriberPushChat(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, raw string) string {
	input := strings.TrimSpace(raw)
	if input == "" && chatID != "" && chatID != subscriber.TelegramUserID {
		input = chatID
	}
	normalized, ok := normalizeBotPushChatInput(input, subscriber)
	if !ok {
		return botPushChatPromptText(config, subscriber)
	}
	if err := s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]any{
		"push_chat_id": normalized,
		"updated_at":   time.Now(),
	}).Error; err != nil {
		return "推送位置保存失败：" + err.Error()
	}
	fresh := subscriber
	fresh.PushChatID = normalized
	target := botSubscriberPushChatID(fresh)
	label := botPushChannelText(config, fresh)
	testText := "推送位置已设置为：" + label + "\n后续监听命中会自动推送到这里。"
	if err := sendTelegramBotMessage(config.Token, target, testText); err != nil {
		return "推送位置已保存为：" + label + "\n但测试发送失败：" + err.Error() + "\n请确认机器人已加入群组/频道并拥有发言权限。"
	}
	return "推送位置已保存并测试成功：\n" + label
}

func (s *Server) botKeywordsText(config models.BotConfig, subscriber models.BotSubscriber) string {
	keywords := jsonStringSlice(subscriber.Keywords)
	limit := botEffectiveKeywordLimit(config, subscriber)
	limitText := "不限制"
	if limit > 0 {
		limitText = fmt.Sprintf("%d 个", limit)
	}
	if len(keywords) == 0 {
		return "当前还没有设置监听关键词。\n关键词上限：" + limitText + "\n请发送 /setkeywords 关键词1，关键词2 或在设置中心点击“监听关键词”。"
	}
	return "当前监听关键词：\n" + strings.Join(keywords, "\n") + "\n\n关键词上限：" + limitText
}

func (s *Server) updateBotKeywords(ctx context.Context, subscriber models.BotSubscriber, keywords []string) error {
	rawKeywords, _ := json.Marshal(keywords)
	if err := s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]any{
		"keywords":   datatypes.JSON(rawKeywords),
		"updated_at": time.Now(),
	}).Error; err != nil {
		return err
	}
	return nil
}

func (s *Server) updateBotMatchMode(ctx context.Context, subscriber models.BotSubscriber, mode string) error {
	if err := s.db.WithContext(ctx).Model(&models.BotSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]any{
		"match_mode": mode,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return err
	}
	return nil
}

func (s *Server) upsertBotRadarRule(ctx context.Context, config models.BotConfig, keywords []string, matchMode string) error {
	if len(keywords) == 0 {
		return nil
	}
	rawKeywords, _ := json.Marshal(gin.H{"list": keywords, "text": strings.Join(keywords, "\n")})
	rawTerminals := config.PrivateTerminalIDs
	if len(rawTerminals) == 0 {
		rawTerminals, _ = json.Marshal([]string{})
	}
	now := time.Now()
	var rule models.SCRMKeywordRule
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND name = ?", config.TenantID, "Bot Radar").First(&rule).Error
	if err == nil {
		rule.Keywords = datatypes.JSON(rawKeywords)
		rule.MonitorTerminalIDs = rawTerminals
		rule.MatchMode = normalizeSCRMMatchMode(matchMode)
		rule.PushToBot = true
		rule.StrikeEnabled = false
		rule.Status = "active"
		rule.UpdatedAt = now
		return s.db.WithContext(ctx).Save(&rule).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	rule = models.SCRMKeywordRule{
		ID:                 uuid.New(),
		TenantID:           config.TenantID,
		Name:               "Bot Radar",
		MonitorTerminalIDs: rawTerminals,
		Keywords:           datatypes.JSON(rawKeywords),
		MatchMode:          normalizeSCRMMatchMode(matchMode),
		PushToBot:          true,
		StrikeEnabled:      false,
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	return s.db.WithContext(ctx).Create(&rule).Error
}

func (s *Server) handleBotListenCommand(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, args string) string {
	action := strings.ToLower(strings.TrimSpace(args))
	switch action {
	case "start", "启动":
		keywords := jsonStringSlice(subscriber.Keywords)
		if len(keywords) > 0 {
			if err := s.upsertBotRadarRule(ctx, config, keywords, subscriber.MatchMode); err != nil {
				return "监听规则保存失败：" + err.Error()
			}
		}
		rule, err := s.loadActiveSCRMRule(ctx, config.TenantID)
		if err != nil {
			return err.Error()
		}
		task, err := s.startSCRMListenerRuntime(ctx, config.TenantID, rule, nil, &subscriber)
		if err != nil {
			return err.Error()
		}
		return "监听任务已启动：" + task.Name + "\n命中后将推送到：" + botPushChannelText(config, subscriber)
	case "pause", "stop", "暂停", "停止":
		s.stopBotSubscriberProcesses(ctx, config.TenantID, subscriber, "completed", "paused", "Bot 用户手动暂停监听，已清理该用户监听和私信进程")
		return "监听任务已暂停，该 Bot 用户的监听进程和自动私信任务已从后台清理"
	default:
		s.listenerMu.Lock()
		runtime := s.listeners[scrmListenerRuntimeKey(config.TenantID, subscriber.ID)]
		s.listenerMu.Unlock()
		if runtime == nil || !listenerRuntimeBelongsToSubscriber(runtime, subscriber.ID) {
			return "监听未运行。发送 /listen start 启动。"
		}
		return fmt.Sprintf("监听运行中：目标 %d 个，监听号 %d 个，命中 %d 次。发送 /listen pause 暂停。", len(runtime.targets), len(runtime.terminals), runtime.matchCount.Load())
	}
}

func (s *Server) handleBotListenCallback(ctx context.Context, config models.BotConfig, subscriber models.BotSubscriber, chatID string, data string) error {
	if !botSubscriberCanUse(subscriber) {
		return sendTelegramBotMessageWithMarkup(config.Token, chatID, "试用已过期或账号未授权，请发送 /activate 卡密 激活。", botBackToSettingsKeyboard())
	}
	action := ""
	switch data {
	case "listen:start":
		action = "start"
	case "listen:pause":
		action = "pause"
	case "listen:status":
		action = "status"
	}
	reply := s.handleBotListenCommand(ctx, config, subscriber, action)
	fresh := s.reloadBotSubscriber(ctx, subscriber)
	return sendTelegramBotMessageWithMarkup(config.Token, chatID, reply, s.botSettingsMarkup(ctx, config, fresh))
}

func (s *Server) botInboxText(ctx context.Context, tenantID uuid.UUID) string {
	var leads []models.SCRMLead
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("COALESCE(hit_at, created_at) desc").Limit(5).Find(&leads).Error; err != nil || len(leads) == 0 {
		return "当前还没有命中线索。"
	}
	lines := []string{"最近命中："}
	for _, lead := range leads {
		lines = append(lines, fmt.Sprintf("- %s / %s / %s", firstNonEmpty(lead.UserAccount, lead.UserNickname, "未知用户"), firstNonEmpty(lead.TriggerWord, "未记录关键词"), firstNonEmpty(lead.SourceChatName, "未知来源")))
	}
	return strings.Join(lines, "\n")
}

func jsonStringSlice(raw datatypes.JSON) []string {
	var items []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &items)
	}
	return items
}

func parseBotKeywordInput(text string) []string {
	normalized := strings.NewReplacer("，", "\n", ",", "\n", "、", "\n", ";", "\n", "；", "\n").Replace(text)
	return collectSCRMKeywords(nil, normalized)
}

func botEffectiveKeywordLimit(config models.BotConfig, subscriber models.BotSubscriber) int {
	if subscriber.KeywordLimit > 0 {
		return subscriber.KeywordLimit
	}
	return maxInt(config.DefaultKeywordLimit, 20)
}
