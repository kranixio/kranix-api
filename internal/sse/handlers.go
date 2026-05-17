package sse

import (
	"encoding/json"
	"net/http"
)

// RegisterRoutes registers SSE routes.
func RegisterRoutes(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("GET /api/sse", service.HandleConnection)
	mux.HandleFunc("GET /api/sse/stats", handleStats(service))
	mux.HandleFunc("POST /api/sse/broadcast", handleBroadcast(service))
}

// handleStats handles SSE statistics.
func handleStats(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"connectedClients": service.GetConnectedClients(),
		})
	}
}

// handleBroadcast handles manual broadcast of events (for testing).
func handleBroadcast(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var message struct {
			Event  string                 `json:"event"`
			Data   interface{}            `json:"data"`
			Filter map[string]string      `json:"filter"`
		}

		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		service.Broadcast(message.Event, message.Data, message.Filter)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Event broadcasted",
		})
	}
}
