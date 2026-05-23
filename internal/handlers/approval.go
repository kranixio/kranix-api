package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleCreateApproval(w http.ResponseWriter, r *http.Request) {
	if s.Approvals == nil {
		http.Error(w, "approval gate not enabled", http.StatusServiceUnavailable)
		return
	}
	var req types.ApprovalCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Tool == "" {
		http.Error(w, "tool is required", http.StatusBadRequest)
		return
	}
	if req.AgentID == "" {
		req.AgentID = r.Header.Get("X-Agent-Id")
	}
	if req.AgentID == "" {
		req.AgentID = r.Header.Get("X-Actor")
	}
	rec := s.Approvals.Create(req)
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleGetApproval(w http.ResponseWriter, r *http.Request) {
	if s.Approvals == nil {
		http.Error(w, "approval gate not enabled", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	rec, ok := s.Approvals.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleListApprovals(w http.ResponseWriter, r *http.Request) {
	if s.Approvals == nil {
		http.Error(w, "approval gate not enabled", http.StatusServiceUnavailable)
		return
	}
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		agentID = r.Header.Get("X-Agent-Id")
	}
	pending := s.Approvals.ListPending(agentID)
	writeJSON(w, http.StatusOK, types.ApprovalListResponse{
		Approvals: pending,
		Count:     len(pending),
	})
}

func (s *Server) handleResolveApproval(w http.ResponseWriter, r *http.Request) {
	if s.Approvals == nil {
		http.Error(w, "approval gate not enabled", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	var req types.ApprovalResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ResolvedBy == "" {
		req.ResolvedBy = r.Header.Get("X-Actor")
	}
	rec, err := s.Approvals.Resolve(id, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.recordAudit(r, "approval.resolve", "approval", id, "success", "", map[string]interface{}{
		"approved": req.Approved,
		"tool":     rec.Tool,
	})
	writeJSON(w, http.StatusOK, rec)
}
