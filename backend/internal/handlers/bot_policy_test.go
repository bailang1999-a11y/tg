package handlers

import (
	"testing"

	"codex3/backend/internal/models"
)

func TestBotPolicyLimitsAllowValuesBelowOldDefaults(t *testing.T) {
	if got := normalizeBotKeywordLimit(12); got != 12 {
		t.Fatalf("normalizeBotKeywordLimit(12) = %d, want 12", got)
	}
	if got := normalizeBotTrialHours(3); got != 3 {
		t.Fatalf("normalizeBotTrialHours(3) = %d, want 3", got)
	}
	limit := botEffectiveKeywordLimit(models.BotConfig{DefaultKeywordLimit: 12}, models.BotSubscriber{})
	if limit != 12 {
		t.Fatalf("botEffectiveKeywordLimit() = %d, want 12", limit)
	}
}
