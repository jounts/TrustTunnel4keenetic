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
	HCEnabled       string `json:"hc_enabled"`
	HCInterval      int    `json:"hc_interval"`
	HCFailThreshold int    `json:"hc_fail_threshold"`
	HCGracePeriod   int    `json:"hc_grace_period"`
	HCTargetURL     string `json:"hc_target_url"`
	HCCurlTimeout   int    `json:"hc_curl_timeout"`
	HCSocks5Proxy   string `json:"hc_socks5_proxy"`
	// Smart routing settings
	SREnabled     string `json:"sr_enabled"`
	SRHomeCountry string `json:"sr_home_country"`
	SRDNSPort     int    `json:"sr_dns_port"`
	SRDNSUpstream string `json:"sr_dns_upstream"`
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
		SREnabled:       "no",
		SRHomeCountry:   "RU",
		SRDNSPort:       5354,
		SRDNSUpstream:   "1.1.1.1",
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
		case "SR_ENABLED":
			info.SREnabled = val
		case "SR_HOME_COUNTRY":
			info.SRHomeCountry = val
		case "SR_DNS_PORT":
			info.SRDNSPort, _ = strconv.Atoi(val)
		case "SR_DNS_UPSTREAM":
			info.SRDNSUpstream = val
		}
	}

	return info, nil
}

func (c *ConfigManager) WriteSRConfig(enabled, homeCountry, dnsUpstream string, dnsPort int) error {
	existing, _ := os.ReadFile(modeConfigPath)

	var content string
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
			case "SR_ENABLED", "SR_HOME_COUNTRY", "SR_DNS_PORT", "SR_DNS_UPSTREAM":
				continue
			default:
				content += line + "\n"
			}
		}
	}

	content += fmt.Sprintf("SR_ENABLED=\"%s\"\n", enabled)
	content += fmt.Sprintf("SR_HOME_COUNTRY=\"%s\"\n", homeCountry)
	content += fmt.Sprintf("SR_DNS_PORT=\"%d\"\n", dnsPort)
	content += fmt.Sprintf("SR_DNS_UPSTREAM=\"%s\"\n", dnsUpstream)

	return os.WriteFile(modeConfigPath, []byte(content), 0644)
}

// SyncVpnMode ensures vpn_mode in the client TOML matches the selected mode.
// Client expects a TOML string: "socks5" or "tun"
func (c *ConfigManager) SyncVpnMode(mode string) error {
	vpnMode := mode // "socks5" or "tun"

	data, _ := os.ReadFile(clientConfigPath)
	lines := strings.Split(string(data), "\n")

	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "vpn_mode") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				lines[i] = fmt.Sprintf("vpn_mode = \"%s\"", vpnMode)
				found = true
				break
			}
		}
	}

	if !found {
		insertIdx := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				insertIdx = i + 1
				continue
			}
			break
		}
		newLine := fmt.Sprintf("vpn_mode = \"%s\"", vpnMode)
		lines = append(lines[:insertIdx], append([]string{newLine, ""}, lines[insertIdx:]...)...)
	}

	return os.WriteFile(clientConfigPath, []byte(strings.Join(lines, "\n")), 0644)
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

