package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kranix-io/kranix-api/internal/audit"
	"github.com/kranix-io/kranix-api/internal/approval"
	"github.com/kranix-io/kranix-api/internal/changelognotify"
	"github.com/kranix-io/kranix-api/internal/coreclient"
	"github.com/kranix-io/kranix-api/internal/sse"
	"github.com/kranix-io/kranix-api/internal/validation"
	"github.com/kranix-io/kranix-api/internal/version"
	"github.com/kranix-io/kranix-api/internal/webhooks"
	"github.com/kranix-io/kranix-packages/types"
)

// Server holds shared handler dependencies.
type Server struct {
	Core            *coreclient.Client
	Audit           *audit.Logger
	Version         *version.Manager
	ChangelogNotify *changelognotify.Service
	Webhooks        *webhooks.Service
	SSE       *sse.Service
	Approvals *approval.Store
}

// NewServer creates a handler server with core and audit dependencies.
func NewServer(core *coreclient.Client, auditLog *audit.Logger, ver *version.Manager, changelog *changelognotify.Service, wh *webhooks.Service) *Server {
	return &Server{Core: core, Audit: auditLog, Version: ver, ChangelogNotify: changelog, Webhooks: wh}
}

func (s *Server) broadcastClusterEvent(event string, data interface{}, namespace string) {
	if s.SSE == nil {
		return
	}
	if event == "workload.changed" {
		if change, ok := data.(*types.WorkloadStateChange); ok {
			s.SSE.BroadcastWorkloadChange(change)
			return
		}
	}
	filter := map[string]string{}
	if namespace != "" {
		filter["namespace"] = namespace
	}
	s.SSE.Broadcast(event, data, filter)
}

// RegisterRoutes registers HTTP handlers on mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/workloads/bulk", s.handleBulkWorkloads)
	mux.HandleFunc("GET /api/v1/audit", s.handleListAudit)
	mux.HandleFunc("GET /api/v1/audit/", s.handleGetAuditEntry)
	mux.HandleFunc("GET /api/v1/audit/resources/{type}/{id}", s.handleAuditResource)

	mux.HandleFunc("GET /api/v1/changelog/subscriptions", s.handleListChangelogSubscriptions)
	mux.HandleFunc("POST /api/v1/changelog/subscriptions", s.handleCreateChangelogSubscription)
	mux.HandleFunc("DELETE /api/v1/changelog/subscriptions/{id}", s.handleDeleteChangelogSubscription)
	mux.HandleFunc("POST /api/v1/changelog/releases", s.handlePublishChangelogRelease)

	mux.HandleFunc("POST /api/v1/workloads", s.handleDeployWorkload)
	mux.HandleFunc("GET /api/v1/workloads", s.handleListWorkloads)
	mux.HandleFunc("GET /api/v1/workloads/{id}/diff", s.handleGetWorkloadDiff)
	mux.HandleFunc("POST /api/v1/workloads/{id}/diff", s.handlePostWorkloadDiff)
	mux.HandleFunc("GET /api/v1/workloads/{id}", s.handleGetWorkloadByID)
	mux.HandleFunc("PATCH /api/v1/workloads/{id}", s.handleUpdateWorkloadByID)
	mux.HandleFunc("DELETE /api/v1/workloads/{id}", s.handleDeleteWorkloadByID)
	mux.HandleFunc("POST /api/v1/workloads/{id}/restart", s.handleRestartWorkloadByID)
	mux.HandleFunc("GET /api/v1/workloads/{id}/revisions", s.handleListRevisionsByID)
	mux.HandleFunc("POST /api/v1/workloads/{id}/rollback", s.handleRollbackWorkloadByID)

	mux.HandleFunc("GET /api/v1/quotas", s.handleListQuotas)
	mux.HandleFunc("GET /api/v1/quotas/{namespace}/usage", s.handleNamespaceQuotaUsage)
	mux.HandleFunc("GET /api/v1/quotas/{namespace}", s.handleGetNamespaceQuota)
	mux.HandleFunc("PUT /api/v1/quotas/{namespace}", s.handlePutNamespaceQuota)
	mux.HandleFunc("DELETE /api/v1/quotas/{namespace}", s.handleDeleteNamespaceQuota)

	mux.HandleFunc("GET /api/v1/workloads/", s.handleListPods)
	mux.HandleFunc("GET /api/v1/pods/", s.handleGetPodLogs)
	mux.HandleFunc("GET /api/v1/pods/", s.handleExecPod)

	mux.HandleFunc("POST /api/v1/namespaces", s.handleCreateNamespace)
	mux.HandleFunc("GET /api/v1/namespaces", s.handleListNamespaces)
	mux.HandleFunc("DELETE /api/v1/namespaces/", s.handleDeleteNamespace)

	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetWorkloadEvents)
	mux.HandleFunc("GET /api/v1/events/", s.handleGetEvent)

	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetDriftReports)
	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetHealthGateStatus)
	mux.HandleFunc("POST /api/v1/workloads/", s.handleEvaluateHealthGate)
	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetScalingHistory)
	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetRolloutStatus)
	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetDependencies)
	mux.HandleFunc("GET /api/v1/tenants/", s.handleGetTenantQuota)
	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetPredictions)
	mux.HandleFunc("GET /api/v1/workloads/", s.handleAnalyzeWorkload)
	mux.HandleFunc("POST /api/v1/manifests/generate", s.handleGenerateManifests)
	mux.HandleFunc("POST /api/v1/templates/kranixapp", s.handleGenerateKranixAppTemplate)
	mux.HandleFunc("GET /api/v1/nodes/health", s.handleListNodeHealth)
	mux.HandleFunc("POST /api/v1/nodes/{name}/drain", s.handleDrainNode)
	mux.HandleFunc("POST /api/v1/workloads/{id}/checkpoint", s.handleCheckpointWorkload)
	mux.HandleFunc("POST /api/v1/workloads/{id}/restore", s.handleRestoreWorkload)
	mux.HandleFunc("GET /api/v1/workloads/{id}/checkpoints", s.handleListCheckpoints)
	mux.HandleFunc("GET /api/v1/runtime/plugins", s.handleListRuntimePlugins)
	mux.HandleFunc("POST /api/v1/ai/ask", s.handleAIAsk)
	mux.HandleFunc("GET /api/v1/cluster/health", s.handleGetClusterHealth)
	mux.HandleFunc("GET /api/v1/cluster/suggestions", s.handleGetClusterSuggestions)
	mux.HandleFunc("POST /api/v1/workloads/", s.handleDiffWorkload)
	mux.HandleFunc("GET /api/v1/workloads/", s.handleGetWorkloadCost)
	mux.HandleFunc("GET /api/v1/cost/summary", s.handleGetCostSummary)
	mux.HandleFunc("POST /api/v1/cost/estimate", s.handleEstimateDeploymentCost)
	mux.HandleFunc("POST /api/v1/approvals", s.handleCreateApproval)
	mux.HandleFunc("GET /api/v1/approvals", s.handleListApprovals)
	mux.HandleFunc("GET /api/v1/approvals/{id}", s.handleGetApproval)
	mux.HandleFunc("POST /api/v1/approvals/{id}/resolve", s.handleResolveApproval)
	mux.HandleFunc("GET /api/v1/templates", s.handleListTemplates)
	mux.HandleFunc("POST /api/v1/templates/get", s.handleGetTemplate)
	mux.HandleFunc("POST /api/v1/coordination/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/v1/coordination/tasks", s.handleListTasks)
	mux.HandleFunc("GET /api/v1/coordination/tasks/", s.handleGetTask)
	mux.HandleFunc("PATCH /api/v1/coordination/tasks/", s.handleUpdateTaskStatus)
	mux.HandleFunc("POST /api/v1/coordination/tasks/", s.handleDelegateTask)
	mux.HandleFunc("POST /api/v1/coordination/tasks/", s.handleClaimTask)
	mux.HandleFunc("POST /api/v1/coordination/tasks/", s.handleCreateSubtask)
	mux.HandleFunc("POST /api/v1/dryrun/mode", s.handleSetDryRunMode)
	mux.HandleFunc("GET /api/v1/dryrun/mode", s.handleGetDryRunMode)
	mux.HandleFunc("GET /api/v1/dryrun/preview", s.handleGetDryRunPreview)
	mux.HandleFunc("DELETE /api/v1/dryrun/actions", s.handleClearDryRunActions)
	mux.HandleFunc("GET /api/v1/incident/runbooks", s.handleListRunbooks)
	mux.HandleFunc("GET /api/v1/incident/runbooks/", s.handleGetRunbook)
	mux.HandleFunc("POST /api/v1/incident/runbooks", s.handleCreateRunbook)
	mux.HandleFunc("POST /api/v1/incident/runbooks/", s.handleExecuteRunbook)
	mux.HandleFunc("GET /api/v1/incident/executions", s.handleListExecutions)
	mux.HandleFunc("GET /api/v1/incident/executions/", s.handleGetExecution)
	mux.HandleFunc("DELETE /api/v1/incident/executions/", s.handleCancelExecution)
}

func (s *Server) recordAudit(r *http.Request, action, resourceType, resourceID, outcome, errMsg string, details map[string]interface{}) {
	if s.Audit == nil {
		return
	}
	actor := r.Header.Get("X-Actor")
	if actor == "" {
		actor = r.Header.Get("X-Agent-Id")
	}
	if actor == "" {
		actor = "api"
	}
	if details == nil {
		details = map[string]interface{}{}
	}
	if agentID := r.Header.Get("X-Agent-Id"); agentID != "" {
		details["agent_id"] = agentID
	}
	s.Audit.Log(types.AuditEntry{
		Actor:        actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcome,
		Error:        errMsg,
		RequestID:    r.Header.Get("X-Request-ID"),
		Details:      details,
	})
}

func (s *Server) handleBulkWorkloads(w http.ResponseWriter, r *http.Request) {
	var req types.BulkWorkloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	switch req.Operation {
	case types.BulkOpDeploy, types.BulkOpRestart, types.BulkOpDelete:
	default:
		http.Error(w, "operation must be deploy, restart, or delete", http.StatusBadRequest)
		return
	}
	if len(req.Workloads) == 0 {
		http.Error(w, "workloads required", http.StatusBadRequest)
		return
	}
	for i := range req.Workloads {
		if req.Operation == types.BulkOpDeploy {
			if err := validation.ValidateWorkloadSpec(&req.Workloads[i].Spec); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	if s.ifDryRun(w, r, "bulk."+string(req.Operation), "workload", "", map[string]interface{}{
		"operation": req.Operation,
		"count":     len(req.Workloads),
		"workloads": req.Workloads,
	}) {
		return
	}

	if s.Core != nil && s.Core.Enabled() {
		resp, err := s.Core.BulkWorkloads(r.Context(), req)
		if err != nil {
			s.recordAudit(r, "bulk."+string(req.Operation), "workload", "", "error", err.Error(), nil)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		s.recordAudit(r, "bulk."+string(req.Operation), "workload", "", "success", "", map[string]interface{}{
			"succeeded": resp.Succeeded,
			"failed":    resp.Failed,
		})
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Local fallback when core is not configured.
	resp := types.BulkWorkloadResponse{Operation: string(req.Operation)}
	for _, item := range req.Workloads {
		res := types.BulkWorkloadResult{ID: item.ID, Success: true}
		resp.Results = append(resp.Results, res)
		resp.Succeeded++
	}
	s.recordAudit(r, "bulk."+string(req.Operation), "workload", "", "success", "", nil)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	if s.Audit == nil {
		writeJSON(w, http.StatusOK, types.AuditListResponse{Entries: []types.AuditEntry{}})
		return
	}
	q := types.AuditQuery{
		ResourceType: r.URL.Query().Get("resource_type"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		Action:       r.URL.Query().Get("action"),
		Actor:        r.URL.Query().Get("actor"),
	}
	if lim := r.URL.Query().Get("limit"); lim != "" {
		q.Limit, _ = strconv.Atoi(lim)
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q.Since = t
		}
	}
	entries := s.Audit.Query(q)
	writeJSON(w, http.StatusOK, types.AuditListResponse{
		ResourceType: q.ResourceType,
		ResourceID:   q.ResourceID,
		Entries:      entries,
	})
}

func (s *Server) handleGetAuditEntry(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/audit/")
	if id == "" || s.Audit == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if entry, ok := s.Audit.Get(id); ok {
		writeJSON(w, http.StatusOK, entry)
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (s *Server) handleAuditResource(w http.ResponseWriter, r *http.Request) {
	resourceType := r.PathValue("type")
	resourceID := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}

	resp := types.AuditListResponse{
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
	if s.Audit != nil {
		resp.Entries = s.Audit.Query(types.AuditQuery{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Limit:        limit,
		})
	}
	if s.Core != nil && s.Core.Enabled() && resourceID != "" {
		coreEvents, err := s.Core.GetAuditResource(r.Context(), resourceType, resourceID, limit)
		if err == nil {
			resp.CoreEvents = coreEvents["events"]
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) auditEntriesFor(resourceID string) []types.AuditEntry {
	if s.Audit == nil {
		return nil
	}
	return s.Audit.Query(types.AuditQuery{ResourceID: resourceID, Limit: 50})
}

func extractWorkloadIDFromPath(path, suffix string) string {
	path = strings.TrimSuffix(strings.Trim(path, "/"), "/"+suffix)
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
