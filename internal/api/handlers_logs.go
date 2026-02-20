package api

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const clientLogPath = "/opt/var/log/trusttunnel.log"
const managerLogPath = "/opt/var/log/trusttunnel_manager.log"

func logPathFromRequest(r *http.Request) string {
	if r.URL.Query().Get("source") == "manager" {
		return managerLogPath
	}
	return clientLogPath
}

func (h *handlers) getLogs(w http.ResponseWriter, r *http.Request) {
	logPath := logPathFromRequest(r)

	lines := 100
	if v := r.URL.Query().Get("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			lines = n
		}
	}

	content, err := tailFile(logPath, lines)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"lines": content,
		"file":  logPath,
		"count": len(content),
	})
}

func (h *handlers) streamLogs(w http.ResponseWriter, r *http.Request) {
	logPath := logPathFromRequest(r)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	f, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\": %q}\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer f.Close()

	f.Seek(0, io.SeekEnd)
	scanner := bufio.NewScanner(f)

	ctx := r.Context()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for scanner.Scan() {
				fmt.Fprintf(w, "data: %s\n\n", scanner.Text())
				flusher.Flush()
			}
		}
	}
}

func (h *handlers) clearLogs(w http.ResponseWriter, r *http.Request) {
	for _, p := range []string{clientLogPath, managerLogPath} {
		if err := os.Truncate(p, 0); err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, "failed to clear "+p+": "+err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if len(allLines) <= n {
		return allLines, nil
	}
	return allLines[len(allLines)-n:], nil
}
