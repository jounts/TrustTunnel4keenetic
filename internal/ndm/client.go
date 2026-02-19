package ndm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
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
		return c.setupProxyInterface(proxyIdx)
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
		fmt.Sprintf("interface %s ip global auto", name),
		fmt.Sprintf("interface %s security-level public", name),
		"system configuration save",
	}
	return c.runNDMC(commands)
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
	return c.runNDMC(commands)
}

func (c *Client) RemoveInterface(name string) error {
	return c.runNDMC([]string{
		fmt.Sprintf("no interface %s", name),
		"system configuration save",
	})
}

func (c *Client) runNDMC(commands []string) error {
	for _, cmd := range commands {
		out, err := exec.Command("ndmc", "-c", cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ndmc -c %q: %w: %s", cmd, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
