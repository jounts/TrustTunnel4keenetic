package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jounts/TrustTunnel4keenetic/internal/ndm"
)

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start).Round(time.Millisecond))
	})
}

type AuthMode int

const (
	AuthNone AuthMode = iota
	AuthNDM
)

func (m AuthMode) String() string {
	switch m {
	case AuthNDM:
		return "ndm"
	default:
		return "none"
	}
}

type AuthConfig struct {
	Mode             AuthMode
	NDMAuthenticator *ndm.Authenticator
}

type authErrorResp struct {
	Error    string `json:"error"`
	AuthMode string `json:"auth_mode"`
}

func sendUnauthorized(w http.ResponseWriter, mode AuthMode) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(authErrorResp{
		Error:    "unauthorized",
		AuthMode: mode.String(),
	})
}

func withAuth(cfg AuthConfig, next http.Handler) http.Handler {
	if cfg.Mode == AuthNone {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var valid bool

		if cfg.Mode == AuthNDM && cfg.NDMAuthenticator != nil {
			if c, err := r.Cookie(ndm.SessionCookieName); err == nil {
				valid = cfg.NDMAuthenticator.ValidateSession(c.Value)
			}
		}

		if !valid {
			sendUnauthorized(w, cfg.Mode)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func spaHandler(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			r.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
