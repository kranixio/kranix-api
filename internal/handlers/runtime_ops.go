package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleCheckpointWorkload(w http.ResponseWriter, r *http.Request) {
	if s.Core == nil || !s.Core.Enabled() {
		http.Error(w, "core not configured", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	var req types.CheckpointRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
	}
	if req.WorkloadID == "" {
		req.WorkloadID = id
	}
	result, err := s.Core.CheckpointWorkload(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.recordAudit(r, "workload.checkpoint", "workload", id, "success", "", nil)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRestoreWorkload(w http.ResponseWriter, r *http.Request) {
	if s.Core == nil || !s.Core.Enabled() {
		http.Error(w, "core not configured", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	var req types.RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.WorkloadID == "" {
		req.WorkloadID = id
	}
	result, err := s.Core.RestoreWorkload(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.recordAudit(r, "workload.restore", "workload", id, "success", "", nil)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListCheckpoints(w http.ResponseWriter, r *http.Request) {
	if s.Core == nil || !s.Core.Enabled() {
		writeJSON(w, http.StatusOK, []types.CheckpointResult{})
		return
	}
	id := r.PathValue("id")
	namespace := r.URL.Query().Get("namespace")
	list, err := s.Core.ListCheckpoints(r.Context(), id, namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListRuntimePlugins(w http.ResponseWriter, r *http.Request) {
	if s.Core == nil || !s.Core.Enabled() {
		writeJSON(w, http.StatusOK, types.RuntimePluginListResponse{Count: 0})
		return
	}
	resp, err := s.Core.ListRuntimePlugins(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
