package handlers

import "testing"

func TestParseProxyLine(t *testing.T) {
	tests := []struct {
		line            string
		defaultProtocol string
		wantProtocol    string
		wantIP          string
		wantPort        int
		wantUsername    string
		wantPassword    string
	}{
		{"1.2.3.4:1080", "sk5", "socks5", "1.2.3.4", 1080, "", ""},
		{"http://1.2.3.4:8080", "sk5", "http", "1.2.3.4", 8080, "", ""},
		{"sk5://user:pass@proxy.example.com:1080", "http", "socks5", "proxy.example.com", 1080, "user", "pass"},
		{"socks5://1.2.3.4:1080:user:pass", "http", "socks5", "1.2.3.4", 1080, "user", "pass"},
		{"user:pass@1.2.3.4:1080", "http", "http", "1.2.3.4", 1080, "user", "pass"},
	}

	for _, tt := range tests {
		got, err := parseProxyLine(tt.line, tt.defaultProtocol)
		if err != nil {
			t.Fatalf("parseProxyLine(%q) error: %v", tt.line, err)
		}
		if got.Protocol != tt.wantProtocol || got.IP != tt.wantIP || got.Port != tt.wantPort || got.Username != tt.wantUsername || got.Password != tt.wantPassword {
			t.Fatalf("parseProxyLine(%q) = %+v", tt.line, got)
		}
	}
}

func TestParseProxyLineRejectsUnsupportedProtocol(t *testing.T) {
	if _, err := parseProxyLine("https://1.2.3.4:443", "sk5"); err == nil {
		t.Fatal("expected unsupported protocol error")
	}
}
