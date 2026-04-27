package telegram_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"codex3/backend/internal/config"
)

type SyncResult struct {
	OK            bool       `json:"ok"`
	Authorized    bool       `json:"authorized"`
	Phone         string     `json:"phone"`
	Nickname      string     `json:"nickname"`
	Bio           string     `json:"bio"`
	Homepage      string     `json:"homepage"`
	AvatarChecked bool       `json:"avatar_checked"`
	AvatarPresent bool       `json:"avatar_present"`
	AvatarPath    string     `json:"avatar_path"`
	AvatarError   string     `json:"avatar_error"`
	Status        string     `json:"status"`
	LastOnlineAt  *time.Time `json:"last_online_at"`
	RiskStatus    string     `json:"risk_status"`
	BanStatus     string     `json:"ban_status"`
	Reason        string     `json:"reason"`
	Source        string     `json:"source"`
}

type rawSyncResult struct {
	OK            bool   `json:"ok"`
	Authorized    bool   `json:"authorized"`
	Phone         string `json:"phone"`
	Nickname      string `json:"nickname"`
	Bio           string `json:"bio"`
	Homepage      string `json:"homepage"`
	AvatarChecked bool   `json:"avatar_checked"`
	AvatarPresent bool   `json:"avatar_present"`
	AvatarPath    string `json:"avatar_path"`
	AvatarError   string `json:"avatar_error"`
	Status        string `json:"status"`
	LastOnlineAt  string `json:"last_online_at"`
	RiskStatus    string `json:"risk_status"`
	BanStatus     string `json:"ban_status"`
	Reason        string `json:"reason"`
	Source        string `json:"source"`
}

type SyncRequest struct {
	FilePath   string
	AccessType string
	AvatarDir  string
	Proxy      ProxyConfig
}

type Inspector struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

func NewInspector(cfg config.Config) Inspector {
	root := backendRoot()
	pythonPath := resolveInspectorPath(root, cfg.TelegramSyncPython, ".venv/bin/python")
	scriptPath := resolveInspectorPath(root, cfg.TelegramSyncScript, "scripts/telegram_profile_sync.py")
	timeout := time.Duration(cfg.TelegramSyncTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	return Inspector{
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		timeout:    timeout,
	}
}

func (i Inspector) Sync(ctx context.Context, req SyncRequest) (SyncResult, error) {
	result := SyncResult{
		Status:     "abnormal",
		RiskStatus: "需重新导入",
		BanStatus:  "正常",
	}

	if strings.TrimSpace(req.FilePath) == "" {
		result.Reason = "缺少本地会话文件"
		return result, errors.New(result.Reason)
	}
	if _, err := os.Stat(i.pythonPath); err != nil {
		result.Reason = "资料同步器不可用"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	if _, err := os.Stat(i.scriptPath); err != nil {
		result.Reason = "资料同步脚本不存在"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}

	absFilePath, err := filepath.Abs(req.FilePath)
	if err != nil {
		result.Reason = "读取终端文件路径失败"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}

	executionFilePath, cleanup, err := PrepareSessionExecutionPath(absFilePath, req.AccessType)
	if err != nil {
		result.Reason = err.Error()
		return result, err
	}
	defer cleanup()

	args := []string{i.scriptPath, "--file", executionFilePath, "--access-type", req.AccessType}
	if strings.TrimSpace(req.AvatarDir) != "" {
		absAvatarDir, avatarDirErr := filepath.Abs(req.AvatarDir)
		if avatarDirErr != nil {
			result.Reason = "读取头像缓存目录失败"
			return result, fmt.Errorf("%s: %w", result.Reason, avatarDirErr)
		}
		args = append(args, "--avatar-dir", absAvatarDir)
	}
	args = AppendProxyArgs(args, req.Proxy)

	runCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, i.pythonPath, args...)
	cmd.Dir = filepath.Dir(i.scriptPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if stdout.Len() > 0 {
		decoded, decodeErr := decodeSyncResult(stdout.Bytes())
		if decodeErr != nil {
			result.Reason = "资料同步结果解析失败"
			return result, fmt.Errorf("%s: %w", result.Reason, decodeErr)
		}
		result = decoded
	}

	if runCtx.Err() == context.DeadlineExceeded {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "资料同步超时"
		}
		return result, errors.New(result.Reason)
	}
	if runErr != nil {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = strings.TrimSpace(stderr.String())
		}
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "资料同步执行失败"
		}
		return result, fmt.Errorf("%s: %w", result.Reason, runErr)
	}
	if !result.OK {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "资料同步失败"
		}
		return result, errors.New(result.Reason)
	}
	return result, nil
}

func decodeSyncResult(data []byte) (SyncResult, error) {
	var raw rawSyncResult
	if err := json.Unmarshal(data, &raw); err != nil {
		return SyncResult{}, err
	}

	result := SyncResult{
		OK:            raw.OK,
		Authorized:    raw.Authorized,
		Phone:         raw.Phone,
		Nickname:      raw.Nickname,
		Bio:           raw.Bio,
		Homepage:      raw.Homepage,
		AvatarChecked: raw.AvatarChecked,
		AvatarPresent: raw.AvatarPresent,
		AvatarPath:    raw.AvatarPath,
		AvatarError:   raw.AvatarError,
		Status:        raw.Status,
		RiskStatus:    raw.RiskStatus,
		BanStatus:     raw.BanStatus,
		Reason:        raw.Reason,
		Source:        raw.Source,
	}
	if strings.TrimSpace(raw.LastOnlineAt) != "" {
		parsed, err := time.Parse(time.RFC3339, raw.LastOnlineAt)
		if err != nil {
			return SyncResult{}, err
		}
		result.LastOnlineAt = &parsed
	}
	return result, nil
}

func resolveInspectorPath(root, configured, fallback string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return filepath.Join(root, fallback)
	}
	if filepath.IsAbs(configured) {
		return configured
	}
	return filepath.Join(root, configured)
}

func backendRoot() string {
	if root := strings.TrimSpace(os.Getenv("CODEX3_BACKEND_ROOT")); root != "" {
		return filepath.Clean(root)
	}
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../.."))
}
