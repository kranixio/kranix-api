package analytics

import (
	"strings"
	"sync"
	"time"

	"github.com/kranix-io/kranix-packages/logging"
	"github.com/kranix-io/kranix-packages/types"
)

// Service manages analytics data collection and querying.
type Service struct {
	metrics        map[string][]*types.AnalyticsMetrics
	deployMetrics  map[string]*types.DeployMetrics
	errorMetrics   map[string]*types.ErrorMetrics
	latencyMetrics map[string]*types.LatencyMetrics
	mu             sync.RWMutex
	logger         *logging.Logger
	retention      time.Duration
}

// Config represents analytics service configuration.
type Config struct {
	Retention time.Duration `yaml:"retention"`
	Enabled   bool          `yaml:"enabled"`
}

// NewService creates a new analytics service.
func NewService(config Config, logger *logging.Logger) *Service {
	return &Service{
		metrics:        make(map[string][]*types.AnalyticsMetrics),
		deployMetrics:  make(map[string]*types.DeployMetrics),
		errorMetrics:   make(map[string]*types.ErrorMetrics),
		latencyMetrics: make(map[string]*types.LatencyMetrics),
		logger:         logger,
		retention:      config.Retention,
	}
}

// RecordMetric records an analytics metric.
func (s *Service) RecordMetric(metric *types.AnalyticsMetrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := metric.ResourceType + ":" + metric.ResourceID
	s.metrics[key] = append(s.metrics[key], metric)

	// Update specific metric types based on metric type
	switch metric.MetricType {
	case "deploy":
		s.updateDeployMetrics(metric)
	case "error":
		s.updateErrorMetrics(metric)
	case "latency":
		s.updateLatencyMetrics(metric)
	}

	s.logger.Info("metric recorded", "type", metric.MetricType, "resource", metric.ResourceID)
	return nil
}

// QueryMetrics queries analytics metrics based on the query parameters.
func (s *Service) QueryMetrics(query *types.AnalyticsQuery) ([]*types.AnalyticsMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*types.AnalyticsMetrics

	for key, metrics := range s.metrics {
		// Filter by resource type
		if query.ResourceType != "" {
			if !strings.HasPrefix(key, query.ResourceType+":") {
				continue
			}
		}

		// Filter by resource ID
		if query.ResourceID != "" {
			if !strings.HasSuffix(key, ":"+query.ResourceID) {
				continue
			}
		}

		// Filter by metric type
		for _, metric := range metrics {
			if query.MetricType != "" && metric.MetricType != query.MetricType {
				continue
			}

			// Filter by time window
			if !query.StartTime.IsZero() && metric.Timestamp.Before(query.StartTime) {
				continue
			}
			if !query.EndTime.IsZero() && metric.Timestamp.After(query.EndTime) {
				continue
			}

			results = append(results, metric)
		}
	}

	// Apply limit and offset
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results, nil
}

// GetDeployMetrics gets deployment metrics for a workload.
func (s *Service) GetDeployMetrics(workloadID, namespace string, startTime, endTime time.Time) (*types.DeployMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	analyticsMetrics, ok := s.metrics["workload:"+workloadID]
	if !ok {
		return &types.DeployMetrics{
			WorkloadID:  workloadID,
			Namespace:   namespace,
			WindowStart: startTime,
			WindowEnd:   endTime,
		}, nil
	}

	var filtered []*types.AnalyticsMetrics
	for _, metric := range analyticsMetrics {
		if metric.Timestamp.After(startTime) && metric.Timestamp.Before(endTime) {
			filtered = append(filtered, metric)
		}
	}

	// Calculate metrics
	deployMetrics := &types.DeployMetrics{
		WorkloadID:  workloadID,
		Namespace:   namespace,
		WindowStart: startTime,
		WindowEnd:   endTime,
	}

	for _, metric := range filtered {
		if metric.MetricType == "deploy_success" {
			deployMetrics.SuccessCount++
		} else if metric.MetricType == "deploy_failure" {
			deployMetrics.FailureCount++
		}
		deployMetrics.DeployCount++
	}

	if deployMetrics.DeployCount > 0 {
		deployMetrics.SuccessRate = float64(deployMetrics.SuccessCount) / float64(deployMetrics.DeployCount)
	}

	return deployMetrics, nil
}

// GetErrorMetrics gets error metrics for a workload.
func (s *Service) GetErrorMetrics(workloadID, namespace string, startTime, endTime time.Time) (*types.ErrorMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := "workload:" + workloadID
	analyticsMetrics, ok := s.metrics[key]
	if !ok {
		return &types.ErrorMetrics{
			WorkloadID:  workloadID,
			Namespace:   namespace,
			WindowStart: startTime,
			WindowEnd:   endTime,
			ErrorTypes:  make(map[string]int64),
		}, nil
	}

	errorMetrics := &types.ErrorMetrics{
		WorkloadID:  workloadID,
		Namespace:   namespace,
		WindowStart: startTime,
		WindowEnd:   endTime,
		ErrorTypes:  make(map[string]int64),
	}

	for _, metric := range analyticsMetrics {
		if metric.Timestamp.After(startTime) && metric.Timestamp.Before(endTime) {
			errorMetrics.ErrorCount++
			errorType := metric.Labels["error_type"]
			if errorType != "" {
				errorMetrics.ErrorTypes[errorType]++
			}
		}
	}

	return errorMetrics, nil
}

// GetLatencyMetrics gets latency metrics for a workload.
func (s *Service) GetLatencyMetrics(workloadID, namespace string, startTime, endTime time.Time) (*types.LatencyMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := "workload:" + workloadID
	analyticsMetrics, ok := s.metrics[key]
	if !ok {
		return &types.LatencyMetrics{
			WorkloadID:  workloadID,
			Namespace:   namespace,
			WindowStart: startTime,
			WindowEnd:   endTime,
		}, nil
	}

	var latencies []time.Duration
	for _, metric := range analyticsMetrics {
		if metric.Timestamp.After(startTime) && metric.Timestamp.Before(endTime) && metric.MetricType == "latency" {
			latency := time.Duration(metric.Value)
			latencies = append(latencies, latency)
		}
	}

	if len(latencies) == 0 {
		return &types.LatencyMetrics{
			WorkloadID:  workloadID,
			Namespace:   namespace,
			WindowStart: startTime,
			WindowEnd:   endTime,
		}, nil
	}

	// Calculate percentiles
	latencyMetrics := &types.LatencyMetrics{
		WorkloadID:  workloadID,
		Namespace:   namespace,
		WindowStart: startTime,
		WindowEnd:   endTime,
	}

	// Simple implementation - in production use proper percentile calculation
	if len(latencies) > 0 {
		latencyMetrics.AverageLatency = calculateAverage(latencies)
		latencyMetrics.MaxLatency = calculateMax(latencies)
		// P50, P95, P99 would be calculated properly in production
		latencyMetrics.P50Latency = latencyMetrics.AverageLatency
		latencyMetrics.P95Latency = latencyMetrics.AverageLatency
		latencyMetrics.P99Latency = latencyMetrics.AverageLatency
	}

	return latencyMetrics, nil
}

// GetUsageSummary gets a summary of usage metrics.
func (s *Service) GetUsageSummary(startTime, endTime time.Time) (*types.UsageSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &types.UsageSummary{
		WindowStart: startTime,
		WindowEnd:   endTime,
		ByNamespace: make(map[string]*types.NamespaceUsage),
		ByTenant:    make(map[string]*types.TenantUsage),
	}

	// Aggregate metrics across all workloads
	for _, metrics := range s.metrics {
		for _, metric := range metrics {
			if metric.Timestamp.After(startTime) && metric.Timestamp.Before(endTime) {
				if metric.MetricType == "deploy" {
					summary.TotalDeploys++
				} else if metric.MetricType == "error" {
					summary.TotalErrors++
				}
			}
		}
	}

	return summary, nil
}

// updateDeployMetrics updates deployment metrics.
func (s *Service) updateDeployMetrics(metric *types.AnalyticsMetrics) {
	if _, ok := s.deployMetrics[metric.ResourceID]; !ok {
		s.deployMetrics[metric.ResourceID] = &types.DeployMetrics{
			WorkloadID: metric.ResourceID,
			Namespace:  metric.Labels["namespace"],
		}
	}
	s.deployMetrics[metric.ResourceID].LastDeployAt = metric.Timestamp
}

// updateErrorMetrics updates error metrics.
func (s *Service) updateErrorMetrics(metric *types.AnalyticsMetrics) {
	if _, ok := s.errorMetrics[metric.ResourceID]; !ok {
		s.errorMetrics[metric.ResourceID] = &types.ErrorMetrics{
			WorkloadID: metric.ResourceID,
			Namespace:  metric.Labels["namespace"],
			ErrorTypes: make(map[string]int64),
		}
	}
	s.errorMetrics[metric.ResourceID].LastErrorAt = metric.Timestamp
	s.errorMetrics[metric.ResourceID].LastErrorType = metric.Labels["error_type"]
}

// updateLatencyMetrics updates latency metrics.
func (s *Service) updateLatencyMetrics(metric *types.AnalyticsMetrics) {
	if _, ok := s.latencyMetrics[metric.ResourceID]; !ok {
		s.latencyMetrics[metric.ResourceID] = &types.LatencyMetrics{
			WorkloadID: metric.ResourceID,
			Namespace:  metric.Labels["namespace"],
		}
	}
}

// calculateAverage calculates the average duration.
func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

// calculateMax calculates the maximum duration.
func calculateMax(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	max := durations[0]
	for _, d := range durations {
		if d > max {
			max = d
		}
	}
	return max
}
