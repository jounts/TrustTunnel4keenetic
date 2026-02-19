package service

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	initScript        = "/opt/etc/init.d/S99trusttunnel"
	pidFile           = "/opt/var/run/trusttunnel.pid"
	watchdogPID       = "/opt/var/run/trusttunnel_watchdog.pid"
	hcStateFile       = "/opt/var/run/trusttunnel_hc_state"
	startTSFile       = "/opt/var/run/trusttunnel_start_ts"
	clientBin         = "/opt/trusttunnel_client/trusttunnel_client"
	clientVersionFile = "/opt/trusttunnel_client/.client_version"
)

type ServiceStatus struct {
	Running       bool   `json:"running"`
	PID           int    `json:"pid"`
	Uptime        int64  `json:"uptime_seconds"`
	Mode          string `json:"mode"`
	WatchdogAlive bool   `json:"watchdog_alive"`
	HealthCheck   string `json:"health_check"`
	ClientVersion string `json:"client_version"`
}

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Status() (*ServiceStatus, error) {
	s := &ServiceStatus{}

	pid := readPIDFile(pidFile)
	if pid > 0 && processAlive(pid) {
		s.Running = true
		s.PID = pid
	}

	if ts := readFileStr(startTSFile); ts != "" {
		if t, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64); err == nil {
			s.Uptime = time.Now().Unix() - t
		}
	}

	wpid := readPIDFile(watchdogPID)
	s.WatchdogAlive = wpid > 0 && processAlive(wpid)

	s.HealthCheck = strings.TrimSpace(readFileStr(hcStateFile))
	if s.HealthCheck == "" {
		s.HealthCheck = "unknown"
	}

	modeConf := NewConfigManager()
	if mode, err := modeConf.ReadMode(); err == nil {
		s.Mode = mode.Mode
	}

	s.ClientVersion = detectClientVersion()

	return s, nil
}

func (m *Manager) Control(action string) (string, error) {
	cmd := exec.Command(initScript, action)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s failed: %w: %s", action, err, string(out))
	}
	return string(out), nil
}

func readPIDFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

func readFileStr(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func detectClientVersion() string {
	if data, err := os.ReadFile(clientVersionFile); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	if _, err := os.Stat(clientBin); err == nil {
		return "installed"
	}
	return "unknown"
}

// parseVersionFromDirName extracts version from archive directory name
// e.g. "trusttunnel_client-v0.99.105-linux-aarch64" → "v0.99.105"
func parseVersionFromDirName(name string) string {
	name = strings.TrimPrefix(name, "trusttunnel_client-")
	parts := strings.SplitN(name, "-", 2)
	if len(parts) > 0 && strings.HasPrefix(parts[0], "v") {
		return parts[0]
	}
	return ""
}
