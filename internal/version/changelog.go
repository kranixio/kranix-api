package version

import (
	"time"

	"github.com/kranix-io/kranix-packages/types"
)

// InitializeDefaultChangelog initializes the default changelog entries.
func (m *Manager) InitializeDefaultChangelog() {
	// Version 1.0.0 - Initial release
	m.AddChangelogEntry(&types.ChangelogEntry{
		Version:     "v1.0.0",
		ReleasedAt:  time.Now(),
		Type:        types.ChangeTypeAdded,
		Category:    "api",
		Title:       "Initial API Release",
		Description: "Initial release of Kranix API with core workload management capabilities",
		Affects:     []string{"/api/v1/*"},
		Breaking:    false,
		Author:      "Kranix Team",
	})

	// Version 1.0.0 - GraphQL support
	m.AddChangelogEntry(&types.ChangelogEntry{
		Version:     "v1.0.0",
		ReleasedAt:  time.Now(),
		Type:        types.ChangeTypeAdded,
		Category:    "feature",
		Title:       "GraphQL API Support",
		Description: "Added GraphQL endpoint alongside REST API for flexible querying",
		Affects:     []string{"/graphql"},
		Breaking:    false,
		Author:      "Kranix Team",
	})

	// Version 1.0.0 - Webhook system
	m.AddChangelogEntry(&types.ChangelogEntry{
		Version:     "v1.0.0",
		ReleasedAt:  time.Now(),
		Type:        types.ChangeTypeAdded,
		Category:    "feature",
		Title:       "Webhook System",
		Description: "Added webhook system for push notifications to Slack, CI, PagerDuty",
		Affects:     []string{"/api/v1/webhooks"},
		Breaking:    false,
		Author:      "Kranix Team",
	})

	// Version 1.0.0 - API key scopes
	m.AddChangelogEntry(&types.ChangelogEntry{
		Version:     "v1.0.0",
		ReleasedAt:  time.Now(),
		Type:        types.ChangeTypeAdded,
		Category:    "security",
		Title:       "Fine-grained API Key Scopes",
		Description: "Added fine-grained API key scopes (read/write/admin) per resource type",
		Affects:     []string{"/api/v1/apikeys"},
		Breaking:    false,
		Author:      "Kranix Team",
	})

	// Version 1.0.0 - Analytics
	m.AddChangelogEntry(&types.ChangelogEntry{
		Version:     "v1.0.0",
		ReleasedAt:  time.Now(),
		Type:        types.ChangeTypeAdded,
		Category:    "feature",
		Title:       "Usage Analytics API",
		Description: "Added usage analytics API for deploy counts, error rates, and latency metrics",
		Affects:     []string{"/api/v1/analytics"},
		Breaking:    false,
		Author:      "Kranix Team",
	})

	// Version 1.0.0 - OIDC/SSO
	m.AddChangelogEntry(&types.ChangelogEntry{
		Version:     "v1.0.0",
		ReleasedAt:  time.Now(),
		Type:        types.ChangeTypeAdded,
		Category:    "security",
		Title:       "OIDC/SSO Login Support",
		Description: "Added OIDC/SSO login support for Google, GitHub, and Okta",
		Affects:     []string{"/auth/*"},
		Breaking:    false,
		Author:      "Kranix Team",
	})
}
