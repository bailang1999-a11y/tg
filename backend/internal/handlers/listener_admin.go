package handlers

import (
	"net/http"
	"strings"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) GetListenerAdminOverview(c *gin.Context) {
	tenantID := s.tenantID(c)
	var summary listenerAdminSummary
	_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerAccount{}).Where("tenant_id = ?", tenantID).Count(&summary.AccountCount).Error
	_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerTarget{}).Where("tenant_id = ?", tenantID).Count(&summary.TargetCount).Error
	_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerProxy{}).Where("tenant_id = ?", tenantID).Count(&summary.ProxyCount).Error
	_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerAccount{}).Where("tenant_id = ? AND proxy_id IS NOT NULL", tenantID).Count(&summary.AssignedCount).Error
	utils.OK(c, summary)
}

func (s *Server) ListListenerAccounts(c *gin.Context) {
	var items []models.ListenerAccount
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", s.tenantID(c)).Order("created_at desc")
	if group := strings.TrimSpace(c.Query("group_id")); group != "" {
		query = query.Where("group_id = ?", group)
	}
	if err := query.Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听账号失败")
		return
	}
	var targetTotal int64
	_ = s.db.WithContext(c.Request.Context()).Model(&models.ListenerTarget{}).Where("tenant_id = ?", s.tenantID(c)).Count(&targetTotal).Error
	rows := make([]listenerAccountRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, listenerAccountRow{
			ListenerAccount:   item,
			PhoneDisplay:      formatTerminalPhoneDisplay(item.Phone),
			AvatarURL:         item.AvatarURL,
			JoinedTargetCount: minInt64(item.JoinedTargets, targetTotal),
			TargetTotalCount:  targetTotal,
			StatusText:        listenerAccountStatusText(item.Status, item.RiskStatus),
		})
	}
	utils.OK(c, rows)
}

func (s *Server) ListListenerTargets(c *gin.Context) {
	var items []models.ListenerTarget
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", s.tenantID(c)).Order("created_at desc")
	if group := strings.TrimSpace(c.Query("group_id")); group != "" {
		query = query.Where("group_id = ?", group)
	}
	if err := query.Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听群失败")
		return
	}
	groupIDs := []uuid.UUID{}
	for _, item := range items {
		if item.GroupID != nil {
			groupIDs = append(groupIDs, *item.GroupID)
		}
	}
	groups := map[uuid.UUID]string{}
	if len(groupIDs) > 0 {
		var groupItems []models.Group
		_ = s.db.WithContext(c.Request.Context()).Where("tenant_id = ? AND id IN ?", s.tenantID(c), groupIDs).Find(&groupItems).Error
		for _, group := range groupItems {
			groups[group.ID] = group.Name
		}
	}
	rows := make([]listenerTargetRow, 0, len(items))
	for _, item := range items {
		groupName := "未分组"
		if item.GroupID != nil && groups[*item.GroupID] != "" {
			groupName = groups[*item.GroupID]
		}
		rows = append(rows, listenerTargetRow{ListenerTarget: item, GroupName: groupName, TypeText: listenerTargetTypeText(item.Type)})
	}
	utils.OK(c, rows)
}

func (s *Server) ListListenerProxies(c *gin.Context) {
	var items []models.ListenerProxy
	query := s.db.WithContext(c.Request.Context()).Where("tenant_id = ?", s.tenantID(c)).Order("created_at desc")
	if group := strings.TrimSpace(c.Query("group_id")); group != "" {
		query = query.Where("group_id = ?", group)
	}
	if err := query.Find(&items).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "读取监听代理失败")
		return
	}
	rows := make([]listenerProxyRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, listenerProxyRowFromModel(item))
	}
	utils.OK(c, rows)
}
