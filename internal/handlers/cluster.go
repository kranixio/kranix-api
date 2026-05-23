package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/kranix-io/kranix-api/internal/suggestions"
	"github.com/kranix-io/kranix-packages/types"
)

// handleGetClusterHealth returns cluster-wide health summary for MCP and CLI consumers.
func (s *Server) handleGetClusterHealth(w http.ResponseWriter, r *http.Request) {
	var health types.ClusterHealth
	if s.Core != nil && s.Core.Enabled() {
		health = suggestions.BuildClusterHealth(r.Context(), s.Core)
	} else {
		health = types.ClusterHealth{
			Status:      "healthy",
			NodesReady:  1,
			NodesTotal:  1,
			PodsRunning: 0,
			PodsTotal:   0,
			LastChecked: time.Now().UTC(),
		}
	}
	writeJSON(w, http.StatusOK, health)
}

// handleGetClusterSuggestions returns context-aware next-action recommendations.
func (s *Server) handleGetClusterSuggestions(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	workload := r.URL.Query().Get("workload")

	health := suggestions.BuildClusterHealth(r.Context(), s.Core)
	resp := suggestions.BuildSuggestions(health, namespace, workload)
	writeJSON(w, http.StatusOK, resp)
}

// handleAnalyzeWorkload handles AI-powered failure analysis with remediation suggestions.
func (s *Server) handleAnalyzeWorkload(w http.ResponseWriter, r *http.Request) {
	workloadID := extractWorkloadIDFromAnalyzePath(r.URL.Path)
	namespace := r.URL.Query().Get("namespace")

	result := types.AnalysisResult{
		WorkloadID: workloadID,
		Namespace:  namespace,
		Status:     "ok",
		AnalyzedAt: time.Now().UTC(),
	}

	if workloadID == "" {
		http.Error(w, "workload id required", http.StatusBadRequest)
		return
	}

	health := suggestions.BuildClusterHealth(r.Context(), s.Core)
	if health.Status == "degraded" || health.Status == "critical" {
		result.Status = "degraded"
		result.Issues = []types.Issue{
			{
				Severity: "warning",
				Type:     "cluster_degraded",
				Message:  "Cluster has degraded workloads that may affect this workload",
			},
		}
		result.Suggestions = []string{
			"Run stream_logs on failing pods",
			"Consider restart_workload if errors are transient",
			"Use chain_tools to run diagnose-and-recover sequence",
		}
		result.ProbableFix = "Inspect pod logs and restart workload if crash loop is detected"
	} else {
		result.Suggestions = []string{
			"Monitor workload metrics",
			"Run get_cluster_health periodically",
		}
		result.ProbableFix = "No immediate action required"
	}

	writeJSON(w, http.StatusOK, result)
}

func extractWorkloadIDFromAnalyzePath(path string) string {
	path = strings.TrimSuffix(path, "/")
	if !strings.HasSuffix(path, "/analyze") {
		return ""
	}
	path = strings.TrimSuffix(path, "/analyze")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
