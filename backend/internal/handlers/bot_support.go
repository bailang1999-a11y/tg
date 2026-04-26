package handlers

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"codex3/backend/internal/models"

	"gorm.io/datatypes"
)

type botPollRuntime struct {
	cancel            context.CancelFunc
	startedAt         time.Time
	lastUpdateID      atomic.Int64
	handledCount      atomic.Int64
	lastError         atomic.Value
	lastMessageAtUnix atomic.Int64
}

type botConfigInput struct {
	Name                string            `json:"name"`
	Token               string            `json:"token"`
	PushChatID          string            `json:"push_chat_id"`
	AdminChatID         string            `json:"admin_chat_id"`
	AdminContact        string            `json:"admin_contact"`
	Enabled             bool              `json:"enabled"`
	ForceJoinEnabled    bool              `json:"force_join_enabled"`
	ForceJoinURL        string            `json:"force_join_url"`
	TrialEnabled        bool              `json:"trial_enabled"`
	TrialHours          int               `json:"trial_hours"`
	TrialFeatures       []string          `json:"trial_features"`
	EnabledCommands     []string          `json:"enabled_commands"`
	CommandLabels       map[string]string `json:"command_labels"`
	DefaultKeywords     []string          `json:"default_keywords"`
	DefaultKeywordLimit int               `json:"default_keyword_limit"`
	DefaultMatchMode    string            `json:"default_match_mode"`
	PrivateTerminalIDs  []string          `json:"private_terminal_ids"`
	WelcomeTitle        string            `json:"welcome_title"`
	ServiceOverview     string            `json:"service_overview"`
	QuickStartText      string            `json:"quick_start_text"`
	FAQText             string            `json:"faq_text"`
	SupportText         string            `json:"support_text"`
	MenuInfoLabel       string            `json:"menu_info_label"`
	MenuSettingsLabel   string            `json:"menu_settings_label"`
	MenuFAQLabel        string            `json:"menu_faq_label"`
	MenuSupportLabel    string            `json:"menu_support_label"`
	MenuPlaceholder     string            `json:"menu_placeholder"`
	ButtonLabels        map[string]string `json:"button_labels"`
	ReplyTemplates      map[string]string `json:"reply_templates"`
	DefaultDMMessages   []string          `json:"default_dm_messages"`
	DMMinDelaySeconds   int               `json:"dm_min_delay_seconds"`
	DMMaxDelaySeconds   int               `json:"dm_max_delay_seconds"`
	DMMaxMessages       int               `json:"dm_max_messages"`
	WebhookURL          string            `json:"webhook_url"`
}

type telegramUpdate struct {
	UpdateID      int64           `json:"update_id"`
	Message       telegramMessage `json:"message"`
	CallbackQuery struct {
		ID   string `json:"id"`
		From struct {
			ID        int64  `json:"id"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"from"`
		Message telegramMessage `json:"message"`
		Data    string          `json:"data"`
	} `json:"callback_query"`
}

type telegramMessage struct {
	MessageID int64 `json:"message_id"`
	From      struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"from"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Text     string `json:"text"`
	Date     int64  `json:"date"`
	Document struct {
		FileID   string `json:"file_id"`
		FileName string `json:"file_name"`
		FileSize int64  `json:"file_size"`
	} `json:"document"`
}

type telegramCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type botCommandDefinition struct {
	Key          string
	Command      string
	Description  string
	SettingsLine string
	Always       bool
}

func botCommandCatalog() []botCommandDefinition {
	return []botCommandDefinition{
		{Key: "start", Command: "start", Description: "开始使用机器人", Always: true},
		{Key: "help", Command: "help", Description: "查看帮助", Always: true},
		{Key: "trial", Command: "trial", Description: "开通试用", SettingsLine: "开通试用：/trial"},
		{Key: "activate", Command: "activate", Description: "输入卡密激活", SettingsLine: "卡密激活：/activate 卡密"},
		{Key: "status", Command: "status", Description: "查看账号状态", SettingsLine: "账号状态：/status"},
		{Key: "keywords", Command: "keywords", Description: "查看关键词", SettingsLine: "查看关键词：/keywords"},
		{Key: "setkeywords", Command: "setkeywords", Description: "设置监听关键词", SettingsLine: "设置关键词：/setkeywords 合作 请教 多少钱"},
		{Key: "match", Command: "match", Description: "设置模糊或精准匹配", SettingsLine: "匹配模式：/match fuzzy 或 /match exact"},
		{Key: "setpush", Command: "setpush", Description: "设置推送位置", SettingsLine: "推送位置：/setpush @channel 或在群里发送 /setpush"},
		{Key: "listen", Command: "listen", Description: "启动或暂停监听", SettingsLine: "监听控制：/listen start 或 /listen pause"},
		{Key: "inbox", Command: "inbox", Description: "查看最新命中", SettingsLine: "最新命中：/inbox"},
	}
}

func defaultBotCommandKeys() []string {
	keys := []string{}
	for _, item := range botCommandCatalog() {
		keys = append(keys, item.Key)
	}
	return keys
}

func requiredBotPolicyCommandKeys() []string {
	return []string{"start", "help", "trial", "keywords", "setkeywords"}
}

func ensureRequiredBotPolicyCommands(keys []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keys)+len(requiredBotPolicyCommandKeys()))
	for _, key := range normalizeBotCommandKeys(keys) {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	for _, key := range requiredBotPolicyCommandKeys() {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func defaultBotCommandLabels() map[string]string {
	out := map[string]string{}
	for _, item := range botCommandCatalog() {
		out[item.Key] = item.Description
	}
	return out
}

func defaultBotButtonLabels() map[string]string {
	return map[string]string{
		"listen_keywords": "🔑 监听关键词",
		"dm_open":         "✉️ 开启私信",
		"dm_close":        "⏸ 关闭私信",
		"listen_start":    "▶️ 启动监听",
		"listen_pause":    "⏸ 暂停监听",
		"member":          "💳 开通会员",
	}
}

func defaultBotReplyTemplates() map[string]string {
	return map[string]string{
		"welcome":         "欢迎使用监听机器人，请点击设置中心开始配置。",
		"keywords_prompt": "请输入监听关键词，一行一个，或者使用“，”分开。",
		"dm_prompt":       "请设置需要发送的第 1 条私信内容。",
		"admin_contact":   "请联系管理员开通权限或处理使用问题。",
	}
}

func normalizeBotFeatures(features []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}
		if _, ok := seen[feature]; ok {
			continue
		}
		seen[feature] = struct{}{}
		out = append(out, feature)
	}
	if len(out) == 0 {
		out = []string{"keyword_monitor"}
	}
	return out
}

func normalizeBotCommandLabels(input map[string]string) map[string]string {
	out := defaultBotCommandLabels()
	for key, value := range input {
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if _, ok := out[key]; ok {
			out[key] = value
		}
	}
	return out
}

func normalizeBotButtonLabels(input map[string]string) map[string]string {
	out := defaultBotButtonLabels()
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func normalizeBotReplyTemplates(input map[string]string) map[string]string {
	out := defaultBotReplyTemplates()
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func botConfigStringMap(raw datatypes.JSON, fallback map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range fallback {
		out[key] = value
	}
	if len(raw) == 0 {
		return out
	}
	var saved map[string]string
	if err := json.Unmarshal(raw, &saved); err != nil {
		return out
	}
	for key, value := range saved {
		if strings.TrimSpace(key) != "" {
			out[key] = value
		}
	}
	return out
}

func botButtonLabel(config models.BotConfig, key string) string {
	return firstNonEmpty(botConfigStringMap(config.ButtonLabels, defaultBotButtonLabels())[key], defaultBotButtonLabels()[key])
}

func botReplyTemplate(config models.BotConfig, key string) string {
	return firstNonEmpty(botConfigStringMap(config.ReplyTemplates, defaultBotReplyTemplates())[key], defaultBotReplyTemplates()[key])
}

func botCommandLabel(config models.BotConfig, key string) string {
	return firstNonEmpty(botConfigStringMap(config.CommandLabels, defaultBotCommandLabels())[key], defaultBotCommandLabels()[key])
}

func normalizeBotDMMessages(input []string) []string {
	out := make([]string, 0, 10)
	for _, value := range input {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func normalizeBotDMDelay(minDelay int, maxDelay int) (int, int) {
	if minDelay <= 0 {
		minDelay = 4
	}
	if maxDelay <= 0 {
		maxDelay = 8
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}
	if maxDelay > 3600 {
		maxDelay = 3600
	}
	return minDelay, maxDelay
}

func normalizeBotDMMaxMessages(value int) int {
	if value <= 0 {
		return 3
	}
	if value > 10 {
		return 10
	}
	return value
}

func normalizeBotCommandKeys(keys []string) []string {
	allowed := map[string]struct{}{}
	for _, item := range botCommandCatalog() {
		allowed[item.Key] = struct{}{}
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if _, ok := allowed[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func botEnabledCommandKeys(config models.BotConfig) []string {
	var keys []string
	if len(config.EnabledCommands) == 0 {
		return defaultBotCommandKeys()
	}
	if err := json.Unmarshal(config.EnabledCommands, &keys); err != nil {
		return defaultBotCommandKeys()
	}
	return normalizeBotCommandKeys(keys)
}

func botCommandEnabled(config models.BotConfig, command string) bool {
	command = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(command)), "/")
	if command == "" {
		return false
	}
	for _, item := range botCommandCatalog() {
		if item.Command != command {
			continue
		}
		if item.Always {
			return true
		}
		for _, key := range botEnabledCommandKeys(config) {
			if key == item.Key {
				return true
			}
		}
		return false
	}
	return false
}

func selectedBotCommands(config models.BotConfig) []telegramCommand {
	enabled := map[string]struct{}{}
	for _, key := range botEnabledCommandKeys(config) {
		enabled[key] = struct{}{}
	}
	commands := []telegramCommand{}
	for _, item := range botCommandCatalog() {
		if item.Always {
			commands = append(commands, telegramCommand{Command: item.Command, Description: botCommandLabel(config, item.Key)})
			continue
		}
		if _, ok := enabled[item.Key]; ok {
			commands = append(commands, telegramCommand{Command: item.Command, Description: botCommandLabel(config, item.Key)})
		}
	}
	return commands
}

func updateChatID(update telegramUpdate) string {
	if update.CallbackQuery.Message.Chat.ID != 0 {
		return strconv.FormatInt(update.CallbackQuery.Message.Chat.ID, 10)
	}
	return strconv.FormatInt(update.Message.Chat.ID, 10)
}

func updateUserID(update telegramUpdate) string {
	if update.CallbackQuery.From.ID != 0 {
		return strconv.FormatInt(update.CallbackQuery.From.ID, 10)
	}
	return strconv.FormatInt(update.Message.From.ID, 10)
}
