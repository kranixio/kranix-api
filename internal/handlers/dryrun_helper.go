package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-api/internal/dryrun"
	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) ifDryRun(w http.ResponseWriter, r *http.Request, action, resource, resourceID string, wouldApply map[string]interface{}) bool {
	return dryrun.Respond(w, r, action, resource, resourceID, wouldApply)
}

func specToMap(spec types.WorkloadSpec) map[string]interface{} {
	b, _ := json.Marshal(spec)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}
