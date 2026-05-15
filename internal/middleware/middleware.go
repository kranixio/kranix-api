package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/kranix-io/kranix-packages/auth"
	"github.com/kranix-io/kranix-packages/logging"
)

// Chain creates a middleware chain.
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Logging adds structured logging to requests.
func Logging(level, format string) func(http.Handler) http.Handler {
	logger := logging.NewWithLevel("kranix-api", parseLogLevel(level))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			logger.Info("incoming request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			next.ServeHTTP(w, r)

			duration := time.Since(start)
			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}

// CORS adds CORS headers to responses.
func CORS() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Auth validates authentication tokens.
func Auth(mode, jwtSecret, oidcIssuer string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health checks and docs
			if r.URL.Path == "/health" || r.URL.Path == "/docs" || r.URL.Path == "/openapi.json" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			// Validate based on mode
			var token *auth.Token
			switch mode {
			case "apikey":
				if !strings.HasPrefix(tokenStr, "krane_") {
					http.Error(w, "Invalid API key format", http.StatusUnauthorized)
					return
				}
				token = &auth.Token{
					Type:  auth.TokenTypeAPIKey,
					Value: tokenStr,
				}
			case "jwt":
				// TODO: Implement JWT validation
				token = &auth.Token{
					Type:  auth.TokenTypeJWT,
					Value: tokenStr,
				}
			case "oidc":
				// TODO: Implement OIDC validation
				token = &auth.Token{
					Type:  auth.TokenTypeJWT,
					Value: tokenStr,
				}
			default:
				http.Error(w, "Invalid auth mode", http.StatusInternalServerError)
				return
			}

			if err := auth.ValidateToken(token); err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit implements rate limiting.
func RateLimit(requestsPerSecond int) func(http.Handler) http.Handler {
	// Simple in-memory rate limiter
	// TODO: Use a proper rate limiting library like golang.org/x/time/rate
	limiter := make(map[string]*rateLimiter)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := r.RemoteAddr

			limiterMu.Lock()
			if _, exists := limiter[clientIP]; !exists {
				limiter[clientIP] = newRateLimiter(requestsPerSecond)
			}
			limiterMu.Unlock()

			if !limiter[clientIP].allow() {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rateLimiter is a simple token bucket rate limiter.
type rateLimiter struct {
	tokens     int
	capacity   int
	lastRefill time.Time
	rate       time.Duration
}

var limiterMu sync.Mutex

func newRateLimiter(requestsPerSecond int) *rateLimiter {
	return &rateLimiter{
		tokens:     requestsPerSecond,
		capacity:   requestsPerSecond,
		rate:       time.Second / time.Duration(requestsPerSecond),
		lastRefill: time.Now(),
	}
}

func (rl *rateLimiter) allow() bool {
	rl.refill()
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *rateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	if elapsed >= rl.rate {
		tokensToAdd := int(elapsed / rl.rate)
		rl.tokens = min(rl.capacity, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseLogLevel converts a string log level to zap level.
func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
