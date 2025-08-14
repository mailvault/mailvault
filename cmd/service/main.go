package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mailvault/app/api"
	v1 "mailvault/app/api/v1"
	"mailvault/domain/auth"
	domainpkg "mailvault/domain/domain"
	"mailvault/domain/email"
	"mailvault/domain/user"
	"mailvault/gateway/repository/pg"
	"net/http"
	"runtime"
	"time"

	"github.com/guilhermebr/gox/logger"
	"github.com/guilhermebr/gox/postgres"
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

	// Repositories
	conn, err := postgres.New(ctx, "")
	if err != nil {
		log.Error("failed to setup postgres",
			slog.String("error", err.Error()),
		)
		return
	}
	defer conn.Close()

	err = conn.Ping(ctx)
	if err != nil {
		log.Error("failed to reach postgres",
			slog.String("error", err.Error()),
		)
		return
	}
	repo := pg.NewRepository(conn)

	// Authentication provider
	// ------------------------------------------
	authProvider, err := auth.NewAuthProvider(auth.Config{
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
	domainUseCase := domainpkg.NewUseCase(repo.DomainRepo)
	emailUseCase := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo)

	// Handlers V1
	apiV1 := v1.ApiHandlers{
		AuthProvider:  authProvider,
		UserUseCase:   userUseCase,
		DomainUseCase: domainUseCase,
		EmailUseCase:  emailUseCase,
		AuthSecretKey: cfg.AuthSecretKey,
		AuthTokenTTL:  cfg.AuthTokenTTL,
	}

	router := api.Router()
	apiV1.Routes(router)

	// SERVER
	// ------------------------------------------
	server := http.Server{
		Handler:           router,
		Addr:              cfg.ApiAddress,
		ReadHeaderTimeout: 60 * time.Second,
	}
	log.Info("server started",
		slog.String("address", server.Addr),
	)

	if serverErr := server.ListenAndServe(); serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
		log.Error("failed to listen and serve server",
			slog.String("error", serverErr.Error()),
		)
	}
}
