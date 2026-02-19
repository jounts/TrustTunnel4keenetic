package routing

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	routingDir    = "/opt/trusttunnel_client/routing"
	domainsFile   = routingDir + "/domains.txt"
	domesticNets  = routingDir + "/domestic_nets.txt"
	smartRoutingSH = "/opt/trusttunnel_client/smart-routing.sh"
	logFile       = "/opt/var/log/trusttunnel.log"
	cidrBaseURL   = "https://raw.githubusercontent.com/herrbischoff/country-ip-blocks/master/ipv4"
)

type Stats struct {
	DomesticEntries int    `json:"domestic_entries"`
	TunnelEntries   int    `json:"tunnel_entries"`
	DnsmasqRunning  bool   `json:"dnsmasq_running"`
	NetsUpdatedAt   string `json:"nets_updated_at"`
}

type Manager struct {
	httpClient *http.Client
}

func NewManager() *Manager {
	return &Manager{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (m *Manager) GetDomains() ([]string, error) {
	data, err := os.ReadFile(domainsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var domains []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			domains = append(domains, line)
		}
	}
	return domains, nil
}

func (m *Manager) SaveDomains(domains []string) error {
	if err := os.MkdirAll(routingDir, 0755); err != nil {
		return fmt.Errorf("create routing dir: %w", err)
	}

	content := strings.Join(domains, "\n")
	if len(domains) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(domainsFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write domains: %w", err)
	}

	m.reloadDnsmasq()
	return nil
}

func (m *Manager) UpdateNets(country string) error {
	country = strings.ToLower(strings.TrimSpace(country))
	if country == "" {
		country = "ru"
	}

	url := fmt.Sprintf("%s/%s.cidr", cidrBaseURL, country)
	resp, err := m.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("download CIDRs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download CIDRs: HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(routingDir, 0755); err != nil {
		return fmt.Errorf("create routing dir: %w", err)
	}

	tmpFile := domesticNets + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("write CIDRs: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpFile, domesticNets); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename CIDRs file: %w", err)
	}

	m.reloadNets()
	return nil
}

func (m *Manager) GetStats() (*Stats, error) {
	stats := &Stats{}

	stats.DomesticEntries = ipsetCount("tt_domestic")
	stats.TunnelEntries = ipsetCount("tt_tunnel")
	stats.DnsmasqRunning = isDnsmasqRunning()

	if info, err := os.Stat(domesticNets); err == nil {
		stats.NetsUpdatedAt = info.ModTime().Format(time.RFC3339)
	}

	return stats, nil
}

func (m *Manager) Apply() error {
	return runSmartRoutingCmd("apply")
}

func ipsetCount(name string) int {
	out, err := exec.Command("ipset", "list", name, "-t").CombinedOutput()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Number of entries:") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				var n int
				fmt.Sscanf(parts[3], "%d", &n)
				return n
			}
		}
	}
	return 0
}

func isDnsmasqRunning() bool {
	pidFile := "/opt/var/run/tt_dnsmasq.pid"
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		return false
	}
	return exec.Command("kill", "-0", pid).Run() == nil
}

func (m *Manager) reloadDnsmasq() {
	runSmartRoutingCmd("reload_dnsmasq")
}

func (m *Manager) reloadNets() {
	runSmartRoutingCmd("reload_nets")
}

func runSmartRoutingCmd(action string) error {
	scriptPath := smartRoutingSH
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		abs, _ := filepath.Abs("scripts/smart-routing.sh")
		if _, err := os.Stat(abs); err == nil {
			scriptPath = abs
		}
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf(
		`. /opt/trusttunnel_client/mode.conf 2>/dev/null; LOG_FILE="%s"; . "%s"; sr_%s`,
		logFile, scriptPath, action,
	))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smart-routing %s: %w: %s", action, err, strings.TrimSpace(string(out)))
	}
	return nil
}
