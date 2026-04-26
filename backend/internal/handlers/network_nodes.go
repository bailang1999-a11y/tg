package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type proxyImportSummary struct {
	Success   int                     `json:"success"`
	Failed    int                     `json:"failed"`
	Duplicate int                     `json:"duplicate"`
	Skipped   int                     `json:"skipped"`
	GroupID   *uuid.UUID              `json:"group_id,omitempty"`
	GroupName string                  `json:"group_name,omitempty"`
	Items     []proxyImportResultItem `json:"items"`
}

type proxyImportResultItem struct {
	Line     string `json:"line"`
	Protocol string `json:"protocol,omitempty"`
	Address  string `json:"address,omitempty"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

type parsedProxy struct {
	Protocol string
	IP       string
	Port     int
	Username string
	Password string
}

func (s *Server) ListNetworkNodes(c *gin.Context) {
	var items []models.NetworkNode
	query := s.db.WithContext(c.Request.Context()).Order("created_at desc")
	query = s.applyTenantAccess(c, query)
	if group := c.Query("group_id"); group != "" {
		query = query.Where("group_id = ?", group)
	}
	if err := query.Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取网络节点失败")
		return
	}
	utils.OK(c, items)
}

func (s *Server) ImportNetworkNodes(c *gin.Context) {
	var req struct {
		Content         string `json:"content" binding:"required"`
		DefaultProtocol string `json:"default_protocol"`
		GroupID         string `json:"group_id"`
		NewGroupName    string `json:"new_group_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "请输入要导入的代理内容")
		return
	}
	defaultProtocol := normalizeProxyProtocol(req.DefaultProtocol)
	if defaultProtocol == "" {
		utils.Fail(c, http.StatusBadRequest, "默认协议必须是 socks5 或 http")
		return
	}

	groupID, groupName, err := s.resolveNetworkGroup(c, strings.TrimSpace(req.GroupID), strings.TrimSpace(req.NewGroupName))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	lines := strings.Split(req.Content, "\n")
	summary := proxyImportSummary{
		GroupID:   groupID,
		GroupName: groupName,
		Items:     []proxyImportResultItem{},
	}

	err = s.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		nextCode, err := s.nextNetworkNodeCode(c, tx)
		if err != nil {
			return err
		}
		for _, raw := range lines {
			line := strings.TrimSpace(raw)
			if line == "" {
				summary.Skipped++
				continue
			}
			proxy, err := parseProxyLine(line, defaultProtocol)
			if err != nil {
				summary.Failed++
				summary.Items = append(summary.Items, proxyImportResultItem{Line: line, Status: "failed", Reason: err.Error()})
				continue
			}

			reason, err := s.networkNodeDuplicateReason(c, tx, proxy)
			if err != nil {
				return err
			}
			address := fmt.Sprintf("%s:%d", proxy.IP, proxy.Port)
			if reason != "" {
				summary.Duplicate++
				summary.Items = append(summary.Items, proxyImportResultItem{Line: line, Protocol: proxy.Protocol, Address: address, Status: "duplicate", Reason: reason})
				continue
			}

			node := models.NetworkNode{
				ID:        uuid.New(),
				TenantID:  s.tenantID(c),
				Code:      fmt.Sprintf("NN-%06d", nextCode),
				IP:        proxy.IP,
				Port:      proxy.Port,
				Protocol:  proxy.Protocol,
				Username:  proxy.Username,
				Password:  proxy.Password,
				Country:   "未知",
				Flag:      "",
				Status:    "untested",
				GroupID:   groupID,
				LatencyMS: 0,
			}
			if err := tx.WithContext(c.Request.Context()).Create(&node).Error; err != nil {
				return err
			}
			nextCode++
			summary.Success++
			summary.Items = append(summary.Items, proxyImportResultItem{Line: line, Protocol: proxy.Protocol, Address: address, Status: "success"})
		}
		return nil
	})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "导入网络节点失败")
		return
	}

	utils.Created(c, summary)
}

func (s *Server) resolveNetworkGroup(c *gin.Context, groupIDText string, newGroupName string) (*uuid.UUID, string, error) {
	if groupIDText != "" && newGroupName != "" {
		return nil, "", fmt.Errorf("请选择已有分组或填写新分组，不能同时使用")
	}
	if newGroupName != "" {
		group := models.Group{
			ID:           uuid.New(),
			TenantID:     s.tenantID(c),
			ResourceType: "network",
			Name:         newGroupName,
		}
		if err := s.db.WithContext(c.Request.Context()).Create(&group).Error; err != nil {
			return nil, "", fmt.Errorf("创建新分组失败")
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
	query := s.db.WithContext(c.Request.Context()).Where("id = ? AND resource_type = ?", parsed, "network")
	query = s.applyTenantAccess(c, query)
	if err := query.First(&group).Error; err != nil {
		return nil, "", fmt.Errorf("网络节点分组不存在")
	}
	return &group.ID, group.Name, nil
}

func (s *Server) nextNetworkNodeCode(c *gin.Context, tx *gorm.DB) (int64, error) {
	var count int64
	query := tx.WithContext(c.Request.Context()).Model(&models.NetworkNode{})
	query = s.applyTenantAccess(c, query)
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count + 1, nil
}

func (s *Server) networkNodeDuplicateReason(c *gin.Context, tx *gorm.DB, proxy parsedProxy) (string, error) {
	var existing int64
	query := tx.WithContext(c.Request.Context()).Model(&models.NetworkNode{}).
		Where("protocol = ? AND ip = ? AND port = ? AND username = ?", proxy.Protocol, proxy.IP, proxy.Port, proxy.Username)
	query = s.applyTenantAccess(c, query)
	if err := query.Count(&existing).Error; err != nil {
		return "", err
	}
	if existing > 0 {
		return "协议、地址、端口和用户名已存在", nil
	}
	return "", nil
}

func parseProxyLine(line string, defaultProtocol string) (parsedProxy, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return parsedProxy{}, fmt.Errorf("空行")
	}
	defaultProtocol = normalizeProxyProtocol(defaultProtocol)
	if defaultProtocol == "" {
		return parsedProxy{}, fmt.Errorf("默认协议必须是 socks5 或 http")
	}

	protocol := defaultProtocol
	body := line
	if idx := strings.Index(body, "://"); idx >= 0 {
		protocol = normalizeProxyProtocol(body[:idx])
		if protocol == "" {
			return parsedProxy{}, fmt.Errorf("协议只支持 socks5 或 http")
		}
		body = body[idx+3:]
	}

	proxy, err := parseProxyBody(body, protocol)
	if err != nil {
		return parsedProxy{}, err
	}
	proxy.Protocol = protocol
	return proxy, nil
}

func parseProxyBody(body string, protocol string) (parsedProxy, error) {
	if strings.Contains(body, "@") {
		parsed, err := url.Parse(protocol + "://" + body)
		if err != nil {
			return parsedProxy{}, fmt.Errorf("代理格式无效")
		}
		port, err := parseProxyPort(parsed.Port())
		if err != nil {
			return parsedProxy{}, err
		}
		proxy := parsedProxy{
			IP:   parsed.Hostname(),
			Port: port,
		}
		if parsed.User != nil {
			proxy.Username = parsed.User.Username()
			proxy.Password, _ = parsed.User.Password()
		}
		if proxy.IP == "" {
			return parsedProxy{}, fmt.Errorf("代理地址不能为空")
		}
		return proxy, nil
	}

	parts := strings.Split(body, ":")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch {
	case len(parts) == 2:
		port, err := parseProxyPort(parts[1])
		if err != nil {
			return parsedProxy{}, err
		}
		return parsedProxy{IP: parts[0], Port: port}, validateProxyHost(parts[0])
	case len(parts) >= 4:
		port, err := parseProxyPort(parts[1])
		if err != nil {
			return parsedProxy{}, err
		}
		return parsedProxy{IP: parts[0], Port: port, Username: parts[2], Password: strings.Join(parts[3:], ":")}, validateProxyHost(parts[0])
	default:
		return parsedProxy{}, fmt.Errorf("代理格式应为 ip:port、ip:port:user:pass 或 user:pass@ip:port")
	}
}

func parseProxyPort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("端口无效")
	}
	return port, nil
}

func validateProxyHost(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("代理地址不能为空")
	}
	return nil
}

func normalizeProxyProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sk5", "socks5":
		return "socks5"
	case "http":
		return "http"
	default:
		return ""
	}
}

func supportedProxyProtocols() []string {
	protocols := []string{"http", "socks5"}
	sort.Strings(protocols)
	return protocols
}
