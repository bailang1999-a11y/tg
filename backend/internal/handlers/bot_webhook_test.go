package handlers

import (
	"testing"

	"codex3/backend/internal/models"
)

func TestNormalizeBotWebhookURLAcceptsBareDomain(t *testing.T) {
	got := normalizeBotWebhookURL("tg.example.com", "SECRET")
	want := "https://tg.example.com/api/v1/bot/webhook/SECRET"
	if got != want {
		t.Fatalf("normalizeBotWebhookURL() = %q, want %q", got, want)
	}
}

func TestBotWebhookConnectionStatusReportsSuccess(t *testing.T) {
	config := models.BotConfig{WebhookURL: "https://tg.example.com/api/v1/bot/webhook/SECRET"}
	info := map[string]any{
		"result": map[string]any{
			"url":                  "https://tg.example.com/api/v1/bot/webhook/SECRET",
			"pending_update_count": float64(2),
		},
	}

	got := botWebhookConnectionStatus(config, info)

	if connected, _ := got["connected"].(bool); !connected {
		t.Fatalf("connected = %v, want true", got["connected"])
	}
	if message, _ := got["message"].(string); message != "Webhook 连接成功" {
		t.Fatalf("message = %q, want success message", message)
	}
}
