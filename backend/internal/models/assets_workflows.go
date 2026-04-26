package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Asset struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID  uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	GroupID   *uuid.UUID `json:"group_id" gorm:"type:uuid;index"`
	Name      string     `json:"name" gorm:"size:180"`
	MimeType  string     `json:"mime_type" gorm:"size:80"`
	MD5       string     `json:"md5" gorm:"size:64;index"`
	URL       string     `json:"url" gorm:"size:500"`
	FilePath  string     `json:"file_path" gorm:"size:500"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Workflow struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID    uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;not null"`
	Name        string         `json:"name" gorm:"size:160;index"`
	Description string         `json:"description" gorm:"size:500"`
	Definition  datatypes.JSON `json:"definition" gorm:"type:jsonb"`
	Status      string         `json:"status" gorm:"size:30;index"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type SystemSetting struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID  uuid.UUID      `json:"tenant_id" gorm:"type:uuid;uniqueIndex;not null"`
	Payload   datatypes.JSON `json:"payload" gorm:"type:jsonb"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type SystemSettingHistory struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID    uuid.UUID      `json:"tenant_id" gorm:"type:uuid;index;not null"`
	ChangedBy   *uuid.UUID     `json:"changed_by" gorm:"type:uuid;index"`
	Section     string         `json:"section" gorm:"size:40;index"`
	Summary     string         `json:"summary" gorm:"size:500"`
	BeforeValue datatypes.JSON `json:"before_value" gorm:"type:jsonb"`
	AfterValue  datatypes.JSON `json:"after_value" gorm:"type:jsonb"`
	CreatedAt   time.Time      `json:"created_at" gorm:"index"`
}
