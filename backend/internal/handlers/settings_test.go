package handlers

import "testing"

func TestNormalizeSystemSettings(t *testing.T) {
	input := systemSettingsPayload{
		Security: systemSecuritySettings{
			EnforceTenantIsolation: false,
			RequireAdminApproval:   false,
			MaskSensitiveLogs:      false,
		},
		Frequency: systemFrequencySettings{
			MaxConcurrentTasks:     0,
			MaxConcurrentOutreach:  99,
			WSLogBatchSize:         5,
			DashboardRefreshSecond: 900,
		},
		Audit: systemAuditSettings{
			LogRetentionDays:  1,
			RealtimeLogStream: false,
			NotifyOnFailure:   false,
		},
		Adapter: systemAdapterSettings{
			TelegramSyncEnabled:  false,
			TelegramApplyEnabled: true,
			OutreachDryRun:       false,
			WorkflowDryRun:       false,
		},
	}

	got := normalizeSystemSettings(input)

	if got.Security.EnforceTenantIsolation {
		t.Fatalf("expected tenant isolation to keep false")
	}
	if got.Frequency.MaxConcurrentTasks != 12 {
		t.Fatalf("expected default task concurrency, got %d", got.Frequency.MaxConcurrentTasks)
	}
	if got.Frequency.MaxConcurrentOutreach != 32 {
		t.Fatalf("expected outreach concurrency clamp, got %d", got.Frequency.MaxConcurrentOutreach)
	}
	if got.Frequency.WSLogBatchSize != 20 {
		t.Fatalf("expected ws batch clamp, got %d", got.Frequency.WSLogBatchSize)
	}
	if got.Frequency.DashboardRefreshSecond != 300 {
		t.Fatalf("expected dashboard refresh clamp, got %d", got.Frequency.DashboardRefreshSecond)
	}
	if got.Audit.LogRetentionDays != 7 {
		t.Fatalf("expected audit retention clamp, got %d", got.Audit.LogRetentionDays)
	}
	if got.Adapter.TelegramApplyEnabled != true {
		t.Fatalf("expected adapter flag to keep true")
	}
	if got.Adapter.OutreachDryRun {
		t.Fatalf("expected outreach dry run to keep false")
	}
}
