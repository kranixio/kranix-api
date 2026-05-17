package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/kranix-io/kranix-packages/logging"
	"github.com/kranix-io/kranix-packages/types"
	"golang.org/x/time/rate"
)

// Service manages rate limiting and quotas.
type Service struct {
	config     *types.RateLimitConfig
	logger     *logging.Logger
	quotas     map[string]*types.NamespaceQuota
	quotasMu   sync.RWMutex
	limiters   map[string]*rate.Limiter
	limitersMu sync.RWMutex
}

// Config represents the rate limiting service configuration.
type Config struct {
	Enabled           bool
	RequestsPerSecond int
	BurstSize         int
}

// NewService creates a new rate limiting service.
func NewService(config *Config, logger *logging.Logger) *Service {
	rateLimitConfig := &types.RateLimitConfig{
		RequestsPerSecond: config.RequestsPerSecond,
		BurstSize:         config.BurstSize,
		WindowDuration:    time.Second,
		Enabled:           config.Enabled,
	}

	return &Service{
		config:   rateLimitConfig,
		logger:   logger,
		quotas:   make(map[string]*types.NamespaceQuota),
		limiters: make(map[string]*rate.Limiter),
	}
}

// Middleware returns rate limiting middleware.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		clientID := s.getClientID(r)
		if !s.allowRequest(clientID) {
			s.writeRateLimitResponse(w, r)
			return
		}

		// Check namespace quota if namespace is present
		if namespace := r.Header.Get("X-Namespace"); namespace != "" {
			if !s.checkNamespaceQuota(namespace) {
				http.Error(w, "Namespace quota exceeded", http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// getClientID extracts a unique client identifier from the request.
func (s *Service) getClientID(r *http.Request) string {
	// Try API key first
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "apikey:" + apiKey
	}

	// Fall back to IP address
	return "ip:" + r.RemoteAddr
}

// allowRequest checks if the request is allowed based on rate limiting.
func (s *Service) allowRequest(clientID string) bool {
	s.limitersMu.RLock()
	limiter, exists := s.limiters[clientID]
	s.limitersMu.RUnlock()

	if !exists {
		s.limitersMu.Lock()
		limiter = rate.NewLimiter(rate.Limit(s.config.RequestsPerSecond), s.config.BurstSize)
		s.limiters[clientID] = limiter
		s.limitersMu.Unlock()
	}

	return limiter.Allow()
}

// checkNamespaceQuota checks if the namespace has quota available.
func (s *Service) checkNamespaceQuota(namespace string) bool {
	s.quotasMu.RLock()
	quota, exists := s.quotas[namespace]
	s.quotasMu.RUnlock()

	if !exists {
		return true // No quota set, allow
	}

	return quota.CurrentWorkloads < quota.MaxWorkloads
}

// writeRateLimitResponse writes a rate limit error response.
func (s *Service) writeRateLimitResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Limit", string(rune(s.config.RequestsPerSecond)))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)

	w.Write([]byte(`{"error": "Rate limit exceeded", "code": "RATE_LIMIT_EXCEEDED"}`))
}

// SetNamespaceQuota sets a quota for a namespace.
func (s *Service) SetNamespaceQuota(quota *types.NamespaceQuota) {
	s.quotasMu.Lock()
	defer s.quotasMu.Unlock()
	s.quotas[quota.Namespace] = quota
	s.logger.Info("Set namespace quota", "namespace", quota.Namespace, "maxWorkloads", quota.MaxWorkloads)
}

// GetNamespaceQuota gets the quota for a namespace.
func (s *Service) GetNamespaceQuota(namespace string) (*types.NamespaceQuota, error) {
	s.quotasMu.RLock()
	defer s.quotasMu.RUnlock()

	quota, exists := s.quotas[namespace]
	if !exists {
		return nil, nil
	}
	return quota, nil
}

// UpdateNamespaceUsage updates the current usage for a namespace.
func (s *Service) UpdateNamespaceUsage(namespace string, delta int64) error {
	s.quotasMu.Lock()
	defer s.quotasMu.Unlock()

	quota, exists := s.quotas[namespace]
	if !exists {
		return nil // No quota set
	}

	quota.CurrentWorkloads += delta
	s.quotas[namespace] = quota
	return nil
}

// GetQuotaUsage returns the current quota usage for a namespace.
func (s *Service) GetQuotaUsage(namespace string) *types.NamespaceQuotaUsage {
	s.quotasMu.RLock()
	defer s.quotasMu.RUnlock()

	quota, exists := s.quotas[namespace]
	if !exists {
		return nil
	}

	usage := &types.NamespaceQuotaUsage{
		Namespace:     namespace,
		WorkloadCount: quota.CurrentWorkloads,
		WorkloadLimit: quota.MaxWorkloads,
		CPULimit:      quota.MaxCPU,
		MemoryLimit:   quota.MaxMemory,
		StorageLimit:  quota.MaxStorage,
	}

	if quota.MaxCPU > 0 {
		usage.CPUUsage = float64(quota.CurrentCPU) / float64(quota.MaxCPU) * 100
	}
	if quota.MaxMemory > 0 {
		usage.MemoryUsage = float64(quota.CurrentMemory) / float64(quota.MaxMemory) * 100
	}
	if quota.MaxStorage > 0 {
		usage.StorageUsage = float64(quota.CurrentStorage) / float64(quota.MaxStorage) * 100
	}

	return usage
}

// ListQuotas lists all namespace quotas.
func (s *Service) ListQuotas() []*types.NamespaceQuota {
	s.quotasMu.RLock()
	defer s.quotasMu.RUnlock()

	quotas := make([]*types.NamespaceQuota, 0, len(s.quotas))
	for _, quota := range s.quotas {
		quotas = append(quotas, quota)
	}
	return quotas
}

// DeleteNamespaceQuota deletes a namespace quota.
func (s *Service) DeleteNamespaceQuota(namespace string) {
	s.quotasMu.Lock()
	defer s.quotasMu.Unlock()
	delete(s.quotas, namespace)
	s.logger.Info("Deleted namespace quota", "namespace", namespace)
}
