package service

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/mailvault/mailvault/app/api"
	"github.com/mailvault/mailvault/app/api/middleware"
	v1 "github.com/mailvault/mailvault/app/api/v1"
	domainpkg "github.com/mailvault/mailvault/domain/domain"
	"github.com/mailvault/mailvault/domain/email"
	"github.com/mailvault/mailvault/domain/email_sending"
	"github.com/mailvault/mailvault/domain/extensions"
	"github.com/mailvault/mailvault/domain/user"
	"github.com/mailvault/mailvault/domain/webhook_config"
	"github.com/mailvault/mailvault/gateways/repository/pg"
	"github.com/mailvault/mailvault/internal/database"
	"github.com/mailvault/mailvault/internal/smtprelay"
	"github.com/mailvault/mailvault/internal/webhook"

	_ "github.com/mailvault/mailvault/docs" // swagger docs

	goxhttp "github.com/guilhermebr/gox/http"
	"github.com/guilhermebr/gox/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Build metadata injected by ldflags. Callers can set these via main.go too,
// but defaults make local dev work without flag wiring.
var (
	BuildCommit = "undefined"
	BuildTime   = "undefined"
)

// Run starts the API service with the given options. It blocks until the
// server shuts down (graceful) or an unrecoverable error occurs.
func Run(ctx context.Context, opts Options) error {
	if opts.AuthProviderBuilder == nil {
		return fmt.Errorf("service.Run: AuthProviderBuilder is required")
	}
	cfg := opts.Config

	log, err := logger.NewLogger("")
	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}
	log = log.With(
		slog.String("environment", cfg.Environment),
		slog.String("build_commit", BuildCommit),
		slog.String("build_time", BuildTime),
		slog.Int("go_max_procs", runtime.GOMAXPROCS(0)),
		slog.Int("runtime_num_cpu", runtime.NumCPU()),
	)

	dbPool, err := database.NewOptimizedPool(ctx, "", log)
	if err != nil {
		return fmt.Errorf("setup database pool: %w", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if cfg.EnableDatabaseMetrics {
		log.Info("Database pool statistics", slog.Any("stats", dbPool.GetStats()))
	}

	if os.Getenv("MIGRATE_ON_BOOT") == "true" {
		if err := runMigrationsOnBoot(log); err != nil {
			return fmt.Errorf("migrations on boot: %w", err)
		}
	}

	repo := pg.NewRepository(dbPool.Pool)

	authProvider, err := opts.AuthProviderBuilder(repo)
	if err != nil {
		return fmt.Errorf("build auth provider: %w", err)
	}

	var domainLimiter extensions.DomainLimiter = extensions.NoopDomainLimiter{}
	if opts.DomainLimiterBuilder != nil {
		domainLimiter = opts.DomainLimiterBuilder(repo)
	}
	// usageTracker is constructed but only consumed by the SMTP daemon today;
	// keeping the builder hook here so commercial overlays can satisfy the
	// option without having to wire two separate services.
	if opts.UsageTrackerBuilder != nil {
		_ = opts.UsageTrackerBuilder(repo)
	}

	// Outbound mail sender: overlays can inject their own; OSS falls back to
	// the local SMTP relay configured via OUTBOUND_SMTP_*.
	var sender email_sending.Sender
	if opts.SenderBuilder != nil {
		sender = opts.SenderBuilder(repo)
	} else {
		sender = smtprelay.New(smtprelay.Config{
			Addr:     cfg.OutboundSMTPAddr,
			Hostname: cfg.OutboundSMTPHostname,
			TLSMode:  smtprelay.TLSMode(cfg.OutboundSMTPTLSMode),
			Username: cfg.OutboundSMTPUsername,
			Password: cfg.OutboundSMTPPassword,
		}, log)
	}
	_ = sender // wired into the email-sending use case below; reference here for clarity.

	webhookClient := webhook.NewHTTPClient(webhook.DefaultClientConfig())
	webhookNotifier := webhook.NewIncomingEmailNotificationService(webhook.NotificationServiceConfig{
		HTTPClient:  webhookClient,
		EnableAsync: true,
	})

	userUseCase := user.NewUseCase(repo.UserRepo)
	domainUseCase := domainpkg.NewUseCase(repo.DomainRepo, domainLimiter)
	emailUseCase := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo, webhook.NewNotificationServiceAdapter(webhookNotifier))
	webhookConfigUseCase := webhook_config.NewUseCase(repo.WebhookConfigRepo, repo.DomainRepo, log)

	metricsMw := middleware.NewMetricsMiddleware(middleware.DefaultMetricsConfig())

	go func() {
		http.Handle("/metrics", metricsMw.PrometheusHandler())
		log.Info("Starting metrics server", slog.String("address", cfg.MetricsAddress))
		srv := &http.Server{Addr: cfg.MetricsAddress, ReadHeaderTimeout: 10 * time.Second}
		if err := srv.ListenAndServe(); err != nil {
			log.Error("Metrics server failed", slog.String("error", err.Error()))
		}
	}()

	apiV1 := v1.ApiHandlers{
		AuthProvider:         authProvider,
		UserUseCase:          userUseCase,
		AuthUseCase:          userUseCase,
		DomainUseCase:        domainUseCase,
		EmailUseCase:         emailUseCase,
		WebhookConfigUseCase: webhookConfigUseCase,
		AuthSecretKey:        cfg.AuthSecretKey,
		AuthTokenTTL:         cfg.AuthTokenTTL,
		Logger:               log,
		HealthChecker:        dbPool,
		MetricsMiddleware:    metricsMw,
	}

	router := api.Router()
	apiV1.Routes(router)

	if opts.AdditionalRoutes != nil {
		opts.AdditionalRoutes(router)
	}

	log.Info("server starting",
		slog.String("address", cfg.ApiAddress),
		slog.String("environment", cfg.Environment),
	)

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
		return fmt.Errorf("server failed: %w", err)
	}
	log.Info("server shutdown completed")
	return nil
}

func runMigrationsOnBoot(log *slog.Logger) error {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("DATABASE_USER"),
		os.Getenv("DATABASE_PASSWORD"),
		cmp.Or(os.Getenv("DATABASE_HOST"), "localhost"),
		cmp.Or(os.Getenv("DATABASE_PORT"), "5432"),
		os.Getenv("DATABASE_NAME"),
		cmp.Or(os.Getenv("DATABASE_SSLMODE"), "disable"),
	)
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("opening sql.DB for migrations: %w", err)
	}
	defer sqlDB.Close()
	if err := pg.MigrateUp(sqlDB); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	log.Info("migrations applied on boot")
	return nil
}
