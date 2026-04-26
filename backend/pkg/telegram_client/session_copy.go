package telegram_client

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func PrepareSessionExecutionPath(filePath, accessType string) (string, func(), error) {
	if NormalizeTelegramAccessType(accessType) != "session" {
		return filePath, func() {}, nil
	}

	absFilePath, err := filepath.Abs(strings.TrimSpace(filePath))
	if err != nil {
		return "", nil, fmt.Errorf("读取终端文件路径失败: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "codex3-telegram-session-")
	if err != nil {
		return "", nil, fmt.Errorf("创建临时会话目录失败: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	baseName := filepath.Base(absFilePath)
	clonedPath := filepath.Join(tempDir, baseName)
	if err := copyFile(absFilePath, clonedPath); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("复制会话文件失败: %w", err)
	}

	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		sourceExtra := absFilePath + suffix
		if _, err := os.Stat(sourceExtra); err == nil {
			if err := copyFile(sourceExtra, clonedPath+suffix); err != nil {
				cleanup()
				return "", nil, fmt.Errorf("复制会话附属文件失败: %w", err)
			}
		}
	}

	return clonedPath, cleanup, nil
}

func copyFile(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return destination.Sync()
}

func NormalizeTelegramAccessType(accessType string) string {
	switch strings.ToLower(strings.TrimSpace(accessType)) {
	case "", "session":
		return "session"
	default:
		return "data"
	}
}
