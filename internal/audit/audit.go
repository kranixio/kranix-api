package audit

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kranix-io/kranix-packages/types"
)

// Logger records API actions for the audit trail.
type Logger struct {
	enabled bool
	sink    string
	mu      sync.RWMutex
	entries []types.AuditEntry
	max     int
	file    *os.File
}

// Config configures the audit logger.
type Config struct {
	Enabled bool
	Sink    string
	Max     int
}

// New creates an audit logger.
func New(cfg Config) *Logger {
	if cfg.Max <= 0 {
		cfg.Max = 5000
	}
	l := &Logger{
		enabled: cfg.Enabled,
		sink:    cfg.Sink,
		max:     cfg.Max,
		entries: make([]types.AuditEntry, 0, cfg.Max),
	}
	if cfg.Enabled && cfg.Sink == "file" {
		f, err := os.OpenFile("kranix-api-audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("audit file open failed: %v", err)
		} else {
			l.file = f
		}
	}
	return l
}

// Log records an audit entry.
func (l *Logger) Log(entry types.AuditEntry) {
	if !l.enabled {
		return
	}
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}
	l.mu.Unlock()

	if l.sink == "stdout" || l.sink == "" {
		b, _ := json.Marshal(entry)
		log.Printf("audit: %s", string(b))
	}
	if l.file != nil {
		b, _ := json.Marshal(entry)
		_, _ = l.file.Write(append(b, '\n'))
	}
}

// Query returns audit entries matching the filter (newest first).
func (l *Logger) Query(q types.AuditQuery) []types.AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	var out []types.AuditEntry
	for i := len(l.entries) - 1; i >= 0 && len(out) < limit; i-- {
		e := l.entries[i]
		if q.ResourceType != "" && e.ResourceType != q.ResourceType {
			continue
		}
		if q.ResourceID != "" && e.ResourceID != q.ResourceID {
			continue
		}
		if q.Action != "" && e.Action != q.Action {
			continue
		}
		if q.Actor != "" && e.Actor != q.Actor {
			continue
		}
		if !q.Since.IsZero() && e.Timestamp.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && e.Timestamp.After(q.Until) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Get returns a single audit entry by ID.
func (l *Logger) Get(id string) (*types.AuditEntry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for i := range l.entries {
		if l.entries[i].ID == id {
			e := l.entries[i]
			return &e, true
		}
	}
	return nil, false
}
