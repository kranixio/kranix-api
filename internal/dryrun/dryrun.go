package dryrun

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kranix-io/kranix-packages/types"
)

// IsDryRun reports whether the request is a dry-run (no mutations applied).
func IsDryRun(r *http.Request) bool {
	q := r.URL.Query()
	if v := strings.ToLower(strings.TrimSpace(q.Get("dryRun"))); v == "true" || v == "1" {
		return true
	}
	if v := strings.ToLower(strings.TrimSpace(q.Get("dry_run"))); v == "true" || v == "1" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Dry-Run")), "true") {
		return true
	}
	return false
}

// Respond writes a dry-run preview response and returns true when dry-run is active.
func Respond(w http.ResponseWriter, r *http.Request, action, resource, resourceID string, wouldApply map[string]interface{}) bool {
	if !IsDryRun(r) {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Dry-Run", "true")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(types.DryRunResponse{
		DryRun:     true,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		WouldApply: wouldApply,
		Message:    "No changes applied (dryRun=true)",
	})
	return true
}
