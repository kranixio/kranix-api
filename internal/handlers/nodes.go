package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleListNodeHealth(w http.ResponseWriter, r *http.Request) {
	if s.Core == nil || !s.Core.Enabled() {
		writeJSON(w, http.StatusOK, types.NodeHealthListResponse{Nodes: []types.NodeHealthReport{}, Count: 0})
		return
	}
	resp, err := s.Core.ListNodeHealth(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDrainNode(w http.ResponseWriter, r *http.Request) {
	if s.Core == nil || !s.Core.Enabled() {
		http.Error(w, "core not configured", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("name")
	var req types.NodeDrainRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
	}
	if req.NodeName == "" {
		req.NodeName = name
	}
	result, err := s.Core.DrainNode(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.recordAudit(r, "node.drain", "node", name, "success", "", map[string]interface{}{
		"phase":       result.Phase,
		"podsEvicted": result.PodsEvicted,
	})
	writeJSON(w, http.StatusOK, result)
}
