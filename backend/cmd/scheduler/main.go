package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"codex3/backend/internal/config"
	"codex3/backend/internal/database"
	"codex3/backend/internal/models"
	appredis "codex3/backend/internal/redis"

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

	interval := cfg.SchedulerInterval
	if interval <= 0 {
		interval = time.Hour
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runMaintenance(ctx, cfg, db)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("scheduler started: interval=%s", interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("scheduler stopping")
			return
		case <-ticker.C:
			runMaintenance(ctx, cfg, db)
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

const schedulerLockKey int64 = 20260425

func tryAcquireSchedulerLock(ctx context.Context, db *gorm.DB) bool {
	var locked bool
	if err := db.WithContext(ctx).Raw("SELECT pg_try_advisory_lock(?)", schedulerLockKey).Scan(&locked).Error; err != nil {
		log.Printf("maintenance lock acquire failed: %v", err)
		return false
	}
	return locked
}

func releaseSchedulerLock(ctx context.Context, db *gorm.DB) {
	var unlocked bool
	if err := db.WithContext(ctx).Raw("SELECT pg_advisory_unlock(?)", schedulerLockKey).Scan(&unlocked).Error; err != nil {
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
