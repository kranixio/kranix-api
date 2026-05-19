package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleListQuotas(w http.ResponseWriter, r *http.Request) {
	if s.Core != nil && s.Core.Enabled() {
		resp, err := s.Core.ListQuotas(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusOK, types.ResourceQuotaListResponse{Quotas: []types.HardResourceQuota{}, Count: 0})
}

func (s *Server) handleGetNamespaceQuota(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("namespace")
	if s.Core != nil && s.Core.Enabled() {
		lim, err := s.Core.GetNamespaceQuota(r.Context(), ns)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, lim)
		return
	}
	http.Error(w, "quota not found", http.StatusNotFound)
}

func (s *Server) handlePutNamespaceQuota(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("namespace")
	var lim types.HardResourceQuota
	if err := json.NewDecoder(r.Body).Decode(&lim); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if s.Core != nil && s.Core.Enabled() {
		if err := s.Core.SetNamespaceQuota(r.Context(), ns, lim); err != nil {
			s.recordAudit(r, "quota.set", "namespace", ns, "error", err.Error(), nil)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "quota.set", "namespace", ns, "success", "", nil)
		writeJSON(w, http.StatusOK, lim)
		return
	}
	s.recordAudit(r, "quota.set", "namespace", ns, "success", "", map[string]interface{}{"mode": "local"})
	writeJSON(w, http.StatusOK, lim)
}

func (s *Server) handleDeleteNamespaceQuota(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("namespace")
	if s.Core != nil && s.Core.Enabled() {
		if err := s.Core.DeleteNamespaceQuota(r.Context(), ns); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "quota.delete", "namespace", ns, "success", "", nil)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, "quota not found", http.StatusNotFound)
}

func (s *Server) handleNamespaceQuotaUsage(w http.ResponseWriter, r *http.Request) {
	ns := r.PathValue("namespace")
	if s.Core != nil && s.Core.Enabled() {
		usage, err := s.Core.GetNamespaceQuotaUsage(r.Context(), ns)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, usage)
		return
	}
	http.Error(w, "quota not found", http.StatusNotFound)
}
