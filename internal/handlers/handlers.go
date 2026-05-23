package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/kranix-io/kranix-api/internal/validation"
	"github.com/kranix-io/kranix-packages/types"
)

// handleDeployWorkload handles workload deployment requests.
func (s *Server) handleDeployWorkload(w http.ResponseWriter, r *http.Request) {
	var spec types.WorkloadSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if err := validation.ValidateWorkloadSpec(&spec); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		id = spec.Name
	}
	if s.ifDryRun(w, r, "workload.deploy", "workload", id, map[string]interface{}{
		"id":   id,
		"spec": specToMap(spec),
	}) {
		return
	}
	if s.Core != nil && s.Core.Enabled() {
		if err := s.Core.DeployWorkload(r.Context(), id, spec); err != nil {
			s.recordAudit(r, "workload.deploy", "workload", id, "error", err.Error(), nil)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "workload.deploy", "workload", id, "success", "", nil)
		writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "deployed"})
		return
	}
	s.recordAudit(r, "workload.deploy", "workload", id, "success", "", map[string]interface{}{"mode": "local"})
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "message": "deploy accepted (core not configured)"})
}

// handleListWorkloads handles listing, filtering, and cursor pagination.
func (s *Server) handleListWorkloads(w http.ResponseWriter, r *http.Request) {
	q := searchQueryFromRequest(r)
	limit := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")
	if s.Core != nil && s.Core.Enabled() {
		resp, err := s.Core.ListWorkloads(r.Context(), q, limit, cursor)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if next, ok := resp["page_info"].(map[string]interface{}); ok {
			if nc, ok := next["next_cursor"].(string); ok && nc != "" {
				w.Header().Set("Link", "</api/v1/workloads?cursor="+nc+">; rel=\"next\"")
			}
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusOK, types.PaginatedWorkloadListResponse{
		Workloads: []types.Workload{},
		PageInfo:  types.PageInfo{Limit: 50},
		Query:     q,
	})
}

// handleGetWorkload handles getting a single workload.
func (s *Server) handleGetWorkload(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	// URL pattern: /api/v1/workloads/{id}
	// TODO: Implement proper path parameter extraction

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Not yet implemented",
	})
}

// handleUpdateWorkload handles updating a workload.
func (s *Server) handleUpdateWorkload(w http.ResponseWriter, r *http.Request) {
	var spec types.WorkloadSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Not yet implemented",
	})
}

// handleDeleteWorkload handles deleting a workload.
func (s *Server) handleDeleteWorkload(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path)
	if id == "" {
		http.Error(w, "workload id required", http.StatusBadRequest)
		return
	}
	if s.ifDryRun(w, r, "workload.delete", "workload", id, map[string]interface{}{"id": id}) {
		return
	}
	if s.Core != nil && s.Core.Enabled() {
		if err := s.Core.DeleteWorkload(r.Context(), id); err != nil {
			s.recordAudit(r, "workload.delete", "workload", id, "error", err.Error(), nil)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	s.recordAudit(r, "workload.delete", "workload", id, "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

// handleRestartWorkload handles restarting a workload.
func (s *Server) handleRestartWorkload(w http.ResponseWriter, r *http.Request) {
	id := extractWorkloadIDFromPath(r.URL.Path, "restart")
	if id == "" {
		id = extractID(r.URL.Path)
	}
	if id == "" {
		http.Error(w, "workload id required", http.StatusBadRequest)
		return
	}
	if s.ifDryRun(w, r, "workload.restart", "workload", id, map[string]interface{}{"id": id}) {
		return
	}
	if s.Core != nil && s.Core.Enabled() {
		if err := s.Core.RestartWorkload(r.Context(), id); err != nil {
			s.recordAudit(r, "workload.restart", "workload", id, "error", err.Error(), nil)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "workload.restart", "workload", id, "success", "", nil)
		writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": "restarted"})
		return
	}
	s.recordAudit(r, "workload.restart", "workload", id, "success", "", map[string]interface{}{"mode": "local"})
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "message": "restart accepted (core not configured)"})
}

// handleListPods handles listing pods for a workload.
func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pods":    []types.Pod{},
		"message": "Not yet implemented",
	})
}

// handleGetPodLogs handles streaming pod logs (SSE).
func (s *Server) handleGetPodLogs(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement SSE streaming
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Not yet implemented",
	})
}

// handleExecPod handles exec into a pod (WebSocket).
func (s *Server) handleExecPod(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement WebSocket
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Not yet implemented",
	})
}

// handleCreateNamespace handles creating a namespace.
func (s *Server) handleCreateNamespace(w http.ResponseWriter, r *http.Request) {
	var namespace types.Namespace
	if err := json.NewDecoder(r.Body).Decode(&namespace); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Not yet implemented",
	})
}

// handleListNamespaces handles listing namespaces.
func (s *Server) handleListNamespaces(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"namespaces": []types.Namespace{},
		"message":    "Not yet implemented",
	})
}

// handleDeleteNamespace handles deleting a namespace.
func (s *Server) handleDeleteNamespace(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core via gRPC
	w.WriteHeader(http.StatusNoContent)
}

// handleGenerateManifests handles generating K8s manifests from intent.
func (s *Server) handleGenerateManifests(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Not yet implemented",
	})
}

// handleAIAsk handles AI assistant queries.
func (s *Server) handleAIAsk(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	prompt, _ := req["prompt"].(string)
	namespace, _ := req["namespace"].(string)

	// TODO: Delegate to kranix-core via gRPC or integrate with AI service
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response":         fmt.Sprintf("AI analysis for: %s in namespace: %s", prompt, namespace),
		"suggested_action": "Review workload configuration and resource limits",
		"code_snippet":     "",
		"confidence":       0.85,
		"message":          "AI integration not yet fully implemented - requires kranix-core integration or AI service",
	})
}

// handleDiffWorkload is a legacy fallback when core is not configured.
func (s *Server) handleDiffWorkload(w http.ResponseWriter, r *http.Request) {
	workloadName := extractWorkloadIDFromPath(r.URL.Path, "diff")
	if workloadName == "" {
		workloadName = extractID(r.URL.Path)
	}
	writeJSON(w, http.StatusOK, types.WorkloadDiffResult{
		WorkloadID: workloadName,
		Changes:    []types.DiffChange{},
		Summary:    types.DiffSummary{},
	})
}

// handleListTemplates handles listing available templates.
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core or template service
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": []map[string]interface{}{
			{
				"name":        "nginx",
				"description": "Basic nginx web server",
				"category":    "web",
				"variables": []map[string]interface{}{
					{
						"name":        "PORT",
						"description": "Container port",
						"default":     "80",
						"required":    false,
					},
				},
			},
			{
				"name":        "nodejs",
				"description": "Node.js application server",
				"category":    "application",
				"variables": []map[string]interface{}{
					{
						"name":        "PORT",
						"description": "Application port",
						"default":     "3000",
						"required":    false,
					},
					{
						"name":        "NODE_ENV",
						"description": "Node environment",
						"default":     "production",
						"required":    false,
					},
				},
			},
			{
				"name":        "postgres",
				"description": "PostgreSQL database",
				"category":    "database",
				"variables": []map[string]interface{}{
					{
						"name":        "POSTGRES_PASSWORD",
						"description": "Database password",
						"default":     "",
						"required":    true,
					},
				},
			},
		},
		"message": "Template listing not yet fully implemented - requires template service integration",
	})
}

// handleGetTemplate handles getting a specific template with variables.
func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	templateName, _ := req["name"].(string)
	_, _ = req["vars"].(map[string]string)

	// TODO: Delegate to kranix-core or template service
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    templateName,
		"content": fmt.Sprintf("# Generated from template: %s\n# Variables: %v\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: %s-app\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: %s\n  template:\n    metadata:\n      labels:\n        app: %s\n    spec:\n      containers:\n      - name: %s\n        image: %s:latest\n", templateName, templateName, templateName, templateName, templateName, templateName, templateName),
		"message": "Template generation not yet fully implemented - requires template service integration",
	})
}

// handleGetWorkloadEvents handles retrieving event history for a workload.
func (s *Server) handleGetWorkloadEvents(w http.ResponseWriter, r *http.Request) {
	workloadID := extractWorkloadIDFromPath(r.URL.Path, "events")
	if workloadID == "" {
		workloadID = extractID(r.URL.Path)
	}
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}
	fromVersion, _ := strconv.ParseInt(r.URL.Query().Get("from_version"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if s.Core != nil && s.Core.Enabled() {
		events, err := s.Core.GetWorkloadEvents(r.Context(), workloadID, fromVersion, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"workload_id": workloadID,
			"events":      events,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workload_id": workloadID,
		"events":      []interface{}{},
		"entries":     s.auditEntriesFor(workloadID),
	})
}

// handleGetEvent handles retrieving a single event by ID.
func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	// Extract event ID from URL path
	eventID := extractID(r.URL.Path)
	if eventID == "" {
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query event sourcing store
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"event_id": eventID,
		"message":  "Event retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleGetDriftReports handles retrieving drift detection reports for a workload.
func (s *Server) handleGetDriftReports(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	workloadID := extractID(r.URL.Path)
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query drift detection reports
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id": workloadID,
		"reports":     []map[string]interface{}{},
		"message":     "Drift report retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleGetScalingHistory handles retrieving scaling history for a workload.
func (s *Server) handleGetScalingHistory(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	workloadID := extractID(r.URL.Path)
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query scaling history
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id":    workloadID,
		"scaling_events": []map[string]interface{}{},
		"message":        "Scaling history retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleGetRolloutStatus handles retrieving rollout status for a workload.
func (s *Server) handleGetRolloutStatus(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	workloadID := extractID(r.URL.Path)
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query rollout status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id": workloadID,
		"rollout_status": map[string]interface{}{
			"type":       "canary",
			"phase":      "in_progress",
			"percentage": 10,
		},
		"message": "Rollout status retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleGetDependencies handles retrieving dependency status for a workload.
func (s *Server) handleGetDependencies(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	workloadID := extractID(r.URL.Path)
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query dependency status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id":  workloadID,
		"dependencies": []map[string]interface{}{},
		"message":      "Dependency status retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleGetTenantQuota handles retrieving tenant quota usage.
func (s *Server) handleGetTenantQuota(w http.ResponseWriter, r *http.Request) {
	// Extract tenant ID from URL path
	tenantID := extractID(r.URL.Path)
	if tenantID == "" {
		http.Error(w, "Invalid tenant ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query tenant quota
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenant_id": tenantID,
		"quota": map[string]interface{}{
			"max_cpu":        "16",
			"max_memory":     "64Gi",
			"max_workloads":  50,
			"used_cpu":       "4",
			"used_memory":    "16Gi",
			"used_workloads": 12,
		},
		"message": "Tenant quota retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleGetPredictions handles retrieving failure predictions for a workload.
func (s *Server) handleGetPredictions(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	workloadID := extractID(r.URL.Path)
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query failure predictions
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id": workloadID,
		"predictions": map[string]interface{}{
			"failure_probability":    0.15,
			"predicted_failure_time": "2026-05-17T15:30:00Z",
			"recommended_actions":    []string{"scale_up", "restart"},
		},
		"message": "Failure prediction retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleGetHealthGateStatus handles retrieving health gate status for a workload.
func (s *Server) handleGetHealthGateStatus(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	workloadID := extractID(r.URL.Path)
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to query health gate status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id":    workloadID,
		"overall_status": "passing",
		"blocked":        false,
		"results":        []map[string]interface{}{},
		"message":        "Health gate status retrieval not yet implemented - requires kranix-core integration",
	})
}

// handleEvaluateHealthGate handles evaluating health gates for a workload.
func (s *Server) handleEvaluateHealthGate(w http.ResponseWriter, r *http.Request) {
	// Extract workload ID from URL path
	workloadID := extractID(r.URL.Path)
	if workloadID == "" {
		http.Error(w, "Invalid workload ID", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC to evaluate health gates
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id":    workloadID,
		"overall_status": "passing",
		"blocked":        false,
		"message":        "Health gate evaluation not yet implemented - requires kranix-core integration",
	})
}

// extractID extracts an ID from a URL path.
func extractID(path string) string {
	// Simple implementation - in production, use a proper router
	parts := splitPath(path)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// splitPath splits a URL path into segments.
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i, c := range path {
		if c == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// Multi-Agent Coordination Handlers

// handleCreateTask handles creating a new coordination task.
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var task map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Task creation not yet implemented - requires kranix-core integration",
		"task":    task,
	})
}

// handleListTasks handles listing coordination tasks.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	status := r.URL.Query().Get("status")

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"status":   status,
		"tasks":    []map[string]interface{}{},
		"message":  "Not yet implemented - requires kranix-core integration",
	})
}

// handleGetTask handles getting a specific task.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := extractID(r.URL.Path)

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id": taskID,
		"message": "Not yet implemented - requires kranix-core integration",
	})
}

// handleUpdateTaskStatus handles updating task status.
func (s *Server) handleUpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := extractID(r.URL.Path)
	var update map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id": taskID,
		"update":  update,
		"message": "Not yet implemented - requires kranix-core integration",
	})
}

// handleDelegateTask handles delegating a task to another agent.
func (s *Server) handleDelegateTask(w http.ResponseWriter, r *http.Request) {
	taskID := extractID(r.URL.Path)
	var delegation map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&delegation); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id":    taskID,
		"delegation": delegation,
		"message":    "Not yet implemented - requires kranix-core integration",
	})
}

// handleClaimTask handles claiming a pending task.
func (s *Server) handleClaimTask(w http.ResponseWriter, r *http.Request) {
	taskID := extractID(r.URL.Path)
	var claim map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&claim); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id": taskID,
		"claim":   claim,
		"message": "Not yet implemented - requires kranix-core integration",
	})
}

// handleCreateSubtask handles creating a sub-task.
func (s *Server) handleCreateSubtask(w http.ResponseWriter, r *http.Request) {
	taskID := extractID(r.URL.Path)
	var subtask map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&subtask); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"parent_task_id": taskID,
		"subtask":        subtask,
		"message":        "Not yet implemented - requires kranix-core integration",
	})
}

// Dry-Run Mode Handlers

// handleSetDryRunMode handles setting the dry-run mode.
func (s *Server) handleSetDryRunMode(w http.ResponseWriter, r *http.Request) {
	var modeReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&modeReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Dry-run mode setting not yet implemented - requires kranix-core integration",
		"mode":    modeReq,
	})
}

// handleGetDryRunMode handles getting the current dry-run mode.
func (s *Server) handleGetDryRunMode(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mode":    "disabled",
		"message": "Not yet implemented - requires kranix-core integration",
	})
}

// handleGetDryRunPreview handles getting dry-run preview.
func (s *Server) handleGetDryRunPreview(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"actions": []map[string]interface{}{},
		"message": "Not yet implemented - requires kranix-core integration",
	})
}

// handleClearDryRunActions handles clearing dry-run actions.
func (s *Server) handleClearDryRunActions(w http.ResponseWriter, r *http.Request) {
	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Dry-run actions cleared - not yet fully implemented - requires kranix-core integration",
	})
}

// Incident Response Handlers

// handleListRunbooks handles listing incident runbooks.
func (s *Server) handleListRunbooks(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"category": category,
		"runbooks": []map[string]interface{}{},
		"message":  "Not yet implemented - requires kranix-core integration",
	})
}

// handleGetRunbook handles getting a specific runbook.
func (s *Server) handleGetRunbook(w http.ResponseWriter, r *http.Request) {
	runbookID := extractID(r.URL.Path)

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"runbook_id": runbookID,
		"message":    "Not yet implemented - requires kranix-core integration",
	})
}

// handleCreateRunbook handles creating a new runbook.
func (s *Server) handleCreateRunbook(w http.ResponseWriter, r *http.Request) {
	var runbook map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&runbook); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Runbook creation not yet implemented - requires kranix-core integration",
		"runbook": runbook,
	})
}

// handleExecuteRunbook handles executing a runbook.
func (s *Server) handleExecuteRunbook(w http.ResponseWriter, r *http.Request) {
	runbookID := extractID(r.URL.Path)
	var executionReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&executionReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"runbook_id": runbookID,
		"execution":  executionReq,
		"message":    "Runbook execution not yet implemented - requires kranix-core integration",
	})
}

// handleListExecutions handles listing runbook executions.
func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	runbookID := r.URL.Query().Get("runbook_id")
	status := r.URL.Query().Get("status")

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"runbook_id": runbookID,
		"status":     status,
		"executions": []map[string]interface{}{},
		"message":    "Not yet implemented - requires kranix-core integration",
	})
}

// handleGetExecution handles getting a specific execution.
func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	executionID := extractID(r.URL.Path)

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"execution_id": executionID,
		"message":      "Not yet implemented - requires kranix-core integration",
	})
}

// handleCancelExecution handles canceling a running execution.
func (s *Server) handleCancelExecution(w http.ResponseWriter, r *http.Request) {
	executionID := extractID(r.URL.Path)

	// TODO: Delegate to kranix-core via gRPC
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"execution_id": executionID,
		"message":      "Execution cancellation not yet implemented - requires kranix-core integration",
	})
}
