package main

import (
	"context"
	"fmt"
	"log/slog"
	"mailvault/app/api"
	"mailvault/app/api/middleware"
	v1 "mailvault/app/api/v1"
	authDomain "mailvault/domain/auth"
	"mailvault/domain/billing"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/email"
	"mailvault/domain/user"
	"mailvault/gateways/repository/pg"
	"mailvault/internal/database"
	"mailvault/internal/webhook"
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

	// Webhook system
	// ------------------------------------------
	webhookClient := webhook.NewHTTPClient(webhook.DefaultClientConfig())
	webhookNotifier := webhook.NewIncomingEmailNotificationService(webhook.NotificationServiceConfig{
		HTTPClient:  webhookClient,
		EnableAsync: true, // Enable async processing for better performance
	})

	// Use cases and their dependencies
	// ------------------------------------------
	userUseCase := user.NewUseCase(repo.UserRepo)
	domainUseCase := domainpkg.NewUseCase(repo.DomainRepo, repo.UserRepo)
	emailUseCase := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo, webhook.NewNotificationServiceAdapter(webhookNotifier))
	billingUseCase := billing.NewUseCase(repo.BillingRepo, log)

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
		AuthProvider:        authProvider,
		UserUseCase:         userUseCase,
		AuthUseCase:         userUseCase,
		DomainUseCase:       domainUseCase,
		EmailUseCase:        emailUseCase,
		BillingUseCase:      billingUseCase,
		AuthSecretKey:       cfg.AuthSecretKey,
		AuthTokenTTL:        cfg.AuthTokenTTL,
		Logger:              log,
		StripeSecretKey:     cfg.StripeSecretKey,
		StripeWebhookSecret: cfg.StripeWebhookSecret,
		HealthChecker:       dbPool,    // optimized database pool implements the Ping interface
		MetricsMiddleware:   metricsMw, // Pass metrics middleware for business metrics
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
