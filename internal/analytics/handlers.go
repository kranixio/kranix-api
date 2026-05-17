package analytics

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/kranix-io/kranix-packages/types"
)

// RegisterRoutes registers analytics HTTP handlers.
func RegisterRoutes(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("GET /api/v1/analytics/metrics", handleQueryMetrics(service))
	mux.HandleFunc("POST /api/v1/analytics/metrics", handleRecordMetric(service))
	mux.HandleFunc("GET /api/v1/analytics/workloads/", handleGetWorkloadMetrics(service))
	mux.HandleFunc("GET /api/v1/analytics/summary", handleGetUsageSummary(service))
}

// handleQueryMetrics handles querying analytics metrics.
func handleQueryMetrics(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := parseAnalyticsQuery(r)
		
		metrics, err := service.QueryMetrics(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metrics": metrics,
			"count":   len(metrics),
		})
	}
}

// handleRecordMetric handles recording an analytics metric.
func handleRecordMetric(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var metric types.AnalyticsMetrics
		if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := service.RecordMetric(&metric); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Metric recorded successfully",
		})
	}
}

// handleGetWorkloadMetrics handles getting metrics for a specific workload.
func handleGetWorkloadMetrics(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workloadID := extractID(r.URL.Path)
		if workloadID == "" {
			http.Error(w, "Invalid workload ID", http.StatusBadRequest)
			return
		}

		namespace := r.URL.Query().Get("namespace")
		metricType := r.URL.Query().Get("type") // deploy, error, latency
		
		startTime := parseTime(r.URL.Query().Get("start_time"), time.Now().Add(-24*time.Hour))
		endTime := parseTime(r.URL.Query().Get("end_time"), time.Now())

		switch metricType {
		case "deploy":
			metrics, err := service.GetDeployMetrics(workloadID, namespace, startTime, endTime)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(metrics)
		case "error":
			metrics, err := service.GetErrorMetrics(workloadID, namespace, startTime, endTime)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(metrics)
		case "latency":
			metrics, err := service.GetLatencyMetrics(workloadID, namespace, startTime, endTime)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(metrics)
		default:
			// Return all metrics
			deployMetrics, _ := service.GetDeployMetrics(workloadID, namespace, startTime, endTime)
			errorMetrics, _ := service.GetErrorMetrics(workloadID, namespace, startTime, endTime)
			latencyMetrics, _ := service.GetLatencyMetrics(workloadID, namespace, startTime, endTime)
			
			json.NewEncoder(w).Encode(map[string]interface{}{
				"deploy":  deployMetrics,
				"error":   errorMetrics,
				"latency": latencyMetrics,
			})
		}
	}
}

// handleGetUsageSummary handles getting usage summary.
func handleGetUsageSummary(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := parseTime(r.URL.Query().Get("start_time"), time.Now().Add(-24*time.Hour))
		endTime := parseTime(r.URL.Query().Get("end_time"), time.Now())

		summary, err := service.GetUsageSummary(startTime, endTime)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	}
}

// parseAnalyticsQuery parses analytics query parameters.
func parseAnalyticsQuery(r *http.Request) *types.AnalyticsQuery {
	query := &types.AnalyticsQuery{
		ResourceType: r.URL.Query().Get("resource_type"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		MetricType:   r.URL.Query().Get("metric_type"),
		Granularity:  r.URL.Query().Get("granularity"),
		StartTime:    parseTime(r.URL.Query().Get("start_time"), time.Now().Add(-24*time.Hour)),
		EndTime:      parseTime(r.URL.Query().Get("end_time"), time.Now()),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			query.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			query.Offset = offset
		}
	}

	return query
}

// parseTime parses a time string.
func parseTime(timeStr string, defaultTime time.Time) time.Time {
	if timeStr == "" {
		return defaultTime
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return defaultTime
	}
	return t
}

// extractID extracts an ID from a URL path.
func extractID(path string) string {
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
