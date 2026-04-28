package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"codex3/backend/internal/middleware"
	"codex3/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const auditBodyLimit = 32 << 10

type auditResponseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w auditResponseWriter) Write(data []byte) (int, error) {
	if w.body.Len() < auditBodyLimit {
		remaining := auditBodyLimit - w.body.Len()
		if len(data) > remaining {
			w.body.Write(auditSafeBytePrefix(data, remaining))
		} else {
			w.body.Write(data)
		}
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) auditActionLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldAuditRequest(c.Request.Method, c.FullPath()) {
			c.Next()
			return
		}

		start := time.Now()
		requestSummary := auditRequestSummary(c)
		writer := &auditResponseWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = writer

		c.Next()

		s.writeAuditActionLog(c, start, requestSummary, writer.body.String())
	}
}

func shouldAuditRequest(method string, path string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	if path == "" {
		return false
	}
	return !strings.HasPrefix(path, "/api/v1/bot/webhook/")
}

func auditRequestSummary(c *gin.Context) string {
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		return "multipart 表单上传"
	}
	if c.Request.Body == nil {
		return "无请求体"
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, auditBodyLimit))
	if err != nil {
		return "请求体读取失败"
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	if len(bytes.TrimSpace(data)) == 0 {
		return "无请求体"
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Sprintf("非 JSON 请求体，大小 %d 字节", len(data))
	}
	return summarizeAuditJSON(payload)
}

func summarizeAuditJSON(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		parts := make([]string, 0, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", key, auditFieldSummary(key, typed[key])))
		}
		if len(parts) == 0 {
			return "空 JSON 对象"
		}
		return strings.Join(parts, "；")
	case []any:
		return fmt.Sprintf("JSON 数组 %d 项", len(typed))
	default:
		return "JSON " + auditValueKind(value)
	}
}

func auditFieldSummary(key string, value any) string {
	lowerKey := strings.ToLower(key)
	if isSensitiveAuditField(lowerKey) {
		return "[已脱敏]"
	}
	if lowerKey == "content" {
		if text, ok := value.(string); ok {
			return fmt.Sprintf("%d 行/%d 字符", countNonEmptyTextLines(text), len(text))
		}
	}
	if strings.HasSuffix(lowerKey, "_ids") || strings.HasSuffix(lowerKey, "ids") {
		if list, ok := value.([]any); ok {
			return fmt.Sprintf("%d 项", len(list))
		}
	}
	return auditValueKind(value)
}

func isSensitiveAuditField(key string) bool {
	sensitiveParts := []string{"password", "token", "secret", "key", "authorization", "cookie", "credential", "license"}
	for _, part := range sensitiveParts {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func auditValueKind(value any) string {
	switch typed := value.(type) {
	case nil:
		return "空"
	case string:
		if typed == "" {
			return "空字符串"
		}
		return fmt.Sprintf("文本 %d 字符", len(typed))
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return "数字"
	case []any:
		return fmt.Sprintf("数组 %d 项", len(typed))
	case map[string]any:
		return fmt.Sprintf("对象 %d 字段", len(typed))
	default:
		return "值"
	}
}

func countNonEmptyTextLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func (s *Server) writeAuditActionLog(c *gin.Context, start time.Time, requestSummary string, responseBody string) {
	claims := middleware.CurrentClaims(c)
	if claims == nil {
		return
	}
	statusCode := c.Writer.Status()
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	path := firstNonEmpty(c.FullPath(), c.Request.URL.Path)
	method := c.Request.Method
	actionText := auditActionText(method, path)
	level := "INFO"
	if statusCode >= 500 {
		level = "ERROR"
	} else if statusCode >= 400 {
		level = "WARN"
	}
	result := auditResultText(statusCode, responseBody)
	durationMS := time.Since(start).Milliseconds()
	requestSummary = auditSafeText(requestSummary)
	result = auditSafeText(result)
	detail := fmt.Sprintf("操作：%s；接口：%s %s；状态：%d；耗时：%dms；输入：%s；结果：%s", actionText, method, path, statusCode, durationMS, requestSummary, result)
	detail = auditTruncateText(detail, 1000)
	payload, _ := json.Marshal(gin.H{
		"method":      method,
		"path":        path,
		"status_code": statusCode,
		"duration_ms": durationMS,
		"request":     requestSummary,
	})
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  claims.TenantID,
		Name:      auditTaskName(method, path, actionText),
		Type:      "audit_action",
		Status:    auditTaskStatus(statusCode),
		Progress:  100,
		Payload:   datatypes.JSON(payload),
		CreatedBy: &claims.UserID,
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		return
	}
	_ = s.createTaskLog(c.Request.Context(), task, level, auditLogAction(method, path), detail, "", "")
}

func auditTaskName(method, path, actionText string) string {
	return fmt.Sprintf("操作日志-%s-%s", actionText, auditPathName(method, path))
}

func auditTaskStatus(statusCode int) string {
	if statusCode >= 500 {
		return "failed"
	}
	if statusCode >= 400 {
		return "partial_success"
	}
	return "success"
}

func auditResultText(statusCode int, body string) string {
	if statusCode < 400 {
		return "成功"
	}
	body = auditSafeText(body)
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err == nil && strings.TrimSpace(payload.Error) != "" {
		return auditTruncateText(payload.Error, 300)
	}
	return "请求失败"
}

func auditSafeText(value string) string {
	return strings.ToValidUTF8(value, "�")
}

func auditTruncateText(value string, maxRunes int) string {
	value = auditSafeText(value)
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func auditSafeBytePrefix(data []byte, maxBytes int) []byte {
	if maxBytes <= 0 {
		return nil
	}
	if len(data) <= maxBytes {
		return data
	}
	prefix := data[:maxBytes]
	for len(prefix) > 0 && !utf8.Valid(prefix) {
		_, size := utf8.DecodeLastRune(prefix)
		if size <= 0 || size > len(prefix) {
			prefix = prefix[:len(prefix)-1]
			continue
		}
		prefix = prefix[:len(prefix)-size]
	}
	return prefix
}

func auditLogAction(method, path string) string {
	switch method {
	case http.MethodPost:
		return "audit_create"
	case http.MethodPut, http.MethodPatch:
		return "audit_update"
	case http.MethodDelete:
		return "audit_delete"
	default:
		return "audit_action"
	}
}

func auditActionText(method, path string) string {
	if strings.Contains(path, "/import") || strings.Contains(path, "/upload") {
		return "导入/上传"
	}
	if strings.Contains(path, "/check") || strings.Contains(path, "/test") || strings.Contains(path, "/refresh") {
		return "检测/刷新"
	}
	if strings.Contains(path, "/start") || strings.Contains(path, "/run") {
		return "启动"
	}
	if strings.Contains(path, "/stop") || strings.Contains(path, "/pause") {
		return "停止/暂停"
	}
	switch method {
	case http.MethodPost:
		return "新增/提交"
	case http.MethodPut, http.MethodPatch:
		return "修改"
	case http.MethodDelete:
		return "删除"
	default:
		return "操作"
	}
}

func auditPathName(method, path string) string {
	path = strings.TrimPrefix(path, "/api/v1/")
	path = strings.ReplaceAll(path, "/", "-")
	path = strings.ReplaceAll(path, ":", "")
	if path == "" {
		path = strings.ToLower(method)
	}
	return path
}
