package version

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// RegisterRoutes registers version HTTP handlers.
func RegisterRoutes(mux *http.ServeMux, manager *Manager) {
	mux.HandleFunc("GET /api/v1/version", handleGetVersion(manager))
	mux.HandleFunc("GET /api/v1/changelog", handleGetChangelog(manager))
}

// handleGetVersion handles getting version information.
func handleGetVersion(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versionInfo := manager.GetVersionInfo()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(versionInfo)
	}
}

// handleGetChangelog handles getting the changelog.
func handleGetChangelog(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 0
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil {
				limit = l
			}
		}

		changelog := manager.GetChangelog(limit)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"changelog": changelog,
			"count":     len(changelog),
		})
	}
}
