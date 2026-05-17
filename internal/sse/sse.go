package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kranix-io/kranix-packages/logging"
	"github.com/kranix-io/kranix-packages/types"
)

// Service manages SSE connections and event broadcasting.
type Service struct {
	clients    map[string]*Client
	clientsMu  sync.RWMutex
	logger     *logging.Logger
	broadcast  chan *types.BroadcastMessage
	register   chan *Client
	unregister chan *Client
}

// Client represents an SSE client connection.
type Client struct {
	ID         string
	ClientID   string
	Send       chan *types.SSEEvent
	Subscriptions map[string]bool
	mu         sync.RWMutex
}

// NewService creates a new SSE service.
func NewService(logger *logging.Logger) *Service {
	s := &Service{
		clients:    make(map[string]*Client),
		logger:     logger,
		broadcast:  make(chan *types.BroadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	go s.run()
	return s
}

// run runs the SSE service event loop.
func (s *Service) run() {
	for {
		select {
		case client := <-s.register:
			s.clientsMu.Lock()
			s.clients[client.ID] = client
			s.clientsMu.Unlock()
			s.logger.Info("SSE client registered", "clientID", client.ID)

		case client := <-s.unregister:
			s.clientsMu.Lock()
			if _, ok := s.clients[client.ID]; ok {
				delete(s.clients, client.ID)
				close(client.Send)
			}
			s.clientsMu.Unlock()
			s.logger.Info("SSE client unregistered", "clientID", client.ID)

		case message := <-s.broadcast:
			s.broadcastMessage(message)
		}
	}
}

// HandleConnection handles new SSE connections.
func (s *Service) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get client ID from query param or generate one
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Get namespaces to subscribe to
	namespaces := r.URL.Query()["namespace"]
	if len(namespaces) == 0 {
		namespaces = []string{"*"} // Subscribe to all
	}

	// Create client
	client := &Client{
		ID:         fmt.Sprintf("%s-%d", clientID, time.Now().UnixNano()),
		ClientID:   clientID,
		Send:       make(chan *types.SSEEvent, 256),
		Subscriptions: make(map[string]bool),
	}

	for _, ns := range namespaces {
		client.Subscriptions[ns] = true
	}

	// Register client
	s.register <- client

	// Send initial connection event
	client.Send <- &types.SSEEvent{
		ID:        "connection",
		Event:     "connected",
		Data:      map[string]string{"clientID": client.ID},
		Timestamp: time.Now(),
	}

	// Handle client disconnect
	ctx := r.Context()
	notify := r.Context().Done()

	go func() {
		<-notify
		s.unregister <- client
	}()

	// Stream events to client
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-client.Send:
			if err := s.writeSSEEvent(w, event); err != nil {
				s.logger.Error("Failed to write SSE event", "error", err)
				return
			}
			flusher.Flush()
		}
	}
}

// writeSSEEvent writes an SSE event to the response writer.
func (s *Service) writeSSEEvent(w http.ResponseWriter, event *types.SSEEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	if event.ID != "" {
		fmt.Fprintf(w, "id: %s\n", event.ID)
	}
	if event.Event != "" {
		fmt.Fprintf(w, "event: %s\n", event.Event)
	}
	if event.Retry > 0 {
		fmt.Fprintf(w, "retry: %d\n", event.Retry)
	}
	fmt.Fprintf(w, "data: %s\n\n", data)

	return nil
}

// Broadcast broadcasts a message to all connected clients.
func (s *Service) Broadcast(event string, data interface{}, filter map[string]string) {
	s.broadcast <- &types.BroadcastMessage{
		Event:  event,
		Data:   data,
		Filter: filter,
	}
}

// broadcastMessage broadcasts a message to matching clients.
func (s *Service) broadcastMessage(message *types.BroadcastMessage) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	event := &types.SSEEvent{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Event:     message.Event,
		Data:      message.Data,
		Timestamp: time.Now(),
		Retry:     3000, // 3 seconds retry
	}

	for _, client := range s.clients {
		if s.shouldSendToClient(client, message.Filter) {
			select {
			case client.Send <- event:
			default:
				// Client channel full, skip
				s.logger.Warn("SSE client channel full, skipping", "clientID", client.ID)
			}
		}
	}
}

// shouldSendToClient checks if a message should be sent to a client based on filters.
func (s *Service) shouldSendToClient(client *Client, filter map[string]string) bool {
	if filter == nil {
		return true
	}

	// Check namespace filter
	if namespace, ok := filter["namespace"]; ok {
		if !client.Subscriptions["*"] && !client.Subscriptions[namespace] {
			return false
		}
	}

	return true
}

// BroadcastWorkloadChange broadcasts a workload state change event.
func (s *Service) BroadcastWorkloadChange(change *types.WorkloadStateChange) {
	s.Broadcast("workload.changed", change, map[string]string{
		"namespace": change.Namespace,
	})
}

// GetConnectedClients returns the number of connected clients.
func (s *Service) GetConnectedClients() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}

// Shutdown gracefully shuts down the SSE service.
func (s *Service) Shutdown(ctx context.Context) {
	s.logger.Info("Shutting down SSE service")

	s.clientsMu.Lock()
	for _, client := range s.clients {
		close(client.Send)
	}
	s.clients = make(map[string]*Client)
	s.clientsMu.Unlock()

	close(s.broadcast)
	close(s.register)
	close(s.unregister)
}
