package api

import (
	"encoding/json"
	"net/http"
)

func (h *handlers) getStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.deps.ServiceManager.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handlers) getSystem(w http.ResponseWriter, r *http.Request) {
	info := h.deps.SystemInfo.Get()
	writeJSON(w, http.StatusOK, info)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
