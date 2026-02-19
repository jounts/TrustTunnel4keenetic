package service

import (
	"encoding/json"
	"fmt"
	"io"
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

	if latest, err := u.latestRelease(clientRepo); err == nil {
		info.ClientLatestVersion = latest
		info.ClientUpdateAvailable = latest != "" && latest != info.ClientCurrentVersion
	}

	info.ManagerCurrentVersion = managerVersion()

	if latest, err := u.latestRelease(managerRepo); err == nil {
		info.ManagerLatestVersion = latest
		info.ManagerUpdateAvailable = latest != "" && latest != info.ManagerCurrentVersion
	}

	return info, nil
}

func (u *Updater) Install() (*UpdateResult, error) {
	arch := detectArch()
	osName := "linux"

	assetSuffix := fmt.Sprintf("-%s-%s.tar.gz", osName, arch)

	downloadURL, err := u.findAssetURL(clientRepo, "trusttunnel_client", assetSuffix)
	if err != nil {
		return nil, fmt.Errorf("find asset: %w", err)
	}

	tmpFile := "/tmp/trusttunnel_update.tar.gz"
	if err := u.download(downloadURL, tmpFile); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpFile)

	svcMgr := NewManager()
	svcMgr.Control("stop")

	cmd := exec.Command("tar", "xzf", tmpFile, "-C", "/opt/trusttunnel_client/")
	if out, err := cmd.CombinedOutput(); err != nil {
		svcMgr.Control("start")
		return nil, fmt.Errorf("extract: %w: %s", err, string(out))
	}

	os.Chmod(clientBin, 0755)
	svcMgr.Control("start")

	newVer := detectClientVersion()
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
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("asset %q not found in release", prefix+"*"+suffix)
}

func (u *Updater) download(url, dest string) error {
	resp, err := u.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
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
