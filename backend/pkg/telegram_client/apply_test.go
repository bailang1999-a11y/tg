package telegram_client

import "testing"

func TestDecodeApplyResultWithStructuredFields(t *testing.T) {
	result, err := decodeApplyResult([]byte(`{
		"ok": true,
		"status": "partial_success",
		"reason": "部分资料修改已提交到 Telegram",
		"source": "session",
		"terminal": "14437323987",
		"requested_count": 4,
		"applied_count": 3,
		"failed_count": 1,
		"fields": {
			"nickname": {"requested": true, "ok": true},
			"bio": {"requested": true, "ok": true},
			"homepage": {"requested": true, "ok": false, "reason": "个人频道已被占用"},
			"avatar": {"requested": true, "ok": true}
		}
	}`))
	if err != nil {
		t.Fatalf("decodeApplyResult returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok result")
	}
	if result.Status != "partial_success" {
		t.Fatalf("expected partial_success, got %q", result.Status)
	}
	if result.RequestedCount != 4 || result.AppliedCount != 3 || result.FailedCount != 1 {
		t.Fatalf("unexpected counts: %+v", result)
	}
	if result.Fields["homepage"].Reason != "个人频道已被占用" {
		t.Fatalf("expected homepage reason to survive decode")
	}
}

func TestDecodeApplyResultDerivesCountsFromFields(t *testing.T) {
	result, err := decodeApplyResult([]byte(`{
		"ok": false,
		"reason": "个人频道格式无效",
		"fields": {
			"homepage": {"requested": true, "ok": false, "reason": "个人频道格式无效"},
			"avatar": {"requested": false, "ok": false, "reason": ""}
		}
	}`))
	if err != nil {
		t.Fatalf("decodeApplyResult returned error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %q", result.Status)
	}
	if result.RequestedCount != 1 || result.AppliedCount != 0 || result.FailedCount != 1 {
		t.Fatalf("unexpected derived counts: %+v", result)
	}
}

func TestDecodeApplyResultNormalizesFrozenReason(t *testing.T) {
	result, err := decodeApplyResult([]byte(`{
		"ok": false,
		"reason": "You tried to use a method that is not available for frozen accounts (caused by InvokeWithoutUpdatesRequest(UpdateProfileRequest))",
		"fields": {
			"nickname": {
				"requested": true,
				"ok": false,
				"reason": "You tried to use a method that is not available for frozen accounts (caused by InvokeWithoutUpdatesRequest(UpdateProfileRequest))"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("decodeApplyResult returned error: %v", err)
	}
	if result.Reason != "账号已被冻结，Telegram 不允许修改资料" {
		t.Fatalf("unexpected normalized reason: %q", result.Reason)
	}
	if result.Fields["nickname"].Reason != "账号已被冻结，Telegram 不允许修改资料" {
		t.Fatalf("unexpected normalized field reason: %q", result.Fields["nickname"].Reason)
	}
}
