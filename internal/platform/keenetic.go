package platform

import (
	"os"
	"runtime"
	"strings"
)

type SystemInfo struct {
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	Architecture string `json:"architecture"`
	Hostname     string `json:"hostname"`
	Uptime       string `json:"uptime"`
}

type Info struct{}

func NewInfo() *Info {
	return &Info{}
}

func (i *Info) Get() *SystemInfo {
	return &SystemInfo{
		Model:        readFirstLine("/tmp/ndm/hw_type"),
		Firmware:     readFirstLine("/tmp/ndm/version"),
		Architecture: runtime.GOARCH,
		Hostname:     getHostname(),
		Uptime:       readFirstLine("/proc/uptime"),
	}
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
