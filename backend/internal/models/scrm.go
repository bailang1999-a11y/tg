package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type SCRMKeywordRule struct {
	ID                 uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID           uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;not null"`
	OwnerUserID        *uuid.UUID     `json:"owner_user_id" gorm:"type:uuid;index"`
	Name               string         `json:"name" gorm:"size:120"`
	ListenGroupID      *uuid.UUID     `json:"listen_group_id" gorm:"type:uuid;index"`
	StrikeGroupID      *uuid.UUID     `json:"strike_group_id" gorm:"type:uuid;index"`
	MonitorGroupID     *uuid.UUID     `json:"monitor_group_id" gorm:"type:uuid;index"`
	MonitorTerminalIDs datatypes.JSON `json:"monitor_terminal_ids" gorm:"type:jsonb"`
	Keywords           datatypes.JSON `json:"keywords" gorm:"type:jsonb"`
	MatchMode          string         `json:"match_mode" gorm:"size:20;index"`
	PushToBot          bool           `json:"push_to_bot"`
	StrikeEnabled      bool           `json:"strike_enabled"`
	Status             string         `json:"status" gorm:"size:30;index"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type SCRMLead struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;not null"`
	OwnerUserID    *uuid.UUID     `json:"owner_user_id" gorm:"type:uuid;index"`
	SourceTaskID   *uuid.UUID     `json:"source_task_id" gorm:"type:uuid;index"`
	TargetID       uuid.UUID      `json:"target_id" gorm:"type:uuid;index;not null"`
	UserNickname   string         `json:"user_nickname" gorm:"size:160"`
	UserAccount    string         `json:"user_account" gorm:"size:160;index"`
	SourceChatID   string         `json:"source_chat_id" gorm:"size:80"`
	SourceChatName string         `json:"source_chat_name" gorm:"size:255"`
	TriggerWord    string         `json:"trigger_word" gorm:"size:80"`
	TriggerMessage string         `json:"trigger_message" gorm:"size:2000"`
	MessageID      string         `json:"message_id" gorm:"size:80"`
	AssignedWorker *uuid.UUID     `json:"assigned_worker" gorm:"type:uuid;index"`
	Status         string         `json:"status" gorm:"size:30;index"`
	RecentSearches datatypes.JSON `json:"recent_searches" gorm:"type:jsonb"`
	HitAt          *time.Time     `json:"hit_at" gorm:"index"`
	BotPushedAt    *time.Time     `json:"bot_pushed_at" gorm:"index"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type SCRMTaskUserBlacklist struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID     uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null;uniqueIndex:idx_scrm_task_user_blacklist,priority:1"`
	TaskID       uuid.UUID  `json:"task_id" gorm:"type:uuid;index;not null;uniqueIndex:idx_scrm_task_user_blacklist,priority:2"`
	OwnerUserID  *uuid.UUID `json:"owner_user_id,omitempty" gorm:"type:uuid;index"`
	UserKey      string     `json:"user_key" gorm:"size:220;index;not null;uniqueIndex:idx_scrm_task_user_blacklist,priority:3"`
	UserAccount  string     `json:"user_account" gorm:"size:160;index"`
	UserNickname string     `json:"user_nickname" gorm:"size:160"`
	TargetID     *uuid.UUID `json:"target_id,omitempty" gorm:"type:uuid;index"`
	SourceLeadID *uuid.UUID `json:"source_lead_id,omitempty" gorm:"type:uuid;index"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty" gorm:"type:uuid;index"`
	Reason       string     `json:"reason" gorm:"size:255"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type SCRMCooldown struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID `json:"tenant_id" gorm:"type:uuid;index;not null"`
	TargetID      uuid.UUID `json:"target_id" gorm:"type:uuid;index;not null"`
	LastContactAt time.Time `json:"last_contact_at"`
}

type SCRMMessage struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID    uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	LeadID      uuid.UUID  `json:"lead_id" gorm:"type:uuid;index;not null"`
	SenderType  string     `json:"sender_type" gorm:"size:20;index"`
	TerminalID  *uuid.UUID `json:"terminal_id" gorm:"type:uuid;index"`
	Content     string     `json:"content" gorm:"type:text"`
	IsRead      bool       `json:"is_read"`
	MessageTime time.Time  `json:"message_time" gorm:"index"`
	CreatedAt   time.Time  `json:"created_at"`
}
