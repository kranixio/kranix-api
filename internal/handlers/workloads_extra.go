package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleGetWorkloadByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.Core != nil && s.Core.Enabled() {
		wl, err := s.Core.GetWorkload(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, wl)
		return
	}
	s.handleGetWorkload(w, r)
}

func (s *Server) handleUpdateWorkloadByID(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateWorkload(w, r)
}

func (s *Server) handleDeleteWorkloadByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id != "" && s.Core != nil && s.Core.Enabled() {
		if err := s.Core.DeleteWorkload(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "workload.delete", "workload", id, "success", "", nil)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.handleDeleteWorkload(w, r)
}

func (s *Server) handleRestartWorkloadByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id != "" && s.Core != nil && s.Core.Enabled() {
		if err := s.Core.RestartWorkload(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "workload.restart", "workload", id, "success", "", nil)
		writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": "restarted"})
		return
	}
	s.handleRestartWorkload(w, r)
}

func (s *Server) handleGetWorkloadDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.Core != nil && s.Core.Enabled() {
		result, err := s.Core.GetWorkloadDiff(r.Context(), id, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	s.handleDiffWorkload(w, r)
}

func (s *Server) handlePostWorkloadDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var spec types.WorkloadSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if s.Core != nil && s.Core.Enabled() {
		result, err := s.Core.GetWorkloadDiff(r.Context(), id, &spec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	s.handleDiffWorkload(w, r)
}

func searchQueryFromRequest(r *http.Request) types.WorkloadSearchQuery {
	q := r.URL.Query()
	return types.WorkloadSearchQuery{
		Namespace:   q.Get("namespace"),
		Phase:       q.Get("phase"),
		Status:      q.Get("status"),
		Image:       q.Get("image"),
		Team:        firstQueryValue(q, "team", "tag.team"),
		Environment: firstQueryValue(q, "environment", "tag.environment"),
		CostCenter:  firstQueryValue(q, "cost_center", "costCenter", "tag.cost_center"),
		LabelKey:    q.Get("label"),
		LabelValue:  q.Get("label_value"),
	}
}

func firstQueryValue(q map[string][]string, keys ...string) string {
	for _, k := range keys {
		if vals, ok := q[k]; ok && len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return ""
}
