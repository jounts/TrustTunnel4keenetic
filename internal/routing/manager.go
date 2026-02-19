package routing

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	scriptPath   = "/opt/trusttunnel_client/smart-routing.sh"
	domainsPath  = "/opt/trusttunnel_client/routing/domains.txt"
	netsFile     = "/opt/trusttunnel_client/routing/domestic_nets.txt"
	netsUpdateTS = "/opt/trusttunnel_client/routing/nets_updated_ts"
	dnsmasqPID   = "/opt/var/run/dnsmasq-sr.pid"
)

type Stats struct {
	DomesticEntries int    `json:"domestic_entries"`
	TunnelEntries   int    `json:"tunnel_entries"`
	DnsmasqRunning  bool   `json:"dnsmasq_running"`
	NetsUpdated     string `json:"nets_updated"`
	FWBackend       string `json:"fw_backend"`
	NDMSMajor       int    `json:"ndms_major"`
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) GetDomains() (string, error) {
	data, err := os.ReadFile(domainsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (m *Manager) SaveDomains(content string) error {
	if err := os.MkdirAll("/opt/trusttunnel_client/routing", 0755); err != nil {
		return fmt.Errorf("create routing dir: %w", err)
	}
	if err := os.WriteFile(domainsPath, []byte(content), 0644); err != nil {
		return err
	}
	return m.reloadDnsmasq()
}

func (m *Manager) UpdateNets() error {
	out, err := m.runScript("sr_update_nets && sr_reload_nets")
	if err != nil {
		return fmt.Errorf("update nets: %w: %s", err, out)
	}
	return nil
}

func (m *Manager) GetStats() *Stats {
	s := &Stats{
		DomesticEntries: m.ipsetCount("tt_domestic"),
		TunnelEntries:   m.ipsetCount("tt_tunnel"),
		DnsmasqRunning:  m.isDnsmasqRunning(),
		FWBackend:       detectFWBackend(),
		NDMSMajor:       detectNDMSMajor(),
	}

	if data, err := os.ReadFile(netsUpdateTS); err == nil {
		ts := strings.TrimSpace(string(data))
		if tsInt, err := strconv.ParseInt(ts, 10, 64); err == nil {
			t := time.Unix(tsInt, 0)
			s.NetsUpdated = t.Format("2006-01-02 15:04:05")
		}
	}

	return s
}

func (m *Manager) Apply() error {
	out, err := m.runScript("sr_start")
	if err != nil {
		return fmt.Errorf("apply smart routing: %w: %s", err, out)
	}
	return nil
}

func (m *Manager) reloadDnsmasq() error {
	out, err := m.runScript("sr_reload_dnsmasq")
	if err != nil {
		return fmt.Errorf("reload dnsmasq: %w: %s", err, out)
	}
	return nil
}

func (m *Manager) runScript(fn string) (string, error) {
	// Source both compat and smart-routing scripts, then call the function
	cmd := fmt.Sprintf(
		`. /opt/trusttunnel_client/ndms-compat.sh 2>/dev/null; `+
			`. /opt/trusttunnel_client/mode.conf 2>/dev/null; `+
			`. %s && %s`,
		scriptPath, fn,
	)
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Printf("smart-routing script error: %s: %s", fn, string(out))
	}
	return strings.TrimSpace(string(out)), err
}

func (m *Manager) ipsetCount(name string) int {
	// Try ipset first, fall back to nft
	out, err := exec.Command("ipset", "list", name, "-t").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Number of entries:") {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					n, _ := strconv.Atoi(parts[3])
					return n
				}
			}
		}
	}

	// Try nft
	out, err = exec.Command("nft", "list", "set", "ip", "trusttunnel", name).CombinedOutput()
	if err == nil {
		count := 0
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "/") || strings.Contains(line, ".") {
				count++
			}
		}
		return count
	}

	return 0
}

func (m *Manager) isDnsmasqRunning() bool {
	data, err := os.ReadFile(dnsmasqPID)
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		return false
	}
	// Check if process is running
	_, err = os.Stat(fmt.Sprintf("/proc/%s", pid))
	return err == nil
}

func detectFWBackend() string {
	out, err := exec.Command("iptables", "--version").CombinedOutput()
	if err == nil {
		if strings.Contains(string(out), "nf_tables") {
			return "nftables"
		}
		return "iptables"
	}
	if _, err := exec.LookPath("nft"); err == nil {
		return "nftables"
	}
	return "unknown"
}

func detectNDMSMajor() int {
	data, err := os.ReadFile("/tmp/ndm/version")
	if err != nil {
		return 0
	}
	ver := strings.TrimSpace(string(data))
	if strings.HasPrefix(ver, "5.") {
		return 5
	}
	if strings.HasPrefix(ver, "4.") {
		return 4
	}
	if strings.HasPrefix(ver, "3.") {
		return 3
	}
	return 0
}
