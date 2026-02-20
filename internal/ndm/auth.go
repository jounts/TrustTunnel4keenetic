package ndm

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Authenticator struct {
	baseURL    string
	httpClient *http.Client
	cache      map[string]cacheEntry
	mu         sync.RWMutex
	cacheTTL   time.Duration
}

type cacheEntry struct {
	valid   bool
	expires time.Time
}

func NewAuthenticator(baseURL string) *Authenticator {
	return &Authenticator{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 3 * time.Second},
		cache:      make(map[string]cacheEntry),
		cacheTTL:   5 * time.Minute,
	}
}

// Verify checks credentials against the Keenetic NDM auth API.
// Results are cached to avoid hitting NDM on every HTTP request.
func (a *Authenticator) Verify(username, password string) bool {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(username+":"+password)))

	a.mu.RLock()
	if entry, ok := a.cache[key]; ok && time.Now().Before(entry.expires) {
		a.mu.RUnlock()
		return entry.valid
	}
	a.mu.RUnlock()

	valid := a.verifyNDM(username, password)

	a.mu.Lock()
	a.cache[key] = cacheEntry{valid: valid, expires: time.Now().Add(a.cacheTTL)}
	// Evict expired entries
	now := time.Now()
	for k, v := range a.cache {
		if now.After(v.expires) {
			delete(a.cache, k)
		}
	}
	a.mu.Unlock()

	return valid
}

func (a *Authenticator) verifyNDM(username, password string) bool {
	// Step 1: GET /auth to obtain challenge
	resp, err := a.httpClient.Get(a.baseURL + "/auth")
	if err != nil {
		return false
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Already authenticated (shouldn't happen with fresh client)
		return true
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return false
	}

	realm := resp.Header.Get("X-NDM-Realm")
	challenge := resp.Header.Get("X-NDM-Challenge")
	if realm == "" || challenge == "" {
		return false
	}

	// Step 2: md5(login:realm:password)
	md5Hash := md5.Sum([]byte(username + ":" + realm + ":" + password))
	md5Hex := fmt.Sprintf("%x", md5Hash)

	// Step 3: sha256(challenge + md5hex)
	shaHash := sha256.Sum256([]byte(challenge + md5Hex))
	shaHex := fmt.Sprintf("%x", shaHash)

	// Step 4: POST /auth with computed password
	body := fmt.Sprintf(`{"login":%q,"password":%q}`, username, shaHex)
	req, err := http.NewRequest("POST", a.baseURL+"/auth", strings.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	authResp, err := a.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer authResp.Body.Close()

	if authResp.StatusCode == http.StatusOK {
		return true
	}

	// Check response body for error details
	var result map[string]interface{}
	json.NewDecoder(authResp.Body).Decode(&result)

	return false
}
