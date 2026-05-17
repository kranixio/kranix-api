package oidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/kranix-io/kranix-packages/auth"
	"github.com/kranix-io/kranix-packages/logging"
)

// Manager manages OIDC authentication.
type Manager struct {
	oidcManager *auth.OIDCManager
	logger      *logging.Logger
}

// NewManager creates a new OIDC manager.
func NewManager(config *auth.OIDCConfig, logger *logging.Logger) *Manager {
	return &Manager{
		oidcManager: auth.NewOIDCManager(config),
		logger:      logger,
	}
}

// GetAuthURL generates the authorization URL for a provider.
func (m *Manager) GetAuthURL(providerName string) (string, string, error) {
	state := generateState()
	authURL, err := m.oidcManager.GetAuthURL(providerName, state)
	if err != nil {
		return "", "", err
	}
	return authURL, state, nil
}

// HandleCallback handles the OAuth callback.
func (m *Manager) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	provider := r.URL.Query().Get("provider")

	if code == "" || state == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := m.oidcManager.ExchangeCodeForToken(provider, code)
	if err != nil {
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}

	// Get user info
	userInfo, err := m.oidcManager.GetUserInfo(provider, token.AccessToken)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Create session
	session, err := m.oidcManager.CreateSession(userInfo, token)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   true,
	})

	// Redirect to dashboard
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// Middleware handles OIDC session validation.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip OIDC for public endpoints
		if isPublicEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Check for session cookie
		cookie, err := r.Cookie("session_id")
		if err != nil {
			// Redirect to login
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		// Validate session
		session, err := m.oidcManager.GetSession(cookie.Value)
		if err != nil {
			// Redirect to login
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		// Add user info to context
		ctx := contextWithSession(r.Context(), session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isPublicEndpoint checks if the endpoint is public (doesn't require auth).
func isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/health",
		"/auth/login",
		"/auth/callback",
		"/docs",
		"/openapi.json",
	}

	for _, publicPath := range publicPaths {
		if path == publicPath {
			return true
		}
	}
	return false
}

// generateState generates a random state parameter.
func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// contextKey is the key for storing session in context.
type contextKey string

const sessionKey contextKey = "session"

// contextWithSession adds session to context.
func contextWithSession(ctx context.Context, session *auth.Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// SessionFromContext extracts session from context.
func SessionFromContext(ctx context.Context) (*auth.Session, bool) {
	session, ok := ctx.Value(sessionKey).(*auth.Session)
	return session, ok
}
