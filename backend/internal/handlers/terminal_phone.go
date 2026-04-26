package handlers

import (
	"net/url"
	"path"
	"strings"
	"time"

	"codex3/backend/internal/models"
)

type phoneRegion struct {
	Code    string
	Country string
	Flag    string
}

// Best-effort calling code map ordered from long to short so prefix matching
// prefers the most specific country/region code.
var phoneRegions = []phoneRegion{
	{Code: "998", Country: "乌兹别克斯坦", Flag: "🇺🇿"},
	{Code: "996", Country: "吉尔吉斯斯坦", Flag: "🇰🇬"},
	{Code: "995", Country: "格鲁吉亚", Flag: "🇬🇪"},
	{Code: "994", Country: "阿塞拜疆", Flag: "🇦🇿"},
	{Code: "993", Country: "土库曼斯坦", Flag: "🇹🇲"},
	{Code: "992", Country: "塔吉克斯坦", Flag: "🇹🇯"},
	{Code: "977", Country: "尼泊尔", Flag: "🇳🇵"},
	{Code: "976", Country: "蒙古", Flag: "🇲🇳"},
	{Code: "975", Country: "不丹", Flag: "🇧🇹"},
	{Code: "974", Country: "卡塔尔", Flag: "🇶🇦"},
	{Code: "973", Country: "巴林", Flag: "🇧🇭"},
	{Code: "972", Country: "以色列", Flag: "🇮🇱"},
	{Code: "971", Country: "阿联酋", Flag: "🇦🇪"},
	{Code: "970", Country: "巴勒斯坦", Flag: "🇵🇸"},
	{Code: "968", Country: "阿曼", Flag: "🇴🇲"},
	{Code: "967", Country: "也门", Flag: "🇾🇪"},
	{Code: "966", Country: "沙特阿拉伯", Flag: "🇸🇦"},
	{Code: "965", Country: "科威特", Flag: "🇰🇼"},
	{Code: "964", Country: "伊拉克", Flag: "🇮🇶"},
	{Code: "963", Country: "叙利亚", Flag: "🇸🇾"},
	{Code: "962", Country: "约旦", Flag: "🇯🇴"},
	{Code: "961", Country: "黎巴嫩", Flag: "🇱🇧"},
	{Code: "960", Country: "马尔代夫", Flag: "🇲🇻"},
	{Code: "886", Country: "中国台湾", Flag: "🇹🇼"},
	{Code: "880", Country: "孟加拉国", Flag: "🇧🇩"},
	{Code: "856", Country: "老挝", Flag: "🇱🇦"},
	{Code: "855", Country: "柬埔寨", Flag: "🇰🇭"},
	{Code: "853", Country: "中国澳门", Flag: "🇲🇴"},
	{Code: "852", Country: "中国香港", Flag: "🇭🇰"},
	{Code: "251", Country: "埃塞俄比亚", Flag: "🇪🇹"},
	{Code: "250", Country: "卢旺达", Flag: "🇷🇼"},
	{Code: "249", Country: "苏丹", Flag: "🇸🇩"},
	{Code: "244", Country: "安哥拉", Flag: "🇦🇴"},
	{Code: "243", Country: "刚果（金）", Flag: "🇨🇩"},
	{Code: "242", Country: "刚果（布）", Flag: "🇨🇬"},
	{Code: "241", Country: "加蓬", Flag: "🇬🇦"},
	{Code: "240", Country: "赤道几内亚", Flag: "🇬🇶"},
	{Code: "239", Country: "圣多美和普林西比", Flag: "🇸🇹"},
	{Code: "238", Country: "佛得角", Flag: "🇨🇻"},
	{Code: "237", Country: "喀麦隆", Flag: "🇨🇲"},
	{Code: "236", Country: "中非", Flag: "🇨🇫"},
	{Code: "235", Country: "乍得", Flag: "🇹🇩"},
	{Code: "234", Country: "尼日利亚", Flag: "🇳🇬"},
	{Code: "233", Country: "加纳", Flag: "🇬🇭"},
	{Code: "232", Country: "塞拉利昂", Flag: "🇸🇱"},
	{Code: "231", Country: "利比里亚", Flag: "🇱🇷"},
	{Code: "230", Country: "毛里求斯", Flag: "🇲🇺"},
	{Code: "228", Country: "多哥", Flag: "🇹🇬"},
	{Code: "227", Country: "尼日尔", Flag: "🇳🇪"},
	{Code: "226", Country: "布基纳法索", Flag: "🇧🇫"},
	{Code: "225", Country: "科特迪瓦", Flag: "🇨🇮"},
	{Code: "224", Country: "几内亚", Flag: "🇬🇳"},
	{Code: "223", Country: "马里", Flag: "🇲🇱"},
	{Code: "222", Country: "毛里塔尼亚", Flag: "🇲🇷"},
	{Code: "221", Country: "塞内加尔", Flag: "🇸🇳"},
	{Code: "220", Country: "冈比亚", Flag: "🇬🇲"},
	{Code: "218", Country: "利比亚", Flag: "🇱🇾"},
	{Code: "216", Country: "突尼斯", Flag: "🇹🇳"},
	{Code: "213", Country: "阿尔及利亚", Flag: "🇩🇿"},
	{Code: "212", Country: "摩洛哥", Flag: "🇲🇦"},
	{Code: "98", Country: "伊朗", Flag: "🇮🇷"},
	{Code: "95", Country: "缅甸", Flag: "🇲🇲"},
	{Code: "94", Country: "斯里兰卡", Flag: "🇱🇰"},
	{Code: "93", Country: "阿富汗", Flag: "🇦🇫"},
	{Code: "92", Country: "巴基斯坦", Flag: "🇵🇰"},
	{Code: "91", Country: "印度", Flag: "🇮🇳"},
	{Code: "90", Country: "土耳其", Flag: "🇹🇷"},
	{Code: "86", Country: "中国", Flag: "🇨🇳"},
	{Code: "84", Country: "越南", Flag: "🇻🇳"},
	{Code: "82", Country: "韩国", Flag: "🇰🇷"},
	{Code: "81", Country: "日本", Flag: "🇯🇵"},
	{Code: "66", Country: "泰国", Flag: "🇹🇭"},
	{Code: "65", Country: "新加坡", Flag: "🇸🇬"},
	{Code: "64", Country: "新西兰", Flag: "🇳🇿"},
	{Code: "63", Country: "菲律宾", Flag: "🇵🇭"},
	{Code: "62", Country: "印度尼西亚", Flag: "🇮🇩"},
	{Code: "61", Country: "澳大利亚", Flag: "🇦🇺"},
	{Code: "60", Country: "马来西亚", Flag: "🇲🇾"},
	{Code: "58", Country: "委内瑞拉", Flag: "🇻🇪"},
	{Code: "57", Country: "哥伦比亚", Flag: "🇨🇴"},
	{Code: "56", Country: "智利", Flag: "🇨🇱"},
	{Code: "55", Country: "巴西", Flag: "🇧🇷"},
	{Code: "54", Country: "阿根廷", Flag: "🇦🇷"},
	{Code: "53", Country: "古巴", Flag: "🇨🇺"},
	{Code: "52", Country: "墨西哥", Flag: "🇲🇽"},
	{Code: "51", Country: "秘鲁", Flag: "🇵🇪"},
	{Code: "49", Country: "德国", Flag: "🇩🇪"},
	{Code: "48", Country: "波兰", Flag: "🇵🇱"},
	{Code: "47", Country: "挪威", Flag: "🇳🇴"},
	{Code: "46", Country: "瑞典", Flag: "🇸🇪"},
	{Code: "45", Country: "丹麦", Flag: "🇩🇰"},
	{Code: "44", Country: "英国", Flag: "🇬🇧"},
	{Code: "43", Country: "奥地利", Flag: "🇦🇹"},
	{Code: "41", Country: "瑞士", Flag: "🇨🇭"},
	{Code: "40", Country: "罗马尼亚", Flag: "🇷🇴"},
	{Code: "39", Country: "意大利", Flag: "🇮🇹"},
	{Code: "36", Country: "匈牙利", Flag: "🇭🇺"},
	{Code: "34", Country: "西班牙", Flag: "🇪🇸"},
	{Code: "33", Country: "法国", Flag: "🇫🇷"},
	{Code: "32", Country: "比利时", Flag: "🇧🇪"},
	{Code: "31", Country: "荷兰", Flag: "🇳🇱"},
	{Code: "30", Country: "希腊", Flag: "🇬🇷"},
	{Code: "27", Country: "南非", Flag: "🇿🇦"},
	{Code: "20", Country: "埃及", Flag: "🇪🇬"},
	{Code: "7", Country: "俄罗斯", Flag: "🇷🇺"},
	{Code: "1", Country: "美国", Flag: "🇺🇸"},
}

type terminalListItem struct {
	ID                string  `json:"id"`
	Phone             string  `json:"phone"`
	PhoneDisplay      string  `json:"phone_display"`
	Nickname          string  `json:"nickname"`
	AvatarURL         string  `json:"avatar_url"`
	Bio               string  `json:"bio"`
	Homepage          string  `json:"homepage"`
	ChannelName       string  `json:"channel_name"`
	Status            string  `json:"status"`
	StatusText        string  `json:"status_text"`
	AccountStatus     string  `json:"account_status"`
	AccountText       string  `json:"account_status_text"`
	OnlineStatus      string  `json:"online_status"`
	OnlineText        string  `json:"online_status_text"`
	LastOnlineAt      *string `json:"last_online_at"`
	LastMessageAt     *string `json:"last_message_at"`
	SleepUntil        *string `json:"sleep_until"`
	DMCooldownUntil   *string `json:"dm_cooldown_until"`
	LastJoinAt        *string `json:"last_join_at"`
	JoinCooldownUntil *string `json:"join_cooldown_until"`
	AccessType        string  `json:"access_type"`
	OriginCountry     string  `json:"origin_country"`
	OriginFlag        string  `json:"origin_flag"`
	ExitIP            string  `json:"exit_ip"`
	ExitCountry       string  `json:"exit_country"`
	ExitFlag          string  `json:"exit_flag"`
	GroupID           *string `json:"group_id"`
	TodaySuccess      int64   `json:"today_success"`
	TotalSuccess      int64   `json:"total_success"`
	TodayFailed       int64   `json:"today_failed"`
	TotalFailed       int64   `json:"total_failed"`
	RiskStatus        string  `json:"risk_status"`
	BanStatus         string  `json:"ban_status"`
	DMHourlyLimit     int     `json:"dm_hourly_limit"`
	DMDailyLimit      int     `json:"dm_daily_limit"`
	JoinHourlyLimit   int     `json:"join_hourly_limit"`
	JoinDailyLimit    int     `json:"join_daily_limit"`
	DMHourlyCount     int     `json:"dm_hourly_count"`
	DMDailyCount      int     `json:"dm_daily_count"`
	JoinHourlyCount   int     `json:"join_hourly_count"`
	JoinDailyCount    int     `json:"join_daily_count"`
	DMHourlyResetAt   *string `json:"dm_hourly_reset_at"`
	DMDailyResetAt    *string `json:"dm_daily_reset_at"`
	JoinHourlyResetAt *string `json:"join_hourly_reset_at"`
	JoinDailyResetAt  *string `json:"join_daily_reset_at"`
}

func normalizeTerminalPhone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func detectPhoneRegion(phone string) phoneRegion {
	digits := normalizeTerminalPhone(phone)
	if digits == "" {
		return phoneRegion{}
	}
	for _, region := range phoneRegions {
		if strings.HasPrefix(digits, region.Code) {
			return region
		}
	}
	return phoneRegion{}
}

func syncTerminalPhoneIdentity(phone, existingCountry, existingFlag string) (string, string, string) {
	normalized := normalizeTerminalPhone(phone)
	if normalized == "" {
		return "", fallbackCountry(existingCountry), strings.TrimSpace(existingFlag)
	}

	region := detectPhoneRegion(normalized)
	if region.Country != "" {
		return normalized, region.Country, region.Flag
	}

	return normalized, fallbackCountry(existingCountry), strings.TrimSpace(existingFlag)
}

func fallbackCountry(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未知"
	}
	return value
}

func formatTerminalPhoneDisplay(phone string) string {
	digits := normalizeTerminalPhone(phone)
	if digits == "" {
		return ""
	}

	region := detectPhoneRegion(digits)
	if region.Code == "" {
		return digits
	}
	if len(digits) <= len(region.Code) {
		return "+" + region.Code
	}
	return "+" + region.Code + " " + digits[len(region.Code):]
}

func terminalChannelName(homepage string) string {
	value := strings.TrimSpace(homepage)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "@") {
		return value
	}
	if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
		parsed, err := url.Parse(value)
		if err == nil {
			handle := strings.Trim(path.Base(parsed.Path), "/")
			handle = strings.TrimSpace(handle)
			if handle != "" && handle != "." {
				if strings.HasPrefix(handle, "@") {
					return handle
				}
				return "@" + handle
			}
		}
	}
	return value
}

func terminalStatusText(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return "在线"
	case "offline":
		return "离线"
	case "abnormal":
		return "异常"
	case "checking":
		return "检测中"
	case "pending":
		return "待处理"
	case "queued":
		return "排队中"
	case "running":
		return "执行中"
	case "paused":
		return "已暂停"
	case "success":
		return "成功"
	case "failed":
		return "失败"
	case "disabled":
		return "已禁用"
	default:
		if strings.TrimSpace(status) == "" {
			return "未知"
		}
		return status
	}
}

func terminalAccountStatus(item models.Terminal) (string, string) {
	risk := strings.TrimSpace(item.RiskStatus)
	ban := strings.TrimSpace(item.BanStatus)
	normalized := strings.ToLower(strings.TrimSpace(item.Status))
	if isProfileRestrictedStatus(risk, ban) {
		return "abnormal", "资料受限"
	}

	if normalized == "abnormal" {
		if risk != "" && risk != "正常" {
			return "abnormal", risk
		}
		return "abnormal", "异常"
	}
	if ban != "" && ban != "正常" {
		return "abnormal", ban
	}
	if risk != "" && risk != "正常" {
		return "warning", risk
	}
	return "normal", "正常"
}

func isProfileRestrictedStatus(risk, ban string) bool {
	text := strings.ToLower(strings.TrimSpace(risk + " " + ban))
	return strings.Contains(text, "冻结") || strings.Contains(text, "frozen") || strings.Contains(text, "受限")
}

func mergeTerminalRiskStatus(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if isProfileRestrictedStatus(existing, "") && (incoming == "" || incoming == "正常") {
		return existing
	}
	return firstNonEmpty(incoming, existing)
}

func mergeTerminalBanStatus(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if isProfileRestrictedStatus("", existing) && (incoming == "" || incoming == "正常") {
		return existing
	}
	return firstNonEmpty(incoming, existing)
}

func terminalOnlineStatus(status string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return "online", "在线"
	case "offline":
		return "offline", "离线"
	case "abnormal":
		return "unknown", "未确认"
	default:
		return "unknown", "未同步"
	}
}

func buildTerminalListItem(item models.Terminal) terminalListItem {
	normalizedPhone, originCountry, originFlag := syncTerminalPhoneIdentity(item.Phone, item.OriginCountry, item.OriginFlag)

	var lastOnlineAt *string
	if item.LastOnlineAt != nil {
		formatted := item.LastOnlineAt.Format(time.RFC3339)
		lastOnlineAt = &formatted
	}
	var lastMessageAt *string
	if item.LastMessageAt != nil {
		formatted := item.LastMessageAt.Format(time.RFC3339)
		lastMessageAt = &formatted
	}
	var sleepUntil *string
	if item.SleepUntil != nil {
		formatted := item.SleepUntil.Format(time.RFC3339)
		sleepUntil = &formatted
	}

	var groupID *string
	if item.GroupID != nil {
		value := item.GroupID.String()
		groupID = &value
	}
	accountStatus, accountText := terminalAccountStatus(item)
	onlineStatus, onlineText := terminalOnlineStatus(item.Status)

	return terminalListItem{
		ID:                item.ID.String(),
		Phone:             normalizedPhone,
		PhoneDisplay:      formatTerminalPhoneDisplay(normalizedPhone),
		Nickname:          item.Nickname,
		AvatarURL:         item.AvatarURL,
		Bio:               item.Bio,
		Homepage:          item.Homepage,
		ChannelName:       terminalChannelName(item.Homepage),
		Status:            item.Status,
		StatusText:        terminalStatusText(item.Status),
		AccountStatus:     accountStatus,
		AccountText:       accountText,
		OnlineStatus:      onlineStatus,
		OnlineText:        onlineText,
		LastOnlineAt:      lastOnlineAt,
		LastMessageAt:     lastMessageAt,
		SleepUntil:        sleepUntil,
		DMCooldownUntil:   formatOptionalTime(item.DMCooldownUntil),
		LastJoinAt:        formatOptionalTime(item.LastJoinAt),
		JoinCooldownUntil: formatOptionalTime(item.JoinCooldownUntil),
		AccessType:        item.AccessType,
		OriginCountry:     originCountry,
		OriginFlag:        originFlag,
		ExitIP:            item.ExitIP,
		ExitCountry:       item.ExitCountry,
		ExitFlag:          item.ExitFlag,
		GroupID:           groupID,
		TodaySuccess:      item.TodaySuccess,
		TotalSuccess:      item.TotalSuccess,
		TodayFailed:       item.TodayFailed,
		TotalFailed:       item.TotalFailed,
		RiskStatus:        item.RiskStatus,
		BanStatus:         item.BanStatus,
		DMHourlyLimit:     item.DMHourlyLimit,
		DMDailyLimit:      item.DMDailyLimit,
		JoinHourlyLimit:   item.JoinHourlyLimit,
		JoinDailyLimit:    item.JoinDailyLimit,
		DMHourlyCount:     item.DMHourlyCount,
		DMDailyCount:      item.DMDailyCount,
		JoinHourlyCount:   item.JoinHourlyCount,
		JoinDailyCount:    item.JoinDailyCount,
		DMHourlyResetAt:   formatOptionalTime(item.DMHourlyResetAt),
		DMDailyResetAt:    formatOptionalTime(item.DMDailyResetAt),
		JoinHourlyResetAt: formatOptionalTime(item.JoinHourlyResetAt),
		JoinDailyResetAt:  formatOptionalTime(item.JoinDailyResetAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}
