package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	TenantID         uuid.UUID  `json:"tenant_id" gorm:"type:uuid;index;not null"`
	Username         string     `json:"username" gorm:"size:80;uniqueIndex;not null"`
	PasswordHash     string     `json:"-" gorm:"size:255;not null"`
	Email            string     `json:"email" gorm:"size:180"`
	Role             string     `json:"role" gorm:"size:20;index;not null"`
	Status           string     `json:"status" gorm:"size:20;index;not null"`
	TelegramUserID   string     `json:"telegram_user_id" gorm:"size:80;index"`
	TelegramUsername string     `json:"telegram_username" gorm:"size:120"`
	TrialEndsAt      *time.Time `json:"trial_ends_at"`
	CreatedBy        *uuid.UUID `json:"created_by" gorm:"type:uuid"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
