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
	"strings"
	"time"

	"codex3/backend/internal/config"
)

type TargetInspectRequest struct {
	FilePath   string
	AccessType string
	Target     string
}

type TargetInspectResult struct {
	OK         bool   `json:"ok"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Username   string `json:"username"`
	Source     string `json:"source"`
}

type TargetInspector struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

func NewTargetInspector(cfg config.Config) TargetInspector {
	root := backendRoot()
	timeout := time.Duration(cfg.TelegramSyncTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	return TargetInspector{
		pythonPath: resolveInspectorPath(root, cfg.TelegramSyncPython, ".venv/bin/python"),
		scriptPath: resolveInspectorPath(root, "scripts/telegram_target_inspect.py", "scripts/telegram_target_inspect.py"),
		timeout:    timeout,
	}
}

func (i TargetInspector) Inspect(ctx context.Context, req TargetInspectRequest) (TargetInspectResult, error) {
	result := TargetInspectResult{Status: "failed", Reason: "刷新失败", Identifier: req.Target}
	if strings.TrimSpace(req.FilePath) == "" {
		result.Reason = "缺少监听号会话文件"
		return result, errors.New(result.Reason)
	}
	if strings.TrimSpace(req.Target) == "" {
		result.Reason = "缺少监听群链接"
		return result, errors.New(result.Reason)
	}
	if _, err := os.Stat(i.pythonPath); err != nil {
		result.Reason = "Telegram 执行器不可用"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	if _, err := os.Stat(i.scriptPath); err != nil {
		result.Reason = "监听群刷新脚本不存在"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	absFilePath, err := filepath.Abs(req.FilePath)
	if err != nil {
		result.Reason = "读取监听号文件路径失败"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	executionFilePath, cleanup, err := PrepareSessionExecutionPath(absFilePath, req.AccessType)
	if err != nil {
		result.Reason = err.Error()
		return result, err
	}
	defer cleanup()

	runCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()
	args := []string{i.scriptPath, "--file", executionFilePath, "--access-type", NormalizeTelegramAccessType(req.AccessType), "--target", req.Target}
	cmd := exec.CommandContext(runCtx, i.pythonPath, args...)
	cmd.Dir = filepath.Dir(i.scriptPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			result.Reason = "监听群刷新结果解析失败"
			return result, fmt.Errorf("%s: %w", result.Reason, err)
		}
	}
	if runCtx.Err() == context.DeadlineExceeded {
		result.Reason = "监听群刷新超时"
		return result, errors.New(result.Reason)
	}
	if runErr != nil {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = strings.TrimSpace(stderr.String())
		}
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "监听群刷新执行失败"
		}
		return result, fmt.Errorf("%s: %w", result.Reason, runErr)
	}
	if !result.OK {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "监听群刷新失败"
		}
		return result, errors.New(result.Reason)
	}
	return result, nil
}
