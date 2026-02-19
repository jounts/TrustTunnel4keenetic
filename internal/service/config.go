package service

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	clientConfigPath = "/opt/trusttunnel_client/trusttunnel_client.toml"
	modeConfigPath   = "/opt/trusttunnel_client/mode.conf"
)

type AllConfig struct {
	ClientConfig string     `json:"client_config"`
	ModeConfig   string     `json:"mode_config"`
	Mode         *ModeInfo  `json:"mode"`
}

type ModeInfo struct {
	Mode     string `json:"mode"`
	TunIdx   int    `json:"tun_idx"`
	ProxyIdx int    `json:"proxy_idx"`
	// Health check settings
	HCEnabled        string `json:"hc_enabled"`
	HCInterval       int    `json:"hc_interval"`
	HCFailThreshold  int    `json:"hc_fail_threshold"`
	HCGracePeriod    int    `json:"hc_grace_period"`
	HCTargetURL      string `json:"hc_target_url"`
	HCCurlTimeout    int    `json:"hc_curl_timeout"`
	HCSocks5Proxy    string `json:"hc_socks5_proxy"`
}

type ConfigManager struct{}

func NewConfigManager() *ConfigManager {
	return &ConfigManager{}
}

func (c *ConfigManager) ReadAll() (*AllConfig, error) {
	clientCfg, _ := os.ReadFile(clientConfigPath)
	modeCfg, _ := os.ReadFile(modeConfigPath)

	mode, _ := c.ReadMode()

	return &AllConfig{
		ClientConfig: string(clientCfg),
		ModeConfig:   string(modeCfg),
		Mode:         mode,
	}, nil
}

func (c *ConfigManager) WriteAll(clientConfig, modeConfig string) error {
	if clientConfig != "" {
		if err := os.WriteFile(clientConfigPath, []byte(clientConfig), 0644); err != nil {
			return fmt.Errorf("write client config: %w", err)
		}
	}
	if modeConfig != "" {
		if err := os.WriteFile(modeConfigPath, []byte(modeConfig), 0644); err != nil {
			return fmt.Errorf("write mode config: %w", err)
		}
	}
	return nil
}

func (c *ConfigManager) ReadMode() (*ModeInfo, error) {
	data, err := os.ReadFile(modeConfigPath)
	if err != nil {
		return &ModeInfo{Mode: "socks5"}, nil
	}

	info := &ModeInfo{
		Mode:            "socks5",
		HCEnabled:       "yes",
		HCInterval:      30,
		HCFailThreshold: 3,
		HCGracePeriod:   60,
		HCTargetURL:     "http://connectivitycheck.gstatic.com/generate_204",
		HCCurlTimeout:   5,
		HCSocks5Proxy:   "127.0.0.1:1080",
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		switch key {
		case "TT_MODE":
			info.Mode = val
		case "TUN_IDX":
			info.TunIdx, _ = strconv.Atoi(val)
		case "PROXY_IDX":
			info.ProxyIdx, _ = strconv.Atoi(val)
		case "HC_ENABLED":
			info.HCEnabled = val
		case "HC_INTERVAL":
			info.HCInterval, _ = strconv.Atoi(val)
		case "HC_FAIL_THRESHOLD":
			info.HCFailThreshold, _ = strconv.Atoi(val)
		case "HC_GRACE_PERIOD":
			info.HCGracePeriod, _ = strconv.Atoi(val)
		case "HC_TARGET_URL":
			info.HCTargetURL = val
		case "HC_CURL_TIMEOUT":
			info.HCCurlTimeout, _ = strconv.Atoi(val)
		case "HC_SOCKS5_PROXY":
			info.HCSocks5Proxy = val
		}
	}

	return info, nil
}

func (c *ConfigManager) WriteMode(mode string, tunIdx, proxyIdx int) error {
	content := fmt.Sprintf(`TT_MODE="%s"
TUN_IDX="%d"
PROXY_IDX="%d"
`, mode, tunIdx, proxyIdx)

	existing, _ := os.ReadFile(modeConfigPath)
	if len(existing) > 0 {
		for _, line := range strings.Split(string(existing), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			switch key {
			case "TT_MODE", "TUN_IDX", "PROXY_IDX":
				continue
			default:
				content += line + "\n"
			}
		}
	}

	return os.WriteFile(modeConfigPath, []byte(content), 0644)
}
