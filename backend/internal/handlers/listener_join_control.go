package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codex3/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func listenerAccountAsTerminal(account models.ListenerAccount, tenantID uuid.UUID) models.Terminal {
	return models.Terminal{
		ID:                account.ID,
		TenantID:          tenantID,
		Phone:             account.Phone,
		Nickname:          account.Nickname,
		AvatarURL:         account.AvatarURL,
		Status:            account.Status,
		AccessType:        account.AccessType,
		FilePath:          account.FilePath,
		SessionHash:       account.SessionHash,
		ExitIP:            account.ExitIP,
		ExitCountry:       account.ExitCountry,
		ExitFlag:          account.ExitFlag,
		GroupID:           account.GroupID,
		RiskStatus:        account.RiskStatus,
		LastOnlineAt:      account.LastOnlineAt,
		LastMessageAt:     account.LastMessageAt,
		LastJoinAt:        account.LastJoinAt,
		JoinDailyLimit:    account.JoinDailyLimit,
		JoinDailyCount:    account.JoinDailyCount,
		JoinDailyResetAt:  account.JoinDailyResetAt,
		JoinCooldownUntil: account.JoinCooldownUntil,
		CreatedAt:         account.CreatedAt,
		UpdatedAt:         account.UpdatedAt,
	}
}

func (s *Server) reserveListenerJoinQuota(ctx context.Context, listenerAccountID uuid.UUID) (models.ListenerAccount, error) {
	var account models.ListenerAccount
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", listenerAccountID).
			First(&account).Error; err != nil {
			return err
		}
		now := time.Now()
		if account.JoinCooldownUntil != nil && account.JoinCooldownUntil.After(now) {
			return fmt.Errorf("监听账号加群冷却中，需等待到 %s", account.JoinCooldownUntil.Local().Format("2006-01-02 15:04:05"))
		}
		if !listenerAccountReadyForJoin(account) {
			return fmt.Errorf("监听账号当前不可用于加群")
		}
		settings := s.readSystemSettings(ctx, uuid.Nil)
		dailyCount, dailyResetAt := resetTerminalQuotaWindow(now, account.JoinDailyCount, account.JoinDailyResetAt, "day")
		dailyLimit := account.JoinDailyLimit
		if dailyLimit <= 0 {
			dailyLimit = settings.RiskControl.JoinDailyLimit
		}
		if dailyLimit > 0 && dailyCount >= dailyLimit {
			return fmt.Errorf("监听账号每日加群限额已达上限（%d）", dailyLimit)
		}
		dailyCount++
		nextCooldown := terminalNextCooldownAt(now, settings.RiskControl.JoinIntervalMinutes, settings.RiskControl.JoinJitterMinutes)
		account.JoinDailyCount = dailyCount
		account.JoinDailyResetAt = dailyResetAt
		account.JoinCooldownUntil = &nextCooldown
		account.LastJoinAt = &now
		return tx.Model(&models.ListenerAccount{}).
			Where("id = ?", listenerAccountID).
			Updates(map[string]any{
				"join_daily_count":    dailyCount,
				"join_daily_reset_at": dailyResetAt,
				"join_cooldown_until": nextCooldown,
				"last_join_at":        now,
				"updated_at":          now,
			}).Error
	})
	return account, err
}

func listenerAccountReadyForJoin(account models.ListenerAccount) bool {
	status := strings.ToLower(strings.TrimSpace(account.Status))
	if status == "abnormal" || status == "disabled" || status == "banned" {
		return false
	}
	if isProfileRestrictedStatus(account.RiskStatus, "") {
		return false
	}
	return strings.TrimSpace(account.FilePath) != "" && isStoredTerminalFileReady(account.FilePath)
}
