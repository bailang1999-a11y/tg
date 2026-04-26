package models

import (
	"time"

	"github.com/google/uuid"
)

type Group struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID     uuid.UUID `json:"tenant_id" gorm:"type:uuid;index;not null"`
	ResourceType string    `json:"resource_type" gorm:"size:40;index;not null"`
	Name         string    `json:"name" gorm:"size:100;not null"`
	Description  string    `json:"description" gorm:"size:255"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Terminal struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID          uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	Phone             string     `json:"phone" gorm:"size:40;index"`
	Nickname          string     `json:"nickname" gorm:"size:120"`
	AvatarURL         string     `json:"avatar_url" gorm:"size:500"`
	Bio               string     `json:"bio" gorm:"size:500"`
	Homepage          string     `json:"homepage" gorm:"size:255"`
	Status            string     `json:"status" gorm:"size:30;index"`
	LastOnlineAt      *time.Time `json:"last_online_at"`
	AccessType        string     `json:"access_type" gorm:"size:20;index"`
	OriginCountry     string     `json:"origin_country" gorm:"size:80"`
	OriginFlag        string     `json:"origin_flag" gorm:"size:16"`
	ExitIP            string     `json:"exit_ip" gorm:"size:80"`
	ExitCountry       string     `json:"exit_country" gorm:"size:80"`
	ExitFlag          string     `json:"exit_flag" gorm:"size:16"`
	GroupID           *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	TodaySuccess      int64      `json:"today_success"`
	TotalSuccess      int64      `json:"total_success"`
	TodayFailed       int64      `json:"today_failed"`
	TotalFailed       int64      `json:"total_failed"`
	RiskStatus        string     `json:"risk_status" gorm:"size:30;index"`
	BanStatus         string     `json:"ban_status" gorm:"size:30;index"`
	FilePath          string     `json:"file_path" gorm:"size:500"`
	SessionHash       string     `json:"session_hash" gorm:"size:128;index"`
	SleepUntil        *time.Time `json:"sleep_until"`
	LastMessageAt     *time.Time `json:"last_message_at"`
	DMCooldownUntil   *time.Time `json:"dm_cooldown_until"`
	LastJoinAt        *time.Time `json:"last_join_at"`
	JoinCooldownUntil *time.Time `json:"join_cooldown_until"`
	DMHourlyLimit     int        `json:"dm_hourly_limit"`
	DMDailyLimit      int        `json:"dm_daily_limit"`
	JoinHourlyLimit   int        `json:"join_hourly_limit"`
	JoinDailyLimit    int        `json:"join_daily_limit"`
	DMHourlyCount     int        `json:"dm_hourly_count"`
	DMDailyCount      int        `json:"dm_daily_count"`
	JoinHourlyCount   int        `json:"join_hourly_count"`
	JoinDailyCount    int        `json:"join_daily_count"`
	DMHourlyResetAt   *time.Time `json:"dm_hourly_reset_at"`
	DMDailyResetAt    *time.Time `json:"dm_daily_reset_at"`
	JoinHourlyResetAt *time.Time `json:"join_hourly_reset_at"`
	JoinDailyResetAt  *time.Time `json:"join_daily_reset_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type NetworkNode struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	Code           string     `json:"code" gorm:"size:60;index"`
	IP             string     `json:"ip" gorm:"size:80;index"`
	Port           int        `json:"port"`
	Protocol       string     `json:"protocol" gorm:"size:20;index"`
	Username       string     `json:"username" gorm:"size:120"`
	Password       string     `json:"-" gorm:"size:255"`
	LatencyMS      int        `json:"latency_ms"`
	Country        string     `json:"country" gorm:"size:80"`
	Flag           string     `json:"flag" gorm:"size:16"`
	BoundTerminals int64      `json:"bound_terminals"`
	Status         string     `json:"status" gorm:"size:30;index"`
	GroupID        *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Target struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID          uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	AvatarURL         string     `json:"avatar_url" gorm:"size:500"`
	Identifier        string     `json:"identifier" gorm:"size:160;index"`
	Name              string     `json:"name" gorm:"size:160;index"`
	Type              string     `json:"type" gorm:"size:30;index"`
	Size              int64      `json:"size"`
	GroupID           *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	NotificationCount int64      `json:"notification_count"`
	LinkedTerminals   int64      `json:"linked_terminals"`
	HasVerification   bool       `json:"has_verification"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type TargetGroupBinding struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID  uuid.UUID `json:"tenant_id" gorm:"type:uuid;index;not null"`
	TargetID  uuid.UUID `json:"target_id" gorm:"type:uuid;index;not null;uniqueIndex:idx_target_group_binding"`
	GroupID   uuid.UUID `json:"group_id" gorm:"type:uuid;index;not null;uniqueIndex:idx_target_group_binding"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TerminalTargetRestriction struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;index:idx_terminal_target_restrictions_lookup,priority:1;not null"`
	TerminalID    uuid.UUID  `json:"terminal_id" gorm:"type:uuid;index;index:idx_terminal_target_restrictions_lookup,priority:2;not null"`
	Action        string     `json:"action" gorm:"size:20;index;index:idx_terminal_target_restrictions_lookup,priority:3;not null"`
	TargetType    string     `json:"target_type" gorm:"size:30"`
	TargetValue   string     `json:"target_value" gorm:"size:255"`
	TargetKey     string     `json:"target_key" gorm:"size:320;index;uniqueIndex:idx_terminal_target_restrictions_lookup,priority:4;not null"`
	Reason        string     `json:"reason" gorm:"size:500"`
	FailCount     int        `json:"fail_count"`
	CooldownUntil *time.Time `json:"cooldown_until" gorm:"index"`
	LastFailedAt  *time.Time `json:"last_failed_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type AccountTargetJoin struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;index:idx_account_target_joins_lookup,priority:1;not null"`
	AccountKind   string     `json:"account_kind" gorm:"size:30;index;index:idx_account_target_joins_lookup,priority:2;not null"`
	AccountID     uuid.UUID  `json:"account_id" gorm:"type:uuid;index;index:idx_account_target_joins_lookup,priority:3;not null"`
	TargetID      *uuid.UUID `json:"target_id" gorm:"type:uuid;index"`
	TargetType    string     `json:"target_type" gorm:"size:30"`
	TargetValue   string     `json:"target_value" gorm:"size:255"`
	TargetKey     string     `json:"target_key" gorm:"size:320;index;uniqueIndex:idx_account_target_joins_lookup,priority:4;not null"`
	SourceTaskID  *uuid.UUID `json:"source_task_id" gorm:"type:uuid;index"`
	Status        string     `json:"status" gorm:"size:30;index;default:active"`
	StatusReason  string     `json:"status_reason" gorm:"size:500"`
	Active        bool       `json:"active" gorm:"index;default:true"`
	JoinedAt      time.Time  `json:"joined_at" gorm:"index"`
	LastCheckedAt *time.Time `json:"last_checked_at" gorm:"index"`
	LastSeenAt    *time.Time `json:"last_seen_at" gorm:"index"`
	RemovedAt     *time.Time `json:"removed_at" gorm:"index"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
