package models

import (
	"time"

	"github.com/google/uuid"
)

type ListenerAccount struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID          uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	GroupID           *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	Phone             string     `json:"phone" gorm:"size:40;index"`
	Nickname          string     `json:"nickname" gorm:"size:120"`
	AvatarURL         string     `json:"avatar_url" gorm:"size:500"`
	Status            string     `json:"status" gorm:"size:30;index"`
	RiskStatus        string     `json:"risk_status" gorm:"size:30;index"`
	AccessType        string     `json:"access_type" gorm:"size:20;index"`
	FilePath          string     `json:"file_path" gorm:"size:500"`
	SessionHash       string     `json:"session_hash" gorm:"size:128;index"`
	ProxyID           *uuid.UUID `json:"proxy_id" gorm:"type:uuid;index"`
	ExitIP            string     `json:"exit_ip" gorm:"size:80"`
	ExitCountry       string     `json:"exit_country" gorm:"size:80"`
	ExitFlag          string     `json:"exit_flag" gorm:"size:16"`
	JoinedTargets     int64      `json:"joined_targets"`
	LastOnlineAt      *time.Time `json:"last_online_at"`
	LastMessageAt     *time.Time `json:"last_message_at"`
	LastJoinAt        *time.Time `json:"last_join_at"`
	JoinDailyLimit    int        `json:"join_daily_limit"`
	JoinDailyCount    int        `json:"join_daily_count"`
	JoinDailyResetAt  *time.Time `json:"join_daily_reset_at"`
	JoinCooldownUntil *time.Time `json:"join_cooldown_until"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ListenerTarget struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID   uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	GroupID    *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	Identifier string     `json:"identifier" gorm:"size:160;index"`
	Name       string     `json:"name" gorm:"size:160;index"`
	Type       string     `json:"type" gorm:"size:30;index"`
	Size       int64      `json:"size"`
	Status     string     `json:"status" gorm:"size:30;index"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type ListenerProxy struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	GroupID        *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	Code           string     `json:"code" gorm:"size:60;index"`
	IP             string     `json:"ip" gorm:"size:80;index"`
	Port           int        `json:"port"`
	Protocol       string     `json:"protocol" gorm:"size:20;index"`
	Username       string     `json:"username" gorm:"size:120"`
	Password       string     `json:"-" gorm:"size:255"`
	ExitIP         string     `json:"exit_ip" gorm:"size:80"`
	LatencyMS      int        `json:"latency_ms"`
	Country        string     `json:"country" gorm:"size:80"`
	Flag           string     `json:"flag" gorm:"size:16"`
	TelegramStatus string     `json:"telegram_status" gorm:"size:30;index"`
	TelegramError  string     `json:"telegram_error" gorm:"size:255"`
	BoundAccounts  int64      `json:"bound_accounts"`
	Status         string     `json:"status" gorm:"size:30;index"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
