package apikeys

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/auth"
)

// RegisterRoutes registers API key HTTP handlers.
func RegisterRoutes(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("POST /api/v1/apikeys", handleCreateAPIKey(service))
	mux.HandleFunc("GET /api/v1/apikeys", handleListAPIKeys(service))
	mux.HandleFunc("GET /api/v1/apikeys/", handleGetAPIKey(service))
	mux.HandleFunc("DELETE /api/v1/apikeys/", handleRevokeAPIKey(service))
}

// handleCreateAPIKey handles API key creation.
func handleCreateAPIKey(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		if len(req.Permissions) == 0 {
			http.Error(w, "At least one permission is required", http.StatusBadRequest)
			return
		}

		// Validate permissions
		for _, perm := range req.Permissions {
			if !isValidResourceType(perm.ResourceType) {
				http.Error(w, "Invalid resource type", http.StatusBadRequest)
				return
			}
			if !isValidScope(perm.Scope) {
				http.Error(w, "Invalid scope", http.StatusBadRequest)
				return
			}
		}

		// Extract user info from context (set by auth middleware)
		createdBy := "system" // TODO: Get from context
		tenantID := "default" // TODO: Get from context

		apiKey, err := service.CreateAPIKey(req.Name, req.Permissions, createdBy, tenantID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(apiKey)
	}
}

// handleListAPIKeys handles listing API keys.
func handleListAPIKeys(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant_id")
		apiKeys := service.ListAPIKeys(tenantID)

		// Don't return the actual key values in list
		sanitizedKeys := make([]*auth.APIKey, 0, len(apiKeys))
		for _, key := range apiKeys {
			sanitizedKey := &auth.APIKey{
				ID:          key.ID,
				Name:        key.Name,
				Key:         "****" + key.Key[len(key.Key)-4:],
				Permissions: key.Permissions,
				ExpiresAt:   key.ExpiresAt,
				CreatedAt:   key.CreatedAt,
				CreatedBy:   key.CreatedBy,
				TenantID:    key.TenantID,
				Revoked:     key.Revoked,
			}
			sanitizedKeys = append(sanitizedKeys, sanitizedKey)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"apikeys": sanitizedKeys,
		})
	}
}

// handleGetAPIKey handles getting a single API key.
func handleGetAPIKey(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path)
		if id == "" {
			http.Error(w, "Invalid API key ID", http.StatusBadRequest)
			return
		}

		apiKey, err := service.GetAPIKey(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Don't return the actual key value
		sanitizedKey := &auth.APIKey{
			ID:          apiKey.ID,
			Name:        apiKey.Name,
			Key:         "****" + apiKey.Key[len(apiKey.Key)-4:],
			Permissions: apiKey.Permissions,
			ExpiresAt:   apiKey.ExpiresAt,
			CreatedAt:   apiKey.CreatedAt,
			CreatedBy:   apiKey.CreatedBy,
			TenantID:    apiKey.TenantID,
			Revoked:     apiKey.Revoked,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sanitizedKey)
	}
}

// handleRevokeAPIKey handles revoking an API key.
func handleRevokeAPIKey(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path)
		if id == "" {
			http.Error(w, "Invalid API key ID", http.StatusBadRequest)
			return
		}

		if err := service.RevokeAPIKey(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// CreateAPIKeyRequest represents a request to create an API key.
type CreateAPIKeyRequest struct {
	Name        string                 `json:"name"`
	Permissions []auth.Permission      `json:"permissions"`
	ExpiresAt   string                 `json:"expiresAt,omitempty"` // ISO 8601 format
}

// isValidResourceType checks if a resource type is valid.
func isValidResourceType(rt auth.ResourceType) bool {
	switch rt {
	case auth.ResourceWorkload,
		auth.ResourceNamespace,
		auth.ResourcePod,
		auth.ResourceEvent,
		auth.ResourceWebhook,
		auth.ResourceTenant,
		auth.ResourceQuota:
		return true
	default:
		return false
	}
}

// isValidScope checks if a scope is valid.
func isValidScope(scope auth.Scope) bool {
	switch scope {
	case auth.ScopeRead, auth.ScopeWrite, auth.ScopeAdmin:
		return true
	default:
		return false
	}
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
