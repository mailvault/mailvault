package main

import (
	"fmt"
	"time"

	"github.com/mailvault/mailvault/app/service"
	authDomain "github.com/mailvault/mailvault/domain/auth"
	"github.com/mailvault/mailvault/domain/auth/local"
	"github.com/mailvault/mailvault/gateways/repository/pg"
)

// newAuthProvider builds the OSS auth provider. OSS supports only the local
// built-in (users + bcrypt + JWT). Commercial overlays add their own providers
// (Supabase etc.) in their own cmd/service/auth.go.
func newAuthProvider(cfg service.Config, tokenTTL time.Duration, repo *pg.Repository) (authDomain.Provider, error) {
	switch cfg.AuthProvider {
	case "local":
		return local.NewProvider(repo.LocalCredsRepo, repo.UserRepo, cfg.AuthSecretKey, tokenTTL)
	default:
		return nil, fmt.Errorf("unsupported auth provider: %q (OSS supports: local)", cfg.AuthProvider)
	}
}
