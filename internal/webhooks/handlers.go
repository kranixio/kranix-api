package webhooks

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

// RegisterRoutes registers webhook HTTP handlers.
func RegisterRoutes(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("POST /api/v1/webhooks", handleCreateWebhook(service))
	mux.HandleFunc("GET /api/v1/webhooks", handleListWebhooks(service))
	mux.HandleFunc("GET /api/v1/webhooks/", handleGetWebhook(service))
	mux.HandleFunc("PATCH /api/v1/webhooks/", handleUpdateWebhook(service))
	mux.HandleFunc("DELETE /api/v1/webhooks/", handleDeleteWebhook(service))
	mux.HandleFunc("GET /api/v1/webhooks/", handleListDeliveries(service))
	mux.HandleFunc("POST /api/v1/webhooks/", handleTestWebhook(service))
}

// handleCreateWebhook handles webhook creation.
func handleCreateWebhook(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var webhook types.Webhook
		if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := service.RegisterWebhook(&webhook); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(webhook)
	}
}

// handleListWebhooks handles listing webhooks.
func handleListWebhooks(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		webhooks := service.ListWebhooks()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"webhooks": webhooks,
		})
	}
}

// handleGetWebhook handles getting a single webhook.
func handleGetWebhook(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path)
		if id == "" {
			http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
			return
		}

		webhook, err := service.GetWebhook(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(webhook)
	}
}

// handleUpdateWebhook handles updating a webhook.
func handleUpdateWebhook(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path)
		if id == "" {
			http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
			return
		}

		var webhook types.Webhook
		if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Unregister old and register new
		service.UnregisterWebhook(id)
		webhook.ID = id
		if err := service.RegisterWebhook(&webhook); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(webhook)
	}
}

// handleDeleteWebhook handles deleting a webhook.
func handleDeleteWebhook(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path)
		if id == "" {
			http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
			return
		}

		if err := service.UnregisterWebhook(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// handleListDeliveries handles listing webhook deliveries.
func handleListDeliveries(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path)
		if id == "" {
			http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
			return
		}

		deliveries := service.ListDeliveries(id)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deliveries": deliveries,
		})
	}
}

// handleTestWebhook handles testing a webhook.
func handleTestWebhook(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractID(r.URL.Path)
		if id == "" {
			http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
			return
		}

		webhook, err := service.GetWebhook(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Create a test payload
		payload := &types.WebhookPayload{
			Event:      types.WebhookEventDeploySuccess,
			Timestamp:  webhook.CreatedAt,
			WorkloadID: "test-workload",
			Namespace:  "default",
			Data: map[string]interface{}{
				"test": true,
			},
		}

		if err := service.Trigger(r.Context(), types.WebhookEventDeploySuccess, payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Test webhook triggered",
		})
	}
}

// extractID extracts an ID from a URL path.
func extractID(path string) string {
	// Simple implementation - extract the last segment
	parts := splitPath(path)
	if len(parts) > 0 {
		return parts[len(parts)-1]
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
