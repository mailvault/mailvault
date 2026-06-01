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
	"time"

	"github.com/guilhermebr/gox/postgres"
	"github.com/mailvault/mailvault/app/worker"
	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/domain/validation"
	"github.com/mailvault/mailvault/gateways/repository/pg"

	goxhttp "github.com/guilhermebr/gox/http"
	"github.com/guilhermebr/gox/logger"
)

// Injected on build time by ldflags.
var (
	BuildCommit = "undefined"
	BuildTime   = "undefined"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		slog.String("service", "worker"),
		slog.String("environment", cfg.Environment),
		slog.String("build_commit", BuildCommit),
		slog.String("build_time", BuildTime),
		slog.Int("go_max_procs", runtime.GOMAXPROCS(0)),
		slog.Int("runtime_num_cpu", runtime.NumCPU()),
	)

	log.Info("Starting MailVault Worker Service",
		slog.String("version", BuildCommit),
		slog.String("build_time", BuildTime),
		slog.String("environment", cfg.Environment),
	)

	// Database Connection Pool
	dbPool, err := postgres.NewOptimized(ctx, "", log)
	if err != nil {
		log.Error("failed to setup optimized database pool",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
	defer dbPool.Close()

	err = dbPool.Ping(ctx)
	if err != nil {
		log.Error("failed to reach database",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// Log database statistics for monitoring
	if cfg.EnableDatabaseMetrics {
		stats := dbPool.GetStats()
		log.Info("Database pool statistics",
			slog.Any("stats", stats),
		)
	}

	// Initialize repositories
	repo := pg.NewRepository(dbPool.Pool)

	// Initialize Prometheus metrics
	workerMetrics := worker.NewWorkerMetrics(worker.DefaultWorkerMetricsConfig())

	// Get worker configuration from config
	workerConfig := cfg.GetWorkerConfig()

	// Initialize DNS service
	dnsService := validation.NewDNSValidator(workerConfig.ValidationConfig, log)

	// Initialize validation use case
	validationUseCase := validation.NewUseCase(
		repo.ValidationRepo,
		repo.DomainRepo,
		dnsService,
		workerConfig.ValidationConfig,
		log,
	)

	// Initialize worker manager
	workerManager := worker.NewManager(
		validationUseCase,
		dnsService,
		workerConfig,
		log,
	)

	// Start metrics server in separate goroutine
	go func() {
		mux := http.NewServeMux()

		// Prometheus metrics endpoint
		mux.Handle("/metrics", workerMetrics.PrometheusHandler())

		// Health check endpoint
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := dbPool.Ping(r.Context()); err != nil {
				http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy","service":"worker"}`))
		})

		// Worker stats endpoint
		mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			if !workerManager.IsRunning() {
				http.Error(w, "Worker manager not running", http.StatusServiceUnavailable)
				return
			}

			stats := workerManager.GetDetailedStats()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			// Simple JSON encoding
			response := fmt.Sprintf(`{
				"manager": {
					"start_time": "%s",
					"jobs_processed": %d,
					"jobs_successful": %d,
					"jobs_failed": %d,
					"last_stats_update": "%s"
				},
				"worker_pool": {
					"total_workers": %d,
					"running_workers": %d,
					"queue_size": %d
				},
				"scheduled_jobs": %d
			}`,
				stats.Manager.StartTime.Format(time.RFC3339),
				stats.Manager.JobsProcessed,
				stats.Manager.JobsSuccessful,
				stats.Manager.JobsFailed,
				stats.Manager.LastStatsUpdate.Format(time.RFC3339),
				stats.WorkerPool.TotalWorkers,
				stats.WorkerPool.RunningWorkers,
				stats.WorkerPool.QueueSize,
				stats.ScheduledJobs,
			)

			_, _ = w.Write([]byte(response))
		})

		// Worker configuration endpoint
		mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			response := fmt.Sprintf(`{
				"worker_count": %d,
				"queue_size": %d,
				"queue_type": "%s",
				"check_interval": "%s",
				"max_retries": %d,
				"enabled": %t
			}`,
				workerConfig.WorkerCount,
				workerConfig.QueueSize,
				workerConfig.QueueType,
				workerConfig.CheckInterval.String(),
				workerConfig.MaxRetries,
				workerConfig.Enabled,
			)

			_, _ = w.Write([]byte(response))
		})

		log.Info("Starting metrics server",
			slog.String("address", cfg.MetricsAddress),
		)

		server := goxhttp.NewServerWithConfig("worker-metrics", mux, goxhttp.Config{
			Address:           cfg.MetricsAddress,
			ReadHeaderTimeout: 30 * time.Second,
			ReadTimeout:       60 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       30 * time.Second,
			ShutdownTimeout:   5 * time.Second,
		}, log)

		if err := server.Start(); err != nil {
			log.Error("Metrics server failed",
				slog.String("error", err.Error()),
			)
		}
	}()

	// Start worker manager
	if err := workerManager.Start(ctx); err != nil {
		log.Error("Failed to start worker manager",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	log.Info("Worker service started successfully",
		slog.String("metrics_address", cfg.MetricsAddress),
		slog.Int("worker_count", workerConfig.WorkerCount),
		slog.String("queue_type", workerConfig.QueueType),
		slog.Int("queue_size", workerConfig.QueueSize),
	)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start a goroutine to process domains that need validation
	go func() {
		ticker := time.NewTicker(workerConfig.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processDomainsNeedingValidation(ctx, workerManager, validationUseCase, log)
			}
		}
	}()

	// Start metrics collection goroutine
	managerStartTime := time.Now()
	go func() {
		ticker := time.NewTicker(workerConfig.StatsInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if workerManager.IsRunning() {
					stats := workerManager.GetDetailedStats()
					workerMetrics.CollectManagerStats(stats, managerStartTime)
				}
			}
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Info("Received shutdown signal",
		slog.String("signal", sig.String()),
	)

	// Graceful shutdown
	log.Info("Shutting down worker service...")

	// Cancel context to stop all operations
	cancel()

	// Stop worker manager
	if err := workerManager.Stop(); err != nil {
		log.Error("Error stopping worker manager",
			slog.String("error", err.Error()),
		)
	}

	log.Info("Worker service shutdown completed")
}

// processDomainsNeedingValidation checks for domains that need validation and queues them
func processDomainsNeedingValidation(ctx context.Context, manager *worker.Manager, useCase *validation.UseCase, logger *slog.Logger) {
	// Get domains that need validation
	domains, err := useCase.GetDomainsNeedingValidation(ctx, 50) // Process up to 50 domains at a time
	if err != nil {
		logger.Error("Failed to get domains needing validation",
			slog.String("error", err.Error()),
		)
		return
	}

	if len(domains) == 0 {
		logger.Debug("No domains need validation")
		return
	}

	logger.Info("Found domains needing validation",
		slog.Int("count", len(domains)),
	)

	// Queue validation jobs for each domain
	for _, domain := range domains {
		// Skip if domain is already verified
		if domain.VerificationStatus == entities.VerificationStatusVerified {
			logger.Debug("Skipping already verified domain",
				slog.String("domain_id", domain.ID.String()),
				slog.String("domain_name", domain.Domain),
				slog.String("status", string(domain.VerificationStatus)),
			)
			continue
		}

		// Determine priority based on attempts and age
		priority := calculateJobPriority(domain)

		err := manager.QueueDomainValidation(domain.ID, domain.Domain, priority)
		if err != nil {
			logger.Error("Failed to queue domain validation",
				slog.String("domain_id", domain.ID.String()),
				slog.String("domain_name", domain.Domain),
				slog.String("error", err.Error()),
			)
			continue
		}

		logger.Info("Queued domain for validation",
			slog.String("domain_id", domain.ID.String()),
			slog.String("domain_name", domain.Domain),
			slog.String("status", string(domain.VerificationStatus)),
			slog.Int("attempts", domain.VerificationAttempts),
			slog.Int("priority", priority),
		)
	}
}

// calculateJobPriority calculates job priority based on domain information
func calculateJobPriority(domain *validation.DomainValidationInfo) int {
	// Base priority
	priority := worker.PriorityNormal

	// Higher priority for newer domains (less than 1 hour old)
	if time.Since(domain.CreatedAt) < 1*time.Hour {
		priority = worker.PriorityHigh
	}

	// Lower priority for domains with many failed attempts
	if domain.VerificationAttempts > 5 {
		priority = worker.PriorityLow
	}

	// Higher priority for first-time validations
	if domain.VerificationAttempts == 0 {
		priority = worker.PriorityHigh
	}

	return priority
}
