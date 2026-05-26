package service

import (
	authDomain "github.com/mailvault/mailvault/domain/auth"
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
	// SaaS builds typically supply billing.NewDomainLimiter(...).
	DomainLimiterBuilder func(repo *pg.Repository) extensions.DomainLimiter

	// UsageTrackerBuilder is optional. Defaults to extensions.NoopUsageTracker{}.
	// SaaS builds supply the billing UseCase (which already satisfies the
	// interface) so that received-email events are metered.
	UsageTrackerBuilder func(repo *pg.Repository) extensions.UsageTracker

	// AdditionalRoutes is an optional hook called after the OSS API v1 routes
	// have been mounted on the router. SaaS uses it to register billing
	// endpoints without OSS having to know about them.
	AdditionalRoutes func(r chi.Router)
}
