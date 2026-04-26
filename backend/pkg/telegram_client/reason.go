package telegram_client

import "strings"

func NormalizeTelegramFailureReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return ""
	}

	lowered := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lowered, "not available for frozen accounts"),
		strings.Contains(lowered, "frozen account"):
		return "账号已被冻结，Telegram 不允许修改资料"
	case strings.Contains(lowered, "database is locked"):
		return "本地会话文件正在被占用，请稍后重试"
	default:
		return trimmed
	}
}

func IsFrozenAccountReason(reason string) bool {
	normalized := strings.TrimSpace(NormalizeTelegramFailureReason(reason))
	if normalized == "" {
		return false
	}
	lowered := strings.ToLower(normalized)
	return strings.Contains(lowered, "frozen account") || strings.Contains(normalized, "账号已被冻结")
}
