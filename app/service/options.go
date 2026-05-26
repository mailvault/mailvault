package service

import (
	authDomain "github.com/mailvault/mailvault/domain/auth"
	"github.com/mailvault/mailvault/domain/email_sending"
	"github.com/mailvault/mailvault/domain/extensions"
	"github.com/mailvault/mailvault/gateways/repository/pg"

	"github.com/go-chi/chi/v5"
)

// Options is what the caller passes to Run. Most fields are optional with
// sensible OSS defaults; commercial overlays override what they need.
type Options struct {
	// Required. The loaded service Config.
	Config Config

	// AuthProviderBuilder is required. It receives the constructed repository
	// (built inside Run) and returns the auth.Provider to use. This callback
	// shape lets cmd/ pick between local, supabase, etc. without leaking
	// provider-specific deps into Options.
	AuthProviderBuilder func(repo *pg.Repository) (authDomain.Provider, error)

	// DomainLimiterBuilder is optional. Defaults to extensions.NoopDomainLimiter{}.
	DomainLimiterBuilder func(repo *pg.Repository) extensions.DomainLimiter

	// UsageTrackerBuilder is optional. Defaults to extensions.NoopUsageTracker{}.
	UsageTrackerBuilder func(repo *pg.Repository) extensions.UsageTracker

	// SenderBuilder is optional. Defaults to a local-SMTP-relay implementation
	// built from cfg.OutboundSMTP* — fine for self-hosters running Postfix on
	// the same machine. Overlays can return their own sender (e.g. multi-
	// provider router) without OSS having to know about the implementations.
	SenderBuilder func(repo *pg.Repository) email_sending.Sender

	// AdditionalRoutes is an optional hook called after the API v1 routes
	// have been mounted on the router.
	AdditionalRoutes func(r chi.Router)
}
