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

type MessageRequest struct {
	FilePath     string
	AccessType   string
	TargetType   string
	Target       string
	StepType     string
	Content      string
	MediaPath    string
	SourceChatID string
	MessageID    string
}

type MessageResult struct {
	OK         bool   `json:"ok"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	TargetType string `json:"target_type"`
	StepType   string `json:"step_type"`
	MessageID  string `json:"message_id"`
}

type Messenger struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

func NewMessenger(cfg config.Config) Messenger {
	root := backendRoot()
	timeout := time.Duration(cfg.TelegramApplyTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return Messenger{
		pythonPath: resolveInspectorPath(root, cfg.TelegramSyncPython, ".venv/bin/python"),
		scriptPath: resolveInspectorPath(root, cfg.TelegramMessageScript, "scripts/telegram_message_send.py"),
		timeout:    timeout,
	}
}

func (m Messenger) Send(ctx context.Context, req MessageRequest) (MessageResult, error) {
	result := MessageResult{
		Status:     "failed",
		Target:     req.Target,
		TargetType: req.TargetType,
		StepType:   req.StepType,
	}

	if strings.TrimSpace(req.FilePath) == "" {
		result.Reason = "缺少本地会话文件"
		return result, errors.New(result.Reason)
	}
	if strings.TrimSpace(req.Target) == "" {
		result.Reason = "缺少目标标识"
		return result, errors.New(result.Reason)
	}
	if strings.TrimSpace(req.StepType) == "" {
		result.Reason = "缺少消息阶段类型"
		return result, errors.New(result.Reason)
	}
	if _, err := os.Stat(m.pythonPath); err != nil {
		result.Reason = "消息发送执行器不可用"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	if _, err := os.Stat(m.scriptPath); err != nil {
		result.Reason = "消息发送脚本不存在"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}

	absFilePath, err := filepath.Abs(req.FilePath)
	if err != nil {
		result.Reason = "读取终端文件路径失败"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}

	args := []string{
		m.scriptPath,
		"--file", absFilePath,
		"--access-type", req.AccessType,
		"--target-type", req.TargetType,
		"--target", req.Target,
		"--step-type", req.StepType,
	}
	if strings.TrimSpace(req.Content) != "" {
		args = append(args, "--content", req.Content)
	}
	if strings.TrimSpace(req.MediaPath) != "" {
		absMediaPath, mediaErr := filepath.Abs(req.MediaPath)
		if mediaErr != nil {
			result.Reason = "读取媒体文件路径失败"
			return result, fmt.Errorf("%s: %w", result.Reason, mediaErr)
		}
		args = append(args, "--media-path", absMediaPath)
	}
	if strings.TrimSpace(req.SourceChatID) != "" {
		args = append(args, "--source-chat-id", req.SourceChatID)
	}
	if strings.TrimSpace(req.MessageID) != "" {
		args = append(args, "--message-id", req.MessageID)
	}

	runCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, m.pythonPath, args...)
	cmd.Dir = filepath.Dir(m.scriptPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if stdout.Len() > 0 {
		decoded, decodeErr := decodeMessageResult(stdout.Bytes())
		if decodeErr != nil {
			result.Reason = "消息发送结果解析失败"
			return result, fmt.Errorf("%s: %w", result.Reason, decodeErr)
		}
		result = decoded
	}

	if runCtx.Err() == context.DeadlineExceeded {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "消息发送超时"
		}
		return result, errors.New(result.Reason)
	}
	if runErr != nil {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = strings.TrimSpace(stderr.String())
		}
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "消息发送执行失败"
		}
		return result, fmt.Errorf("%s: %w", result.Reason, runErr)
	}
	if !result.OK {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "消息发送失败"
		}
		return result, errors.New(result.Reason)
	}
	return result, nil
}

func decodeMessageResult(data []byte) (MessageResult, error) {
	var result MessageResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MessageResult{}, err
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
