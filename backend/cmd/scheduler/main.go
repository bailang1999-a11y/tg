package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"codex3/backend/internal/config"
	"codex3/backend/internal/database"
	"codex3/backend/internal/models"
	appnats "codex3/backend/internal/nats"
	appredis "codex3/backend/internal/redis"
	"codex3/backend/internal/taskqueue"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if _, err := appredis.Connect(cfg); err != nil {
		log.Printf("redis unavailable: %v", err)
	}
	natsClient, err := appnats.Connect(cfg)
	if err != nil {
		log.Printf("nats unavailable: scheduled task dispatch will retry: %v", err)
	}
	if natsClient != nil && natsClient.Conn != nil {
		defer natsClient.Conn.Drain()
	}
	publisher := taskqueue.NewPublisher(natsClient)

	interval := cfg.SchedulerInterval
	if interval <= 0 {
		interval = time.Hour
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runMaintenance(ctx, cfg, db)
	lastMaintenanceAt := time.Now()
	runScheduledListenerAccountChecks(ctx, db, publisher)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	log.Printf("scheduler started: maintenance_interval=%s", interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("scheduler stopping")
			return
		case <-ticker.C:
			if publisher == nil {
				var connectErr error
				natsClient, connectErr = appnats.Connect(cfg)
				if connectErr != nil {
					log.Printf("nats retry failed: %v", connectErr)
				} else {
					log.Println("nats retry connected: scheduled task dispatch enabled")
					publisher = taskqueue.NewPublisher(natsClient)
				}
			}
			if time.Since(lastMaintenanceAt) >= interval {
				runMaintenance(ctx, cfg, db)
				lastMaintenanceAt = time.Now()
			}
			runScheduledListenerAccountChecks(ctx, db, publisher)
		}
	}
}

func runMaintenance(ctx context.Context, cfg config.Config, db *gorm.DB) {
	if !tryAcquireSchedulerLock(ctx, db) {
		log.Println("maintenance skipped: scheduler lock held by another instance")
		return
	}
	defer releaseSchedulerLock(context.Background(), db)

	retentionDays := cfg.LogRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&models.TaskLog{})
	if result.Error != nil {
		log.Printf("maintenance log cleanup failed: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("maintenance log cleanup removed %d rows older than %s", result.RowsAffected, cutoff.Format(time.RFC3339))
	}
	cleanupImportStages(cfg)
}

type scheduledSystemSettings struct {
	ListenerHealth struct {
		AutoAccountCheckEnabled     bool `json:"auto_account_check_enabled"`
		AccountCheckIntervalMinutes int  `json:"account_check_interval_minutes"`
	} `json:"listener_health"`
}

func runScheduledListenerAccountChecks(ctx context.Context, db *gorm.DB, publisher *taskqueue.Publisher) {
	if publisher == nil {
		return
	}
	if !tryAcquireNamedSchedulerLock(ctx, db, scheduledListenerAccountCheckLockKey) {
		return
	}
	defer releaseNamedSchedulerLock(context.Background(), db, scheduledListenerAccountCheckLockKey)

	var setting models.SystemSetting
	if err := db.WithContext(ctx).Order("created_at asc").First(&setting).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Printf("scheduled listener account check read settings failed: %v", err)
		}
		return
	}
	var settings scheduledSystemSettings
	if err := json.Unmarshal(setting.Payload, &settings); err != nil {
		log.Printf("scheduled listener account check parse settings failed: %v", err)
		return
	}
	if settings.ListenerHealth.AccountCheckIntervalMinutes == 0 {
		settings.ListenerHealth.AutoAccountCheckEnabled = true
		settings.ListenerHealth.AccountCheckIntervalMinutes = 60
	}
	if !settings.ListenerHealth.AutoAccountCheckEnabled {
		return
	}
	intervalMinutes := settings.ListenerHealth.AccountCheckIntervalMinutes
	if intervalMinutes <= 0 {
		intervalMinutes = 60
	}
	if intervalMinutes < 5 {
		intervalMinutes = 5
	}
	cutoff := time.Now().Add(-time.Duration(intervalMinutes) * time.Minute)
	var recent int64
	if err := db.WithContext(ctx).Model(&models.Task{}).
		Where("tenant_id = ? AND type = ? AND created_at >= ?", setting.TenantID, "listener_account_check", cutoff).
		Where("status IN ?", []string{"queued", "running", "success", "partial_success", "failed"}).
		Count(&recent).Error; err != nil {
		log.Printf("scheduled listener account check recent task lookup failed: %v", err)
		return
	}
	if recent > 0 {
		return
	}

	payload, _ := json.Marshal(map[string]string{"source": "scheduled"})
	task := models.Task{
		ID:       uuid.New(),
		TenantID: setting.TenantID,
		Name:     "监听账号定时状态检测",
		Type:     "listener_account_check",
		Status:   "queued",
		Progress: 0,
		Payload:  datatypes.JSON(payload),
	}
	if err := db.WithContext(ctx).Create(&task).Error; err != nil {
		log.Printf("scheduled listener account check create task failed: %v", err)
		return
	}
	logTask := models.TaskLog{
		ID:        uuid.New(),
		TenantID:  task.TenantID,
		TaskID:    task.ID,
		Level:     "INFO",
		Category:  "task",
		Action:    "scheduled",
		Details:   "监听账号定时状态检测任务已创建",
		TraceID:   uuid.NewString(),
		CreatedAt: time.Now(),
	}
	_ = db.WithContext(ctx).Create(&logTask).Error
	if err := publisher.PublishTask(ctx, taskqueue.TaskMessage{
		TaskID:    task.ID,
		TenantID:  task.TenantID,
		Type:      task.Type,
		Action:    "run",
		CreatedAt: time.Now(),
	}); err != nil {
		log.Printf("scheduled listener account check publish task failed: %v", err)
		return
	}
	log.Printf("scheduled listener account check queued task id=%s interval=%dm", task.ID, intervalMinutes)
}

const schedulerLockKey int64 = 20260425
const scheduledListenerAccountCheckLockKey int64 = 20260426

func tryAcquireSchedulerLock(ctx context.Context, db *gorm.DB) bool {
	return tryAcquireNamedSchedulerLock(ctx, db, schedulerLockKey)
}

func tryAcquireNamedSchedulerLock(ctx context.Context, db *gorm.DB, key int64) bool {
	var locked bool
	if err := db.WithContext(ctx).Raw("SELECT pg_try_advisory_lock(?)", key).Scan(&locked).Error; err != nil {
		log.Printf("maintenance lock acquire failed: %v", err)
		return false
	}
	return locked
}

func releaseSchedulerLock(ctx context.Context, db *gorm.DB) {
	releaseNamedSchedulerLock(ctx, db, schedulerLockKey)
}

func releaseNamedSchedulerLock(ctx context.Context, db *gorm.DB, key int64) {
	var unlocked bool
	if err := db.WithContext(ctx).Raw("SELECT pg_advisory_unlock(?)", key).Scan(&unlocked).Error; err != nil {
		log.Printf("maintenance lock release failed: %v", err)
	}
}

func cleanupImportStages(cfg config.Config) {
	retentionHours := cfg.ImportStageRetentionHours
	if retentionHours <= 0 {
		retentionHours = 24
	}
	cutoff := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
	tenantsRoot := filepath.Join("storage", "uploads")
	tenantDirs, err := os.ReadDir(tenantsRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("maintenance import stage scan failed: %v", err)
		}
		return
	}
	for _, tenantDir := range tenantDirs {
		if !tenantDir.IsDir() {
			continue
		}
		jobRoot := filepath.Join(tenantsRoot, tenantDir.Name(), "import-jobs")
		jobDirs, err := os.ReadDir(jobRoot)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("maintenance import stage scan failed for %s: %v", jobRoot, err)
			}
			continue
		}
		for _, jobDir := range jobDirs {
			if !jobDir.IsDir() {
				continue
			}
			jobPath := filepath.Join(jobRoot, jobDir.Name())
			info, err := os.Stat(jobPath)
			if err != nil {
				log.Printf("maintenance import stage stat failed for %s: %v", jobPath, err)
				continue
			}
			if info.ModTime().After(cutoff) {
				continue
			}
			if err := os.RemoveAll(jobPath); err != nil {
				log.Printf("maintenance import stage cleanup failed for %s: %v", jobPath, err)
				continue
			}
			log.Printf("maintenance import stage cleanup removed %s", jobPath)
		}
	}
}
