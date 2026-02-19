package api

import (
	"encoding/json"
	"io"
	"net/http"
)

func (h *handlers) routingHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getRouting(w, r)
	case http.MethodPut:
		h.putRouting(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *handlers) routingDomainsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getRoutingDomains(w, r)
	case http.MethodPut:
		h.putRoutingDomains(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *handlers) getRouting(w http.ResponseWriter, r *http.Request) {
	mode, err := h.deps.ConfigManager.ReadMode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	stats, err := h.deps.RoutingManager.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"config": map[string]any{
			"sr_enabled":      mode.SREnabled,
			"sr_home_country": mode.SRHomeCountry,
			"sr_dns_port":     mode.SRDNSPort,
			"sr_dns_upstream": mode.SRDNSUpstream,
		},
		"stats": stats,
	})
}

func (h *handlers) putRouting(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req struct {
		Enabled     string `json:"sr_enabled"`
		HomeCountry string `json:"sr_home_country"`
		DNSPort     int    `json:"sr_dns_port"`
		DNSUpstream string `json:"sr_dns_upstream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Enabled != "yes" && req.Enabled != "no" {
		writeError(w, http.StatusBadRequest, "sr_enabled must be 'yes' or 'no'")
		return
	}
	if req.DNSPort <= 0 || req.DNSPort > 65535 {
		req.DNSPort = 5354
	}
	if req.HomeCountry == "" {
		req.HomeCountry = "RU"
	}
	if req.DNSUpstream == "" {
		req.DNSUpstream = "1.1.1.1"
	}

	if err := h.deps.ConfigManager.WriteSRConfig(req.Enabled, req.HomeCountry, req.DNSUpstream, req.DNSPort); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *handlers) getRoutingDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.deps.RoutingManager.GetDomains()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"domains": domains})
}

func (h *handlers) putRoutingDomains(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var req struct {
		Domains []string `json:"domains"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := h.deps.RoutingManager.SaveDomains(req.Domains); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *handlers) updateRoutingNets(w http.ResponseWriter, r *http.Request) {
	mode, err := h.deps.ConfigManager.ReadMode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	country := mode.SRHomeCountry
	if country == "" {
		country = "RU"
	}

	if err := h.deps.RoutingManager.UpdateNets(country); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "country": country})
}
