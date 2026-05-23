package approval

import (
	"fmt"
	"sync"
	"time"

	"github.com/kranix-io/kranix-packages/types"
)

// Store holds pending and resolved approval gates in memory.
type Store struct {
	mu         sync.RWMutex
	records    map[string]*types.ApprovalRecord
	defaultTTL time.Duration
}

func NewStore(defaultTTL time.Duration) *Store {
	if defaultTTL <= 0 {
		defaultTTL = 10 * time.Minute
	}
	return &Store{
		records:    make(map[string]*types.ApprovalRecord),
		defaultTTL: defaultTTL,
	}
}

func (s *Store) Create(req types.ApprovalCreateRequest) *types.ApprovalRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	ttl := s.defaultTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("appr-%d", now.UnixNano())
	rec := &types.ApprovalRecord{
		ID:        id,
		Status:    types.ApprovalPending,
		Tool:      req.Tool,
		Action:    req.Action,
		Resource:  req.Resource,
		Namespace: req.Namespace,
		Reason:    req.Reason,
		Inputs:    copyMap(req.Inputs),
		AgentID:   req.AgentID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	s.records[id] = rec
	return rec
}

func (s *Store) Get(id string) (*types.ApprovalRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return nil, false
	}
	if rec.Status == types.ApprovalPending && time.Now().UTC().After(rec.ExpiresAt) {
		rec.Status = types.ApprovalExpired
	}
	cp := *rec
	return &cp, true
}

func (s *Store) ListPending(agentID string) []types.ApprovalRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]types.ApprovalRecord, 0)
	now := time.Now().UTC()
	for _, rec := range s.records {
		if rec.Status != types.ApprovalPending {
			continue
		}
		if now.After(rec.ExpiresAt) {
			rec.Status = types.ApprovalExpired
			continue
		}
		if agentID != "" && rec.AgentID != agentID {
			continue
		}
		cp := *rec
		out = append(out, cp)
	}
	return out
}

func (s *Store) Resolve(id string, req types.ApprovalResolveRequest) (*types.ApprovalRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return nil, fmt.Errorf("approval not found: %s", id)
	}
	if rec.Status != types.ApprovalPending {
		return nil, fmt.Errorf("approval is not pending: %s", rec.Status)
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		rec.Status = types.ApprovalExpired
		return nil, fmt.Errorf("approval expired")
	}
	now := time.Now().UTC()
	rec.ResolvedAt = &now
	rec.ResolvedBy = req.ResolvedBy
	rec.Comment = req.Comment
	if req.Approved {
		rec.Status = types.ApprovalApproved
	} else {
		rec.Status = types.ApprovalDenied
	}
	cp := *rec
	return &cp, nil
}

func (s *Store) ValidateForExecution(id, tool, agentID string) (*types.ApprovalRecord, error) {
	rec, ok := s.Get(id)
	if !ok {
		return nil, fmt.Errorf("approval not found: %s", id)
	}
	if rec.Status != types.ApprovalApproved {
		return nil, fmt.Errorf("approval not approved (status: %s)", rec.Status)
	}
	if rec.Tool != tool {
		return nil, fmt.Errorf("approval tool mismatch: expected %q, got %q", rec.Tool, tool)
	}
	if agentID != "" && rec.AgentID != agentID {
		return nil, fmt.Errorf("approval agent mismatch")
	}
	return rec, nil
}

func copyMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
