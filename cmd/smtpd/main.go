package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"mailvault/app/smtp"
	domainUseCase "mailvault/domain/domain"
	"mailvault/domain/email"
	"mailvault/gateways/repository/pg"

	"github.com/guilhermebr/gox/logger"
	"github.com/guilhermebr/gox/postgres"
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

	// Initialize database connection
	db, err := postgres.New(context.Background(), "")
	if err != nil {
		log.Error("failed to connect to database",
			slog.String("error", err.Error()),
		)
		return
	}
	defer db.Close()

	// Initialize repositories
	repo := pg.NewRepository(db)

	// Initialize use cases
	domainUseCase := domainUseCase.NewUseCase(repo.DomainRepo, repo.UserRepo)
	emailUseCase := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo)

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

	backend := smtp.NewBackend(domainUseCase, emailUseCase, logger)

	smtpServer, err := smtp.NewServer(smtpCfg, backend, logger)
	if err != nil {
		log.Error("failed to create SMTP server",
			slog.String("error", err.Error()),
		)
		return
	}

	// Start server in goroutine
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
