package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"codex3/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type systemVersionResponse struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	LatestURL       string `json:"latest_url"`
	UpdateAvailable bool   `json:"update_available"`
	UpdateEnabled   bool   `json:"update_enabled"`
}

type systemUpdateResponse struct {
	Status    string `json:"status"`
	ExecID    string `json:"exec_id"`
	Message   string `json:"message"`
	Command   string `json:"command"`
	Container string `json:"container"`
}

type githubReleaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func (s *Server) GetSystemVersion(c *gin.Context) {
	latestVersion, latestURL := s.fetchLatestRelease(c.Request.Context())
	current := normalizeVersion(s.cfg.AppVersion)
	latest := normalizeVersion(latestVersion)
	utils.OK(c, systemVersionResponse{
		CurrentVersion:  firstNonEmpty(current, "unknown"),
		LatestVersion:   firstNonEmpty(latest, current, "unknown"),
		LatestURL:       latestURL,
		UpdateAvailable: current != "" && latest != "" && current != latest,
		UpdateEnabled:   s.cfg.UpdateEnabled,
	})
}

func (s *Server) StartSystemUpdate(c *gin.Context) {
	if !s.cfg.UpdateEnabled {
		utils.Fail(c, http.StatusBadRequest, "一键更新未启用，请使用新版 docker-compose.yml 重新创建编排")
		return
	}
	command := strings.TrimSpace(s.cfg.UpdateCommand)
	container := strings.TrimSpace(s.cfg.UpdateDockerContainer)
	if command == "" || container == "" {
		utils.Fail(c, http.StatusInternalServerError, "更新命令未配置")
		return
	}

	execID, err := s.startDockerExec(c.Request.Context(), container, []string{"sh", "-lc", command})
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, fmt.Sprintf("启动更新失败：%s", err.Error()))
		return
	}
	utils.OK(c, systemUpdateResponse{
		Status:    "started",
		ExecID:    execID,
		Message:   "更新任务已启动，1Panel 会重新构建并重启服务，请稍后刷新页面。",
		Command:   command,
		Container: container,
	})
}

func (s *Server) fetchLatestRelease(ctx context.Context) (string, string) {
	url := strings.TrimSpace(s.cfg.UpdateLatestReleaseURL)
	if url == "" {
		return "", ""
	}
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "TG-Marketing-Assistant")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", ""
	}
	var release githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", ""
	}
	return release.TagName, release.HTMLURL
}

func (s *Server) dockerClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", s.cfg.UpdateDockerSocket)
			},
		},
	}
}

func (s *Server) startDockerExec(ctx context.Context, container string, cmd []string) (string, error) {
	client := s.dockerClient()
	createPayload := map[string]any{
		"AttachStdout": false,
		"AttachStderr": false,
		"Tty":          false,
		"Cmd":          cmd,
	}
	body, _ := json.Marshal(createPayload)
	createURL := fmt.Sprintf("http://docker/containers/%s/exec", container)
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := client.Do(createReq)
	if err != nil {
		return "", err
	}
	defer createResp.Body.Close()
	if createResp.StatusCode < 200 || createResp.StatusCode >= 300 {
		return "", fmt.Errorf("Docker exec create 返回 %s", createResp.Status)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", fmt.Errorf("Docker 未返回 exec id")
	}

	startPayload := []byte(`{"Detach":true,"Tty":false}`)
	startURL := fmt.Sprintf("http://docker/exec/%s/start", created.ID)
	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, bytes.NewReader(startPayload))
	if err != nil {
		return "", err
	}
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := client.Do(startReq)
	if err != nil {
		return "", err
	}
	defer startResp.Body.Close()
	if startResp.StatusCode < 200 || startResp.StatusCode >= 300 {
		return "", fmt.Errorf("Docker exec start 返回 %s", startResp.Status)
	}
	return created.ID, nil
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	return value
}
