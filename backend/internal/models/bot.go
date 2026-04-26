package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type BotConfig struct {
	ID                  uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID            uuid.UUID      `json:"tenant_id" gorm:"type:uuid;uniqueIndex;not null"`
	Name                string         `json:"name" gorm:"size:120"`
	Token               string         `json:"token" gorm:"size:255"`
	Username            string         `json:"username" gorm:"size:120"`
	PushChatID          string         `json:"push_chat_id" gorm:"size:120"`
	AdminChatID         string         `json:"admin_chat_id" gorm:"size:120"`
	AdminContact        string         `json:"admin_contact" gorm:"size:255"`
	Enabled             bool           `json:"enabled"`
	Running             bool           `json:"running"`
	ForceJoinEnabled    bool           `json:"force_join_enabled"`
	ForceJoinURL        string         `json:"force_join_url" gorm:"size:500"`
	ForceJoinHandle     string         `json:"force_join_handle" gorm:"size:120"`
	TrialEnabled        bool           `json:"trial_enabled"`
	TrialHours          int            `json:"trial_hours"`
	TrialFeatures       datatypes.JSON `json:"trial_features" gorm:"type:jsonb"`
	EnabledCommands     datatypes.JSON `json:"enabled_commands" gorm:"type:jsonb"`
	CommandLabels       datatypes.JSON `json:"command_labels" gorm:"type:jsonb"`
	DefaultKeywords     datatypes.JSON `json:"default_keywords" gorm:"type:jsonb"`
	DefaultKeywordLimit int            `json:"default_keyword_limit"`
	DefaultMatchMode    string         `json:"default_match_mode" gorm:"size:20"`
	PrivateTerminalIDs  datatypes.JSON `json:"private_terminal_ids" gorm:"type:jsonb"`
	WelcomeTitle        string         `json:"welcome_title" gorm:"size:180"`
	ServiceOverview     string         `json:"service_overview" gorm:"size:1200"`
	QuickStartText      string         `json:"quick_start_text" gorm:"size:800"`
	FAQText             string         `json:"faq_text" gorm:"size:2000"`
	SupportText         string         `json:"support_text" gorm:"size:1200"`
	MenuInfoLabel       string         `json:"menu_info_label" gorm:"size:80"`
	MenuSettingsLabel   string         `json:"menu_settings_label" gorm:"size:80"`
	MenuFAQLabel        string         `json:"menu_faq_label" gorm:"size:80"`
	MenuSupportLabel    string         `json:"menu_support_label" gorm:"size:80"`
	MenuPlaceholder     string         `json:"menu_placeholder" gorm:"size:120"`
	ButtonLabels        datatypes.JSON `json:"button_labels" gorm:"type:jsonb"`
	ReplyTemplates      datatypes.JSON `json:"reply_templates" gorm:"type:jsonb"`
	DefaultDMMessages   datatypes.JSON `json:"default_dm_messages" gorm:"type:jsonb"`
	DMMinDelaySeconds   int            `json:"dm_min_delay_seconds"`
	DMMaxDelaySeconds   int            `json:"dm_max_delay_seconds"`
	DMMaxMessages       int            `json:"dm_max_messages"`
	WebhookURL          string         `json:"webhook_url" gorm:"size:500"`
	WebhookSecret       string         `json:"webhook_secret" gorm:"size:120;index"`
	LastWebhookStatus   string         `json:"last_webhook_status" gorm:"size:30"`
	LastWebhookMessage  string         `json:"last_webhook_message" gorm:"size:500"`
	LastWebhookAt       *time.Time     `json:"last_webhook_at"`
	LastTestStatus      string         `json:"last_test_status" gorm:"size:30"`
	LastTestMessage     string         `json:"last_test_message" gorm:"size:500"`
	LastTestAt          *time.Time     `json:"last_test_at"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type BotLicense struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	Code          string     `json:"code" gorm:"size:80;uniqueIndex;not null"`
	Status        string     `json:"status" gorm:"size:30;index"`
	DurationHour  int        `json:"duration_hour"`
	MaxBind       int        `json:"max_bind"`
	BoundCount    int        `json:"bound_count"`
	BoundUserID   string     `json:"bound_user_id" gorm:"size:80;index"`
	BoundUsername string     `json:"bound_username" gorm:"size:120"`
	UsedAt        *time.Time `json:"used_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type BotSubscriber struct {
	ID                       uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID                 uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;index:idx_bot_subscribers_tenant_user,priority:1;index:idx_bot_subscribers_tenant_status_updated,priority:1;not null"`
	TelegramUserID           string         `json:"telegram_user_id" gorm:"size:80;index;not null"`
	UserID                   *uuid.UUID     `json:"user_id" gorm:"type:uuid;index;index:idx_bot_subscribers_tenant_user,priority:2"`
	Username                 string         `json:"username" gorm:"size:120"`
	Nickname                 string         `json:"nickname" gorm:"size:160"`
	InviteCode               string         `json:"invite_code" gorm:"size:80;index"`
	InvitedByID              *uuid.UUID     `json:"invited_by_id" gorm:"type:uuid;index"`
	ForceJoined              bool           `json:"force_joined"`
	PushEnabled              bool           `json:"push_enabled"`
	PushChatID               string         `json:"push_chat_id" gorm:"size:120"`
	Keywords                 datatypes.JSON `json:"keywords" gorm:"type:jsonb"`
	KeywordLimit             int            `json:"keyword_limit"`
	MatchMode                string         `json:"match_mode" gorm:"size:20"`
	KeywordBlacklistEnabled  bool           `json:"keyword_blacklist_enabled"`
	UserBlacklistEnabled     bool           `json:"user_blacklist_enabled"`
	GroupBlacklistEnabled    bool           `json:"group_blacklist_enabled"`
	NicknameBlacklistEnabled bool           `json:"nickname_blacklist_enabled"`
	OnlyUsernameMessages     bool           `json:"only_username_messages"`
	RiskControlEnabled       bool           `json:"risk_control_enabled"`
	SearchHistoryEnabled     bool           `json:"search_history_enabled"`
	PushIntervalMinutes      int            `json:"push_interval_minutes"`
	MessageDedupMinutes      int            `json:"message_dedup_minutes"`
	DMQuotaTotal             int64          `json:"dm_quota_total"`
	DMQuotaUsed              int64          `json:"dm_quota_used"`
	PrivateTerminalGroupIDs  datatypes.JSON `json:"private_terminal_group_ids" gorm:"type:jsonb"`
	Status                   string         `json:"status" gorm:"size:30;index;index:idx_bot_subscribers_tenant_status_updated,priority:2"`
	Plan                     string         `json:"plan" gorm:"size:30;index"`
	TrialStartedAt           *time.Time     `json:"trial_started_at"`
	TrialEndsAt              *time.Time     `json:"trial_ends_at"`
	LicenseID                *uuid.UUID     `json:"license_id" gorm:"type:uuid;index"`
	AuthorizedAt             *time.Time     `json:"authorized_at"`
	ExpiresAt                *time.Time     `json:"expires_at"`
	LastSeenAt               *time.Time     `json:"last_seen_at"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at" gorm:"index:idx_bot_subscribers_tenant_status_updated,priority:3"`
}

type BotPrivateAccountGroup struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID     uuid.UUID `json:"tenant_id" gorm:"type:uuid;index;index:idx_bot_private_account_groups_tenant_subscriber,priority:1;not null"`
	SubscriberID uuid.UUID `json:"subscriber_id" gorm:"type:uuid;index;index:idx_bot_private_account_groups_tenant_subscriber,priority:2;not null"`
	Name         string    `json:"name" gorm:"size:120;not null"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BotPrivateAccount struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID      uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;index:idx_bot_private_accounts_tenant_subscriber,priority:1;not null"`
	SubscriberID  uuid.UUID  `json:"subscriber_id" gorm:"type:uuid;index;index:idx_bot_private_accounts_tenant_subscriber,priority:2;not null"`
	GroupID       *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	Phone         string     `json:"phone" gorm:"size:40;index"`
	Nickname      string     `json:"nickname" gorm:"size:120"`
	Status        string     `json:"status" gorm:"size:30;index"`
	RiskStatus    string     `json:"risk_status" gorm:"size:30;index"`
	AccessType    string     `json:"access_type" gorm:"size:20"`
	FilePath      string     `json:"file_path" gorm:"size:500"`
	SessionHash   string     `json:"session_hash" gorm:"size:128;index"`
	LastMessageAt *time.Time `json:"last_message_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type BotPrivateUpload struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID     uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;index:idx_bot_private_uploads_tenant_subscriber,priority:1;not null"`
	SubscriberID uuid.UUID      `json:"subscriber_id" gorm:"type:uuid;index;index:idx_bot_private_uploads_tenant_subscriber,priority:2;not null"`
	GroupID      *uuid.UUID     `json:"group_id" gorm:"type:uuid;index"`
	FileName     string         `json:"file_name" gorm:"size:240"`
	Total        int            `json:"total"`
	Success      int            `json:"success"`
	Duplicate    int            `json:"duplicate"`
	Failed       int            `json:"failed"`
	Summary      datatypes.JSON `json:"summary" gorm:"type:jsonb"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type BotDMTask struct {
	ID               uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID         uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;index:idx_bot_dm_tasks_tenant_subscriber_created,priority:1;index:idx_bot_dm_tasks_tenant_status_created,priority:1;not null"`
	SubscriberID     uuid.UUID      `json:"subscriber_id" gorm:"type:uuid;index;index:idx_bot_dm_tasks_tenant_subscriber_created,priority:2;not null"`
	Name             string         `json:"name" gorm:"size:160"`
	AccountGroupID   *uuid.UUID     `json:"account_group_id" gorm:"type:uuid;index"`
	AccountGroupName string         `json:"account_group_name" gorm:"size:120"`
	CooldownMinutes  int            `json:"cooldown_minutes"`
	Keywords         datatypes.JSON `json:"keywords" gorm:"type:jsonb"`
	Messages         datatypes.JSON `json:"messages" gorm:"type:jsonb"`
	MinDelaySeconds  int            `json:"min_delay_seconds"`
	MaxDelaySeconds  int            `json:"max_delay_seconds"`
	SentCount        int64          `json:"sent_count"`
	Status           string         `json:"status" gorm:"size:30;index;index:idx_bot_dm_tasks_tenant_status_created,priority:2"`
	StartedAt        *time.Time     `json:"started_at"`
	EndedAt          *time.Time     `json:"ended_at"`
	CreatedAt        time.Time      `json:"created_at" gorm:"index:idx_bot_dm_tasks_tenant_subscriber_created,priority:3;index:idx_bot_dm_tasks_tenant_status_created,priority:3"`
	UpdatedAt        time.Time      `json:"updated_at" gorm:"index"`
}

type BotReferral struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID    uuid.UUID `json:"tenant_id" gorm:"type:uuid;index;index:idx_bot_referrals_tenant_inviter,priority:1;not null"`
	InviterID   uuid.UUID `json:"inviter_id" gorm:"type:uuid;index;index:idx_bot_referrals_tenant_inviter,priority:2;not null"`
	InviteeID   uuid.UUID `json:"invitee_id" gorm:"type:uuid;index;not null;uniqueIndex"`
	RewardHours int       `json:"reward_hours"`
	CreatedAt   time.Time `json:"created_at"`
}

type BotConversationState struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;not null"`
	SubscriberID   uuid.UUID      `json:"subscriber_id" gorm:"type:uuid;index;not null"`
	TelegramUserID string         `json:"telegram_user_id" gorm:"size:80;index"`
	State          string         `json:"state" gorm:"size:80;index"`
	Payload        datatypes.JSON `json:"payload" gorm:"type:jsonb"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type BotUserBlacklist struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID `json:"tenant_id" gorm:"type:uuid;index;not null"`
	SubscriberID   uuid.UUID `json:"subscriber_id" gorm:"type:uuid;index;not null;uniqueIndex:idx_bot_user_blacklist"`
	UserAccount    string    `json:"user_account" gorm:"size:160;index;uniqueIndex:idx_bot_user_blacklist"`
	UserNickname   string    `json:"user_nickname" gorm:"size:160"`
	SourceChatName string    `json:"source_chat_name" gorm:"size:255"`
	Reason         string    `json:"reason" gorm:"size:255"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BotSourceBlacklist struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID       uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	SubscriberID   uuid.UUID  `json:"subscriber_id" gorm:"type:uuid;index;not null;uniqueIndex:idx_bot_source_blacklist"`
	SourceKey      string     `json:"source_key" gorm:"size:180;index;not null;uniqueIndex:idx_bot_source_blacklist"`
	SourceChatID   string     `json:"source_chat_id" gorm:"size:80;index"`
	SourceChatName string     `json:"source_chat_name" gorm:"size:255"`
	TargetID       *uuid.UUID `json:"target_id" gorm:"type:uuid;index"`
	Reason         string     `json:"reason" gorm:"size:255"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
