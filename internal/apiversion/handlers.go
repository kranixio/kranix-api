package apiversion

import (
	"encoding/json"
	"net/http"
)

// RegisterRoutes registers API versioning routes.
func RegisterRoutes(mux *http.ServeMux, manager *Manager) {
	mux.HandleFunc("GET /api/versions", handleListVersions(manager))
	mux.HandleFunc("GET /api/versions/{version}", handleGetVersion(manager))
}

// handleListVersions handles listing all API versions.
func handleListVersions(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versions := manager.ListVersions()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"versions": versions,
			"count":    len(versions),
		})
	}
}

// handleGetVersion handles getting information about a specific API version.
func handleGetVersion(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version := r.PathValue("version")
		version = trimPrefix(version)

		versionInfo, err := manager.GetVersionInfo(version)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if versionInfo == nil {
			http.Error(w, "Version not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(versionInfo)
	}
}

// trimPrefix removes 'v' prefix from version string.
func trimPrefix(version string) string {
	if len(version) > 0 && version[0] == 'v' {
		return version[1:]
	}
	return version
}
