package telegram_client

import (
	"testing"
	"time"
)

func TestDecodeSyncResultParsesLastOnlineAt(t *testing.T) {
	result, err := decodeSyncResult([]byte(`{
		"ok": true,
		"authorized": true,
		"phone": "14437323987",
		"nickname": "Jalissa Mok",
		"bio": "hello",
		"homepage": "https://t.me/example",
		"avatar_checked": true,
		"avatar_present": true,
		"avatar_path": "/tmp/avatar.jpg",
		"avatar_error": "",
		"status": "offline",
		"last_online_at": "2026-04-08T12:56:02Z",
		"risk_status": "正常",
		"ban_status": "正常",
		"reason": "已同步 Telegram 资料",
		"source": "session"
	}`))
	if err != nil {
		t.Fatalf("decodeSyncResult returned error: %v", err)
	}
	if result.Phone != "14437323987" {
		t.Fatalf("unexpected phone: %q", result.Phone)
	}
	if !result.AvatarChecked || !result.AvatarPresent || result.AvatarPath != "/tmp/avatar.jpg" {
		t.Fatalf("expected avatar metadata to be decoded: %+v", result)
	}
	if result.LastOnlineAt == nil {
		t.Fatal("expected last_online_at to be parsed")
	}
	if got := result.LastOnlineAt.UTC().Format(time.RFC3339); got != "2026-04-08T12:56:02Z" {
		t.Fatalf("unexpected last_online_at: %s", got)
	}
}

func TestResolveInspectorPathHonorsAbsolutePath(t *testing.T) {
	got := resolveInspectorPath("/tmp/codex3/backend", "/opt/custom/python", ".venv/bin/python")
	if got != "/opt/custom/python" {
		t.Fatalf("resolveInspectorPath returned %q", got)
	}
}

func TestBackendRootHonorsEnvironment(t *testing.T) {
	t.Setenv("CODEX3_BACKEND_ROOT", "/app")
	if got := backendRoot(); got != "/app" {
		t.Fatalf("backendRoot returned %q", got)
	}
}
