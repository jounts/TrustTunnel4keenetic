package api

import (
	"crypto/subtle"
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

// AuthMode determines how credentials are verified.
type AuthMode int

const (
	AuthNone  AuthMode = iota // no auth (password empty)
	AuthLocal                 // local username/password from manager.conf
	AuthNDM                   // validate against Keenetic NDM API
)

type AuthConfig struct {
	Mode          AuthMode
	Username      string
	Password      string
	NDMAuthenticator *ndm.Authenticator
}

func withAuth(cfg AuthConfig, next http.Handler) http.Handler {
	if cfg.Mode == AuthNone {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="TrustTunnel Manager"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var valid bool
		switch cfg.Mode {
		case AuthNDM:
			if cfg.NDMAuthenticator != nil {
				valid = cfg.NDMAuthenticator.Verify(u, p)
			}
		case AuthLocal:
			valid = subtle.ConstantTimeCompare([]byte(u), []byte(cfg.Username)) == 1 &&
				subtle.ConstantTimeCompare([]byte(p), []byte(cfg.Password)) == 1
		}

		if !valid {
			w.Header().Set("WWW-Authenticate", `Basic realm="TrustTunnel Manager"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
