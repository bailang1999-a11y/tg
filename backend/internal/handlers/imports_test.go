package handlers

import (
	"context"
	"testing"
)

func TestExtractPhonePrefersAccountFileOrFolder(t *testing.T) {
	tests := map[string]string{
		"+1_美国_(2)_500203087451776520591.zip/14437323987/14437323987.session":                 "14437323987",
		"+1_美国_(2)_500203087451776520591.zip/14452719040/tdata/D877F783D5D3EF8C/maps":         "14452719040",
		"+1_美国_(2)_500203087451776520591.zip/14452719040/tdata/key_datas":                     "14452719040",
		"+1_美国_(2)_500203087451776520591.zip/14452719040/key_data":                            "14452719040",
		"+1_美国_(2)_500203087451776520591.zip/key_data":                                        "",
		"/Users/demo/accounts/+1_美国_(2)_14452719040/14452719040.session":                      "14452719040",
		"/Users/demo/accounts/+1_美国_(2)_14452719040/tdata/D877F783D5D3EF8C/D877F783D5D3EF8Cs": "14452719040",
		"/Users/demo/accounts/+1_美国_(2)_14452719040/key_data":                                 "14452719040",
	}

	for input, want := range tests {
		if got := extractPhone(input); got != want {
			t.Fatalf("extractPhone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTDataGroupKeySkipsArchiveContainerName(t *testing.T) {
	tests := []string{
		"+1_美国_(2)_500203087451776520591.zip/key_data",
		"+1_美国_(2)_500203087451776520591.zip/D877F783D5D3EF8C/maps",
	}

	for _, input := range tests {
		if got, ok := tdataGroupKey(input); ok {
			t.Fatalf("tdataGroupKey(%q) = %q, want not detected", input, got)
		}
	}
}

func TestTDataGroupKey(t *testing.T) {
	tests := map[string]string{
		"+1_美国_(2)_500203087451776520591.zip/14452719040/tdata/key_datas":                     "+1_美国_(2)_500203087451776520591.zip/14452719040/tdata",
		"+1_美国_(2)_500203087451776520591.zip/14452719040/key_data":                            "+1_美国_(2)_500203087451776520591.zip/14452719040",
		"accounts/14452719040/tdata/D877F783D5D3EF8C/maps":                                    "accounts/14452719040/tdata",
		"accounts/14452719040/D877F783D5D3EF8C/maps":                                          "accounts/14452719040",
		"/Users/demo/accounts/+1_美国_(2)_14452719040/tdata/D877F783D5D3EF8C/D877F783D5D3EF8Cs": "/Users/demo/accounts/+1_美国_(2)_14452719040/tdata",
		"/Users/demo/accounts/+1_美国_(2)_14452719040/key_data":                                 "/Users/demo/accounts/+1_美国_(2)_14452719040",
	}

	for input, want := range tests {
		got, ok := tdataGroupKey(input)
		if !ok {
			t.Fatalf("tdataGroupKey(%q) not detected", input)
		}
		if got != want {
			t.Fatalf("tdataGroupKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMergeMixedAccountUnitsPrefersTData(t *testing.T) {
	sessionUnits := []importUnit{
		{Name: "bundle/14437323987/14437323987.session", AccessType: "session"},
		{Name: "bundle/14452719040/14452719040.session", AccessType: "session"},
		{Name: "bundle/no-phone/custom.session", AccessType: "session"},
	}
	tdataUnits := []importUnit{
		{Name: "bundle/14437323987/tdata.zip", AccessType: "data"},
	}

	merged, skipped := mergeMixedAccountUnits(sessionUnits, tdataUnits)

	if len(merged) != 3 {
		t.Fatalf("merged units = %d, want 3", len(merged))
	}
	if merged[0].AccessType != "data" || merged[0].Name != "bundle/14437323987/tdata.zip" {
		t.Fatalf("expected first merged unit to be tdata, got %+v", merged[0])
	}
	if merged[1].Name != "bundle/14452719040/14452719040.session" {
		t.Fatalf("expected non-conflicting session to survive, got %+v", merged[1])
	}
	if merged[2].Name != "bundle/no-phone/custom.session" {
		t.Fatalf("expected session without phone to survive, got %+v", merged[2])
	}

	if len(skipped) != 1 {
		t.Fatalf("skipped units = %d, want 1", len(skipped))
	}
	if skipped[0].Unit.Name != "bundle/14437323987/14437323987.session" {
		t.Fatalf("expected colliding session to be skipped, got %+v", skipped[0])
	}
	if skipped[0].Reason != "同账号已识别 TData，Session 已合并跳过" {
		t.Fatalf("unexpected skip reason: %q", skipped[0].Reason)
	}
}

func TestParseProxyExitPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantIP  string
	}{
		{name: "ip api", payload: `{"status":"success","query":"198.51.100.8","country":"Test","countryCode":"US"}`, wantIP: "198.51.100.8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProxyExitPayload(context.Background(), []byte(tt.payload))
			if got.IP != tt.wantIP {
				t.Fatalf("parseProxyExitPayload() IP = %q, want %q", got.IP, tt.wantIP)
			}
		})
	}
}
