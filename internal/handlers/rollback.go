package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleListRevisionsByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.Core != nil && s.Core.Enabled() {
		resp, err := s.Core.ListRevisions(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	writeJSON(w, http.StatusOK, types.RevisionListResponse{
		WorkloadID: id,
		Revisions:  []types.WorkloadRevision{},
		Count:      0,
	})
}

func (s *Server) handleRollbackWorkloadByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req types.RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if s.ifDryRun(w, r, "workload.rollback", "workload", id, map[string]interface{}{
		"id":         id,
		"revisionId": req.RevisionID,
	}) {
		return
	}

	if s.Core != nil && s.Core.Enabled() {
		result, err := s.Core.RollbackWorkload(r.Context(), id, req.RevisionID)
		if err != nil {
			s.recordAudit(r, "workload.rollback", "workload", id, "error", err.Error(), nil)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "workload.rollback", "workload", id, "success", "", map[string]interface{}{
			"revisionId": result.RevisionID,
		})
		writeJSON(w, http.StatusOK, result)
		return
	}

	writeJSON(w, http.StatusOK, types.RollbackResult{
		WorkloadID: id,
		Status:     "rolled_back",
		Message:    "rollback accepted (core not connected — no-op)",
	})
}
