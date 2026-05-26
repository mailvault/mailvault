// Package smtpd exposes the reusable SMTP-daemon wiring used by both the OSS
// and commercial (SaaS) build of MailVault. cmd/smtpd binaries call Run(opts)
// and inject build-specific extensions (quota limiter, usage tracker).
package smtpd

import (
	"errors"
	"fmt"

	"github.com/ardanlabs/conf/v3"
	_ "github.com/joho/godotenv/autoload"
)

// Config is the shared SMTP server configuration used by both OSS and SaaS builds.
type Config struct {
	Environment string `conf:"env:ENVIRONMENT,default:development"`
	Addr        string `conf:"env:SMTP_ADDR,default:127.0.0.1:2525"`
	Domain      string `conf:"env:SMTP_DOMAIN,default:localhost"`
	Debug       bool   `conf:"env:SMTP_DEBUG,default:true"`

	TLSMode     string `conf:"env:SMTP_TLS_MODE,default:off"`
	TLSCert     string `conf:"env:SMTP_TLS_CERT"`
	TLSKey      string `conf:"env:SMTP_TLS_KEY"`
	TLSImplicit bool   `conf:"env:SMTP_TLS_IMPLICIT,default:false"`

	EnableDatabaseMetrics bool `conf:"env:ENABLE_DATABASE_METRICS,default:true"`

	// ForwardingRelayAddr is the SMTP relay used to forward incoming emails
	// to external mailboxes. Leave empty to disable forwarding.
	ForwardingRelayAddr string `conf:"env:SMTP_FORWARDING_RELAY_ADDR"`
}

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
