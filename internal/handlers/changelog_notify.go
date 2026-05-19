package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleListChangelogSubscriptions(w http.ResponseWriter, r *http.Request) {
	if s.ChangelogNotify == nil || !s.ChangelogNotify.Enabled() {
		writeJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": []types.ChangelogSubscription{}, "count": 0})
		return
	}
	subs := s.ChangelogNotify.ListSubscriptions()
	writeJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": subs, "count": len(subs)})
}

func (s *Server) handleCreateChangelogSubscription(w http.ResponseWriter, r *http.Request) {
	if s.ChangelogNotify == nil || !s.ChangelogNotify.Enabled() {
		http.Error(w, "changelog notifications disabled", http.StatusServiceUnavailable)
		return
	}
	var sub types.ChangelogSubscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if s.ifDryRun(w, r, "changelog.subscribe", "changelog_subscription", sub.Email, map[string]interface{}{
		"subscription": sub,
	}) {
		return
	}
	created, err := s.ChangelogNotify.Subscribe(&sub)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.recordAudit(r, "changelog.subscribe", "changelog_subscription", created.ID, "success", "", nil)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteChangelogSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.ChangelogNotify == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if s.ifDryRun(w, r, "changelog.unsubscribe", "changelog_subscription", id, map[string]interface{}{"id": id}) {
		return
	}
	if !s.ChangelogNotify.Unsubscribe(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.recordAudit(r, "changelog.unsubscribe", "changelog_subscription", id, "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePublishChangelogRelease(w http.ResponseWriter, r *http.Request) {
	if s.Version == nil {
		http.Error(w, "version manager unavailable", http.StatusServiceUnavailable)
		return
	}
	var req types.PublishChangelogReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Version == "" {
		http.Error(w, "version is required", http.StatusBadRequest)
		return
	}
	if s.ifDryRun(w, r, "changelog.publish", "api_version", req.Version, map[string]interface{}{
		"version": req.Version,
		"entries": len(req.Entries),
		"notify":  req.Notify,
	}) {
		return
	}
	notify := req.Notify
	if !notify && s.ChangelogNotify != nil && s.ChangelogNotify.Enabled() {
		for _, e := range req.Entries {
			if e.Breaking {
				notify = true
				break
			}
		}
	}
	result := s.Version.PublishRelease(req.Version, req.Entries, notify, s.ChangelogNotify, s.Webhooks)
	s.recordAudit(r, "changelog.publish", "api_version", req.Version, "success", "", map[string]interface{}{
		"breaking_notify": result,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":      req.Version,
		"entries":      len(req.Entries),
		"notification": result,
	})
}
