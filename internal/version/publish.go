package version

import (
	"context"
	"time"

	"github.com/kranix-io/kranix-api/internal/changelognotify"
	"github.com/kranix-io/kranix-api/internal/webhooks"
	"github.com/kranix-io/kranix-packages/types"
)

// PublishRelease records changelog entries and optionally notifies subscribers of breaking changes.
func (m *Manager) PublishRelease(version string, entries []types.ChangelogEntry, notify bool, cn *changelognotify.Service, wh *webhooks.Service) types.ChangelogNotifyResult {
	for i := range entries {
		e := entries[i]
		if e.Version == "" {
			e.Version = version
		}
		if e.ReleasedAt.IsZero() {
			e.ReleasedAt = time.Now().UTC()
		}
		m.AddChangelogEntry(&e)
		entries[i] = e
	}

	m.currentVersion.Version = version
	m.currentVersion.ReleasedAt = time.Now().UTC()

	var result types.ChangelogNotifyResult
	if notify && cn != nil && cn.Enabled() {
		result = cn.NotifyBreakingRelease(context.Background(), version, entries)
	}

	if wh != nil {
		var breaking []types.ChangelogEntry
		for _, e := range entries {
			if e.Breaking {
				breaking = append(breaking, e)
			}
		}
		if len(breaking) > 0 {
			payload := &types.WebhookPayload{
				Event:     types.WebhookEventChangelogBreaking,
				Timestamp: time.Now().UTC(),
				Data: map[string]any{
					"version":         version,
					"breakingChanges": breaking,
				},
			}
			_ = wh.Trigger(context.Background(), types.WebhookEventChangelogBreaking, payload)
		}
	}
	return result
}
