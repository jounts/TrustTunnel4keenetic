package platform

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type SystemInfo struct {
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	NDMSMajor    int    `json:"ndms_major"`
	FWBackend    string `json:"fw_backend"`
	Architecture string `json:"architecture"`
	Hostname     string `json:"hostname"`
	Uptime       string `json:"uptime"`
}

type Info struct{}

func NewInfo() *Info {
	return &Info{}
}

func (i *Info) Get() *SystemInfo {
	firmware := readFirstLine("/tmp/ndm/version")
	return &SystemInfo{
		Model:        readFirstLine("/tmp/ndm/hw_type"),
		Firmware:     firmware,
		NDMSMajor:    detectNDMSMajor(firmware),
		FWBackend:    detectFWBackend(),
		Architecture: runtime.GOARCH,
		Hostname:     getHostname(),
		Uptime:       readFirstLine("/proc/uptime"),
	}
}

func detectNDMSMajor(firmware string) int {
	if strings.HasPrefix(firmware, "5.") {
		return 5
	}
	if strings.HasPrefix(firmware, "3.") {
		return 3
	}
	if strings.HasPrefix(firmware, "4.") {
		return 4
	}
	return 0
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

func readFirstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	s := strings.TrimSpace(string(data))
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "keenetic"
	}
	return h
}
