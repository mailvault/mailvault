// Package service exposes the reusable API-server wiring used by both the OSS
// and commercial (SaaS) build of MailVault. cmd/ binaries call Run(opts) and
// inject build-specific extensions (auth provider, quota limiter, billing routes).
package service

import (
	"errors"
	"fmt"

	"github.com/ardanlabs/conf/v3"
	_ "github.com/joho/godotenv/autoload"
)

// Config is the shared configuration for the API service. Both OSS and SaaS
// builds use this same struct; defaults are tuned for self-hosted OSS so that
// "AUTH_PROVIDER unset" picks the built-in local provider.
type Config struct {
	Environment    string `conf:"env:ENVIRONMENT,default:development"`
	DatabaseEngine string `conf:"env:DATABASE_ENGINE,default:postgres"`
	ApiAddress     string `conf:"env:API_ADDRESS,default:0.0.0.0:3000"`
	ApiBaseURL     string `conf:"env:API_BASE_URL,default:http://localhost:3000"`
	AuthSecretKey string `conf:"env:AUTH_SECRET_KEY,default:dev-secret-change-me"`
	AuthTokenTTL  string `conf:"env:AUTH_TOKEN_TTL,default:24h"`
	AuthProvider  string `conf:"env:AUTH_PROVIDER,default:local"`

	MetricsAddress string `conf:"env:METRICS_ADDRESS,default::8080"`

	EnableDatabaseMetrics      bool `conf:"env:ENABLE_DATABASE_METRICS,default:true"`
	EnableQueryInstrumentation bool `conf:"env:ENABLE_QUERY_INSTRUMENTATION,default:true"`

	// Outbound SMTP relay — /api/v1/send hands accepted messages to this
	// host, which is expected to be a real MTA (Postfix, sendmail, a smart-
	// host) that owns queueing/retry/DKIM. Default works for any host that
	// runs Postfix locally.
	OutboundSMTPAddr     string `conf:"env:OUTBOUND_SMTP_ADDR,default:localhost:25"`
	OutboundSMTPHostname string `conf:"env:OUTBOUND_SMTP_HOSTNAME,default:localhost"`
	OutboundSMTPTLSMode  string `conf:"env:OUTBOUND_SMTP_TLS_MODE,default:none"` // none|starttls|implicit
	OutboundSMTPUsername string `conf:"env:OUTBOUND_SMTP_USERNAME"`
	OutboundSMTPPassword string `conf:"env:OUTBOUND_SMTP_PASSWORD"`
}

// Load populates the Config from environment variables (with optional prefix).
func (c *Config) Load(prefix string) error {
	if help, err := conf.Parse(prefix, c); err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return err
		}
		return err
	}
	return nil
}
