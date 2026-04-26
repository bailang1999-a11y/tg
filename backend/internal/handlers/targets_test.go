package handlers

import "testing"

func TestParseTargetLine(t *testing.T) {
	tests := []struct {
		line           string
		wantIdentifier string
		wantName       string
		wantType       string
	}{
		{"https://t.me/AIGOGGGG", "AIGOGGGG", "@AIGOGGGG", "channel"},
		{"t.me/AIGOGGGG", "AIGOGGGG", "@AIGOGGGG", "channel"},
		{"@AIGOGGGG", "AIGOGGGG", "@AIGOGGGG", "channel"},
		{"https://telegram.me/+InviteHash123", "+InviteHash123", "+InviteHash123", "invite"},
		{"https://t.me/joinchat/InviteHash123", "joinchat/InviteHash123", "joinchat/InviteHash123", "invite"},
		{"https://t.me/c/123456/789", "c/123456", "c/123456", "private_channel"},
		{"https://t.me/s/AIGOGGGG", "AIGOGGGG", "@AIGOGGGG", "channel"},
	}

	for _, tt := range tests {
		got, err := parseTargetLine(tt.line)
		if err != nil {
			t.Fatalf("parseTargetLine(%q) error: %v", tt.line, err)
		}
		if got.Identifier != tt.wantIdentifier || got.Name != tt.wantName || got.Type != tt.wantType {
			t.Fatalf("parseTargetLine(%q) = %+v", tt.line, got)
		}
	}
}

func TestParseTargetLineRejectsInvalidTarget(t *testing.T) {
	tests := []string{
		"https://example.com/AIGOGGGG",
		"ftp://t.me/AIGOGGGG",
		"https://t.me/abc",
		"not-a-link",
	}

	for _, line := range tests {
		if _, err := parseTargetLine(line); err == nil {
			t.Fatalf("parseTargetLine(%q) expected error", line)
		}
	}
}
