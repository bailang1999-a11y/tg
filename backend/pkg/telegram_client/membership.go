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

type MembershipCheckRequest struct {
	FilePath   string
	AccessType string
	TargetType string
	Identifier string
	Proxy      ProxyConfig
}

type MembershipCheckTarget struct {
	Ref        string `json:"ref,omitempty"`
	TargetType string `json:"target_type"`
	Identifier string `json:"target"`
}

type MembershipBatchCheckRequest struct {
	FilePath   string
	AccessType string
	Targets    []MembershipCheckTarget
	Proxy      ProxyConfig
}

type MembershipCheckResult struct {
	OK         bool   `json:"ok"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	TargetType string `json:"target_type"`
	Ref        string `json:"ref,omitempty"`
}

type membershipBatchScriptResult struct {
	OK      bool                    `json:"ok"`
	Status  string                  `json:"status"`
	Reason  string                  `json:"reason"`
	Source  string                  `json:"source"`
	Results []MembershipCheckResult `json:"results"`
}

type MembershipChecker struct {
	pythonPath string
	scriptPath string
	timeout    time.Duration
}

func NewMembershipChecker(cfg config.Config) MembershipChecker {
	root := backendRoot()
	timeout := time.Duration(cfg.TelegramSyncTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 35 * time.Second
	}
	return MembershipChecker{
		pythonPath: resolveInspectorPath(root, cfg.TelegramSyncPython, ".venv/bin/python"),
		scriptPath: resolveInspectorPath(root, "", "scripts/telegram_membership_check.py"),
		timeout:    timeout,
	}
}

func (m MembershipChecker) Check(ctx context.Context, req MembershipCheckRequest) (MembershipCheckResult, error) {
	result := MembershipCheckResult{
		Status:     "failed",
		Target:     req.Identifier,
		TargetType: req.TargetType,
	}
	if strings.TrimSpace(req.FilePath) == "" {
		result.Status = "account_invalid"
		result.Reason = "缺少本地会话文件"
		return result, errors.New(result.Reason)
	}
	if strings.TrimSpace(req.Identifier) == "" {
		result.Status = "target_invalid"
		result.Reason = "缺少目标标识"
		return result, errors.New(result.Reason)
	}
	if _, err := os.Stat(m.pythonPath); err != nil {
		result.Reason = "成员状态检测器不可用"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	if _, err := os.Stat(m.scriptPath); err != nil {
		result.Reason = "成员状态检测脚本不存在"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	absFilePath, err := filepath.Abs(req.FilePath)
	if err != nil {
		result.Reason = "读取账号文件路径失败"
		return result, fmt.Errorf("%s: %w", result.Reason, err)
	}
	args := []string{
		m.scriptPath,
		"--file", absFilePath,
		"--access-type", req.AccessType,
		"--target-type", req.TargetType,
		"--target", req.Identifier,
	}
	args = AppendProxyArgs(args, req.Proxy)

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
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			result.Reason = "成员状态检测结果解析失败"
			return result, fmt.Errorf("%s: %w", result.Reason, err)
		}
	}
	if runCtx.Err() == context.DeadlineExceeded {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "成员状态检测超时"
		}
		return result, errors.New(result.Reason)
	}
	if runErr != nil {
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = strings.TrimSpace(stderr.String())
		}
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "成员状态检测失败"
		}
		return result, fmt.Errorf("%s: %w", result.Reason, runErr)
	}
	if strings.TrimSpace(result.Status) == "" {
		if result.OK {
			result.Status = "active"
		} else {
			result.Status = "failed"
		}
	}
	if !result.OK && membershipCheckHardFailure(result.Status) {
		return result, errors.New(firstMembershipReason(result.Reason, "成员状态已失效"))
	}
	return result, nil
}

func (m MembershipChecker) CheckBatch(ctx context.Context, req MembershipBatchCheckRequest) ([]MembershipCheckResult, error) {
	results := make([]MembershipCheckResult, 0, len(req.Targets))
	if len(req.Targets) == 0 {
		return results, nil
	}
	for _, target := range req.Targets {
		results = append(results, MembershipCheckResult{
			Status:     "failed",
			Target:     target.Identifier,
			TargetType: target.TargetType,
			Ref:        target.Ref,
		})
	}
	if strings.TrimSpace(req.FilePath) == "" {
		return markMembershipBatchFailed(results, "account_invalid", "缺少本地会话文件"), errors.New("缺少本地会话文件")
	}
	if _, err := os.Stat(m.pythonPath); err != nil {
		return markMembershipBatchFailed(results, "failed", "成员状态检测器不可用"), fmt.Errorf("成员状态检测器不可用: %w", err)
	}
	if _, err := os.Stat(m.scriptPath); err != nil {
		return markMembershipBatchFailed(results, "failed", "成员状态检测脚本不存在"), fmt.Errorf("成员状态检测脚本不存在: %w", err)
	}
	absFilePath, err := filepath.Abs(req.FilePath)
	if err != nil {
		return markMembershipBatchFailed(results, "failed", "读取账号文件路径失败"), fmt.Errorf("读取账号文件路径失败: %w", err)
	}
	targetPayload, err := json.Marshal(req.Targets)
	if err != nil {
		return markMembershipBatchFailed(results, "failed", "成员状态检测参数编码失败"), fmt.Errorf("成员状态检测参数编码失败: %w", err)
	}

	args := []string{
		m.scriptPath,
		"--file", absFilePath,
		"--access-type", req.AccessType,
		"--targets-json", string(targetPayload),
	}
	args = AppendProxyArgs(args, req.Proxy)
	runCtx, cancel := context.WithTimeout(ctx, m.batchTimeout(len(req.Targets)))
	defer cancel()

	cmd := exec.CommandContext(runCtx, m.pythonPath, args...)
	cmd.Dir = filepath.Dir(m.scriptPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if stdout.Len() > 0 {
		var decoded membershipBatchScriptResult
		if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
			return markMembershipBatchFailed(results, "failed", "成员状态检测结果解析失败"), fmt.Errorf("成员状态检测结果解析失败: %w", err)
		}
		if len(decoded.Results) > 0 {
			results = normalizeMembershipBatchResults(req.Targets, decoded.Results)
		}
	}
	if runCtx.Err() == context.DeadlineExceeded {
		return markMembershipBatchFailed(results, "failed", "成员状态检测超时"), errors.New("成员状态检测超时")
	}
	if runErr != nil {
		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = firstMembershipReason(firstBatchReason(results), "成员状态检测失败")
		}
		return markMembershipBatchFailed(results, "failed", reason), fmt.Errorf("%s: %w", reason, runErr)
	}
	return results, nil
}

func (m MembershipChecker) batchTimeout(targetCount int) time.Duration {
	if targetCount <= 1 {
		return m.timeout
	}
	timeout := m.timeout + time.Duration(targetCount)*8*time.Second
	if timeout > 20*time.Minute {
		return 20 * time.Minute
	}
	return timeout
}

func markMembershipBatchFailed(results []MembershipCheckResult, status string, reason string) []MembershipCheckResult {
	for index := range results {
		results[index].OK = false
		results[index].Status = status
		results[index].Reason = reason
	}
	return results
}

func normalizeMembershipBatchResults(targets []MembershipCheckTarget, results []MembershipCheckResult) []MembershipCheckResult {
	byRef := make(map[string]MembershipCheckResult, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Ref) != "" {
			byRef[result.Ref] = result
		}
	}
	normalized := make([]MembershipCheckResult, 0, len(targets))
	for index, target := range targets {
		result := MembershipCheckResult{}
		if strings.TrimSpace(target.Ref) != "" {
			result = byRef[target.Ref]
		}
		if strings.TrimSpace(result.Target) == "" && index < len(results) && strings.TrimSpace(target.Ref) == "" {
			result = results[index]
		}
		if strings.TrimSpace(result.Target) == "" {
			result.Target = target.Identifier
		}
		if strings.TrimSpace(result.TargetType) == "" {
			result.TargetType = target.TargetType
		}
		if strings.TrimSpace(result.Ref) == "" {
			result.Ref = target.Ref
		}
		if strings.TrimSpace(result.Status) == "" {
			if result.OK {
				result.Status = "active"
			} else {
				result.Status = "failed"
			}
		}
		normalized = append(normalized, result)
	}
	return normalized
}

func firstBatchReason(results []MembershipCheckResult) string {
	for _, result := range results {
		if strings.TrimSpace(result.Reason) != "" {
			return result.Reason
		}
	}
	return ""
}

func membershipCheckHardFailure(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "not_member", "kicked", "banned", "inaccessible", "account_invalid", "target_invalid":
		return true
	default:
		return false
	}
}

func firstMembershipReason(reason string, fallback string) string {
	if strings.TrimSpace(reason) != "" {
		return strings.TrimSpace(reason)
	}
	return fallback
}
