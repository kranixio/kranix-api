package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kranix-io/kranix-api/internal/analytics"
	"github.com/kranix-io/kranix-api/internal/apikeys"
	"github.com/kranix-io/kranix-api/internal/approval"
	"github.com/kranix-io/kranix-api/internal/audit"
	"github.com/kranix-io/kranix-api/internal/changelognotify"
	"github.com/kranix-io/kranix-api/internal/coreclient"
	"github.com/kranix-io/kranix-api/internal/graphql"
	"github.com/kranix-io/kranix-api/internal/handlers"
	"github.com/kranix-io/kranix-api/internal/middleware"
	"github.com/kranix-io/kranix-api/internal/oidc"
	"github.com/kranix-io/kranix-api/internal/sse"
	"github.com/kranix-io/kranix-api/internal/stream"
	"github.com/kranix-io/kranix-api/internal/version"
	"github.com/kranix-io/kranix-api/internal/webhooks"
	"github.com/kranix-io/kranix-packages/auth"
	"github.com/kranix-io/kranix-packages/logging"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	API struct {
		Port           int           `yaml:"port"`
		GRPCPort       int           `yaml:"grpc_port"`
		ReadTimeout    time.Duration `yaml:"read_timeout"`
		WriteTimeout   time.Duration `yaml:"write_timeout"`
		GraphQLEnabled bool          `yaml:"graphql_enabled"`
		GraphQLPath    string        `yaml:"graphql_path"`
		Version        string        `yaml:"version"`
	} `yaml:"api"`
	Auth struct {
		Mode            string        `yaml:"mode"` // jwt | apikey | oidc
		JWTSecret       string        `yaml:"jwt_secret"`
		OIDCIssuer      string        `yaml:"oidc_issuer"`
		EnableScopes    bool          `yaml:"enable_scopes"`
		OIDCEnabled     bool          `yaml:"oidc_enabled"`
		OIDCCallbackURL string        `yaml:"oidc_callback_url"`
		OIDCSessionTTL  time.Duration `yaml:"oidc_session_ttl"`
	} `yaml:"auth"`
	Core struct {
		Address    string `yaml:"address"`     // legacy gRPC address (reserved)
		HTTPBaseURL string `yaml:"http_base_url"` // REST base URL of kranix-core, e.g. http://localhost:8081
	} `yaml:"core"`
	Logging struct {
		Level  string `yaml:"level"`  // debug, info, warn, error
		Format string `yaml:"format"` // json, console
	} `yaml:"logging"`
	Audit struct {
		Enabled bool   `yaml:"enabled"`
		Sink    string `yaml:"sink"` // stdout | file | kafka
	} `yaml:"audit"`
	Webhooks struct {
		Enabled       bool          `yaml:"enabled"`
		MaxRetries    int           `yaml:"max_retries"`
		RetryInterval time.Duration `yaml:"retry_interval"`
		Timeout       time.Duration `yaml:"timeout"`
	} `yaml:"webhooks"`
	Analytics struct {
		Enabled   bool          `yaml:"enabled"`
		Retention time.Duration `yaml:"retention"`
	} `yaml:"analytics"`
	OIDCProviders struct {
		Google struct {
			Enabled      bool   `yaml:"enabled"`
			ClientID     string `yaml:"client_id"`
			ClientSecret string `yaml:"client_secret"`
		} `yaml:"google"`
		GitHub struct {
			Enabled      bool   `yaml:"enabled"`
			ClientID     string `yaml:"client_id"`
			ClientSecret string `yaml:"client_secret"`
		} `yaml:"github"`
		Okta struct {
			Enabled      bool   `yaml:"enabled"`
			Domain       string `yaml:"domain"`
			ClientID     string `yaml:"client_id"`
			ClientSecret string `yaml:"client_secret"`
		} `yaml:"okta"`
	} `yaml:"oidc_providers"`
	ChangelogNotifications struct {
		Enabled bool `yaml:"enabled"`
		Email   struct {
			Enabled  bool   `yaml:"enabled"`
			SMTPHost string `yaml:"smtp_host"`
			SMTPPort string `yaml:"smtp_port"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
			From     string `yaml:"from"`
		} `yaml:"email"`
	} `yaml:"changelog_notifications"`
	SSE struct {
		Enabled        bool `yaml:"enabled"`
		MaxConnections int  `yaml:"max_connections"`
	} `yaml:"sse"`
	Approval struct {
		Enabled    bool          `yaml:"enabled"`
		DefaultTTL time.Duration `yaml:"default_ttl"`
	} `yaml:"approval"`
}

func main() {
	configPath := flag.String("config", "./config/local.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger := logging.NewWithLevel("kranix-api", parseLogLevel(config.Logging.Level))

	// Initialize version manager
	versionManager := version.NewManager()
	versionManager.InitializeDefaultChangelog()

	// Initialize analytics service
	var analyticsService *analytics.Service
	if config.Analytics.Enabled {
		analyticsConfig := analytics.Config{
			Retention: config.Analytics.Retention,
			Enabled:   true,
		}
		analyticsService = analytics.NewService(analyticsConfig, logger)
		log.Println("Analytics service initialized")
	}

	// Initialize OIDC manager
	var oidcManager *oidc.Manager
	if config.Auth.OIDCEnabled {
		oidcConfig := &auth.OIDCConfig{
			Providers:     make(map[string]*auth.OIDCProvider),
			CallbackURL:   config.Auth.OIDCCallbackURL,
			SessionSecret: "default-secret",
			SessionTTL:    config.Auth.OIDCSessionTTL,
		}

		if config.OIDCProviders.Google.Enabled {
			oidcConfig.Providers["google"] = auth.GetGoogleProvider(
				config.OIDCProviders.Google.ClientID,
				config.OIDCProviders.Google.ClientSecret,
				config.Auth.OIDCCallbackURL,
			)
		}
		if config.OIDCProviders.GitHub.Enabled {
			oidcConfig.Providers["github"] = auth.GetGitHubProvider(
				config.OIDCProviders.GitHub.ClientID,
				config.OIDCProviders.GitHub.ClientSecret,
				config.Auth.OIDCCallbackURL,
			)
		}
		if config.OIDCProviders.Okta.Enabled {
			oidcConfig.Providers["okta"] = auth.GetOktaProvider(
				config.OIDCProviders.Okta.Domain,
				config.OIDCProviders.Okta.ClientID,
				config.OIDCProviders.Okta.ClientSecret,
				config.Auth.OIDCCallbackURL,
			)
		}

		oidcManager = oidc.NewManager(oidcConfig, logger)
		log.Println("OIDC manager initialized")
	}

	// Initialize webhook service
	var webhookService *webhooks.Service
	if config.Webhooks.Enabled {
		webhookConfig := webhooks.Config{
			MaxRetries:    config.Webhooks.MaxRetries,
			RetryInterval: config.Webhooks.RetryInterval,
			Timeout:       config.Webhooks.Timeout,
		}
		webhookService = webhooks.NewService(webhookConfig, logger)
		log.Println("Webhook service initialized")
	}

	apiKeyService := apikeys.NewService()

	// Initialize GraphQL server
	var graphqlServer *graphql.Server
	if config.API.GraphQLEnabled {
		graphqlServer, err = graphql.NewServer()
		if err != nil {
			log.Fatalf("Failed to initialize GraphQL server: %v", err)
		}
		log.Println("GraphQL server initialized")
	}

	// Initialize HTTP server
	mux := http.NewServeMux()

	// Apply middleware
	chain := middleware.Chain(
		versionManager.Middleware,
		middleware.Logging(config.Logging.Level, config.Logging.Format),
		middleware.CORS(),
		middleware.Auth(config.Auth.Mode, config.Auth.JWTSecret, config.Auth.OIDCIssuer, apiKeyService),
		middleware.RateLimit(100),
	)

	coreHTTP := config.Core.HTTPBaseURL
	if coreHTTP == "" {
		coreHTTP = "http://localhost:8081"
	}
	auditLogger := audit.New(audit.Config{
		Enabled: config.Audit.Enabled,
		Sink:    config.Audit.Sink,
	})
	changelogNotify := changelognotify.New(changelognotify.Config{
		Enabled: config.ChangelogNotifications.Enabled,
		Email: changelognotify.EmailConfig{
			Enabled:  config.ChangelogNotifications.Email.Enabled,
			SMTPHost: config.ChangelogNotifications.Email.SMTPHost,
			SMTPPort: config.ChangelogNotifications.Email.SMTPPort,
			Username: config.ChangelogNotifications.Email.Username,
			Password: config.ChangelogNotifications.Email.Password,
			From:     config.ChangelogNotifications.Email.From,
		},
	})
	handlerServer := handlers.NewServer(coreclient.New(coreHTTP), auditLogger, versionManager, changelogNotify, webhookService)

	var sseService *sse.Service
	if config.SSE.Enabled {
		sseService = sse.NewService(logger)
		sse.RegisterRoutes(mux, sseService)
		handlerServer.SSE = sseService
		log.Println("SSE service enabled at GET /api/sse")
	}

	if config.Approval.Enabled {
		ttl := config.Approval.DefaultTTL
		if ttl == 0 {
			ttl = 10 * time.Minute
		}
		handlerServer.Approvals = approval.NewStore(ttl)
		log.Println("Approval gate enabled at /api/v1/approvals")
	}

	handlerServer.RegisterRoutes(mux)
	stream.RegisterRoutes(mux)

	// Analytics handlers
	if analyticsService != nil {
		analytics.RegisterRoutes(mux, analyticsService)
	}

	// OIDC handlers
	if oidcManager != nil {
		oidc.RegisterRoutes(mux, oidcManager)
	}

	// Webhook handlers
	if webhookService != nil {
		webhooks.RegisterRoutes(mux, webhookService)
	}

	// API key handlers
	apikeys.RegisterRoutes(mux, apiKeyService)

	// Version handlers
	version.RegisterRoutes(mux, versionManager)

	// GraphQL endpoint
	if graphqlServer != nil {
		mux.HandleFunc(config.API.GraphQLPath, graphqlServer.ServeHTTP)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.API.Port),
		Handler:      chain(mux),
		ReadTimeout:  config.API.ReadTimeout,
		WriteTimeout: config.API.WriteTimeout,
	}

	// Start server in background
	go func() {
		log.Printf("Starting API server on port %d", config.API.Port)
		log.Printf("API Version: %s", config.API.Version)
		if config.API.GraphQLEnabled {
			log.Printf("GraphQL endpoint available at %s", config.API.GraphQLPath)
		}
		if config.Analytics.Enabled {
			log.Printf("Analytics API enabled")
		}
		if config.Auth.OIDCEnabled {
			log.Printf("OIDC/SSO login enabled")
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Start gRPC server (placeholder)
	log.Printf("gRPC server would start on port %d", config.API.GRPCPort)

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// loadConfig reads and parses the configuration file.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	if jwtSecret := os.Getenv("KRANE_JWT_SECRET"); jwtSecret != "" {
		config.Auth.JWTSecret = jwtSecret
	}

	return &config, nil
}

// parseLogLevel converts a string log level to zapcore.Level.
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
