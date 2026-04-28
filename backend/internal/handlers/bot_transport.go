package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func testTelegramBot(token string, chatID string) (string, string, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", strings.TrimSpace(token))
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var getMe struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&getMe); err != nil {
		return "", "", err
	}
	if !getMe.OK {
		return "", "", errors.New(firstNonEmpty(getMe.Description, "Bot Token 验证失败"))
	}

	username := getMe.Result.Username
	if strings.TrimSpace(chatID) == "" {
		return username, "Bot Token 有效", nil
	}

	payload, _ := json.Marshal(gin.H{
		"chat_id": chatID,
		"text":    "Codex3 Bot 连接测试成功",
	})
	sendEndpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", strings.TrimSpace(token))
	sendResp, err := client.Post(sendEndpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return username, "", err
	}
	defer sendResp.Body.Close()
	var sendResult struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(sendResp.Body).Decode(&sendResult); err != nil {
		return username, "", err
	}
	if !sendResult.OK {
		return username, "", errors.New(firstNonEmpty(sendResult.Description, "测试消息发送失败"))
	}
	return username, "Bot Token 有效，测试消息已发送", nil
}

func sendTelegramBotMessage(token string, chatID string, text string) error {
	return sendTelegramBotMessageWithMarkup(token, chatID, text, nil)
}

func sendTelegramBotMessageWithMarkup(token string, chatID string, text string, replyMarkup any) error {
	return sendTelegramBotMessageWithOptions(token, chatID, text, replyMarkup, "")
}

func sendTelegramBotMessageHTML(token string, chatID string, text string, replyMarkup any) error {
	return sendTelegramBotMessageWithOptions(token, chatID, text, replyMarkup, "HTML")
}

func sendTelegramBotMessageWithOptions(token string, chatID string, text string, replyMarkup any, parseMode string) error {
	client := telegramBotHTTPClient(8 * time.Second)
	payloadMap := gin.H{
		"chat_id": chatID,
		"text":    text,
	}
	if strings.TrimSpace(parseMode) != "" {
		payloadMap["parse_mode"] = parseMode
		payloadMap["disable_web_page_preview"] = true
	}
	if replyMarkup != nil {
		payloadMap["reply_markup"] = replyMarkup
	}
	payload, _ := json.Marshal(payloadMap)
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", strings.TrimSpace(token))
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return errors.New(redactTelegramBotError(token, err.Error()))
	}
	defer resp.Body.Close()
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if !result.OK {
		return errors.New(firstNonEmpty(result.Description, "消息发送失败"))
	}
	return nil
}

func answerTelegramCallbackQuery(token string, callbackID string, text string) error {
	payload := gin.H{"callback_query_id": callbackID}
	if strings.TrimSpace(text) != "" {
		payload["text"] = text
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := telegramBotPost(token, "answerCallbackQuery", payload, &result); err != nil {
		return err
	}
	if !result.OK {
		return errors.New(firstNonEmpty(result.Description, "按钮响应失败"))
	}
	return nil
}

func getTelegramChatMemberStatus(token string, chatID string, userID string) (string, error) {
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := telegramBotPost(token, "getChatMember", gin.H{"chat_id": chatID, "user_id": userID}, &result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", errors.New(firstNonEmpty(result.Description, "加群状态校验失败"))
	}
	return result.Result.Status, nil
}

func downloadTelegramDocument(token string, fileID string, fallbackName string) ([]byte, string, error) {
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := telegramBotPost(token, "getFile", gin.H{"file_id": fileID}, &result); err != nil {
		return nil, "", err
	}
	if !result.OK || strings.TrimSpace(result.Result.FilePath) == "" {
		return nil, "", errors.New(firstNonEmpty(result.Description, "读取 Telegram 文件失败"))
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", strings.TrimSpace(token), result.Result.FilePath)
	resp, err := telegramBotHTTPClient(60 * time.Second).Get(endpoint)
	if err != nil {
		return nil, "", errors.New(redactTelegramBotError(token, err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("Telegram 文件下载失败：%s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImportFileBytes*20))
	if err != nil {
		return nil, "", err
	}
	name := strings.TrimSpace(fallbackName)
	if name == "" {
		name = filepath.Base(result.Result.FilePath)
	}
	return data, name, nil
}

func setTelegramBotCommands(token string, commands []telegramCommand) error {
	clearPayloads := []gin.H{
		{},
		{"language_code": "zh"},
	}
	for _, payload := range clearPayloads {
		var clearResult struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		if err := telegramBotPost(token, "deleteMyCommands", payload, &clearResult); err != nil {
			return err
		}
		if !clearResult.OK {
			return errors.New(firstNonEmpty(clearResult.Description, "指令菜单清空失败"))
		}
	}

	if len(commands) == 0 {
		return nil
	}

	setPayloads := []gin.H{
		{"commands": commands},
		{"commands": commands, "language_code": "zh"},
	}
	for _, payload := range setPayloads {
		var result struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		if err := telegramBotPost(token, "setMyCommands", payload, &result); err != nil {
			return err
		}
		if !result.OK {
			return errors.New(firstNonEmpty(result.Description, "指令菜单设置失败"))
		}
	}
	return nil
}

func setTelegramWebhook(token string, webhookURL string) (string, error) {
	payload := gin.H{
		"url": webhookURL,
		"allowed_updates": []string{
			"message",
			"callback_query",
		},
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := telegramBotPost(token, "setWebhook", payload, &result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", errors.New(firstNonEmpty(result.Description, "Webhook 设置失败"))
	}
	return firstNonEmpty(result.Description, "Webhook 已设置"), nil
}

func getTelegramWebhookInfo(token string) (map[string]any, error) {
	client := telegramBotHTTPClient(8 * time.Second)
	resp, err := client.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getWebhookInfo", strings.TrimSpace(token)))
	if err != nil {
		return nil, errors.New(redactTelegramBotError(token, err.Error()))
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if ok, _ := result["ok"].(bool); !ok {
		if description, _ := result["description"].(string); description != "" {
			return nil, errors.New(description)
		}
		return nil, errors.New("Webhook 状态读取失败")
	}
	return result, nil
}

func deleteTelegramWebhook(token string, dropPending bool) (string, error) {
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := telegramBotPost(token, "deleteWebhook", gin.H{"drop_pending_updates": dropPending}, &result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", errors.New(firstNonEmpty(result.Description, "Webhook 关闭失败"))
	}
	return firstNonEmpty(result.Description, "Webhook 已关闭"), nil
}

func getTelegramUpdates(token string, offset int64) ([]telegramUpdate, error) {
	client := telegramBotHTTPClient(35 * time.Second)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=25&allowed_updates=%s", strings.TrimSpace(token), `%5B%22message%22%2C%22callback_query%22%5D`)
	if offset > 0 {
		url += fmt.Sprintf("&offset=%d", offset)
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, errors.New(redactTelegramBotError(token, err.Error()))
	}
	defer resp.Body.Close()
	var result struct {
		OK          bool             `json:"ok"`
		Description string           `json:"description"`
		Result      []telegramUpdate `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, errors.New(firstNonEmpty(result.Description, "获取 Bot 消息失败"))
	}
	return result.Result, nil
}

func telegramBotPost(token string, method string, payload any, out any) error {
	client := telegramBotHTTPClient(10 * time.Second)
	raw, _ := json.Marshal(payload)
	resp, err := client.Post(fmt.Sprintf("https://api.telegram.org/bot%s/%s", strings.TrimSpace(token), method), "application/json", bytes.NewReader(raw))
	if err != nil {
		return errors.New(redactTelegramBotError(token, err.Error()))
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func redactTelegramBotError(token string, message string) string {
	token = strings.TrimSpace(token)
	if token == "" || message == "" {
		return message
	}
	message = strings.ReplaceAll(message, token, "[redacted-bot-token]")
	message = strings.ReplaceAll(message, "bot"+token, "bot[redacted-bot-token]")
	return message
}

func telegramBotHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", address)
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func normalizeBotWebhookURL(value string, secret string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value != "" && !strings.Contains(value, "://") {
		value = "https://" + value
	}
	if strings.Contains(value, "/api/v1/bot/webhook/") {
		return value
	}
	if strings.HasSuffix(value, "/api/v1") {
		return value + "/bot/webhook/" + secret
	}
	return value + "/api/v1/bot/webhook/" + secret
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func randomDelaySeconds(minDelay int, maxDelay int) int {
	minDelay, maxDelay = normalizeBotDMDelay(minDelay, maxDelay)
	if maxDelay <= minDelay {
		return minDelay
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxDelay-minDelay+1)))
	if err != nil {
		return minDelay
	}
	return minDelay + int(n.Int64())
}

func randomCode(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return strings.ToUpper(uuid.NewString()[:size])
	}
	return strings.ToUpper(hex.EncodeToString(buf))[:size]
}
