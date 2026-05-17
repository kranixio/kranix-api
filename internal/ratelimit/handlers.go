package ratelimit

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/types"
)

// RegisterRoutes registers rate limiting and quota routes.
func RegisterRoutes(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("POST /api/quota", handleSetQuota(service))
	mux.HandleFunc("GET /api/quota/{namespace}", handleGetQuota(service))
	mux.HandleFunc("GET /api/quota/{namespace}/usage", handleGetQuotaUsage(service))
	mux.HandleFunc("GET /api/quota", handleListQuotas(service))
	mux.HandleFunc("DELETE /api/quota/{namespace}", handleDeleteQuota(service))
}

// handleSetQuota handles setting a namespace quota.
func handleSetQuota(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var quota types.NamespaceQuota
		if err := json.NewDecoder(r.Body).Decode(&quota); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		service.SetNamespaceQuota(&quota)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Quota set successfully",
		})
	}
}

// handleGetQuota handles getting a namespace quota.
func handleGetQuota(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		quota, err := service.GetNamespaceQuota(namespace)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if quota == nil {
			http.Error(w, "Quota not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(quota)
	}
}

// handleGetQuotaUsage handles getting quota usage for a namespace.
func handleGetQuotaUsage(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		usage := service.GetQuotaUsage(namespace)
		if usage == nil {
			http.Error(w, "Quota not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(usage)
	}
}

// handleListQuotas handles listing all quotas.
func handleListQuotas(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		quotas := service.ListQuotas()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"quotas": quotas,
			"count":  len(quotas),
		})
	}
}

// handleDeleteQuota handles deleting a namespace quota.
func handleDeleteQuota(service *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		service.DeleteNamespaceQuota(namespace)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Quota deleted successfully",
		})
	}
}
