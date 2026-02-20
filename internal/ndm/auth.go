package ndm

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Authenticator struct {
	rciURL     string
	authURL    string
	httpClient *http.Client
	sessions   map[string]session
	mu         sync.RWMutex
	sessionTTL time.Duration
}

type session struct {
	user    string
	expires time.Time
}

const SessionCookieName = "tt_session"

// NewAuthenticator creates an authenticator that validates credentials
// against the Keenetic web UI. rciURL is the local RCI endpoint
// (e.g. http://localhost:79) used to auto-detect the router's LAN IP,
// because the /auth endpoint requires a LAN-level connection.
func NewAuthenticator(rciURL string) *Authenticator {
	a := &Authenticator{
		rciURL: strings.TrimRight(rciURL, "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		sessions:   make(map[string]session),
		sessionTTL: 24 * time.Hour,
	}

	a.authURL = a.resolveAuthURL()

	go a.cleanupLoop()
	return a
}

func (a *Authenticator) resolveAuthURL() string {
	if ip, err := a.detectLANIP(); err == nil {
		url := fmt.Sprintf("http://%s", ip)
		log.Printf("NDM auth URL: %s (auto-detected from Bridge0)", url)
		return url
	}

	for _, iface := range []string{"Bridge0", "Home", "ISP"} {
		if ip := a.detectInterfaceIP(iface); ip != "" {
			url := fmt.Sprintf("http://%s", ip)
			log.Printf("NDM auth URL: %s (from interface %s)", url, iface)
			return url
		}
	}

	fallback := "http://192.168.1.1"
	log.Printf("NDM auth URL: %s (hardcoded fallback, all interface detection failed)", fallback)
	return fallback
}

func (a *Authenticator) detectInterfaceIP(name string) string {
	resp, err := a.httpClient.Get(a.rciURL + "/rci/show/interface/" + name)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var iface struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&iface); err != nil {
		return ""
	}
	return iface.Address
}

func (a *Authenticator) detectLANIP() (string, error) {
	resp, err := a.httpClient.Get(a.rciURL + "/rci/show/interface/Bridge0")
	if err != nil {
		return "", fmt.Errorf("RCI request failed: %w", err)
	}
	defer resp.Body.Close()

	var iface struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&iface); err != nil {
		return "", fmt.Errorf("decode Bridge0: %w", err)
	}
	if iface.Address == "" {
		return "", fmt.Errorf("Bridge0 has no IPv4 address")
	}
	return iface.Address, nil
}

// Login validates credentials against the Keenetic NDM API using
// the challenge-response protocol. Returns a session token on success.
func (a *Authenticator) Login(username, password string) (string, error) {
	authURL := a.authURL + "/auth"

	resp, err := a.httpClient.Get(authURL)
	if err != nil {
		return "", fmt.Errorf("NDM unreachable at %s: %w", authURL, err)
	}
	resp.Body.Close()

	// Follow scheme-change redirects (HTTP → HTTPS).
	if isRedirect(resp.StatusCode) {
		if loc := resp.Header.Get("Location"); loc != "" {
			log.Printf("NDM auth: following redirect to %s", loc)
			authURL = loc
			resp, err = a.httpClient.Get(authURL)
			if err != nil {
				return "", fmt.Errorf("NDM unreachable after redirect to %s: %w", authURL, err)
			}
			resp.Body.Close()
		}
	}

	if resp.StatusCode == http.StatusOK {
		return a.createSession(username), nil
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return "", fmt.Errorf("unexpected NDM status %d from %s", resp.StatusCode, authURL)
	}

	realm := resp.Header.Get("X-NDM-Realm")
	challenge := resp.Header.Get("X-NDM-Challenge")
	if realm == "" || challenge == "" {
		return "", fmt.Errorf("NDM did not return auth challenge (realm=%q, challenge=%q)", realm, challenge)
	}

	cookies := resp.Cookies()

	md5sum := md5.Sum([]byte(username + ":" + realm + ":" + password))
	md5hex := fmt.Sprintf("%x", md5sum)
	shasum := sha256.Sum256([]byte(challenge + md5hex))
	shahex := fmt.Sprintf("%x", shasum)

	body := fmt.Sprintf(`{"login":%q,"password":%q}`, username, shahex)
	req, err := http.NewRequest("POST", authURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	authResp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("NDM auth POST failed: %w", err)
	}
	authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid credentials (NDM returned %d)", authResp.StatusCode)
	}

	return a.createSession(username), nil
}

func isRedirect(code int) bool {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308
}

// ValidateSession checks if a session token is valid and not expired.
func (a *Authenticator) ValidateSession(token string) bool {
	if token == "" {
		return false
	}
	a.mu.RLock()
	s, ok := a.sessions[token]
	a.mu.RUnlock()
	return ok && time.Now().Before(s.expires)
}

// DestroySession removes a session token.
func (a *Authenticator) DestroySession(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

func (a *Authenticator) createSession(user string) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	a.mu.Lock()
	a.sessions[token] = session{user: user, expires: time.Now().Add(a.sessionTTL)}
	a.mu.Unlock()

	return token
}

func (a *Authenticator) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		a.mu.Lock()
		for k, s := range a.sessions {
			if now.After(s.expires) {
				delete(a.sessions, k)
			}
		}
		a.mu.Unlock()
	}
}
