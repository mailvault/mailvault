// @title MailVault API
// @version 1.0
// @description A private email service API that provides secure email management with domain-based configuration, encrypted storage, and developer-friendly endpoints.
// @description
// @description MailVault allows users to:
// @description - Manage custom domains for email services
// @description - Create and configure email addresses with forwarding and catch-all options
// @description - Send emails via API using domain API keys
// @description - Receive and store encrypted emails
// @description - Access received emails through secure endpoints
// @description
// @description ## Authentication
// @description The API uses JWT tokens for user authentication. Some endpoints require API keys for domain-specific operations.
// @description
// @description ## Rate Limiting
// @description API endpoints are rate limited to prevent abuse. See individual endpoint documentation for specific limits.
// @termsOfService https://mailvault.sh/terms
// @contact.name MailVault Support
// @contact.url https://mailvault.sh/support
// @contact.email support@mailvault.sh
// @license.name MIT
// @license.url https://github.com/guilhermebr/mailvault/blob/main/LICENSE
// @host :3000
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT token authentication. Format: "Bearer {token}"
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description Domain API key for sending emails
package main

import (
	"context"
	"fmt"
	"log/slog"
	"mailvault/app/api"
	"mailvault/app/api/middleware"
	v1 "mailvault/app/api/v1"
	authDomain "mailvault/domain/auth"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/email"
	"mailvault/domain/user"
	"mailvault/gateways/repository/pg"
	"mailvault/internal/database"
	"net/http"
	"runtime"
	"time"

	_ "mailvault/docs" // swagger docs

	goxhttp "github.com/guilhermebr/gox/http"
	"github.com/guilhermebr/gox/logger"
)

// Injected on build time by ldflags.
var (
	BuildCommit = "undefined"
	BuildTime   = "undefined"
)

func main() {
	ctx := context.Background()

	var cfg Config
	if err := cfg.Load(""); err != nil {
		panic(fmt.Errorf("loading config: %w", err))
	}

	// Logger
	log, err := logger.NewLogger("")
	if err != nil {
		panic(fmt.Errorf("creating logger: %w", err))
	}

	log = log.With(
		slog.String("environment", cfg.Environment),
		slog.String("build_commit", BuildCommit),
		slog.String("build_time", BuildTime),
		slog.Int("go_max_procs", runtime.GOMAXPROCS(0)),
		slog.Int("runtime_num_cpu", runtime.NumCPU()),
	)

	// Optimized Database Connection Pool
	dbPool, err := database.NewOptimizedPool(ctx, "", log)
	if err != nil {
		log.Error("failed to setup optimized database pool",
			slog.String("error", err.Error()),
		)
		return
	}
	defer dbPool.Close()

	err = dbPool.Ping(ctx)
	if err != nil {
		log.Error("failed to reach database",
			slog.String("error", err.Error()),
		)
		return
	}

	// Log database statistics for monitoring
	if cfg.EnableDatabaseMetrics {
		stats := dbPool.GetStats()
		log.Info("Database pool statistics",
			slog.Any("stats", stats),
		)
	}

	repo := pg.NewRepository(dbPool.Pool)

	// Authentication provider
	// ------------------------------------------
	authProvider, err := authDomain.NewAuthProvider(authDomain.Config{
		Provider:       cfg.AuthProvider,
		SupabaseURL:    cfg.SupabaseURL,
		SupabaseAPIKey: cfg.SupabaseAPIKey,
	})
	if err != nil {
		log.Error("failed to setup auth provider",
			slog.String("error", err.Error()),
		)
		return
	}

	// Use cases and their dependencies
	// ------------------------------------------
	userUseCase := user.NewUseCase(repo.UserRepo)
	domainUseCase := domainpkg.NewUseCase(repo.DomainRepo, repo.UserRepo)
	emailUseCase := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo)

	// Initialize metrics middleware for separate server
	metricsMw := middleware.NewMetricsMiddleware(middleware.DefaultMetricsConfig())

	// Start metrics server in separate goroutine
	go func() {
		metricsHandler := metricsMw.PrometheusHandler()
		http.Handle("/metrics", metricsHandler)

		log.Info("Starting metrics server",
			slog.String("address", cfg.MetricsAddress),
		)

		if err := http.ListenAndServe(cfg.MetricsAddress, nil); err != nil {
			log.Error("Metrics server failed",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Handlers V1
	apiV1 := v1.ApiHandlers{
		AuthProvider:      authProvider,
		UserUseCase:       userUseCase,
		AuthUseCase:       userUseCase,
		DomainUseCase:     domainUseCase,
		EmailUseCase:      emailUseCase,
		AuthSecretKey:     cfg.AuthSecretKey,
		AuthTokenTTL:      cfg.AuthTokenTTL,
		Logger:            log,
		HealthChecker:     dbPool, // optimized database pool implements the Ping interface
		MetricsMiddleware: metricsMw, // Pass metrics middleware for business metrics
	}

	router := api.Router()
	apiV1.Routes(router)

	// SERVER
	// ------------------------------------------
	log.Info("server starting",
		slog.String("address", cfg.ApiAddress),
		slog.String("environment", cfg.Environment),
	)

	// Configure server with graceful shutdown using gox/http
	serverConfig := goxhttp.Config{
		Address:           cfg.ApiAddress,
		ReadHeaderTimeout: 60 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   30 * time.Second,
	}

	httpServer := goxhttp.NewServerWithConfig("api", router, serverConfig, log)

	log.Info("server address", slog.String("address", httpServer.Address()))

	if err := httpServer.StartWithGracefulShutdown(); err != nil {
		log.Error("server failed",
			slog.String("error", err.Error()),
		)
	}

	log.Info("server shutdown completed")
}
