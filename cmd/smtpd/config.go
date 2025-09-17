package main

import (
	"errors"
	"fmt"

	"github.com/ardanlabs/conf/v3"
	_ "github.com/joho/godotenv/autoload"
)

// Config holds SMTP server configuration
type Config struct {
	Environment string `conf:"env:ENVIRONMENT,default:development"`
	Addr        string `conf:"env:SMTP_ADDR,default:127.0.0.1:2525"`
	Domain      string `conf:"env:SMTP_DOMAIN,default:localhost"`
	Debug       bool   `conf:"env:SMTP_DEBUG,default:true"`

	// TLS Configuration
	TLSMode string `conf:"env:SMTP_TLS_MODE,default:off"` // off, cert
	TLSCert string `conf:"env:SMTP_TLS_CERT"`
	TLSKey  string `conf:"env:SMTP_TLS_KEY"`
	// If true, the listener will expect TLS from the start (implicit TLS, e.g. port 465).
	// If false and TLSMode is cert, STARTTLS will be offered on the plaintext connection (e.g. port 25/587).
	TLSImplicit bool `conf:"env:SMTP_TLS_IMPLICIT,default:false"`

	// Database optimization settings
	EnableDatabaseMetrics bool `conf:"env:ENABLE_DATABASE_METRICS,default:true"`
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
