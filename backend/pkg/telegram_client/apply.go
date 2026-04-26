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

type ApplyRequest struct {
	FilePath   string
	AccessType string
	Nickname   string
	Bio        string
	Homepage   string
	AvatarPath string
}

type ApplyResult struct {
	OK             bool                        `json:"ok"`
	Status         string                      `json:"status"`
	Reason         string                      `json:"reason"`
	Source         string                      `json:"source"`
	Terminal       string                      `json:"terminal"`
	RequestedCount int                         `json:"requested_count"`
	AppliedCount   int                         `json:"applied_count"`
	FailedCount    int                         `json:"failed_count"`
	Fields         map[string]ApplyFieldResult `json:"fields"`
}

type ApplyFieldResult struct {
	Requested bool   `json:"requested"`
	OK        bool   `json:"ok"`
	Reason    string `json:"reason,omitempty"`
}

type rawApplyResult struct {
	OK             bool                        `json:"ok"`
	Status         string                      `json:"status"`
	Reason         string                      `json:"reason"`
	Source         string                      `json:"source"`
	Terminal       string                      `json:"terminal"`
	RequestedCount int                         `json:"requested_count"`
	AppliedCount   int                         `json:"applied_count"`
	FailedCount    int                         `json:"failed_count"`
	Fields         map[string]ApplyFieldResult `json:"fields"`
}

type Applicator struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

func NewApplicator(cfg config.Config) Applicator {
	root := backendRoot()
	timeout := time.Duration(cfg.TelegramApplyTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return Applicator{
		pythonPath: resolveInspectorPath(root, cfg.TelegramSyncPython, ".venv/bin/python"),
		scriptPath: resolveInspectorPath(root, cfg.TelegramApplyScript, "scripts/telegram_profile_apply.py"),
		timeout:    timeout,
	}
}

func (a Applicator) Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
	result := ApplyResult{}

	if strings.TrimSpace(req.FilePath) == "" {
		result.Reason = "缺少本地会话文件"
		return result, errors.New(result.Reason)
	}
	if _, err := os.Stat(a.pythonPath); err != nil {
		result.Reason = "资料修改执行器不可用"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	if _, err := os.Stat(a.scriptPath); err != nil {
		result.Reason = "资料修改脚本不存在"
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

	args := []string{a.scriptPath, "--file", executionFilePath, "--access-type", req.AccessType}
	if strings.TrimSpace(req.Nickname) != "" {
		args = append(args, "--nickname", req.Nickname)
	}
	if strings.TrimSpace(req.Bio) != "" {
		args = append(args, "--bio", req.Bio)
	}
	if strings.TrimSpace(req.Homepage) != "" {
		args = append(args, "--homepage", req.Homepage)
	}
	if strings.TrimSpace(req.AvatarPath) != "" {
		absAvatarPath, avatarErr := filepath.Abs(req.AvatarPath)
		if avatarErr != nil {
			result.Reason = "读取头像文件路径失败"
			return result, fmt.Errorf("%s: %w", result.Reason, avatarErr)
		}
		args = append(args, "--avatar-path", absAvatarPath)
	}

	runCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, a.pythonPath, args...)
	cmd.Dir = filepath.Dir(a.scriptPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if stdout.Len() > 0 {
		decoded, decodeErr := decodeApplyResult(stdout.Bytes())
		if decodeErr != nil {
			result.Reason = "资料修改结果解析失败"
			return result, fmt.Errorf("%s: %w", result.Reason, decodeErr)
		}
		result = decoded
	}
	result = normalizeApplyResult(result)

	if runCtx.Err() == context.DeadlineExceeded {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "资料修改超时"
		}
		return result, errors.New(result.Reason)
	}
	if runErr != nil {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = NormalizeTelegramFailureReason(strings.TrimSpace(stderr.String()))
		}
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "资料修改执行失败"
		}
		return result, fmt.Errorf("%s: %w", result.Reason, runErr)
	}
	if !result.OK {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "资料修改失败"
		}
		return result, errors.New(result.Reason)
	}
	return result, nil
}

func normalizeApplyResult(result ApplyResult) ApplyResult {
	result.Reason = NormalizeTelegramFailureReason(result.Reason)
	if len(result.Fields) == 0 {
		return result
	}
	fields := make(map[string]ApplyFieldResult, len(result.Fields))
	for field, fieldResult := range result.Fields {
		fieldResult.Reason = NormalizeTelegramFailureReason(fieldResult.Reason)
		fields[field] = fieldResult
	}
	result.Fields = fields
	return result
}

func decodeApplyResult(data []byte) (ApplyResult, error) {
	var raw rawApplyResult
	if err := json.Unmarshal(data, &raw); err != nil {
		return ApplyResult{}, err
	}
	requestedCount, appliedCount, failedCount := deriveApplyCounts(raw)
	fields := make(map[string]ApplyFieldResult, len(raw.Fields))
	for field, fieldResult := range raw.Fields {
		fieldResult.Reason = NormalizeTelegramFailureReason(fieldResult.Reason)
		fields[field] = fieldResult
	}
	return ApplyResult{
		OK:             raw.OK,
		Status:         deriveApplyStatus(raw),
		Reason:         NormalizeTelegramFailureReason(raw.Reason),
		Source:         raw.Source,
		Terminal:       raw.Terminal,
		RequestedCount: requestedCount,
		AppliedCount:   appliedCount,
		FailedCount:    failedCount,
		Fields:         fields,
	}, nil
}

func deriveApplyStatus(raw rawApplyResult) string {
	status := strings.TrimSpace(raw.Status)
	if status != "" {
		return status
	}
	requestedCount, appliedCount, failedCount := deriveApplyCounts(raw)
	switch {
	case requestedCount == 0:
		return "failed"
	case failedCount == 0 && appliedCount > 0:
		return "success"
	case appliedCount > 0:
		return "partial_success"
	default:
		return "failed"
	}
}

func deriveApplyCounts(raw rawApplyResult) (int, int, int) {
	if raw.RequestedCount > 0 || raw.AppliedCount > 0 || raw.FailedCount > 0 {
		return raw.RequestedCount, raw.AppliedCount, raw.FailedCount
	}
	requestedCount := 0
	appliedCount := 0
	failedCount := 0
	for _, field := range raw.Fields {
		if !field.Requested {
			continue
		}
		requestedCount++
		if field.OK {
			appliedCount++
			continue
		}
		failedCount++
	}
	return requestedCount, appliedCount, failedCount
}
