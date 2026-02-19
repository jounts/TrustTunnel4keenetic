package api

import (
	"encoding/json"
	"net/http"
)

type routingConfigRequest struct {
	Enabled     string `json:"sr_enabled"`
	HomeCountry string `json:"sr_home_country"`
	DNSPort     int    `json:"sr_dns_port"`
	DNSUpstream string `json:"sr_dns_upstream"`
}

type routingInfoResponse struct {
	Config routingConfigRequest `json:"config"`
	Stats  any                  `json:"stats"`
}

type routingDomainsRequest struct {
	Domains string `json:"domains"`
}

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

func (h *handlers) getRouting(w http.ResponseWriter, r *http.Request) {
	mode, err := h.deps.ConfigManager.ReadMode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var stats any
	if h.deps.RoutingManager != nil {
		stats = h.deps.RoutingManager.GetStats()
	}

	resp := routingInfoResponse{
		Config: routingConfigRequest{
			Enabled:     mode.SREnabled,
			HomeCountry: mode.SRHomeCountry,
			DNSPort:     mode.SRDNSPort,
			DNSUpstream: mode.SRDNSUpstream,
		},
		Stats: stats,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *handlers) putRouting(w http.ResponseWriter, r *http.Request) {
	var req routingConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Enabled == "" {
		req.Enabled = "no"
	}
	if req.HomeCountry == "" {
		req.HomeCountry = "RU"
	}
	if req.DNSPort == 0 {
		req.DNSPort = 5354
	}
	if req.DNSUpstream == "" {
		req.DNSUpstream = "1.1.1.1"
	}

	if err := h.deps.ConfigManager.WriteSRConfig(req.Enabled, req.HomeCountry, req.DNSUpstream, req.DNSPort); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.deps.RoutingManager != nil && req.Enabled == "yes" {
		if err := h.deps.RoutingManager.Apply(); err != nil {
			writeError(w, http.StatusInternalServerError, "config saved but apply failed: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

func (h *handlers) getRoutingDomains(w http.ResponseWriter, r *http.Request) {
	if h.deps.RoutingManager == nil {
		writeJSON(w, http.StatusOK, routingDomainsRequest{Domains: ""})
		return
	}
	domains, err := h.deps.RoutingManager.GetDomains()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, routingDomainsRequest{Domains: domains})
}

func (h *handlers) putRoutingDomains(w http.ResponseWriter, r *http.Request) {
	var req routingDomainsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if h.deps.RoutingManager == nil {
		writeError(w, http.StatusInternalServerError, "routing manager not initialized")
		return
	}

	if err := h.deps.RoutingManager.SaveDomains(req.Domains); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) updateRoutingNets(w http.ResponseWriter, r *http.Request) {
	if h.deps.RoutingManager == nil {
		writeError(w, http.StatusInternalServerError, "routing manager not initialized")
		return
	}

	if err := h.deps.RoutingManager.UpdateNets(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
