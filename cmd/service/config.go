package main

import (
	"errors"
	"fmt"

	"github.com/ardanlabs/conf/v3"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Environment    string `conf:"env:ENVIRONMENT,default:development"`
	DatabaseEngine string `conf:"env:DATABASE_ENGINE,default:postgres"`
	ApiAddress     string `conf:"env:API_ADDRESS,default:0.0.0.0:3000"`
	ApiBaseURL     string `conf:"env:API_BASE_URL,default:http://localhost:3000"`
	AuthSecretKey  string `conf:"env:AUTH_SECRET_KEY,default:dev-secret-change-me"`
	AuthTokenTTL   string `conf:"env:AUTH_TOKEN_TTL,default:24h"`
	AuthProvider   string `conf:"env:AUTH_PROVIDER,default:basic"`

	// Metrics server configuration
	MetricsAddress string `conf:"env:METRICS_ADDRESS,default::8080"`

	// Database optimization settings
	EnableDatabaseMetrics      bool `conf:"env:ENABLE_DATABASE_METRICS,default:true"`
	EnableQueryInstrumentation bool `conf:"env:ENABLE_QUERY_INSTRUMENTATION,default:true"`

	// Stripe billing configuration
	StripeSecretKey      string `conf:"env:STRIPE_SECRET_KEY"`
	StripeWebhookSecret  string `conf:"env:STRIPE_WEBHOOK_SECRET"`
	StripePublishableKey string `conf:"env:STRIPE_PUBLISHABLE_KEY"`
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
