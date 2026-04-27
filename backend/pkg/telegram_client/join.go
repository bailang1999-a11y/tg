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

type JoinRequest struct {
	FilePath   string
	AccessType string
	TargetType string
	Identifier string
	Proxy      ProxyConfig
}

type JoinResult struct {
	OK         bool   `json:"ok"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	TargetType string `json:"target_type"`
}

type Joiner struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

func NewJoiner(cfg config.Config) Joiner {
	root := backendRoot()
	timeout := time.Duration(cfg.TelegramApplyTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return Joiner{
		pythonPath: resolveInspectorPath(root, cfg.TelegramSyncPython, ".venv/bin/python"),
		scriptPath: resolveInspectorPath(root, "", "scripts/telegram_join_target.py"),
		timeout:    timeout,
	}
}

func (j Joiner) Join(ctx context.Context, req JoinRequest) (JoinResult, error) {
	result := JoinResult{
		Status:     "failed",
		Target:     req.Identifier,
		TargetType: req.TargetType,
	}

	if strings.TrimSpace(req.FilePath) == "" {
		result.Reason = "缺少本地会话文件"
		return result, errors.New(result.Reason)
	}
	if strings.TrimSpace(req.Identifier) == "" {
		result.Reason = "缺少目标标识"
		return result, errors.New(result.Reason)
	}
	if _, err := os.Stat(j.pythonPath); err != nil {
		result.Reason = "加群执行器不可用"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	if _, err := os.Stat(j.scriptPath); err != nil {
		result.Reason = "加群脚本不存在"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}

	absFilePath, err := filepath.Abs(req.FilePath)
	if err != nil {
		result.Reason = "读取终端文件路径失败"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}

	args := []string{
		j.scriptPath,
		"--file", absFilePath,
		"--access-type", req.AccessType,
		"--target-type", req.TargetType,
		"--target", req.Identifier,
	}
	args = AppendProxyArgs(args, req.Proxy)

	runCtx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, j.pythonPath, args...)
	cmd.Dir = filepath.Dir(j.scriptPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if stdout.Len() > 0 {
		decoded, decodeErr := decodeJoinResult(stdout.Bytes())
		if decodeErr != nil {
			result.Reason = "加群结果解析失败"
			return result, fmt.Errorf("%s: %w", result.Reason, decodeErr)
		}
		result = decoded
	}

	if runCtx.Err() == context.DeadlineExceeded {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "加群执行超时"
		}
		return result, errors.New(result.Reason)
	}
	if runErr != nil {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = strings.TrimSpace(stderr.String())
		}
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "加群执行失败"
		}
		return result, fmt.Errorf("%s: %w", result.Reason, runErr)
	}
	if !result.OK {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "加群失败"
		}
		return result, errors.New(result.Reason)
	}
	return result, nil
}

func decodeJoinResult(data []byte) (JoinResult, error) {
	var result JoinResult
	if err := json.Unmarshal(data, &result); err != nil {
		return JoinResult{}, err
	}
	if strings.TrimSpace(result.Status) == "" {
		if result.OK {
			result.Status = "success"
		} else {
			result.Status = "failed"
		}
	}
	return result, nil
}
