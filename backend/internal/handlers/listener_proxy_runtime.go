package handlers

import (
	"context"
	"strings"

	"codex3/backend/internal/models"
	"codex3/backend/pkg/telegram_client"

	"github.com/google/uuid"
)

func (s *Server) listenerAccountProxyConfig(ctx context.Context, tenantID uuid.UUID, account models.ListenerAccount) telegram_client.ProxyConfig {
	if account.ProxyID == nil {
		return telegram_client.ProxyConfig{}
	}
	var item models.ListenerProxy
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, *account.ProxyID).First(&item).Error; err != nil {
		return telegram_client.ProxyConfig{}
	}
	return telegram_client.ProxyConfig{
		Protocol: strings.ToLower(strings.TrimSpace(item.Protocol)),
		Host:     strings.TrimSpace(item.IP),
		Port:     item.Port,
		Username: item.Username,
		Password: item.Password,
	}
}

func (s *Server) listenerAccountProxyConfigByID(ctx context.Context, tenantID uuid.UUID, accountID uuid.UUID) telegram_client.ProxyConfig {
	var account models.ListenerAccount
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, accountID).First(&account).Error; err != nil {
		return telegram_client.ProxyConfig{}
	}
	return s.listenerAccountProxyConfig(ctx, tenantID, account)
}
