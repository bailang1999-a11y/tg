package database

import (
	"context"
	"time"

	"codex3/backend/internal/config"
	"codex3/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type tenantContextKey struct{}

var TenantContextKey = tenantContextKey{}

func Connect(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	maxIdleConns := cfg.DBMaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 10
	}
	maxOpenConns := cfg.DBMaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 50
	}
	connMaxLifetime := cfg.DBConnMaxLifetime
	if connMaxLifetime <= 0 {
		connMaxLifetime = time.Hour
	}
	connMaxIdleTime := cfg.DBConnMaxIdleTime
	if connMaxIdleTime <= 0 {
		connMaxIdleTime = 5 * time.Minute
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	if err := db.Use(TenantPlugin{}); err != nil {
		return nil, err
	}

	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	if err := normalizeConstraints(db); err != nil {
		return err
	}
	return db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.Terminal{},
		&models.NetworkNode{},
		&models.Target{},
		&models.TargetGroupBinding{},
		&models.TerminalTargetRestriction{},
		&models.AccountTargetJoin{},
		&models.ListenerAccount{},
		&models.ListenerTarget{},
		&models.ListenerProxy{},
		&models.Asset{},
		&models.Workflow{},
		&models.SystemSetting{},
		&models.SystemSettingHistory{},
		&models.BotConfig{},
		&models.BotLicense{},
		&models.BotSubscriber{},
		&models.BotPrivateAccountGroup{},
		&models.BotPrivateAccount{},
		&models.BotPrivateUpload{},
		&models.BotDMTask{},
		&models.BotReferral{},
		&models.BotConversationState{},
		&models.BotUserBlacklist{},
		&models.BotSourceBlacklist{},
		&models.Task{},
		&models.TaskLog{},
		&models.SCRMKeywordRule{},
		&models.SCRMLead{},
		&models.SCRMTaskUserBlacklist{},
		&models.SCRMCooldown{},
		&models.SCRMMessage{},
	)
}

func normalizeConstraints(db *gorm.DB) error {
	return db.Exec(`
DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conname = 'users_username_key'
	) AND NOT EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conname = 'uni_users_username'
	) THEN
		ALTER TABLE users RENAME CONSTRAINT users_username_key TO uni_users_username;
	END IF;
END $$;
`).Error
}

func WithTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, TenantContextKey, tenantID)
}

type TenantPlugin struct{}

func (TenantPlugin) Name() string {
	return "tenant_scope"
}

func (TenantPlugin) Initialize(db *gorm.DB) error {
	apply := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Unscoped {
			return
		}
		if _, ok := tx.Statement.Schema.FieldsByDBName["tenant_id"]; !ok {
			return
		}
		tenantID, ok := tx.Statement.Context.Value(TenantContextKey).(uuid.UUID)
		if !ok || tenantID == uuid.Nil {
			return
		}
		tx.Statement.AddClause(clause.Where{
			Exprs: []clause.Expression{
				clause.Eq{
					Column: clause.Column{Table: clause.CurrentTable, Name: "tenant_id"},
					Value:  tenantID,
				},
			},
		})
	}

	if err := db.Callback().Query().Before("gorm:query").Register("tenant_scope:query", apply); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("tenant_scope:update", apply); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("tenant_scope:delete", apply); err != nil {
		return err
	}
	return nil
}
