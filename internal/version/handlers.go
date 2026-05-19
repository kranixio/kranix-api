package version

import (
	"encoding/json"
	"net/http"
	"github.com/kranix-io/kranix-packages/pagination"
	"github.com/kranix-io/kranix-packages/types"
)

type changelogEntryID struct {
	*types.ChangelogEntry
}

func (e changelogEntryID) GetID() string {
	if e.ChangelogEntry == nil {
		return ""
	}
	return e.ChangelogEntry.ID
}

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

		all := manager.GetChangelog(0)
		pageParams := pagination.ParseParams(limitStr, r.URL.Query().Get("cursor"))
		wrapped := make([]pagination.IDProvider, len(all))
		for i, e := range all {
			wrapped[i] = changelogEntryID{e}
		}
		page, pageInfo := pagination.SlicePage(wrapped, pageParams)
		entries := make([]*types.ChangelogEntry, len(page))
		for i, e := range page {
			entries[i] = e.(changelogEntryID).ChangelogEntry
		}

		w.Header().Set("Content-Type", "application/json")
		if pageInfo.NextCursor != "" {
			w.Header().Set("Link", "</api/v1/changelog?cursor="+pageInfo.NextCursor+">; rel=\"next\"")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"changelog": entries,
			"page_info": pageInfo,
			"count":     len(entries),
		})
	}
}
