package suggestions

import (
	"context"
	"strings"
	"time"

	"github.com/kranix-io/kranix-api/internal/coreclient"
	"github.com/kranix-io/kranix-packages/types"
)

// BuildClusterHealth derives cluster health from workload listings.
func BuildClusterHealth(ctx context.Context, core *coreclient.Client) types.ClusterHealth {
	health := types.ClusterHealth{
		Status:      "healthy",
		NodesReady:  1,
		NodesTotal:  1,
		LastChecked: time.Now().UTC(),
	}

	if core == nil || !core.Enabled() {
		return health
	}

	resp, err := core.ListWorkloads(ctx, types.WorkloadSearchQuery{}, "100", "")
	if err != nil {
		health.Status = "unknown"
		return health
	}

	workloads := extractWorkloads(resp)
	degraded := 0
	for _, wl := range workloads {
		health.PodsTotal++
		status := strings.ToLower(getStringField(wl, "status", "phase"))
		switch {
		case strings.Contains(status, "running"), strings.Contains(status, "ready"), strings.Contains(status, "ok"):
			health.PodsRunning++
		case strings.Contains(status, "fail"), strings.Contains(status, "error"), strings.Contains(status, "crash"):
			degraded++
		}
	}

	health.DegradedWorkloads = degraded
	if degraded > 0 {
		health.Status = "degraded"
	}
	if degraded > 2 {
		health.Status = "critical"
	}
	return health
}

func extractWorkloads(resp map[string]interface{}) []map[string]interface{} {
	raw, ok := resp["workloads"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func getStringField(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return ""
}

// BuildSuggestions returns context-aware next actions from cluster state.
func BuildSuggestions(health types.ClusterHealth, namespace, workload string) types.SuggestionsResponse {
	resp := types.SuggestionsResponse{
		ClusterStatus: health.Status,
		Context:       map[string]interface{}{},
		Suggestions:   make([]types.ActionSuggestion, 0),
		GeneratedAt:   time.Now().UTC(),
	}

	if namespace != "" {
		resp.Context["namespace"] = namespace
	}
	if workload != "" {
		resp.Context["workload"] = workload
	}

	switch health.Status {
	case "degraded", "critical":
		inputs := map[string]interface{}{}
		if namespace != "" {
			inputs["namespace"] = namespace
		}
		resp.Suggestions = append(resp.Suggestions, types.ActionSuggestion{
			Tool:       "list_workloads",
			Reason:     "Cluster is degraded; inspect failing workloads",
			Priority:   "high",
			Inputs:     inputs,
			Confidence: 0.9,
		})
		if workload != "" && namespace != "" {
			resp.Suggestions = append(resp.Suggestions, types.ActionSuggestion{
				Tool:     "analyze_workload",
				Reason:   "Run failure analysis on the target workload",
				Priority: "high",
				Inputs: map[string]interface{}{
					"name":      workload,
					"namespace": namespace,
				},
				Confidence: 0.85,
			})
			resp.Suggestions = append(resp.Suggestions, types.ActionSuggestion{
				Tool:     "chain_tools",
				Reason:   "Execute diagnose-and-recover tool chain",
				Priority: "high",
				Inputs: map[string]interface{}{
					"name": "diagnose-and-recover",
					"steps": []map[string]interface{}{
						{"tool": "analyze_workload", "inputs": map[string]interface{}{"name": workload, "namespace": namespace}},
						{"tool": "list_pods", "inputs": map[string]interface{}{"workload": workload, "namespace": namespace}, "on_failure": "continue"},
						{"tool": "restart_workload", "inputs": map[string]interface{}{"name": workload, "namespace": namespace}},
					},
				},
				Confidence: 0.8,
			})
		}
		resp.Suggestions = append(resp.Suggestions, types.ActionSuggestion{
			Tool:       "list_runbooks",
			Reason:     "Review incident runbooks for automated recovery",
			Priority:   "medium",
			Confidence: 0.7,
		})
	default:
		resp.Suggestions = append(resp.Suggestions, types.ActionSuggestion{
			Tool:       "get_cluster_health",
			Reason:     "Cluster appears healthy; periodic health checks are recommended",
			Priority:   "low",
			Confidence: 0.6,
		})
	}

	return resp
}
