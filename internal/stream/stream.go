package stream

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kranix-io/kranix-packages/types"
)

// RegisterRoutes registers streaming routes.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/pods/", handleStreamLogs)
	mux.HandleFunc("GET /api/v1/pods/", handleExecPod)
}

// handleStreamLogs handles SSE streaming of pod logs.
func handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Extract pod ID from URL path
	podID := extractPodID(r.URL.Path)
	if podID == "" {
		sendSSEError(w, "Invalid pod ID")
		return
	}

	// Parse log options (unused for now, will be used when connecting to kranix-core)
	_ = &types.LogOptions{
		Follow: r.URL.Query().Get("follow") == "true",
		Tail:   parseInt64(r.URL.Query().Get("tail"), 100),
	}

	// TODO: Connect to kranix-core to stream logs
	// For now, send placeholder events
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send a heartbeat
			sendSSEEvent(w, "heartbeat", "ping")
		}
	}
}

// handleExecPod handles WebSocket execution into a pod.
func handleExecPod(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement WebSocket upgrade
	// For now, return not implemented
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	fmt.Fprintf(w, `{"error": "WebSocket exec not yet implemented"}`)
}

// sendSSEEvent sends an SSE event to the client.
func sendSSEEvent(w http.ResponseWriter, eventType, data string) {
	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// sendSSEError sends an SSE error event to the client.
func sendSSEError(w http.ResponseWriter, message string) {
	sendSSEEvent(w, "error", message)
}

// extractPodID extracts the pod ID from the URL path.
func extractPodID(path string) string {
	// Simple implementation - in production, use a proper router
	// URL pattern: /api/v1/pods/{id}/logs or /api/v1/pods/{id}/exec
	parts := splitPath(path)
	for i, part := range parts {
		if part == "pods" && i+1 < len(parts) {
			return parts[i+1]
		}
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

// parseInt64 parses a string to int64 with a default value.
func parseInt64(s string, defaultValue int64) int64 {
	var result int64
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}
