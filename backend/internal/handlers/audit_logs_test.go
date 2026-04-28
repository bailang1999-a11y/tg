package handlers

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAuditTruncateTextKeepsValidUTF8(t *testing.T) {
	input := strings.Repeat("配置机器人", 400) + "🙂"

	got := auditTruncateText(input, 1000)

	if !utf8.ValidString(got) {
		t.Fatalf("auditTruncateText returned invalid UTF-8")
	}
	if utf8.RuneCountInString(got) > 1000 {
		t.Fatalf("auditTruncateText length = %d, want <= 1000", utf8.RuneCountInString(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("auditTruncateText suffix = %q, want ellipsis", got[len(got)-3:])
	}
}

func TestAuditSafeBytePrefixKeepsValidUTF8(t *testing.T) {
	input := []byte("Webhook 设置失败：需要 HTTPS 公网地址")

	got := auditSafeBytePrefix(input, len("Webhook 设置失败：需")+1)

	if !utf8.Valid(got) {
		t.Fatalf("auditSafeBytePrefix returned invalid UTF-8: %v", got)
	}
	if len(got) > len("Webhook 设置失败：需")+1 {
		t.Fatalf("auditSafeBytePrefix length = %d, want <= limit", len(got))
	}
}
