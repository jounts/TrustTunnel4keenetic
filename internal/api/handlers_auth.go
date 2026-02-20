package api

import (
	"encoding/json"
	"net/http"

	"github.com/jounts/TrustTunnel4keenetic/internal/ndm"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type checkResponse struct {
	Authenticated bool   `json:"authenticated"`
	AuthMode      string `json:"auth_mode"`
}

func (h *handlers) authLogin(w http.ResponseWriter, r *http.Request) {
	if h.deps.Auth.Mode != AuthNDM || h.deps.Auth.NDMAuthenticator == nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "NDM auth not configured"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "invalid request"})
		return
	}

	token, err := h.deps.Auth.NDMAuthenticator.Login(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "Неверное имя пользователя или пароль"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     ndm.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	writeJSON(w, http.StatusOK, loginResponse{OK: true})
}

func (h *handlers) authLogout(w http.ResponseWriter, r *http.Request) {
	if h.deps.Auth.NDMAuthenticator != nil {
		if c, err := r.Cookie(ndm.SessionCookieName); err == nil {
			h.deps.Auth.NDMAuthenticator.DestroySession(c.Value)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     ndm.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handlers) authCheck(w http.ResponseWriter, r *http.Request) {
	mode := h.deps.Auth.Mode
	authenticated := false

	switch mode {
	case AuthNone:
		authenticated = true
	case AuthNDM:
		if h.deps.Auth.NDMAuthenticator != nil {
			if c, err := r.Cookie(ndm.SessionCookieName); err == nil {
				authenticated = h.deps.Auth.NDMAuthenticator.ValidateSession(c.Value)
			}
		}
	case AuthLocal:
		u, p, ok := r.BasicAuth()
		if ok {
			authenticated = u == h.deps.Auth.Username && p == h.deps.Auth.Password
		}
	}

	writeJSON(w, http.StatusOK, checkResponse{
		Authenticated: authenticated,
		AuthMode:      mode.String(),
	})
}

