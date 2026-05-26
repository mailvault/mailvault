package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mailvault/mailvault/app/service"
	"github.com/mailvault/mailvault/gateways/repository/pg"

	authDomain "github.com/mailvault/mailvault/domain/auth"
)

// Injected on build time by ldflags.
var (
	BuildCommit = "undefined"
	BuildTime   = "undefined"
)

func main() {
	// Pass build metadata through to the service package so it shows up in logs.
	service.BuildCommit = BuildCommit
	service.BuildTime = BuildTime

	var cfg service.Config
	if err := cfg.Load(""); err != nil {
		panic(fmt.Errorf("loading config: %w", err))
	}

	err := service.Run(context.Background(), service.Options{
		Config: cfg,
		AuthProviderBuilder: func(repo *pg.Repository) (authDomain.Provider, error) {
			tokenTTL, err := time.ParseDuration(cfg.AuthTokenTTL)
			if err != nil {
				return nil, fmt.Errorf("parse AUTH_TOKEN_TTL: %w", err)
			}
			return newAuthProvider(cfg, tokenTTL, repo)
		},
		// OSS defaults: extensions are no-op. Self-hosters get unlimited everything.
	})
	if err != nil {
		slog.Default().Error("service failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
