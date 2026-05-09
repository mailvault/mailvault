package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"mailvault/app/smtp"
	"mailvault/domain/billing"
	domainUseCase "mailvault/domain/domain"
	"mailvault/domain/email"
	"mailvault/domain/smtp_stats"
	"mailvault/gateways/repository/pg"
	"mailvault/internal/database"
	"mailvault/internal/webhook"

	"github.com/guilhermebr/gox/logger"
)

// Injected on build time by ldflags.
var (
	BuildCommit = "undefined"
	BuildTime   = "undefined"
)

func main() {
	var cfg Config
	if err := cfg.Load(""); err != nil {
		panic(fmt.Errorf("loading config: %w", err))
	}

	// Initialize logger
	logger, err := logger.NewLogger("")
	if err != nil {
		panic(fmt.Errorf("creating logger: %w", err))
	}

	logger = logger.With(
		slog.String("environment", cfg.Environment),
	)

	log := logger.With(
		slog.String("environment", cfg.Environment),
		slog.String("build_commit", BuildCommit),
		slog.String("build_time", BuildTime),
		slog.Int("go_max_procs", runtime.GOMAXPROCS(0)),
		slog.Int("runtime_num_cpu", runtime.NumCPU()),
	)

	// Initialize optimized database connection pool
	dbPool, err := database.NewOptimizedPool(context.Background(), "", log)
	if err != nil {
		log.Error("failed to connect to database",
			slog.String("error", err.Error()),
		)
		return
	}
	defer dbPool.Close()

	// Log database pool statistics
	stats := dbPool.GetStats()
	log.Info("Database pool statistics",
		slog.Any("stats", stats),
	)

	// Initialize repositories
	repo := pg.NewRepository(dbPool.Pool)

	// Webhook system
	webhookClient := webhook.NewHTTPClient(webhook.DefaultClientConfig())
	webhookNotifier := webhook.NewIncomingEmailNotificationService(webhook.NotificationServiceConfig{
		HTTPClient:  webhookClient,
		EnableAsync: true, // Enable async processing for better performance
	})

	// Initialize use cases
	domainUseCase := domainUseCase.NewUseCase(repo.DomainRepo, repo.UserRepo)
	emailUseCase := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo, webhook.NewNotificationServiceAdapter(webhookNotifier))

	// Initialize SMTP stats repository and use case
	smtpStatsRepo := pg.NewSMTPStatsRepository(dbPool.Pool)
	smtpStatsUseCase := smtp_stats.NewUseCase(smtpStatsRepo)

	// Initialize billing use case for usage tracking
	billingUseCase := billing.NewUseCase(repo.BillingRepo, log)

	// Create SMTP server
	smtpCfg := smtp.Config{
		Addr:        cfg.Addr,
		Domain:      cfg.Domain,
		Debug:       cfg.Debug,
		TLSMode:     smtp.TLSMode(cfg.TLSMode),
		TLSCert:     cfg.TLSCert,
		TLSKey:      cfg.TLSKey,
		TLSImplicit: cfg.TLSImplicit,
	}

	backend := smtp.NewBackend(domainUseCase, emailUseCase, smtpStatsUseCase, billingUseCase, logger)

	// Configure email forwarding relay if an address is provided.
	backend.ConfigureForwarding(smtp.ForwardingConfig{
		RelayAddr: cfg.ForwardingRelayAddr,
		Hostname:  cfg.Domain,
	})

	smtpServer, err := smtp.NewServer(smtpCfg, backend, logger)
	if err != nil {
		log.Error("failed to create SMTP server",
			slog.String("error", err.Error()),
		)
		return
	}

	// Start HTTP server for metrics
	go func() {
		metricsHandler := backend.GetMetrics().PrometheusHandler()
		http.Handle("/metrics", metricsHandler)

		log.Info("Starting metrics server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Error("Metrics server failed", slog.String("error", err.Error()))
		}
	}()

	// Start SMTP server in goroutine
	go func() {
		if err := smtpServer.Start(); err != nil {
			log.Error("SMTP server failed",
				slog.String("error", err.Error()),
			)
			return
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("shutting down SMTP server...")

	// Stop server gracefully
	ctx := context.Background()
	if err := smtpServer.Stop(ctx); err != nil {
		log.Error("error during SMTP server shutdown",
			slog.String("error", err.Error()),
		)
	}

	log.Info("SMTP server stopped")
}
