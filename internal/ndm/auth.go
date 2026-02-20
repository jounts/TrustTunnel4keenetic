package ndm

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"sort"
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
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cache:    make(map[string]cacheEntry),
		cacheTTL: 60 * time.Second,
	}
}

// VerifyByCookies checks if the provided cookies represent a valid
// Keenetic NDM session by forwarding them to the NDM auth endpoint.
func (a *Authenticator) VerifyByCookies(cookies []*http.Cookie) bool {
	if len(cookies) == 0 {
		return false
	}

	key := cookieCacheKey(cookies)

	a.mu.RLock()
	if entry, ok := a.cache[key]; ok && time.Now().Before(entry.expires) {
		a.mu.RUnlock()
		return entry.valid
	}
	a.mu.RUnlock()

	valid := a.checkNDMSession(cookies)

	a.mu.Lock()
	a.cache[key] = cacheEntry{valid: valid, expires: time.Now().Add(a.cacheTTL)}
	now := time.Now()
	for k, v := range a.cache {
		if now.After(v.expires) {
			delete(a.cache, k)
		}
	}
	a.mu.Unlock()

	return valid
}

func (a *Authenticator) checkNDMSession(cookies []*http.Cookie) bool {
	req, err := http.NewRequest("GET", a.baseURL+"/auth", nil)
	if err != nil {
		return false
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func cookieCacheKey(cookies []*http.Cookie) string {
	pairs := make([]string, 0, len(cookies))
	for _, c := range cookies {
		pairs = append(pairs, c.Name+"="+c.Value)
	}
	sort.Strings(pairs)
	h := sha256.Sum256([]byte(strings.Join(pairs, ";")))
	return fmt.Sprintf("%x", h)
}
