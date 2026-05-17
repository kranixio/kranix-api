package oidc

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kranix-io/kranix-packages/auth"
)

// RegisterRoutes registers OIDC HTTP handlers.
func RegisterRoutes(mux *http.ServeMux, manager *Manager) {
	mux.HandleFunc("GET /auth/login", handleLogin(manager))
	mux.HandleFunc("GET /auth/callback", handleCallback(manager))
	mux.HandleFunc("POST /auth/logout", handleLogout(manager))
	mux.HandleFunc("GET /auth/providers", handleListProviders(manager))
}

// handleLogin handles the login page.
func handleLogin(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providers := make([]string, 0)
		for name, provider := range manager.oidcManager.GetConfig().Providers {
			if provider.Enabled {
				providers = append(providers, name)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"providers": providers,
			"message":   "Choose a provider to login",
		})
	}
}

// handleCallback handles the OAuth callback.
func handleCallback(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		manager.HandleCallback(w, r)
	}
}

// handleLogout handles logout.
func handleLogout(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err == nil {
			manager.oidcManager.DeleteSession(cookie.Value)
		}

		// Clear session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   true,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Logged out successfully",
		})
	}
}

// handleListProviders lists available OIDC providers.
func handleListProviders(manager *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providers := make(map[string]*auth.OIDCProvider)
		for name, provider := range manager.oidcManager.GetConfig().Providers {
			// Don't include sensitive information
			providers[name] = &auth.OIDCProvider{
				Name:        provider.Name,
				DisplayName: provider.DisplayName,
				Enabled:     provider.Enabled,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"providers": providers,
		})
	}
}
