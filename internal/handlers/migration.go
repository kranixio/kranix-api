package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleMigrateWorkload(w http.ResponseWriter, r *http.Request) {
	if s.Core == nil || !s.Core.Enabled() {
		http.Error(w, "core not configured", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	var req types.WorkloadMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.WorkloadID == "" {
		req.WorkloadID = id
	}
	result, err := s.Core.MigrateWorkload(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.recordAudit(r, "workload.migrate", "workload", id, "success", "", nil)
	writeJSON(w, http.StatusOK, result)
}
