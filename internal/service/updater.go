package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	clientRepo     = "TrustTunnel/TrustTunnelClient"
	managerRepo    = "jounts/TrustTunnel4keenetic"
	githubAPI      = "https://api.github.com/repos"
)

type UpdateInfo struct {
	ClientCurrentVersion  string `json:"client_current_version"`
	ClientLatestVersion   string `json:"client_latest_version"`
	ClientUpdateAvailable bool   `json:"client_update_available"`
	ClientCheckError      string `json:"client_check_error,omitempty"`
	ManagerCurrentVersion string `json:"manager_current_version"`
	ManagerLatestVersion  string `json:"manager_latest_version"`
	ManagerUpdateAvailable bool  `json:"manager_update_available"`
	ManagerCheckError     string `json:"manager_check_error,omitempty"`
}

type UpdateResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Version string `json:"version"`
}

const userAgent = "TrustTunnel-Manager/1.0"

type etagEntry struct {
	etag    string
	version string
}

type Updater struct {
	mu         sync.Mutex
	httpClient *http.Client
	cache      *UpdateInfo
	cacheTime  time.Time
	etagCache  map[string]etagEntry
}

func NewUpdater() *Updater {
	return &Updater{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		etagCache:  make(map[string]etagEntry),
	}
}

func (u *Updater) InvalidateCache() {
	u.mu.Lock()
	u.cache = nil
	u.mu.Unlock()
}

func (u *Updater) Check() (*UpdateInfo, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.cache != nil && time.Since(u.cacheTime) < 30*time.Minute {
		log.Printf("[update] returning cached result (age %s)", time.Since(u.cacheTime).Round(time.Second))
		return u.cache, nil
	}

	info := &UpdateInfo{}

	info.ClientCurrentVersion = detectClientVersion()
	log.Printf("[update] client current version: %s", info.ClientCurrentVersion)

	if latest, err := u.latestRelease(clientRepo); err == nil {
		info.ClientLatestVersion = latest
		info.ClientUpdateAvailable = latest != "" && latest != info.ClientCurrentVersion
	} else {
		info.ClientCheckError = err.Error()
		log.Printf("[update] failed to check client release: %v", err)
	}

	info.ManagerCurrentVersion = managerVersion()
	log.Printf("[update] manager current version: %s", info.ManagerCurrentVersion)

	if latest, err := u.latestRelease(managerRepo); err == nil {
		info.ManagerLatestVersion = latest
		info.ManagerUpdateAvailable = latest != "" && latest != info.ManagerCurrentVersion
	} else {
		info.ManagerCheckError = err.Error()
		log.Printf("[update] failed to check manager release: %v", err)
	}

	log.Printf("[update] check complete: client=%s->%s manager=%s->%s",
		info.ClientCurrentVersion, info.ClientLatestVersion,
		info.ManagerCurrentVersion, info.ManagerLatestVersion)

	u.cache = info
	u.cacheTime = time.Now()
	return info, nil
}

func (u *Updater) Install() (*UpdateResult, error) {
	arch := detectArch()
	osName := "linux"

	assetSuffix := fmt.Sprintf("-%s-%s.tar.gz", osName, arch)
	log.Printf("[update] searching asset: prefix=%q suffix=%q", "trusttunnel_client", assetSuffix)

	downloadURL, err := u.findAssetURL(clientRepo, "trusttunnel_client", assetSuffix)
	if err != nil {
		return nil, fmt.Errorf("find asset: %w", err)
	}
	log.Printf("[update] downloading %s", downloadURL)

	tmpFile := "/tmp/trusttunnel_update.tar.gz"
	if err := u.download(downloadURL, tmpFile); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpFile)

	if info, err := os.Stat(tmpFile); err == nil {
		log.Printf("[update] downloaded %d bytes", info.Size())
	}

	svcMgr := NewManager()
	log.Printf("[update] stopping TrustTunnel client")
	svcMgr.Control("stop")

	log.Printf("[update] extracting to /opt/trusttunnel_client/")
	tmpDir := "/tmp/trusttunnel_extract"
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("tar", "xzf", tmpFile, "-C", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[update] extract failed: %v: %s", err, string(out))
		svcMgr.Control("start")
		return nil, fmt.Errorf("extract: %w: %s", err, string(out))
	}

	// BusyBox tar has no --strip-components; find the binary inside extracted subdirectory
	entries, _ := os.ReadDir(tmpDir)
	srcDir := tmpDir
	dirName := ""
	if len(entries) == 1 && entries[0].IsDir() {
		dirName = entries[0].Name()
		srcDir = tmpDir + "/" + dirName
	}

	srcBin := srcDir + "/trusttunnel_client"
	if _, err := os.Stat(srcBin); err != nil {
		svcMgr.Control("start")
		return nil, fmt.Errorf("binary not found in archive at %s", srcBin)
	}

	if out, err := exec.Command("cp", "-f", srcBin, clientBin).CombinedOutput(); err != nil {
		log.Printf("[update] copy failed: %v: %s", err, string(out))
		svcMgr.Control("start")
		return nil, fmt.Errorf("copy binary: %w: %s", err, string(out))
	}
	os.Chmod(clientBin, 0755)

	// Parse version from directory name (e.g. "trusttunnel_client-v0.99.105-linux-aarch64")
	newVer := parseVersionFromDirName(dirName)
	if newVer != "" {
		os.WriteFile(clientVersionFile, []byte(newVer+"\n"), 0644)
		log.Printf("[update] saved client version: %s", newVer)
	}

	log.Printf("[update] starting TrustTunnel client")
	svcMgr.Control("start")
	u.cache = nil
	log.Printf("[update] complete, version: %s", newVer)
	return &UpdateResult{
		Success: true,
		Message: "Updated successfully",
		Version: newVer,
	}, nil
}

const (
	managerBin        = "/opt/trusttunnel_client/trusttunnel-manager"
	managerInitScript = "/opt/etc/init.d/S98trusttunnel-manager"
)

func (u *Updater) InstallManager() (*UpdateResult, error) {
	arch := detectArch()
	assetName := fmt.Sprintf("trusttunnel-manager-linux-%s", arch)
	log.Printf("[update-manager] searching asset: %s", assetName)

	downloadURL, err := u.findAssetURL(managerRepo, assetName, "")
	if err != nil {
		return nil, fmt.Errorf("find asset: %w", err)
	}
	log.Printf("[update-manager] downloading %s", downloadURL)

	tmpFile := "/tmp/trusttunnel-manager-new"
	if err := u.download(downloadURL, tmpFile); err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("download: %w", err)
	}

	if info, err := os.Stat(tmpFile); err == nil {
		log.Printf("[update-manager] downloaded %d bytes", info.Size())
	}

	if err := os.Chmod(tmpFile, 0755); err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("chmod: %w", err)
	}

	if out, err := exec.Command("cp", "-f", tmpFile, managerBin).CombinedOutput(); err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("replace binary: %w: %s", err, string(out))
	}
	os.Remove(tmpFile)

	latestVer, _ := u.latestRelease(managerRepo)
	log.Printf("[update-manager] binary replaced, scheduling restart, new version: %s", latestVer)

	// Detached restart: survives current process termination
	exec.Command("sh", "-c",
		fmt.Sprintf("sleep 1 && %s restart", managerInitScript),
	).Start()

	return &UpdateResult{
		Success: true,
		Message: "Manager updated, restarting...",
		Version: latestVer,
	}, nil
}

func (u *Updater) latestRelease(repo string) (string, error) {
	url := fmt.Sprintf("%s/%s/releases?per_page=1", githubAPI, repo)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", userAgent)

	if cached, ok := u.etagCache[repo]; ok && cached.etag != "" {
		req.Header.Set("If-None-Match", cached.etag)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		if cached, ok := u.etagCache[repo]; ok && cached.version != "" {
			log.Printf("[update] request to %s failed: %v, using cached version %s", url, err, cached.version)
			return cached.version, nil
		}
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	remaining := resp.Header.Get("X-Ratelimit-Remaining")
	log.Printf("[update] %s HTTP %d (rate-limit remaining: %s)", url, resp.StatusCode, remaining)

	if resp.StatusCode == http.StatusNotModified {
		if cached, ok := u.etagCache[repo]; ok {
			return cached.version, nil
		}
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 429 {
		if cached, ok := u.etagCache[repo]; ok && cached.version != "" {
			log.Printf("[update] rate limited, using cached version %s", cached.version)
			return cached.version, nil
		}
		return "", fmt.Errorf("GitHub API rate limit exceeded (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)

	type releaseObj struct {
		TagName string `json:"tag_name"`
	}
	var list []releaseObj
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("json decode: %w", err)
	}
	if len(list) == 0 || list[0].TagName == "" {
		return "", fmt.Errorf("no releases found for %s", repo)
	}

	version := list[0].TagName
	etag := resp.Header.Get("ETag")
	u.etagCache[repo] = etagEntry{etag: etag, version: version}

	return version, nil
}

func (u *Updater) findAssetURL(repo, prefix, suffix string) (string, error) {
	url := fmt.Sprintf("%s/%s/releases?per_page=1", githubAPI, repo)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)

	type asset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	type releaseInfo struct {
		Assets []asset `json:"assets"`
	}

	var list []releaseInfo
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("json decode: %w", err)
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no releases found")
	}

	for _, a := range list[0].Assets {
		if strings.HasPrefix(a.Name, prefix) && (suffix == "" || strings.HasSuffix(a.Name, suffix)) {
			log.Printf("[update] matched asset: %s", a.Name)
			return a.BrowserDownloadURL, nil
		}
	}

	names := make([]string, 0, len(list[0].Assets))
	for _, a := range list[0].Assets {
		names = append(names, a.Name)
	}
	log.Printf("[update] no match for %s*%s in assets: %v", prefix, suffix, names)
	return "", fmt.Errorf("asset %q not found in release", prefix+"*"+suffix)
}

func (u *Updater) download(url, dest string) error {
	dlClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := dlClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	log.Printf("[update] saved %d bytes to %s", n, dest)
	return nil
}

func detectArch() string {
	switch runtime.GOARCH {
	case "mipsle":
		return "mipsel"
	case "mips":
		return "mips"
	case "arm64":
		return "aarch64"
	case "arm":
		return "armv7"
	default:
		return runtime.GOARCH
	}
}

var mgrVersion = "dev"

func SetManagerVersion(v string) {
	mgrVersion = v
}

func managerVersion() string {
	return mgrVersion
}
