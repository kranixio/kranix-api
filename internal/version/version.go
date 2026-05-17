package version

import (
	"net/http"
	"time"

	"github.com/kranix-io/kranix-packages/types"
)

const (
	// CurrentAPIVersion is the current API version
	CurrentAPIVersion = "v1.0.0"
)

// Manager manages version information and deprecation notices.
type Manager struct {
	currentVersion *types.APIVersion
	changelog      []*types.ChangelogEntry
}

// NewManager creates a new version manager.
func NewManager() *Manager {
	currentVersion := &types.APIVersion{
		Version:    CurrentAPIVersion,
		Major:      1,
		Minor:      0,
		Patch:      0,
		ReleasedAt: time.Now(),
		Deprecated: false,
		Supported:  true,
	}

	return &Manager{
		currentVersion: currentVersion,
		changelog:      make([]*types.ChangelogEntry, 0),
	}
}

// AddChangelogEntry adds a changelog entry.
func (m *Manager) AddChangelogEntry(entry *types.ChangelogEntry) {
	entry.ID = generateID()
	m.changelog = append(m.changelog, entry)
}

// GetChangelog retrieves the changelog.
func (m *Manager) GetChangelog(limit int) []*types.ChangelogEntry {
	if limit > 0 && limit < len(m.changelog) {
		return m.changelog[:limit]
	}
	return m.changelog
}

// GetVersionInfo returns the current version information.
func (m *Manager) GetVersionInfo() *types.APIVersion {
	return m.currentVersion
}

// Middleware adds version headers and deprecation notices to responses.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add API version header
		w.Header().Set("X-API-Version", m.currentVersion.Version)
		
		// Add deprecation warning if applicable
		if m.currentVersion.Deprecated && m.currentVersion.DeprecationInfo != nil {
			w.Header().Set("X-Deprecation-Warning", m.currentVersion.DeprecationInfo.Message)
			w.Header().Set("X-Sunset-Date", m.currentVersion.DeprecationInfo.SunsetDate.Format(time.RFC3339))
		}

		// Add request ID header for tracing
		requestID := generateRequestID()
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}

// GetResponseMetadata returns response metadata.
func (m *Manager) GetResponseMetadata(requestID string) *types.APIResponseMetadata {
	metadata := &types.APIResponseMetadata{
		APIVersion: m.currentVersion.Version,
		RequestID:  requestID,
		Timestamp:  time.Now(),
	}

	if m.currentVersion.Deprecated {
		metadata.DeprecationNotice = m.currentVersion.DeprecationInfo
	}

	return metadata
}

// generateID generates a unique ID.
func generateID() string {
	return time.Now().Format("20060102-150405")
}

// generateRequestID generates a unique request ID.
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
