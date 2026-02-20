package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	trusttunnel "github.com/jounts/TrustTunnel4keenetic"
	"github.com/jounts/TrustTunnel4keenetic/internal/api"
	"github.com/jounts/TrustTunnel4keenetic/internal/ndm"
	"github.com/jounts/TrustTunnel4keenetic/internal/platform"
	"github.com/jounts/TrustTunnel4keenetic/internal/routing"
	"github.com/jounts/TrustTunnel4keenetic/internal/service"
)

var version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	devMode := flag.Bool("dev", false, "development mode (proxy to Vite)")
	showVer := flag.Bool("version", false, "print version and exit")
	configPath := flag.String("config", "/opt/trusttunnel_client/manager.conf", "manager config path")
	flag.Parse()

	if *showVer {
		fmt.Println("trusttunnel-manager", version)
		os.Exit(0)
	}

	service.SetManagerVersion(version)

	cfg := loadConfig(*configPath, *addr)

	svcManager := service.NewManager()
	cfgManager := service.NewConfigManager()
	updater := service.NewUpdater()
	ndmClient := ndm.NewClient("http://localhost:79")
	routingMgr := routing.NewManager()
	sysInfo := platform.NewInfo()

	// Ensure vpn_mode in client TOML matches the selected mode
	if mode, err := cfgManager.ReadMode(); err == nil {
		if err := cfgManager.SyncVpnMode(mode.Mode); err != nil {
			log.Printf("Warning: failed to sync vpn_mode: %v", err)
		}
	}

	var staticFS http.FileSystem
	if *devMode {
		log.Println("Development mode: serving from web/dist or proxy to Vite")
		staticFS = http.Dir("web/dist")
	} else {
		distFS, err := fs.Sub(trusttunnel.WebFS, "web/dist")
		if err != nil {
			log.Fatalf("Failed to load embedded web assets: %v", err)
		}
		staticFS = http.FS(distFS)
	}

	authCfg := buildAuthConfig(cfg)

	router := api.NewRouter(api.Dependencies{
		ServiceManager: svcManager,
		ConfigManager:  cfgManager,
		Updater:        updater,
		NDMClient:      ndmClient,
		RoutingManager: routingMgr,
		SystemInfo:     sysInfo,
		StaticFS:       staticFS,
		Auth:           authCfg,
	})

	log.Printf("trusttunnel-manager %s listening on %s", version, cfg.addr)
	if err := http.ListenAndServe(cfg.addr, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

type appConfig struct {
	addr     string
	authMode string // "ndm", "local", "" (empty = auto)
	username string
	password string
}

func loadConfig(path, defaultAddr string) appConfig {
	cfg := appConfig{
		addr:     defaultAddr,
		username: "admin",
		password: "",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	for _, line := range splitLines(string(data)) {
		k, v := parseKV(line)
		switch k {
		case "LISTEN_ADDR":
			cfg.addr = v
		case "AUTH_MODE":
			cfg.authMode = v
		case "USERNAME":
			cfg.username = v
		case "PASSWORD":
			cfg.password = v
		}
	}
	return cfg
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func parseKV(line string) (string, string) {
	if len(line) == 0 || line[0] == '#' {
		return "", ""
	}
	for i := 0; i < len(line); i++ {
		if line[i] == '=' {
			k := line[:i]
			v := line[i+1:]
			if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
				v = v[1 : len(v)-1]
			}
			return k, v
		}
	}
	return "", ""
}

func buildAuthConfig(cfg appConfig) api.AuthConfig {
	switch cfg.authMode {
	case "ndm":
		log.Printf("Auth mode: NDM (Keenetic router accounts)")
		return api.AuthConfig{
			Mode:             api.AuthNDM,
			NDMAuthenticator: ndm.NewAuthenticator("http://localhost:79"),
		}
	case "local":
		if cfg.password == "" {
			log.Printf("Auth mode: none (AUTH_MODE=local but PASSWORD is empty)")
			return api.AuthConfig{Mode: api.AuthNone}
		}
		log.Printf("Auth mode: local (static username/password)")
		return api.AuthConfig{
			Mode:     api.AuthLocal,
			Username: cfg.username,
			Password: cfg.password,
		}
	case "none", "off":
		log.Printf("Auth mode: none (disabled)")
		return api.AuthConfig{Mode: api.AuthNone}
	default:
		// Auto-detect: if password is set → local, otherwise → ndm
		if cfg.password != "" {
			log.Printf("Auth mode: local (auto, PASSWORD is set)")
			return api.AuthConfig{
				Mode:     api.AuthLocal,
				Username: cfg.username,
				Password: cfg.password,
			}
		}
		log.Printf("Auth mode: NDM (auto, no PASSWORD configured)")
		return api.AuthConfig{
			Mode:             api.AuthNDM,
			NDMAuthenticator: ndm.NewAuthenticator("http://localhost:79"),
		}
	}
}
