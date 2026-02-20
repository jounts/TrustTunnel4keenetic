package ndm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type parseRequest struct {
	Parse string `json:"parse"`
}

type parseResponse struct {
	Parse struct {
		Status []rciStatus `json:"status,omitempty"`
	} `json:"parse"`
}

type rciStatus struct {
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	ndmsMajor  int
}

func NewClient(baseURL string) *Client {
	c := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		ndmsMajor:  detectNDMSMajor(),
	}
	log.Printf("NDM client initialized (NDMS %d detected)", c.ndmsMajor)
	return c
}

func (c *Client) NDMSMajor() int {
	return c.ndmsMajor
}

func (c *Client) ShowInterface(name string) (map[string]any, error) {
	url := fmt.Sprintf("%s/rci/show/interface/%s", c.baseURL, name)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) RecreateInterface(mode string, tunIdx, proxyIdx int) error {
	if mode == "socks5" {
		// Remove TUN interface when switching to SOCKS5
		tunName := fmt.Sprintf("OpkgTun%d", tunIdx)
		if err := c.RemoveInterface(tunName); err != nil {
			log.Printf("ndmc: failed to remove %s (may not exist): %v", tunName, err)
		}
		return c.setupProxyInterface(proxyIdx)
	}
	// Remove Proxy interface when switching to TUN
	proxyName := fmt.Sprintf("Proxy%d", proxyIdx)
	if err := c.RemoveInterface(proxyName); err != nil {
		log.Printf("ndmc: failed to remove %s (may not exist): %v", proxyName, err)
	}
	return c.setupTunInterface(tunIdx)
}

func (c *Client) setupProxyInterface(idx int) error {
	name := fmt.Sprintf("Proxy%d", idx)
	commands := []string{
		fmt.Sprintf("interface %s", name),
		fmt.Sprintf("interface %s description \"TrustTunnel Proxy %d\"", name, idx),
		fmt.Sprintf("interface %s proxy protocol socks5", name),
		fmt.Sprintf("interface %s proxy upstream 127.0.0.1 1080", name),
		fmt.Sprintf("interface %s proxy connect", name),
	}

	// ip global auto syntax is the same on NDMS 4 and 5
	commands = append(commands,
		fmt.Sprintf("interface %s ip global auto", name),
		fmt.Sprintf("interface %s security-level public", name),
		"system configuration save",
	)
	return c.runRCI(commands)
}

func (c *Client) setupTunInterface(idx int) error {
	name := fmt.Sprintf("OpkgTun%d", idx)
	commands := []string{
		fmt.Sprintf("interface %s", name),
		fmt.Sprintf("interface %s description \"TrustTunnel TUN %d\"", name, idx),
		fmt.Sprintf("interface %s ip address 172.16.219.2 255.255.255.255", name),
		fmt.Sprintf("interface %s ip global auto", name),
		fmt.Sprintf("interface %s ip mtu 1280", name),
		fmt.Sprintf("interface %s ip tcp adjust-mss pmtu", name),
		fmt.Sprintf("interface %s security-level public", name),
		fmt.Sprintf("interface %s up", name),
		fmt.Sprintf("ip route default 172.16.219.2 %s", name),
		"system configuration save",
	}
	return c.runRCI(commands)
}

func (c *Client) RemoveInterface(name string) error {
	// "no interface" removes the interface; also remove associated route
	cmds := []string{
		fmt.Sprintf("no interface %s", name),
	}
	// Clean up default route pointing to TUN interfaces
	if strings.HasPrefix(name, "OpkgTun") {
		cmds = append(cmds, fmt.Sprintf("no ip route default 172.16.219.2 %s", name))
	}
	cmds = append(cmds, "system configuration save")
	return c.runRCI(cmds)
}

// runRCI sends CLI commands to the router via HTTP RCI API (POST /rci/).
// Each command is sent as {"parse": "..."} in a JSON array.
// Falls back to the ndmc CLI binary if the HTTP request itself fails.
func (c *Client) runRCI(commands []string) error {
	batch := make([]parseRequest, len(commands))
	for i, cmd := range commands {
		batch[i] = parseRequest{Parse: cmd}
	}

	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("rci: marshal: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/rci/", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("RCI HTTP request failed, falling back to ndmc: %v", err)
		return c.runNDMCFallback(commands)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("rci: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("RCI returned %d, falling back to ndmc", resp.StatusCode)
		return c.runNDMCFallback(commands)
	}

	var results []parseResponse
	if err := json.Unmarshal(respBody, &results); err != nil {
		log.Printf("RCI response parse error (%s), falling back to ndmc", err)
		return c.runNDMCFallback(commands)
	}

	for i, res := range results {
		cmd := ""
		if i < len(commands) {
			cmd = commands[i]
		}
		for _, st := range res.Parse.Status {
			if st.Status == "error" {
				if isNonCriticalRCIError(cmd, st) {
					log.Printf("rci warning (NDMS %d): %q: %s (%s)", c.ndmsMajor, cmd, st.Message, st.Code)
					continue
				}
				return fmt.Errorf("rci %q: %s: %s", cmd, st.Code, st.Message)
			}
		}
	}
	return nil
}

// runNDMCFallback executes commands one-by-one via the ndmc CLI binary.
// Used when the HTTP RCI endpoint is unreachable.
func (c *Client) runNDMCFallback(commands []string) error {
	for _, cmd := range commands {
		out, err := exec.Command("ndmc", "-c", cmd).CombinedOutput()
		if err != nil {
			outStr := strings.TrimSpace(string(out))
			if isNonCriticalNDMCOutput(cmd, outStr) {
				log.Printf("ndmc warning (NDMS %d): %q: %s", c.ndmsMajor, cmd, outStr)
				continue
			}
			return fmt.Errorf("ndmc -c %q: %w: %s", cmd, err, outStr)
		}
	}
	return nil
}

func isNonCriticalRCIError(cmd string, st rciStatus) bool {
	lower := strings.ToLower(st.Message)
	code := strings.ToLower(st.Code)

	if strings.Contains(lower, "already") {
		return true
	}
	if strings.Contains(lower, "exist") && strings.Contains(cmd, "ip route") {
		return true
	}
	if strings.HasPrefix(cmd, "no ") && (strings.Contains(lower, "not found") ||
		strings.Contains(lower, "unable to find") ||
		strings.Contains(lower, "no such") ||
		strings.Contains(code, "not_found")) {
		return true
	}
	if strings.Contains(lower, "unsupported") {
		return true
	}
	return false
}

func isNonCriticalNDMCOutput(cmd, output string) bool {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "already") {
		return true
	}
	if strings.Contains(lower, "exist") && strings.Contains(cmd, "ip route") {
		return true
	}
	if strings.HasPrefix(cmd, "no ") && (strings.Contains(lower, "not found") || strings.Contains(lower, "unable to find") || strings.Contains(lower, "no such")) {
		return true
	}
	if strings.Contains(lower, "unsupported") {
		return true
	}
	return false
}

func detectNDMSMajor() int {
	ver := ""
	if data, err := os.ReadFile("/tmp/ndm/version"); err == nil {
		ver = strings.TrimSpace(string(data))
	}

	// Fallback: RCI HTTP API
	if ver == "" {
		ver = detectVersionViaRCI()
	}

	// Last resort: ndmc CLI
	if ver == "" {
		if out, err := exec.Command("ndmc", "-c", "show version").CombinedOutput(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "release" {
					ver = strings.TrimSpace(v)
					break
				}
			}
		}
	}

	switch {
	case strings.HasPrefix(ver, "5."):
		return 5
	case strings.HasPrefix(ver, "3."):
		return 3
	default:
		return 4
	}
}

func detectVersionViaRCI() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:79/rci/show/version")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Title   string `json:"title"`
		Release string `json:"release"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	if result.Release != "" {
		return result.Release
	}
	return result.Title
}
