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

// SyncVpnMode ensures the client TOML has the correct listener section for the
// selected mode (tun/socks5) and that vpn_mode is set (default "general").
func (c *ConfigManager) SyncVpnMode(mode string) error {
	data, _ := os.ReadFile(clientConfigPath)
	if len(data) == 0 {
		return nil
	}

	content := string(data)

	// Ensure vpn_mode is present (default "general", preserve "selective")
	content = ensureVpnMode(content)

	// Ensure correct [endpoint] structure (convert flat endpoint export if needed)
	content = ensureEndpointSection(content)

	// Manage [listener.*] sections based on mode
	content = removeTomlSection(content, "[listener.tun]")
	content = removeTomlSection(content, "[listener.socks]")

	content = strings.TrimRight(content, "\n") + "\n"

	if mode == "tun" {
		content += "\n[listener.tun]\nmtu_size = 1280\n"
	} else {
		content += "\n[listener.socks]\naddress = \"127.0.0.1:1080\"\n"
	}

	return os.WriteFile(clientConfigPath, []byte(content), 0644)
}

func ensureVpnMode(content string) string {
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "vpn_mode") && strings.Contains(trimmed, "=") {
			found = true
			val := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[1])
			val = strings.Trim(val, "\"")
			if val != "general" && val != "selective" {
				lines[i] = `vpn_mode = "general"`
			}
			break
		}
	}
	if !found {
		// Insert before the first non-comment, non-empty line
		insertIdx := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				insertIdx = i + 1
				continue
			}
			break
		}
		lines = append(lines[:insertIdx], append([]string{`vpn_mode = "general"`, ""}, lines[insertIdx:]...)...)
	}
	return strings.Join(lines, "\n")
}

// ensureEndpointSection wraps top-level endpoint fields in [endpoint] if that
// section doesn't already exist. Handles the flat format exported by the
// TrustTunnel endpoint binary.
func ensureEndpointSection(content string) string {
	if strings.Contains(content, "[endpoint]") {
		return content
	}

	endpointKeys := map[string]bool{
		"hostname": true, "addresses": true, "has_ipv6": true,
		"username": true, "password": true, "skip_verification": true,
		"certificate": true, "upstream_protocol": true,
		"upstream_fallback_protocol": true, "anti_dpi": true, "client_random": true,
	}

	lines := strings.Split(content, "\n")
	var topLines []string       // lines before endpoint fields
	var endpointLines []string  // endpoint key-value lines
	inMultiline := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inMultiline {
			endpointLines = append(endpointLines, line)
			if strings.Contains(trimmed, `"""`) {
				inMultiline = false
			}
			continue
		}

		if strings.HasPrefix(trimmed, "[") {
			topLines = append(topLines, line)
			continue
		}

		key := ""
		if eqIdx := strings.Index(trimmed, "="); eqIdx > 0 {
			key = strings.TrimSpace(trimmed[:eqIdx])
		}

		if endpointKeys[key] {
			endpointLines = append(endpointLines, line)
			val := ""
			if eqIdx := strings.Index(trimmed, "="); eqIdx > 0 {
				val = strings.TrimSpace(trimmed[eqIdx+1:])
			}
			if strings.HasPrefix(val, `"""`) && !strings.HasSuffix(val, `"""`) {
				inMultiline = true
			}
		} else {
			topLines = append(topLines, line)
		}
	}

	if len(endpointLines) == 0 {
		return content
	}

	var result []string
	result = append(result, topLines...)
	result = append(result, "", "[endpoint]")
	result = append(result, endpointLines...)
	return strings.Join(result, "\n")
}

// removeTomlSection removes a TOML section header and all its key-value lines
// up to the next section header or end of file.
func removeTomlSection(content, section string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inSection := false
	inMultiline := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inMultiline {
			if !inSection {
				result = append(result, line)
			}
			if strings.Contains(trimmed, `"""`) {
				inMultiline = false
			}
			continue
		}

		if trimmed == section {
			inSection = true
			continue
		}

		if inSection {
			if strings.HasPrefix(trimmed, "[") {
				inSection = false
				result = append(result, line)
			}
			continue
		}

		result = append(result, line)

		if eqIdx := strings.Index(trimmed, "="); eqIdx > 0 {
			val := strings.TrimSpace(trimmed[eqIdx+1:])
			if strings.HasPrefix(val, `"""`) && !strings.HasSuffix(val, `"""`) {
				inMultiline = true
			}
		}
	}
	return strings.Join(result, "\n")
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

