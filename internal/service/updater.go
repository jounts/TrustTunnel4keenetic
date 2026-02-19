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
	ManagerCurrentVersion string `json:"manager_current_version"`
	ManagerLatestVersion  string `json:"manager_latest_version"`
	ManagerUpdateAvailable bool  `json:"manager_update_available"`
}

type UpdateResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Version string `json:"version"`
}

type Updater struct {
	httpClient *http.Client
}

func NewUpdater() *Updater {
	return &Updater{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (u *Updater) Check() (*UpdateInfo, error) {
	info := &UpdateInfo{}

	info.ClientCurrentVersion = detectClientVersion()
	log.Printf("[update] client current version: %s", info.ClientCurrentVersion)

	if latest, err := u.latestRelease(clientRepo); err == nil {
		info.ClientLatestVersion = latest
		info.ClientUpdateAvailable = latest != "" && latest != info.ClientCurrentVersion
	} else {
		log.Printf("[update] failed to check client release: %v", err)
	}

	info.ManagerCurrentVersion = managerVersion()
	log.Printf("[update] manager current version: %s", info.ManagerCurrentVersion)

	if latest, err := u.latestRelease(managerRepo); err == nil {
		info.ManagerLatestVersion = latest
		info.ManagerUpdateAvailable = latest != "" && latest != info.ManagerCurrentVersion
	} else {
		log.Printf("[update] failed to check manager release: %v", err)
	}

	log.Printf("[update] check complete: client=%s->%s manager=%s->%s",
		info.ClientCurrentVersion, info.ClientLatestVersion,
		info.ManagerCurrentVersion, info.ManagerLatestVersion)
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
	log.Printf("[update] complete, version: %s", newVer)
	return &UpdateResult{
		Success: true,
		Message: "Updated successfully",
		Version: newVer,
	}, nil
}

func (u *Updater) latestRelease(repo string) (string, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", githubAPI, repo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func (u *Updater) findAssetURL(repo, prefix, suffix string) (string, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", githubAPI, repo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	for _, a := range release.Assets {
		if strings.HasPrefix(a.Name, prefix) && strings.HasSuffix(a.Name, suffix) {
			log.Printf("[update] matched asset: %s", a.Name)
			return a.BrowserDownloadURL, nil
		}
	}
	names := make([]string, 0, len(release.Assets))
	for _, a := range release.Assets {
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
