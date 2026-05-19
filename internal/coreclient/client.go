package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kranix-io/kranix-packages/types"
)

// Client calls the kranix-core HTTP API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a core HTTP client. baseURL is e.g. http://localhost:8081
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Enabled reports whether a core base URL is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var errBody map[string]string
		_ = json.Unmarshal(data, &errBody)
		if msg := errBody["error"]; msg != "" {
			return fmt.Errorf("core api %s %s: %s", method, path, msg)
		}
		return fmt.Errorf("core api %s %s: status %d", method, path, resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// BulkWorkloads runs a bulk deploy/restart/delete against core.
func (c *Client) BulkWorkloads(ctx context.Context, req types.BulkWorkloadRequest) (*types.BulkWorkloadResponse, error) {
	var resp types.BulkWorkloadResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/workloads/bulk", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RestartWorkload triggers a rolling restart in core.
func (c *Client) RestartWorkload(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/api/v1/workloads/"+id+"/restart", nil, nil)
}

// DeleteWorkload removes a workload from core.
func (c *Client) DeleteWorkload(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/workloads/"+id, nil, nil)
}

// DeployWorkload creates or updates a workload in core.
func (c *Client) DeployWorkload(ctx context.Context, id string, spec types.WorkloadSpec) error {
	body := map[string]interface{}{"id": id, "spec": spec}
	return c.do(ctx, http.MethodPost, "/api/v1/workloads", body, nil)
}

// GetWorkloadEvents returns domain events for a workload from core event store.
func (c *Client) GetWorkloadEvents(ctx context.Context, id string, fromVersion int64, limit int) (interface{}, error) {
	path := fmt.Sprintf("/api/v1/workloads/%s/events?from_version=%d&limit=%d", id, fromVersion, limit)
	var out interface{}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAuditResource returns audit/history events for a resource from core.
func (c *Client) GetAuditResource(ctx context.Context, resourceType, resourceID string, limit int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/audit/resources/%s/%s?limit=%d", resourceType, resourceID, limit)
	var out map[string]interface{}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// NotifySecretRotated informs core that a secret version changed.
func (c *Client) NotifySecretRotated(ctx context.Context, namespace, name, version string) ([]string, error) {
	body := map[string]string{
		"namespace": namespace,
		"name":      name,
		"version":   version,
	}
	var out map[string]interface{}
	if err := c.do(ctx, http.MethodPost, "/api/v1/secrets/rotated", body, &out); err != nil {
		return nil, err
	}
	raw, _ := out["affected_workloads"].([]interface{})
	var ids []string
	for _, v := range raw {
		if s, ok := v.(string); ok {
			ids = append(ids, s)
		}
	}
	return ids, nil
}
