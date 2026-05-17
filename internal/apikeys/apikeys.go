package apikeys

import (
	"sync"

	"github.com/kranix-io/kranix-packages/auth"
)

// Service manages API keys with fine-grained scopes.
type Service struct {
	apiKeys map[string]*auth.APIKey
	mu      sync.RWMutex
}

// NewService creates a new API key service.
func NewService() *Service {
	return &Service{
		apiKeys: make(map[string]*auth.APIKey),
	}
}

// CreateAPIKey creates a new API key with specific permissions.
func (s *Service) CreateAPIKey(name string, permissions []auth.Permission, createdBy string, tenantID string) (*auth.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKey := auth.GenerateScopedAPIKey(name, permissions)
	apiKey.CreatedBy = createdBy
	apiKey.TenantID = tenantID

	s.apiKeys[apiKey.ID] = apiKey
	return apiKey, nil
}

// GetAPIKey retrieves an API key by ID.
func (s *Service) GetAPIKey(id string) (*auth.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKey, ok := s.apiKeys[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return apiKey, nil
}

// GetAPIKeyByKey retrieves an API key by its key value.
func (s *Service) GetAPIKeyByKey(key string) (*auth.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, apiKey := range s.apiKeys {
		if apiKey.Key == key && !apiKey.Revoked {
			return apiKey, nil
		}
	}
	return nil, ErrAPIKeyNotFound
}

// ListAPIKeys lists all API keys.
func (s *Service) ListAPIKeys(tenantID string) []*auth.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*auth.APIKey, 0)
	for _, apiKey := range s.apiKeys {
		if tenantID == "" || apiKey.TenantID == tenantID {
			keys = append(keys, apiKey)
		}
	}
	return keys
}

// RevokeAPIKey revokes an API key.
func (s *Service) RevokeAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKey, ok := s.apiKeys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	apiKey.Revoked = true
	return nil
}

// DeleteAPIKey permanently deletes an API key.
func (s *Service) DeleteAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.apiKeys[id]; !ok {
		return ErrAPIKeyNotFound
	}

	delete(s.apiKeys, id)
	return nil
}

// Errors
var (
	ErrAPIKeyNotFound = &APIKeyError{Message: "API key not found"}
	ErrInvalidScope   = &APIKeyError{Message: "Invalid scope"}
)

// APIKeyError represents an API key error.
type APIKeyError struct {
	Message string
}

func (e *APIKeyError) Error() string {
	return e.Message
}
