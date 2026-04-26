package handlers

import "testing"

func TestNormalizeTerminalPhone(t *testing.T) {
	if got := normalizeTerminalPhone("+86 133-6666-8888"); got != "8613366668888" {
		t.Fatalf("unexpected normalized phone: %s", got)
	}
}

func TestSyncTerminalPhoneIdentity(t *testing.T) {
	phone, country, flag := syncTerminalPhoneIdentity("8613366668888", "", "")
	if phone != "8613366668888" {
		t.Fatalf("unexpected phone: %s", phone)
	}
	if country != "中国" || flag != "🇨🇳" {
		t.Fatalf("unexpected origin: %s %s", country, flag)
	}
}

func TestFormatTerminalPhoneDisplay(t *testing.T) {
	if got := formatTerminalPhoneDisplay("8613366668888"); got != "+86 13366668888" {
		t.Fatalf("unexpected phone display: %s", got)
	}
	if got := formatTerminalPhoneDisplay("1326558987"); got != "+1 326558987" {
		t.Fatalf("unexpected us phone display: %s", got)
	}
}

func TestTerminalChannelName(t *testing.T) {
	if got := terminalChannelName("https://t.me/AIGOGGGG"); got != "@AIGOGGGG" {
		t.Fatalf("unexpected channel name: %s", got)
	}
}
