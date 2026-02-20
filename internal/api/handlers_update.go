package api

import (
	"net/http"
)

func (h *handlers) checkUpdate(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("force") == "true" {
		h.deps.Updater.InvalidateCache()
	}
	info, err := h.deps.Updater.Check()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *handlers) installUpdate(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Updater.Install()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handlers) installManagerUpdate(w http.ResponseWriter, r *http.Request) {
	result, err := h.deps.Updater.InstallManager()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
