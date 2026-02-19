package api

import (
	"net/http"
)

func (h *handlers) serviceAction(w http.ResponseWriter, r *http.Request) {
	action := extractPathSuffix(r.URL.Path, "/api/service/")
	switch action {
	case "start", "stop", "restart", "reload":
		output, err := h.deps.ServiceManager.Control(action)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"action": action,
			"output": output,
		})
	default:
		writeError(w, http.StatusBadRequest, "unknown action: "+action)
	}
}
