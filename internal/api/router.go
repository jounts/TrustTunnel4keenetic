package api

import (
	"net/http"
	"strings"

	"github.com/jounts/TrustTunnel4keenetic/internal/ndm"
	"github.com/jounts/TrustTunnel4keenetic/internal/platform"
	"github.com/jounts/TrustTunnel4keenetic/internal/routing"
	"github.com/jounts/TrustTunnel4keenetic/internal/service"
)

type Dependencies struct {
	ServiceManager *service.Manager
	ConfigManager  *service.ConfigManager
	Updater        *service.Updater
	NDMClient      *ndm.Client
	RoutingManager *routing.Manager
	SystemInfo     *platform.Info
	StaticFS       http.FileSystem
	Auth           AuthConfig
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	h := &handlers{deps: deps}

	mux.HandleFunc("/api/status", methodOnly("GET", h.getStatus))
	mux.HandleFunc("/api/service/", methodOnly("POST", h.serviceAction))
	mux.HandleFunc("/api/config", h.configHandler)
	mux.HandleFunc("/api/mode", h.modeHandler)
	mux.HandleFunc("/api/logs", h.logsHandler)
	mux.HandleFunc("/api/logs/stream", h.streamLogs)
	mux.HandleFunc("/api/update/check", methodOnly("GET", h.checkUpdate))
	mux.HandleFunc("/api/update/install", methodOnly("POST", h.installUpdate))
	mux.HandleFunc("/api/update/install-manager", methodOnly("POST", h.installManagerUpdate))
	mux.HandleFunc("/api/system", methodOnly("GET", h.getSystem))
	mux.HandleFunc("/api/routing", h.routingHandler)
	mux.HandleFunc("/api/routing/domains", h.routingDomainsHandler)
	mux.HandleFunc("/api/routing/update-nets", methodOnly("POST", h.updateRoutingNets))

	apiHandler := withAuth(deps.Auth, withCORS(mux))

	// Auth endpoints are outside withAuth (accessible without session)
	authMux := http.NewServeMux()
	authMux.HandleFunc("/api/auth/login", methodOnly("POST", h.authLogin))
	authMux.HandleFunc("/api/auth/logout", methodOnly("POST", h.authLogout))
	authMux.HandleFunc("/api/auth/check", methodOnly("GET", h.authCheck))

	root := http.NewServeMux()
	root.Handle("/api/auth/", withCORS(authMux))
	root.Handle("/api/", apiHandler)
	root.Handle("/", spaHandler(deps.StaticFS))

	return withLogging(root)
}

type handlers struct {
	deps Dependencies
}

func (h *handlers) configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPut:
		h.putConfig(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *handlers) logsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getLogs(w, r)
	case http.MethodDelete:
		h.clearLogs(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *handlers) modeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getMode(w, r)
	case http.MethodPut:
		h.putMode(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func methodOnly(method string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != method {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

func extractPathSuffix(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimSuffix(s, "/")
	return s
}
