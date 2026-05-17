package apiversion

import (
	"context"
	"net/http"
	"strings"

	"github.com/kranix-io/kranix-packages/logging"
	"github.com/kranix-io/kranix-packages/types"
)

// Manager manages API versioning.
type Manager struct {
	config *types.APIVersionConfig
	logger *logging.Logger
}

// NewManager creates a new API version manager.
func NewManager(config *types.APIVersionConfig, logger *logging.Logger) *Manager {
	return &Manager{
		config: config,
		logger: logger,
	}
}

// Middleware handles API versioning.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.EnableVersioning {
			next.ServeHTTP(w, r)
			return
		}

		// Extract version from header or query param
		version := m.extractVersion(r)
		if version == "" {
			version = m.config.DefaultVersion
		}

		// Add version to context
		ctx := contextWithVersion(r.Context(), version)

		// Add version headers
		w.Header().Set("X-API-Version", version)
		w.Header().Set("X-Default-Version", m.config.DefaultVersion)

		// Check if version is deprecated
		for _, v := range m.config.Versions {
			if v.Version == version && v.Deprecated {
				w.Header().Set("X-API-Deprecated", "true")
				w.Header().Set("X-API-Sunset-Date", v.SunsetDate.Format("2006-01-02"))
				w.Header().Set("Warning", `299 - "API version is deprecated"`)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractVersion extracts the API version from the request.
func (m *Manager) extractVersion(r *http.Request) string {
	// Check header first
	if version := r.Header.Get(m.config.HeaderName); version != "" {
		return strings.TrimPrefix(version, "v")
	}

	// Check query param
	if version := r.URL.Query().Get(m.config.QueryParam); version != "" {
		return strings.TrimPrefix(version, "v")
	}

	// Check URL path for /api/v1/ pattern
	pathParts := strings.Split(r.URL.Path, "/")
	for i, part := range pathParts {
		if part == "api" && i+1 < len(pathParts) {
			versionPart := strings.TrimPrefix(pathParts[i+1], "v")
			// Validate it's a version number
			if versionPart == "1" || versionPart == "2" {
				return versionPart
			}
		}
	}

	return ""
}

// VersionFromContext extracts the version from context.
func VersionFromContext(ctx context.Context) string {
	if version, ok := ctx.Value(versionKey).(string); ok {
		return version
	}
	return ""
}

// contextKey is the key for storing version in context.
type contextKey string

const versionKey contextKey = "apiVersion"

// contextWithVersion adds version to context.
func contextWithVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, versionKey, version)
}

// GetVersionInfo returns information about a specific version.
func (m *Manager) GetVersionInfo(version string) (*types.APIRouteVersion, error) {
	for _, v := range m.config.Versions {
		if v.Version == version {
			return &v, nil
		}
	}
	return nil, nil
}

// ListVersions returns all available API versions.
func (m *Manager) ListVersions() []types.APIRouteVersion {
	return m.config.Versions
}

// IsDeprecated checks if a version is deprecated.
func (m *Manager) IsDeprecated(version string) bool {
	for _, v := range m.config.Versions {
		if v.Version == version {
			return v.Deprecated
		}
	}
	return false
}
