package smtpd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/mailvault/mailvault/app/smtp"
	domainUseCase "github.com/mailvault/mailvault/domain/domain"
	"github.com/mailvault/mailvault/domain/email"
	"github.com/mailvault/mailvault/domain/extensions"
	"github.com/mailvault/mailvault/domain/smtp_stats"
	"github.com/mailvault/mailvault/gateways/repository/pg"
	"github.com/mailvault/mailvault/internal/database"
	"github.com/mailvault/mailvault/internal/webhook"

	goxhttp "github.com/guilhermebr/gox/http"
	"github.com/guilhermebr/gox/logger"
)

// Build metadata injected by ldflags.
var (
	BuildCommit = "undefined"
	BuildTime   = "undefined"
)

// Options is what the caller passes to Run. Builder callbacks receive the
// repository (constructed inside Run) so commercial overlays can wire
// billing-backed limiters and trackers without touching internal/ packages.
type Options struct {
	Config Config

	// DomainLimiterBuilder is optional. Defaults to extensions.NoopDomainLimiter{}.
	DomainLimiterBuilder func(repo *pg.Repository) extensions.DomainLimiter

	// UsageTrackerBuilder is optional. Defaults to extensions.NoopUsageTracker{}.
	// SaaS supplies billing.NewUseCase(...) so received-email events are metered.
	UsageTrackerBuilder func(repo *pg.Repository) extensions.UsageTracker
}

// Run starts the SMTP daemon. Blocks until SIGINT/SIGTERM, then shuts down gracefully.
func Run(ctx context.Context, opts Options) error {
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
		return fmt.Errorf("connect to database: %w", err)
	}
	defer dbPool.Close()

	if cfg.EnableDatabaseMetrics {
		log.Info("Database pool statistics", slog.Any("stats", dbPool.GetStats()))
	}

	repo := pg.NewRepository(dbPool.Pool)

	webhookClient := webhook.NewHTTPClient(webhook.DefaultClientConfig())
	webhookNotifier := webhook.NewIncomingEmailNotificationService(webhook.NotificationServiceConfig{
		HTTPClient:  webhookClient,
		EnableAsync: true,
	})

	var domainLimiter extensions.DomainLimiter = extensions.NoopDomainLimiter{}
	if opts.DomainLimiterBuilder != nil {
		domainLimiter = opts.DomainLimiterBuilder(repo)
	}
	var usageTracker extensions.UsageTracker = extensions.NoopUsageTracker{}
	if opts.UsageTrackerBuilder != nil {
		usageTracker = opts.UsageTrackerBuilder(repo)
	}

	domainUC := domainUseCase.NewUseCase(repo.DomainRepo, domainLimiter)
	emailUC := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo, webhook.NewNotificationServiceAdapter(webhookNotifier))

	smtpStatsRepo := pg.NewSMTPStatsRepository(dbPool.Pool)
	smtpStatsUC := smtp_stats.NewUseCase(smtpStatsRepo)

	smtpCfg := smtp.Config{
		Addr:        cfg.Addr,
		Domain:      cfg.Domain,
		Debug:       cfg.Debug,
		TLSMode:     smtp.TLSMode(cfg.TLSMode),
		TLSCert:     cfg.TLSCert,
		TLSKey:      cfg.TLSKey,
		TLSImplicit: cfg.TLSImplicit,
	}

	backend := smtp.NewBackend(domainUC, emailUC, smtpStatsUC, usageTracker, log)
	backend.ConfigureForwarding(smtp.ForwardingConfig{
		RelayAddr: cfg.ForwardingRelayAddr,
		Hostname:  cfg.Domain,
	})

	smtpServer, err := smtp.NewServer(smtpCfg, backend, log)
	if err != nil {
		return fmt.Errorf("create SMTP server: %w", err)
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", backend.GetMetrics().PrometheusHandler())
		metricsServer := goxhttp.NewServerWithConfig("smtpd-metrics", mux, goxhttp.Config{
			Address:           ":8080",
			ReadHeaderTimeout: 10 * time.Second,
			ShutdownTimeout:   5 * time.Second,
		}, log)
		if err := metricsServer.Start(); err != nil {
			log.Error("Metrics server failed", slog.String("error", err.Error()))
		}
	}()

	go func() {
		if err := smtpServer.Start(); err != nil {
			log.Error("SMTP server failed", slog.String("error", err.Error()))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("shutting down SMTP server...")
	shutdownCtx := context.Background()
	if err := smtpServer.Stop(shutdownCtx); err != nil {
		log.Error("error during SMTP server shutdown", slog.String("error", err.Error()))
	}
	log.Info("SMTP server stopped")
	return nil
}
