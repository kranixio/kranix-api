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

	"github.com/kranix-io/kranix-api/internal/apikeys"
	"github.com/kranix-io/kranix-api/internal/graphql"
	"github.com/kranix-io/kranix-api/internal/handlers"
	"github.com/kranix-io/kranix-api/internal/middleware"
	"github.com/kranix-io/kranix-api/internal/stream"
	"github.com/kranix-io/kranix-api/internal/webhooks"
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
	} `yaml:"api"`
	Auth struct {
		Mode         string `yaml:"mode"` // jwt | apikey | oidc
		JWTSecret    string `yaml:"jwt_secret"`
		OIDCIssuer   string `yaml:"oidc_issuer"`
		EnableScopes bool   `yaml:"enable_scopes"`
	} `yaml:"auth"`
	Core struct {
		Address string `yaml:"address"` // gRPC address of kranix-core
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

	// Initialize services
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
		middleware.Logging(config.Logging.Level, config.Logging.Format),
		middleware.CORS(),
		middleware.Auth(config.Auth.Mode, config.Auth.JWTSecret, config.Auth.OIDCIssuer),
		middleware.RateLimit(100), // 100 requests per second
	)

	// Register handlers
	handlers.RegisterRoutes(mux)

	// Stream handlers
	stream.RegisterRoutes(mux)

	// Webhook handlers
	if webhookService != nil {
		webhooks.RegisterRoutes(mux, webhookService)
	}

	// API key handlers
	apikeys.RegisterRoutes(mux, apiKeyService)

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
		if config.API.GraphQLEnabled {
			log.Printf("GraphQL endpoint available at %s", config.API.GraphQLPath)
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Start gRPC server (placeholder)
	// TODO: Implement gRPC server
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
