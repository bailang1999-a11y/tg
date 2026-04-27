package handlers

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/net/proxy"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const listenerProxyProbeTarget = "www.gstatic.com:80"

func (s *Server) ImportListenerTargets(c *gin.Context) {
	var req struct {
		Content      string `json:"content" binding:"required"`
		GroupID      string `json:"group_id"`
		NewGroupName string `json:"new_group_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入监听群链接")
		return
	}
	groupID, groupName, err := s.resolveListenerGroup(c, "listener_target", strings.TrimSpace(req.GroupID), firstNonEmpty(strings.TrimSpace(req.NewGroupName), "监听群"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	summary := listenerImportSummary{GroupID: groupID, GroupName: groupName, Items: []listenerImportResult{}}
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for _, raw := range strings.Split(req.Content, "\n") {
			line := cleanTargetImportLine(raw)
			if line == "" {
				summary.Skipped++
				continue
			}
			target, err := parseTargetLine(line)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Status: "failed", Reason: err.Error()})
				continue
			}
			var existing int64
			if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerTarget{}).Where("tenant_id = ? AND identifier = ?", s.tenantID(c), target.Identifier).Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				summary.Duplicate++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: target.Identifier, Status: "duplicate", Reason: "监听群已存在"})
				continue
			}
			item := models.ListenerTarget{ID: uuid.New(), TenantID: s.tenantID(c), GroupID: groupID, Identifier: target.Identifier, Name: target.Name, Type: target.Type, Status: "active"}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			summary.Success++
			summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: target.Identifier, Status: "success"})
		}
		return nil
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "导入监听群失败")
		return
	}
	utils.Created(c, summary)
}

func (s *Server) ImportListenerProxies(c *gin.Context) {
	var req struct {
		Content          string `json:"content" binding:"required"`
		DefaultProtocol  string `json:"default_protocol"`
		GroupID          string `json:"group_id"`
		NewGroupName     string `json:"new_group_name"`
		AccountGroupID   string `json:"account_group_id"`
		AssignToAccounts bool   `json:"assign_to_accounts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入监听代理")
		return
	}
	defaultProtocol := normalizeProxyProtocol(req.DefaultProtocol)
	if defaultProtocol == "" {
		defaultProtocol = "socks5"
	}
	lines := importNonEmptyLines(req.Content)
	task := s.createListenerProxyImportTask(c, len(lines), defaultProtocol, req.AssignToAccounts)
	if task != nil {
		_ = s.createTaskLog(c.Request.Context(), *task, "INFO", "created", fmt.Sprintf("开始导入监听代理：%d 行，默认协议 %s", len(lines), defaultProtocol), "", "")
	}
	groupID, groupName, err := s.resolveListenerGroup(c, "listener_proxy", strings.TrimSpace(req.GroupID), firstNonEmpty(strings.TrimSpace(req.NewGroupName), "监听代理"))
	if err != nil {
		if task != nil {
			s.finishListenerProxyImportTask(c, *task, "failed", 100, fmt.Sprintf("导入失败：%s", err.Error()))
		}
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	summary := listenerImportSummary{GroupID: groupID, GroupName: groupName, Items: []listenerImportResult{}}
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		nextCode, err := s.nextListenerProxyCode(c, tx)
		if err != nil {
			return err
		}
		for index, line := range lines {
			proxy, err := parseProxyLine(line, defaultProtocol)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Status: "failed", Reason: err.Error()})
				if task != nil {
					_ = s.createTaskLog(c.Request.Context(), *task, "ERROR", "import_proxies", fmt.Sprintf("第 %d 行导入失败：%s", index+1, err.Error()), "", "")
				}
				continue
			}
			var existing int64
			if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Where("tenant_id = ? AND protocol = ? AND ip = ? AND port = ? AND username = ?", s.tenantID(c), proxy.Protocol, proxy.IP, proxy.Port, proxy.Username).Count(&existing).Error; err != nil {
				return err
			}
			address := fmt.Sprintf("%s:%d", proxy.IP, proxy.Port)
			if existing > 0 {
				summary.Duplicate++
				summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: address, Status: "duplicate", Reason: "监听代理已存在"})
				if task != nil {
					_ = s.createTaskLog(c.Request.Context(), *task, "WARN", "import_proxies", fmt.Sprintf("第 %d 行重复：%s 已存在", index+1, address), "", "")
				}
				continue
			}
			item := models.ListenerProxy{ID: uuid.New(), TenantID: s.tenantID(c), GroupID: groupID, Code: fmt.Sprintf("LP-%06d", nextCode), IP: proxy.IP, Port: proxy.Port, Protocol: proxy.Protocol, Username: proxy.Username, Password: proxy.Password, Country: "未知", Status: "untested"}
			if err := tx.WithContext(c.Request.Context()).Create(&item).Error; err != nil {
				return err
			}
			nextCode++
			summary.Success++
			summary.Items = append(summary.Items, listenerImportResult{Line: line, Identifier: address, Status: "success"})
			if task != nil {
				_ = s.createTaskLog(c.Request.Context(), *task, "INFO", "import_proxies", fmt.Sprintf("第 %d 行导入成功：%s %s", index+1, proxy.Protocol, address), "", "")
			}
		}
		return nil
	})
	if err != nil {
		if task != nil {
			s.finishListenerProxyImportTask(c, *task, "failed", 100, "导入监听代理失败："+err.Error())
		}
		utils.Fail(c, http.StatusInternalServerError, "导入监听代理失败")
		return
	}
	var assign listenerAdminAssignSummary
	assignmentError := ""
	if req.AssignToAccounts {
		assign, err = s.assignListenerProxies(c, groupID, strings.TrimSpace(req.AccountGroupID))
		if err != nil {
			assignmentError = err.Error()
			if task != nil {
				_ = s.createTaskLog(c.Request.Context(), *task, "WARN", "import_proxies", "代理已导入，但自动分配失败："+assignmentError, "", "")
			}
		} else if task != nil {
			_ = s.createTaskLog(c.Request.Context(), *task, "INFO", "import_proxies", fmt.Sprintf("自动分配完成：成功 %d，跳过 %d", assign.Assigned, assign.Skipped), "", "")
		}
	}
	status := listenerProxyImportTaskStatus(summary, assignmentError)
	detail := fmt.Sprintf("监听代理导入完成：成功 %d，重复 %d，失败 %d，跳过 %d", summary.Success, summary.Duplicate, summary.Failed, summary.Skipped)
	if assignmentError != "" {
		detail += "；自动分配失败：" + assignmentError
	}
	if task != nil {
		s.finishListenerProxyImportTask(c, *task, status, 100, detail)
	}
	utils.Created(c, gin.H{"import": summary, "assignment": assign, "assignment_error": assignmentError})
}

func importNonEmptyLines(content string) []string {
	lines := []string{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s *Server) createListenerProxyImportTask(c *gin.Context, lineCount int, defaultProtocol string, assignToAccounts bool) *models.Task {
	payload, _ := json.Marshal(gin.H{
		"line_count":         lineCount,
		"default_protocol":   defaultProtocol,
		"assign_to_accounts": assignToAccounts,
	})
	task := models.Task{
		ID:        uuid.New(),
		TenantID:  s.tenantID(c),
		Name:      "导入代理",
		Type:      "import_proxies",
		Status:    "running",
		Progress:  10,
		Payload:   datatypes.JSON(payload),
		CreatedBy: s.userIDPtr(c),
	}
	if err := s.db.WithContext(c.Request.Context()).Create(&task).Error; err != nil {
		return nil
	}
	return &task
}

func listenerProxyImportTaskStatus(summary listenerImportSummary, assignmentError string) string {
	if summary.Success == 0 && (summary.Failed > 0 || assignmentError != "") {
		return "failed"
	}
	if summary.Failed > 0 || summary.Duplicate > 0 || assignmentError != "" {
		return "partial_success"
	}
	return "success"
}

func (s *Server) finishListenerProxyImportTask(c *gin.Context, task models.Task, status string, progress int, detail string) {
	s.updateTaskState(c.Request.Context(), task.ID, status, progress, nil)
	level := "INFO"
	if status == "failed" {
		level = "ERROR"
	} else if status == "partial_success" {
		level = "WARN"
	}
	_ = s.createTaskLog(c.Request.Context(), task, level, "summary", detail, "", "")
}

func (s *Server) CheckListenerAccounts(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id"`
	}
	_ = c.ShouldBindJSON(&req)

	tenantID := s.tenantID(c)
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).Order("created_at desc")
	if strings.TrimSpace(req.GroupID) != "" {
		groupID, err := uuid.Parse(strings.TrimSpace(req.GroupID))
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "监听号分组无效")
			return
		}
		query = query.Where("group_id = ?", groupID)
	}
	var accounts []models.ListenerAccount
	if err := query.Find(&accounts).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听账号失败")
		return
	}

	var targetTotal int64
	_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerTarget{}).Where("tenant_id = ?", tenantID).Count(&targetTotal).Error
	now := time.Now()
	syncer := telegram_client.NewInspector(s.cfg)
	summary := listenerCheckSummary{Total: len(accounts)}
	for _, account := range accounts {
		status := "normal"
		riskStatus := firstNonEmpty(strings.TrimSpace(account.RiskStatus), "正常")
		reason := "监听号已登记"
		phone := account.Phone
		nickname := account.Nickname
		avatarURL := account.AvatarURL
		lastOnlineAt := account.LastOnlineAt
		joinedTargets := account.JoinedTargets

		if account.FilePath != "" && isStoredTerminalFileReady(account.FilePath) {
			avatarDir, avatarDirErr := prepareTerminalAvatarSyncDir(tenantID, account.ID)
			if avatarDirErr != nil {
				reason = firstNonEmpty(reason, "准备头像缓存目录失败")
			}
			syncResult, syncErr := syncer.Sync(c.Request.Context(), telegram_client.SyncRequest{
				FilePath:   account.FilePath,
				AccessType: telegram_client.NormalizeTelegramAccessType(account.AccessType),
				AvatarDir:  avatarDir,
			})
			if syncErr == nil || strings.TrimSpace(syncResult.Reason) != "" {
				status = listenerNormalizeAccountStatus(syncResult.Status)
				reason = firstNonEmpty(syncResult.Reason, reason)
				phone = firstNonEmpty(syncResult.Phone, phone)
				nickname = firstNonEmpty(syncResult.Nickname, nickname)
				riskStatus = firstNonEmpty(syncResult.RiskStatus, riskStatus)
				lastOnlineAt = syncResult.LastOnlineAt
				avatarURL, reason = s.persistTerminalAvatar(tenantID, models.Terminal{ID: account.ID}, avatarURL, syncResult, reason)
			} else {
				status = "abnormal"
				riskStatus = "检测失败"
				reason = syncErr.Error()
			}
			if avatarDir != "" {
				_ = os.RemoveAll(avatarDir)
			}
		} else if strings.TrimSpace(account.FilePath) == "" {
			status = "abnormal"
			riskStatus = "需重新导入"
			reason = "缺少本地会话文件"
		} else if !isStoredTerminalFileReady(account.FilePath) {
			status = "abnormal"
			riskStatus = "需重新导入"
			reason = "本地会话文件不存在"
		} else if strings.TrimSpace(account.Phone) == "" {
			status = "abnormal"
			riskStatus = "缺少手机号"
		}

		if isListenerAccountNormal(status, riskStatus) {
			summary.Normal++
			joinedTargets = s.countActiveAccountTargetJoins(c.Request.Context(), uuid.Nil, accountJoinKindListener, account.ID)
		} else if status == "offline" {
			summary.Offline++
			joinedTargets = s.countActiveAccountTargetJoins(c.Request.Context(), uuid.Nil, accountJoinKindListener, account.ID)
		} else {
			summary.Abnormal++
			s.markAccountJoinRecordsUnavailable(c.Request.Context(), uuid.Nil, accountJoinKindListener, account.ID, reason)
			joinedTargets = s.countActiveAccountTargetJoins(c.Request.Context(), uuid.Nil, accountJoinKindListener, account.ID)
		}
		if joinedTargets > targetTotal {
			joinedTargets = targetTotal
		}
		normalizedPhone, _, _ := syncTerminalPhoneIdentity(phone, "", "")
		if normalizedPhone == "" {
			normalizedPhone = phone
		}
		if err := s.db.WithContext(c.Request.Context()).Model(&models.ListenerAccount{}).Where("tenant_id = ? AND id = ?", tenantID, account.ID).Updates(map[string]any{
			"phone":          normalizedPhone,
			"nickname":       nickname,
			"avatar_url":     avatarURL,
			"status":         status,
			"risk_status":    riskStatus,
			"last_online_at": lastOnlineAt,
			"joined_targets": joinedTargets,
			"updated_at":     now,
		}).Error; err != nil {
			utils.Fail(c, http.StatusInternalServerError, "更新监听账号状态失败："+reason)
			return
		}
	}
	utils.OK(c, summary)
}

func (s *Server) DeleteAbnormalListenerAccounts(c *gin.Context) {
	tenantID := s.tenantID(c)
	var accounts []models.ListenerAccount
	_ = s.db.WithContext(c.Request.Context()).Where(
		"tenant_id = ? AND (status IN ? OR risk_status IN ?)",
		tenantID,
		[]string{"abnormal", "failed", "disabled"},
		[]string{"需重新导入", "资料受限", "检测失败", "缺少手机号", "封禁", "冻结"},
	).Find(&accounts).Error
	result := s.db.WithContext(c.Request.Context()).Where(
		"tenant_id = ? AND (status IN ? OR risk_status IN ?)",
		tenantID,
		[]string{"abnormal", "failed", "disabled"},
		[]string{"需重新导入", "资料受限", "检测失败", "缺少手机号", "封禁", "冻结"},
	).Delete(&models.ListenerAccount{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除异常监听账号失败")
		return
	}
	for _, account := range accounts {
		s.markAccountJoinRecordsUnavailable(c.Request.Context(), uuid.Nil, accountJoinKindListener, account.ID, "异常监听账号已删除")
	}
	utils.OK(c, gin.H{"deleted": result.RowsAffected})
}

func (s *Server) CheckListenerProxies(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id"`
	}
	_ = c.ShouldBindJSON(&req)
	tenantID := s.tenantID(c)
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).Order("created_at desc")
	if strings.TrimSpace(req.GroupID) != "" {
		groupID, err := uuid.Parse(strings.TrimSpace(req.GroupID))
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "代理分组无效")
			return
		}
		query = query.Where("group_id = ?", groupID)
	}
	var proxies []models.ListenerProxy
	if err := query.Find(&proxies).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听代理失败")
		return
	}
	summary := listenerProxyCheckSummary{Total: len(proxies)}
	for _, item := range proxies {
		latency, status := measureListenerProxy(item)
		country, flag := lookupProxyCountry(c.Request.Context(), item.IP)
		if country == "" {
			country = firstNonEmpty(item.Country, "未知")
			flag = item.Flag
		}
		switch status {
		case "normal":
			summary.Normal++
		case "timeout":
			summary.Timeout++
		default:
			summary.Failed++
		}
		if err := s.db.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Where("tenant_id = ? AND id = ?", tenantID, item.ID).Updates(map[string]any{
			"latency_ms": latency,
			"status":     status,
			"country":    country,
			"flag":       flag,
			"updated_at": time.Now(),
		}).Error; err != nil {
			utils.Fail(c, http.StatusInternalServerError, "更新代理延迟失败")
			return
		}
	}
	utils.OK(c, summary)
}

type proxyGeoResponse struct {
	Status      string `json:"status"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

func measureListenerProxy(item models.ListenerProxy) (int64, string) {
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	switch protocol {
	case "socks5", "sk5":
		return measureSOCKS5Proxy(item)
	case "http", "https":
		return measureHTTPProxy(item)
	default:
		return measureProxyEndpoint(item.IP, item.Port)
	}
}

func measureSOCKS5Proxy(item models.ListenerProxy) (int64, string) {
	address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
	baseDialer := &net.Dialer{Timeout: 5 * time.Second}
	var auth *proxy.Auth
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		auth = &proxy.Auth{User: item.Username, Password: item.Password}
	}
	dialer, err := proxy.SOCKS5("tcp", address, auth, baseDialer)
	if err != nil {
		return 0, "failed"
	}
	start := time.Now()
	conn, err := dialer.Dial("tcp", listenerProxyProbeTarget)
	if err != nil {
		if os.IsTimeout(err) {
			return 0, "timeout"
		}
		return 0, "failed"
	}
	_ = conn.Close()
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed, "normal"
}

func measureHTTPProxy(item models.ListenerProxy) (int64, string) {
	address := net.JoinHostPort(strings.TrimSpace(item.IP), fmt.Sprintf("%d", item.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		if os.IsTimeout(err) {
			return 0, "timeout"
		}
		return 0, "failed"
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := writeHTTPProxyProbe(conn, item); err != nil {
		return 0, "failed"
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		if os.IsTimeout(err) {
			return 0, "timeout"
		}
		return 0, "failed"
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "failed"
	}
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed, "normal"
}

func writeHTTPProxyProbe(conn net.Conn, item models.ListenerProxy) error {
	var builder strings.Builder
	builder.WriteString("CONNECT ")
	builder.WriteString(listenerProxyProbeTarget)
	builder.WriteString(" HTTP/1.1\r\nHost: ")
	builder.WriteString(listenerProxyProbeTarget)
	builder.WriteString("\r\nProxy-Connection: close\r\n")
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		token := base64.StdEncoding.EncodeToString([]byte(item.Username + ":" + item.Password))
		builder.WriteString("Proxy-Authorization: Basic ")
		builder.WriteString(token)
		builder.WriteString("\r\n")
	}
	builder.WriteString("\r\n")
	_, err := conn.Write([]byte(builder.String()))
	return err
}

func measureProxyEndpoint(ip string, port int) (int64, string) {
	address := net.JoinHostPort(strings.TrimSpace(ip), fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		if os.IsTimeout(err) {
			return 0, "timeout"
		}
		return 0, "failed"
	}
	_ = conn.Close()
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed, "normal"
}

func lookupProxyCountry(ctx context.Context, ip string) (string, string) {
	ip = strings.TrimSpace(ip)
	if ip == "" || net.ParseIP(ip) == nil {
		return "", ""
	}
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	url := "http://ip-api.com/json/" + ip + "?fields=status,country,countryCode"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", ""
	}
	var geo proxyGeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return "", ""
	}
	if !strings.EqualFold(geo.Status, "success") || strings.TrimSpace(geo.Country) == "" {
		return "", ""
	}
	return strings.TrimSpace(geo.Country), countryFlagEmoji(geo.CountryCode)
}

func countryFlagEmoji(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return ""
	}
	runes := []rune(code)
	if runes[0] < 'A' || runes[0] > 'Z' || runes[1] < 'A' || runes[1] > 'Z' {
		return ""
	}
	return string([]rune{0x1F1E6 + runes[0] - 'A', 0x1F1E6 + runes[1] - 'A'})
}

func (s *Server) RefreshListenerTargets(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id"`
	}
	_ = c.ShouldBindJSON(&req)
	tenantID := s.tenantID(c)
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).Order("created_at desc")
	if strings.TrimSpace(req.GroupID) != "" {
		groupID, err := uuid.Parse(strings.TrimSpace(req.GroupID))
		if err != nil {
			utils.Fail(c, http.StatusBadRequest, "监听群分组无效")
			return
		}
		query = query.Where("group_id = ?", groupID)
	}
	var targets []models.ListenerTarget
	if err := query.Find(&targets).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听群失败")
		return
	}
	account, ok := s.pickListenerInspectorAccount(c, tenantID)
	if !ok {
		utils.Fail(c, http.StatusBadRequest, "没有可用于刷新监听群资料的监听号，请先导入并检测监听号")
		return
	}
	inspector := telegram_client.NewTargetInspector(s.cfg)
	summary := listenerTargetRefreshSummary{Total: len(targets)}
	for _, target := range targets {
		result, err := inspector.Inspect(c.Request.Context(), telegram_client.TargetInspectRequest{
			FilePath:   account.FilePath,
			AccessType: account.AccessType,
			Target:     target.Identifier,
		})
		updates := map[string]any{"updated_at": time.Now()}
		if err == nil && result.OK {
			updates["identifier"] = firstNonEmpty(result.Identifier, target.Identifier)
			updates["name"] = firstNonEmpty(result.Name, target.Name)
			updates["type"] = firstNonEmpty(result.Type, target.Type)
			updates["size"] = result.Size
			updates["status"] = firstNonEmpty(result.Status, "active")
			summary.Success++
		} else {
			updates["status"] = "failed"
			summary.Failed++
		}
		if err := s.db.WithContext(c.Request.Context()).Model(&models.ListenerTarget{}).Where("tenant_id = ? AND id = ?", tenantID, target.ID).Updates(updates).Error; err != nil {
			utils.Fail(c, http.StatusInternalServerError, "更新监听群资料失败")
			return
		}
	}
	utils.OK(c, summary)
}

func (s *Server) pickListenerInspectorAccount(c *gin.Context, tenantID uuid.UUID) (models.ListenerAccount, bool) {
	var account models.ListenerAccount
	err := s.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND file_path <> '' AND status NOT IN ?", tenantID, []string{"abnormal", "failed"}).
		Order("updated_at asc").
		First(&account).Error
	if err != nil || strings.TrimSpace(account.FilePath) == "" || !isStoredTerminalFileReady(account.FilePath) {
		return models.ListenerAccount{}, false
	}
	return account, true
}

func (s *Server) DeleteListenerAccount(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "监听账号 ID 无效")
		return
	}
	var account models.ListenerAccount
	_ = s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).First(&account).Error
	result := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).Delete(&models.ListenerAccount{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除监听账号失败")
		return
	}
	if account.FilePath != "" {
		_ = removeStoredAssetFile(account.FilePath)
	}
	s.markAccountJoinRecordsUnavailable(c.Request.Context(), uuid.Nil, accountJoinKindListener, id, "监听账号已删除")
	utils.OK(c, gin.H{"deleted": result.RowsAffected})
}

func (s *Server) DeleteListenerTarget(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "监听群 ID 无效")
		return
	}
	result := s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id = ?", s.tenantID(c), id).Delete(&models.ListenerTarget{})
	if result.Error != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除监听群失败")
		return
	}
	utils.OK(c, gin.H{"deleted": result.RowsAffected})
}

func (s *Server) DeleteListenerProxy(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "代理 ID 无效")
		return
	}
	tenantID := s.tenantID(c)
	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&models.ListenerProxy{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.ListenerAccount{}).Where("tenant_id = ? AND proxy_id = ?", tenantID, id).Updates(map[string]any{
			"proxy_id":     nil,
			"exit_ip":      "",
			"exit_country": "",
			"exit_flag":    "",
			"updated_at":   time.Now(),
		}).Error
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "删除监听代理失败")
		return
	}
	utils.OK(c, gin.H{"deleted": 1})
}

func (s *Server) AssignListenerProxies(c *gin.Context) {
	var req struct {
		ProxyGroupID   string `json:"proxy_group_id"`
		AccountGroupID string `json:"account_group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请选择代理分组")
		return
	}
	proxyGroupID, err := uuid.Parse(strings.TrimSpace(req.ProxyGroupID))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "代理分组无效")
		return
	}
	summary, err := s.assignListenerProxies(c, &proxyGroupID, strings.TrimSpace(req.AccountGroupID))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.OK(c, summary)
}

func (s *Server) assignListenerProxies(c *gin.Context, proxyGroupID *uuid.UUID, accountGroupText string) (listenerAdminAssignSummary, error) {
	return s.assignListenerProxiesToAccounts(c, proxyGroupID, accountGroupText, nil, false)
}

func (s *Server) assignListenerProxiesToAccounts(c *gin.Context, proxyGroupID *uuid.UUID, accountGroupText string, accountIDs []uuid.UUID, onlyUnassigned bool) (listenerAdminAssignSummary, error) {
	tenantID := s.tenantID(c)
	var proxies []models.ListenerProxy
	query := s.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND status NOT IN ?", tenantID, []string{"failed", "timeout"}).
		Order("bound_accounts asc, created_at asc")
	if proxyGroupID != nil {
		query = query.Where("group_id = ?", *proxyGroupID)
	}
	if err := query.Find(&proxies).Error; err != nil {
		return listenerAdminAssignSummary{}, err
	}
	if len(proxies) == 0 {
		return listenerAdminAssignSummary{}, fmt.Errorf("当前监听代理分组没有可分配代理")
	}
	var accounts []models.ListenerAccount
	aq := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).Order("created_at asc")
	if len(accountIDs) > 0 {
		aq = aq.Where("id IN ?", accountIDs)
	}
	if onlyUnassigned {
		aq = aq.Where("proxy_id IS NULL")
	}
	if accountGroupText != "" {
		groupID, err := uuid.Parse(accountGroupText)
		if err != nil {
			return listenerAdminAssignSummary{}, fmt.Errorf("监听号分组无效")
		}
		aq = aq.Where("group_id = ?", groupID)
	}
	if err := aq.Find(&accounts).Error; err != nil {
		return listenerAdminAssignSummary{}, err
	}
	if len(accounts) == 0 {
		return listenerAdminAssignSummary{}, fmt.Errorf("没有可分配的监听号")
	}
	usage := map[uuid.UUID]int64{}
	for _, proxy := range proxies {
		usage[proxy.ID] = proxy.BoundAccounts
	}
	summary := listenerAdminAssignSummary{Accounts: len(accounts), Proxies: len(proxies)}
	now := time.Now()
	for _, account := range accounts {
		bestIndex := -1
		for index := range proxies {
			if usage[proxies[index].ID] >= 3 {
				continue
			}
			if bestIndex < 0 || usage[proxies[index].ID] < usage[proxies[bestIndex].ID] {
				bestIndex = index
			}
		}
		if bestIndex < 0 {
			summary.Skipped++
			continue
		}
		proxy := proxies[bestIndex]
		if err := s.db.WithContext(c.Request.Context()).Model(&models.ListenerAccount{}).Where("tenant_id = ? AND id = ?", tenantID, account.ID).Updates(map[string]any{
			"proxy_id":     proxy.ID,
			"exit_ip":      proxy.IP,
			"exit_country": proxy.Country,
			"exit_flag":    proxy.Flag,
			"updated_at":   now,
		}).Error; err != nil {
			return summary, err
		}
		usage[proxy.ID]++
		summary.Assigned++
	}
	for _, proxy := range proxies {
		_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Where("tenant_id = ? AND id = ?", tenantID, proxy.ID).Updates(map[string]any{"bound_accounts": usage[proxy.ID], "updated_at": now}).Error
	}
	return summary, nil
}

func (s *Server) resolveListenerGroup(c *gin.Context, resourceType string, groupIDText string, newGroupName string) (*uuid.UUID, string, error) {
	if groupIDText != "" && newGroupName != "" {
		return nil, "", fmt.Errorf("请选择已有分组或填写新分组，不能同时使用")
	}
	if newGroupName != "" {
		var existing models.Group
		err := s.db.WithContext(c.Request.Context()).
			Where("tenant_id = ? AND resource_type = ? AND name = ?", s.tenantID(c), resourceType, newGroupName).
			First(&existing).Error
		if err == nil {
			return &existing.ID, existing.Name, nil
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, "", fmt.Errorf("读取分组失败")
		}
		group := models.Group{ID: uuid.New(), TenantID: s.tenantID(c), ResourceType: resourceType, Name: newGroupName}
		if err := s.db.WithContext(c.Request.Context()).Create(&group).Error; err != nil {
			return nil, "", fmt.Errorf("创建分组失败")
		}
		return &group.ID, group.Name, nil
	}
	if groupIDText == "" {
		return nil, "", nil
	}
	parsed, err := uuid.Parse(groupIDText)
	if err != nil {
		return nil, "", fmt.Errorf("分组 ID 无效")
	}
	var group models.Group
	if err := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", parsed, resourceType).First(&group).Error; err != nil {
		return nil, "", fmt.Errorf("分组不存在")
	}
	return &group.ID, group.Name, nil
}

func (s *Server) nextListenerProxyCode(c *gin.Context, tx *gorm.DB) (int64, error) {
	var count int64
	if err := tx.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count + 1, nil
}
