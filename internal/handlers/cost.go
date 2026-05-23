package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/cost"
	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleEstimateDeploymentCost(w http.ResponseWriter, r *http.Request) {
	var req types.CostEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	name := req.Name
	if name == "" {
		name = req.Spec.Name
	}
	namespace := req.Namespace
	if namespace == "" {
		namespace = req.Spec.Namespace
	}

	resp := cost.EstimateFromSpec(name, namespace, req.Spec, req.Duration)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetWorkloadCost(w http.ResponseWriter, r *http.Request) {
	workloadName := extractID(r.URL.Path)
	namespace := r.URL.Query().Get("namespace")
	duration := r.URL.Query().Get("duration")

	if s.Core != nil && s.Core.Enabled() {
		wl, err := s.Core.GetWorkload(r.Context(), workloadName)
		if err == nil && wl != nil {
			resp := cost.EstimateFromWorkload(wl, duration)
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	spec := types.WorkloadSpec{
		Name:      workloadName,
		Namespace: namespace,
		Replicas:  1,
		Resources: types.ResourceSpec{CPURequest: "100m", CPULimit: "500m"},
	}
	resp := cost.EstimateFromSpec(workloadName, namespace, spec, duration)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetCostSummary(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	duration := r.URL.Query().Get("duration")

	var top []map[string]interface{}
	var total float64
	count := 0

	if s.Core != nil && s.Core.Enabled() {
		q := types.WorkloadSearchQuery{Namespace: namespace}
		resp, err := s.Core.ListWorkloads(r.Context(), q, "100", "")
		if err == nil {
			if workloads, ok := resp["workloads"].([]interface{}); ok {
				for _, item := range workloads {
					raw, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					spec := mapToWorkloadSpec(raw)
					name, _ := raw["name"].(string)
					ns, _ := raw["namespace"].(string)
					estimate := cost.EstimateFromSpec(name, ns, spec, duration)
					total += estimate.TotalCost
					count++
					top = append(top, map[string]interface{}{
						"workload_name": name,
						"namespace":     ns,
						"total_cost":    estimate.TotalCost,
					})
				}
			}
		}
	}

	if count == 0 {
		estimate := cost.EstimateFromSpec("sample", namespace, types.WorkloadSpec{
			Replicas:  1,
			Resources: types.ResourceSpec{CPURequest: "250m", CPULimit: "500m"},
		}, duration)
		total = estimate.TotalCost
		count = 1
		top = []map[string]interface{}{{
			"workload_name": "sample",
			"total_cost":    estimate.TotalCost,
		}}
	}

	if len(top) > 5 {
		top = top[:5]
	}

	avg := 0.0
	if count > 0 {
		avg = total / float64(count)
	}

	writeJSON(w, http.StatusOK, types.CostSummaryResponse{
		Namespace:        namespace,
		Duration:         duration,
		TotalCost:        roundCost(total),
		WorkloadCount:    count,
		AverageCost:      roundCost(avg),
		TopCostWorkloads: top,
		Message:          "cost summary from shared estimator",
	})
}

func mapToWorkloadSpec(raw map[string]interface{}) types.WorkloadSpec {
	spec := types.WorkloadSpec{Replicas: 1}
	if s, ok := raw["spec"].(map[string]interface{}); ok {
		if image, ok := s["image"].(string); ok {
			spec.Image = image
		}
		if replicas, ok := s["replicas"].(float64); ok {
			spec.Replicas = int(replicas)
		}
		if res, ok := s["resources"].(map[string]interface{}); ok {
			if v, ok := res["cpuRequest"].(string); ok {
				spec.Resources.CPURequest = v
			}
			if v, ok := res["cpuLimit"].(string); ok {
				spec.Resources.CPULimit = v
			}
			if v, ok := res["memoryLimit"].(string); ok {
				spec.Resources.MemoryLimit = v
			}
		}
	}
	return spec
}

func roundCost(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
