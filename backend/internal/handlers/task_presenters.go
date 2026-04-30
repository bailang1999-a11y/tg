package handlers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"codex3/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type taskUserRef struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Role             string `json:"role"`
	TelegramUserID   string `json:"telegram_user_id,omitempty"`
	TelegramUsername string `json:"telegram_username,omitempty"`
}

type taskBotUserRef struct {
	ID             string `json:"id"`
	Nickname       string `json:"nickname"`
	Username       string `json:"username"`
	TelegramUserID string `json:"telegram_user_id"`
	Status         string `json:"status"`
	Plan           string `json:"plan"`
}

type taskSettingItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type taskListItem struct {
	models.Task
	Creator       *taskUserRef       `json:"creator,omitempty"`
	BotUser       *taskBotUserRef    `json:"bot_user,omitempty"`
	Settings      []taskSettingItem  `json:"settings"`
	BotDMSettings []taskSettingItem  `json:"bot_dm_settings,omitempty"`
	BotDMTasks    []models.BotDMTask `json:"bot_dm_tasks,omitempty"`
}

type taskLogItem struct {
	models.TaskLog
	Task       *taskLogTaskItem `json:"task,omitempty"`
	LevelText  string           `json:"level_text"`
	ActionText string           `json:"action_text"`
}

type taskLogTaskItem struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	Progress  int             `json:"progress"`
	Payload   datatypes.JSON  `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Creator   *taskUserRef    `json:"creator,omitempty"`
	BotUser   *taskBotUserRef `json:"bot_user,omitempty"`
}

func taskBotSubscriberID(task models.Task) (uuid.UUID, bool) {
	var payload struct {
		BotSubscriberID string `json:"bot_subscriber_id"`
	}
	if len(task.Payload) == 0 || !json.Valid(task.Payload) {
		return uuid.Nil, false
	}
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(payload.BotSubscriberID))
	return id, err == nil
}

func taskSettings(task models.Task) []taskSettingItem {
	settings := []taskSettingItem{
		{Label: "任务类型", Value: taskTypeText(task.Type)},
		{Label: "任务状态", Value: taskStatusText(task.Status)},
		{Label: "进度", Value: fmt.Sprintf("%d%%", task.Progress)},
	}
	var summary map[string]any
	if len(task.Summary) > 0 && json.Valid(task.Summary) {
		_ = json.Unmarshal(task.Summary, &summary)
	}
	addSummary := func(key, label string) {
		if value, ok := summary[key]; ok {
			settings = append(settings, taskSettingItem{Label: label, Value: fmt.Sprint(value)})
		}
	}
	switch task.Type {
	case "bot_dm":
		addPayloadSetting(task.Payload, &settings, "account_group", "私信号池分组")
		addPayloadSetting(task.Payload, &settings, "sent_count", "已发送")
		addPayloadSetting(task.Payload, &settings, "min_delay_seconds", "最小延迟")
		addPayloadSetting(task.Payload, &settings, "max_delay_seconds", "最大延迟")
	case "scrm_listener":
		addSummary("target_count", "监听目标数")
		addSummary("terminal_count", "监听账号数")
		addSummary("match_mode", "匹配模式")
		addSummary("strike_enabled", "自动私信")
	case "mass_messaging":
		addSummary("total", "预计投递")
		addSummary("success", "成功")
		addSummary("failed", "失败")
		addSummary("top_skip_reason", "主要拦截原因")
	case "direct_messages":
		addSummary("lead_count", "线索数")
		addSummary("terminal_count", "私信账号")
		addSummary("success", "成功")
		addSummary("failed", "失败")
		addSummary("skipped", "跳过")
		addSummary("top_skip_reason", "主要拦截原因")
	case "join_targets":
		addSummary("total", "目标总数")
		addSummary("success", "成功")
		addSummary("failed", "失败")
		addSummary("skipped", "跳过")
		addSummary("pending", "剩余")
		addSummary("current_target", "当前目标")
		addSummary("waiting_reason", "等待状态")
		addSummary("waiting_until", "预计恢复")
		addSummary("top_skip_reason", "主要拦截原因")
	case "listener_join_targets":
		addSummary("total", "监听群总数")
		addSummary("success", "成功")
		addSummary("failed", "失败")
		addSummary("skipped", "跳过")
		addSummary("pending", "剩余")
		addSummary("current_target", "当前监听群")
		addSummary("waiting_reason", "等待状态")
		addSummary("waiting_until", "预计恢复")
		addSummary("top_skip_reason", "主要拦截原因")
	case "listener_target_refresh":
		addSummary("total", "监听群总数")
		addSummary("success", "刷新成功")
		addSummary("failed", "刷新失败")
	case "listener_proxy_check":
		addSummary("total", "代理总数")
		addSummary("normal", "正常")
		addSummary("failed", "失败")
		addSummary("timeout", "超时")
	case "listener_account_check":
		addSummary("total", "监听号总数")
		addSummary("normal", "正常")
		addSummary("offline", "会话有效")
		addSummary("abnormal", "异常")
	case "import":
		addSummary("success", "导入成功")
		addSummary("duplicates", "重复")
		addSummary("failed", "失败")
	}
	if task.TerminalGroupID != nil {
		settings = append(settings, taskSettingItem{Label: "终端分组", Value: task.TerminalGroupID.String()})
	}
	if task.TargetGroupID != nil {
		settings = append(settings, taskSettingItem{Label: "目标分组", Value: task.TargetGroupID.String()})
	}
	return settings
}

func addPayloadSetting(payload datatypes.JSON, settings *[]taskSettingItem, key, label string) {
	if len(payload) == 0 || !json.Valid(payload) {
		return
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return
	}
	if value, ok := data[key]; ok {
		*settings = append(*settings, taskSettingItem{Label: label, Value: fmt.Sprint(value)})
	}
}

func botDMTaskSettings(tasks []models.BotDMTask) []taskSettingItem {
	if len(tasks) == 0 {
		return nil
	}
	settings := []taskSettingItem{{Label: "私信任务数", Value: fmt.Sprintf("%d", len(tasks))}}
	active := 0
	totalSent := int64(0)
	for _, task := range tasks {
		if strings.EqualFold(task.Status, "active") || strings.EqualFold(task.Status, "running") {
			active++
		}
		totalSent += task.SentCount
		var messages []string
		_ = json.Unmarshal(task.Messages, &messages)
		settings = append(settings, taskSettingItem{Label: "私信任务", Value: fmt.Sprintf("%s：%d 条消息，延迟 %d-%d 秒，已发 %d", firstNonEmpty(task.Name, "未命名"), len(messages), task.MinDelaySeconds, task.MaxDelaySeconds, task.SentCount)})
	}
	settings = append([]taskSettingItem{
		{Label: "启用私信", Value: fmt.Sprintf("%d 个", active)},
		{Label: "已发送私信", Value: fmt.Sprintf("%d 条", totalSent)},
	}, settings...)
	return settings
}

func taskTypeText(value string) string {
	switch value {
	case "import":
		return "导入任务"
	case "terminal_check":
		return "账号检测"
	case "network_test":
		return "代理检测"
	case "import_proxies":
		return "导入代理"
	case "profile_modify":
		return "资料修改"
	case "mass_messaging":
		return "通知工作流"
	case "direct_messages":
		return "监听私信"
	case "scrm_listener":
		return "监听任务"
	case "bot_dm":
		return "Bot 私信任务"
	case "bot_config":
		return "Bot 配置修改"
	case "join_targets":
		return "终端加入目标池"
	case "listener_join_targets":
		return "监听号自动加群"
	case "listener_target_refresh":
		return "监听群资料刷新"
	case "listener_proxy_check":
		return "监听代理检测"
	case "listener_account_check":
		return "监听账号检测"
	case "audit_action":
		return "操作日志"
	default:
		return firstNonEmpty(value, "未分类")
	}
}

func taskStatusText(value string) string {
	switch strings.ToLower(value) {
	case "active":
		return "进行中"
	case "dry_run":
		return "演练完成"
	case "success", "completed", "finished":
		return "执行成功"
	case "partial_success":
		return "部分成功"
	case "failed":
		return "执行失败"
	case "queued":
		return "排队中"
	case "running":
		return "执行中"
	case "pending":
		return "待执行"
	case "paused":
		return "已暂停"
	case "stopped":
		return "已停止"
	case "cancelled":
		return "已取消"
	default:
		return value
	}
}

func taskLogLevelText(value string) string {
	switch strings.ToUpper(value) {
	case "ERROR":
		return "错误"
	case "WARN", "WARNING":
		return "警告"
	case "INFO":
		return "信息"
	default:
		return value
	}
}

func taskLogActionText(value string) string {
	action := strings.ToLower(strings.TrimSpace(value))
	switch action {
	case "start":
		return "开始执行"
	case "created":
		return "任务创建"
	case "summary":
		return "执行汇总"
	case "pause":
		return "暂停"
	case "resume":
		return "恢复"
	case "stop":
		return "停止"
	case "restart":
		return "重启"
	case "force":
		return "强制停止"
	case "match":
		return "关键词命中"
	case "ready":
		return "监听就绪"
	case "worker_exit":
		return "进程退出"
	case "worker_wait":
		return "进程状态回传"
	case "listener_stdout":
		return "监听进程输出"
	case "listener_stderr":
		return "监听进程告警"
	case "subscriber_start":
		return "启动监听任务"
	case "subscriber_stop":
		return "暂停监听任务"
	case "subscriber_resume":
		return "恢复监听任务"
	case "subscriber_save":
		return "保存监听配置"
	case "dm_task_start":
		return "启动私信任务"
	case "dm_task_stop":
		return "停止私信任务"
	case "dm_task_pause":
		return "暂停私信任务"
	case "dm_task_resume":
		return "恢复私信任务"
	case "dm_task_complete":
		return "私信任务完成"
	case "bot_dm":
		return "自动私信处理"
	case "bot_push":
		return "Bot 线索推送"
	case "update_config":
		return "修改配置"
	case "update_bot_config":
		return "修改 Bot 配置"
	case "bot_user_bind":
		return "绑定 Bot 用户"
	case "bot_user_update":
		return "更新 Bot 用户设置"
	case "bot_license_create":
		return "生成卡密"
	case "bot_license_bind":
		return "卡密绑定用户"
	case "bot_license_toggle":
		return "切换卡密状态"
	case "bot_license_delete":
		return "删除卡密"
	case "import_accounts":
		return "导入账号"
	case "import_targets":
		return "导入目标"
	case "import_proxies":
		return "导入代理"
	case "import_session":
		return "导入 Session"
	case "import_tdata":
		return "导入 TData"
	case "import_validation":
		return "导入校验"
	case "check_terminal_status":
		return "检测账号状态"
	case "test_proxy_latency":
		return "检测代理延迟"
	case "check_listener_account":
		return "检测监听账号"
	case "account_check_start":
		return "开始检测监听账号"
	case "join_success":
		return "加入目标成功"
	case "join_failed":
		return "加入目标失败"
	case "join_skipped":
		return "加入目标跳过"
	case "target_refresh_success":
		return "刷新群资料成功"
	case "target_refresh_failed":
		return "刷新群资料失败"
	case "listener_join_targets":
		return "监听号自动加群"
	case "adapter":
		return "执行适配器状态"
	case "script":
		return "脚本检查"
	case "terminal_path":
		return "会话路径检查"
	case "terminal_copy":
		return "会话副本准备"
	case "start_worker":
		return "启动监听进程"
	case "audit_create":
		return "操作提交"
	case "audit_update":
		return "操作修改"
	case "audit_delete":
		return "操作删除"
	case "audit_action":
		return "后台操作"
	case "stdout":
		return "标准输出"
	case "stderr":
		return "标准错误"
	case "listener_parse":
		return "监听输出解析"
	case "warning":
		return "运行告警"
	case "listener_error":
		return "监听错误"
	case "match_skip":
		return "命中跳过"
	case "history_skip":
		return "历史消息跳过"
	case "persist_lead":
		return "线索入库"
	case "dispatch_policy":
		return "投递策略"
	case "round":
		return "发送轮次"
	case "step":
		return "发送阶段"
	case "delay":
		return "发送延迟"
	case "interval":
		return "发送间隔"
	default:
		return firstNonEmpty(humanizeUnknownAction(action), "日志")
	}
}

func taskLogDetailText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return text
	}
	if normalized, ok := normalizeTaskLogJSONDetail(text); ok {
		text = normalized
	}
	replacements := map[string]string{
		"exit status":               "退出状态",
		"listener_stderr":           "监听进程告警",
		"listener_stdout":           "监听进程输出",
		"worker_wait":               "进程状态回传",
		"Could not open key_data":   "无法读取 key_data 文件",
		"could not open key_data":   "无法读取 key_data 文件",
		"FileNotFound":              "文件不存在",
		"file not found":            "文件不存在",
		"subscriber manual pause":   "用户手动暂停监听",
		"paused_for_polling":        "轮询模式已接管（Webhook 暂停）",
		"bot dm":                    "Bot 私信",
		"Bot user":                  "Bot 用户",
		"dry-run":                   "演练模式",
		"dry run":                   "演练模式",
		"timeout":                   "超时",
		"forbidden":                 "权限不足",
		"unauthorized":              "未授权",
		"bad request":               "请求参数错误",
		"internal server error":     "服务内部错误",
		"too many requests":         "请求过于频繁",
		"connection reset":          "连接被重置",
		"connection refused":        "连接被拒绝",
		"network is unreachable":    "网络不可达",
		"context deadline exceeded": "请求超时",
	}
	for oldText, newText := range replacements {
		text = strings.ReplaceAll(text, oldText, newText)
	}
	text = translateTelegramReasonToChinese(text)
	text = regexp.MustCompile(`(?i)exit status\s+(\d+)`).ReplaceAllString(text, "退出状态 $1")
	text = strings.ReplaceAll(text, "TFileNotFound", "文件不存在")
	text = strings.ReplaceAll(text, "opentele.exception.", "")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func translateTelegramReasonToChinese(reason string) string {
	text := strings.TrimSpace(reason)
	if text == "" {
		return text
	}
	text = regexp.MustCompile(`(?i)No user has "([^"]+)" as username`).ReplaceAllString(text, `Telegram 未找到用户名「$1」，请检查目标群链接是否失效或拼写错误`)
	translations := []struct {
		pattern *regexp.Regexp
		value   string
	}{
		{regexp.MustCompile(`(?i)The username is not occupied by anyone`), "Telegram 未找到这个用户名，请检查目标群链接是否失效或拼写错误"},
		{regexp.MustCompile(`(?i)Nobody is using this username`), "Telegram 未找到这个用户名，请检查目标群链接是否失效或拼写错误"},
		{regexp.MustCompile(`(?i)Username not found`), "Telegram 未找到这个用户名，请检查目标群链接是否失效或拼写错误"},
		{regexp.MustCompile(`(?i)Cannot find any entity corresponding to`), "无法识别目标群，请检查链接或用户名是否正确"},
		{regexp.MustCompile(`(?i)Could not find the input entity`), "无法读取目标群信息，请检查链接或账号权限"},
		{regexp.MustCompile(`(?i)The invite link is expired or has been revoked`), "邀请链接已过期或已被撤销"},
		{regexp.MustCompile(`(?i)Invite hash expired`), "邀请链接已过期"},
		{regexp.MustCompile(`(?i)Invite hash invalid`), "邀请链接无效"},
		{regexp.MustCompile(`(?i)You have successfully requested to join this chat or channel`), "已提交入群申请，等待群管理员审核"},
		{regexp.MustCompile(`(?i)You have joined too many channels/supergroups`), "该账号加入的群组过多，Telegram 已限制继续加群"},
		{regexp.MustCompile(`(?i)The user is already a participant|User already participant`), "账号已经在这个群里"},
		{regexp.MustCompile(`(?i)The channel specified is private`), "目标群是私密群，当前账号无法直接访问"},
		{regexp.MustCompile(`(?i)AUTH_KEY_UNREGISTERED|SESSION_REVOKED`), "账号会话已失效，需要重新导入"},
		{regexp.MustCompile(`(?i)USER_DEACTIVATED`), "账号已被 Telegram 停用"},
		{regexp.MustCompile(`(?i)PHONE_NUMBER_BANNED`), "手机号已被 Telegram 封禁"},
	}
	for _, item := range translations {
		text = item.pattern.ReplaceAllString(text, item.value)
	}
	return text
}

func humanizeUnknownAction(action string) string {
	if strings.TrimSpace(action) == "" {
		return ""
	}
	parts := strings.Split(action, "_")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		labels = append(labels, part)
	}
	if len(labels) == 0 {
		return action
	}
	return "日志动作：" + strings.Join(labels, "·")
}

func normalizeTaskLogJSONDetail(text string) (string, bool) {
	if !json.Valid([]byte(text)) {
		return "", false
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil || len(data) == 0 {
		return "", false
	}
	segments := make([]string, 0, 8)
	if action := strings.TrimSpace(fmt.Sprint(data["action"])); action != "" && action != "<nil>" {
		segments = append(segments, "动作："+taskLogActionText(action))
	}
	if field := strings.TrimSpace(fmt.Sprint(data["field"])); field != "" && field != "<nil>" {
		segments = append(segments, "字段："+field)
	}
	if oldValue := stringifyJSONValue(data["old"], data["before"]); oldValue != "" {
		segments = append(segments, "修改前："+oldValue)
	}
	if newValue := stringifyJSONValue(data["new"], data["after"]); newValue != "" {
		segments = append(segments, "修改后："+newValue)
	}
	if reason := stringifyJSONValue(data["reason"], data["error"]); reason != "" {
		segments = append(segments, "原因："+reason)
	}
	if status := strings.TrimSpace(fmt.Sprint(data["status"])); status != "" && status != "<nil>" {
		segments = append(segments, "状态："+taskStatusText(status))
	}
	if len(segments) == 0 {
		return "", false
	}
	return strings.Join(segments, "；"), true
}

func stringifyJSONValue(values ...any) string {
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			text := strings.TrimSpace(typed)
			if text != "" {
				return text
			}
		case float64:
			if typed == float64(int64(typed)) {
				return strconv.FormatInt(int64(typed), 10)
			}
			return fmt.Sprintf("%g", typed)
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func botPlanText(value string) string {
	switch strings.ToLower(value) {
	case "trial":
		return "试用"
	case "paid", "vip":
		return "会员"
	default:
		return firstNonEmpty(value, "未设置")
	}
}
