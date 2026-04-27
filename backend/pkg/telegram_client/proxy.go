package telegram_client

import (
	"encoding/json"
	"strings"
)

type ProxyConfig struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (p ProxyConfig) Enabled() bool {
	return strings.TrimSpace(p.Host) != "" && p.Port > 0
}

func AppendProxyArgs(args []string, proxy ProxyConfig) []string {
	if !proxy.Enabled() {
		return args
	}
	payload, err := json.Marshal(proxy)
	if err != nil {
		return args
	}
	return append(args, "--proxy-json", string(payload))
}
