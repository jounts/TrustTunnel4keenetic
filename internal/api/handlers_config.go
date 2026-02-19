package api

import (
	"encoding/json"
	"io"
	"net/http"
)

func (h *handlers) getConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.deps.ConfigManager.ReadAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *handlers) putConfig(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req struct {
		ClientConfig string `json:"client_config"`
		ModeConfig   string `json:"mode_config"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := h.deps.ConfigManager.WriteAll(req.ClientConfig, req.ModeConfig); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.ClientConfig != "" {
		mode, _ := h.deps.ConfigManager.ReadMode()
		if mode != nil {
			h.deps.ConfigManager.SyncVpnMode(mode.Mode)
		}
	}

	// Return the transformed config so the UI can update
	cfg, _ := h.deps.ConfigManager.ReadAll()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "saved",
		"client_config": cfg.ClientConfig,
	})
}

func (h *handlers) getMode(w http.ResponseWriter, r *http.Request) {
	mode, err := h.deps.ConfigManager.ReadMode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mode)
}

func (h *handlers) putMode(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req struct {
		Mode     string `json:"mode"`
		TunIdx   int    `json:"tun_idx"`
		ProxyIdx int    `json:"proxy_idx"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Mode != "socks5" && req.Mode != "tun" {
		writeError(w, http.StatusBadRequest, "mode must be 'socks5' or 'tun'")
		return
	}

	if err := h.deps.ConfigManager.WriteMode(req.Mode, req.TunIdx, req.ProxyIdx); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.deps.ConfigManager.SyncVpnMode(req.Mode); err != nil {
		writeError(w, http.StatusInternalServerError, "mode saved but failed to sync vpn_mode in client config: "+err.Error())
		return
	}

	if err := h.deps.NDMClient.RecreateInterface(req.Mode, req.TunIdx, req.ProxyIdx); err != nil {
		writeError(w, http.StatusInternalServerError, "config saved but interface error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "mode changed", "mode": req.Mode})
}
