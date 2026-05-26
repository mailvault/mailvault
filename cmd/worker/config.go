package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mailvault/mailvault/app/worker"
	"github.com/mailvault/mailvault/domain/validation"

	"github.com/ardanlabs/conf/v3"
	_ "github.com/joho/godotenv/autoload"
)

// Config holds the worker service configuration
type Config struct {
	// Environment
	Environment string `conf:"env:ENVIRONMENT,default:development"`

	// Database
	DatabaseURL string `conf:"env:DATABASE_URL"`

	// Worker configuration
	WorkerCount   int           `conf:"env:WORKER_COUNT,default:3"`
	QueueSize     int           `conf:"env:QUEUE_SIZE,default:1000"`
	QueueType     string        `conf:"env:QUEUE_TYPE,default:priority"`
	CheckInterval time.Duration `conf:"env:CHECK_INTERVAL,default:30s"`
	MaxRetries    int           `conf:"env:MAX_RETRIES,default:3"`
	StatsInterval time.Duration `conf:"env:STATS_INTERVAL,default:1m"`
	Enabled       bool          `conf:"env:WORKER_ENABLED,default:true"`

	// DNS validation
	ExpectedMXServers    string        `conf:"env:EXPECTED_MX_SERVERS,default:mail.mailvault.sh,mail2.mailvault.sh"`
	DNSServer            string        `conf:"env:DNS_SERVER,default:8.8.8.8:53"`
	MXCheckTimeout       time.Duration `conf:"env:MX_CHECK_TIMEOUT,default:30s"`
	TXTRecordPrefix      string        `conf:"env:TXT_RECORD_PREFIX,default:mailvault-verification"`
	TXTCheckTimeout      time.Duration `conf:"env:TXT_CHECK_TIMEOUT,default:30s"`
	ValidationMaxRetries int           `conf:"env:VALIDATION_MAX_RETRIES,default:3"`
	ValidationRetryDelay time.Duration `conf:"env:VALIDATION_RETRY_DELAY,default:5m"`
	ValidationTimeout    time.Duration `conf:"env:VALIDATION_TIMEOUT,default:60s"`
	TokenExpiry          time.Duration `conf:"env:TOKEN_EXPIRY,default:24h"`

	// Logging
	LogLevel string `conf:"env:LOG_LEVEL,default:info"`

	// Metrics
	MetricsAddress        string `conf:"env:METRICS_ADDRESS,default::8080"`
	EnableDatabaseMetrics bool   `conf:"env:ENABLE_DATABASE_METRICS,default:true"`
}

// Load loads configuration from environment variables using ardanlabs/conf/v3
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

// GetWorkerConfig returns the worker configuration struct
func (c *Config) GetWorkerConfig() worker.WorkerConfig {
	// Parse MX servers from comma-separated string
	expectedMXServers := parseMXServers(c.ExpectedMXServers)

	return worker.WorkerConfig{
		WorkerCount:   c.WorkerCount,
		QueueSize:     c.QueueSize,
		QueueType:     c.QueueType,
		CheckInterval: c.CheckInterval,
		MaxRetries:    c.MaxRetries,
		StatsInterval: c.StatsInterval,
		Enabled:       c.Enabled,
		ValidationConfig: validation.ValidationConfig{
			ExpectedMXServers: expectedMXServers,
			MXCheckTimeout:    c.MXCheckTimeout,
			TXTRecordPrefix:   c.TXTRecordPrefix,
			TXTCheckTimeout:   c.TXTCheckTimeout,
			MaxRetries:        c.ValidationMaxRetries,
			RetryDelay:        c.ValidationRetryDelay,
			DNSServer:         c.DNSServer,
			ValidationTimeout: c.ValidationTimeout,
			TokenExpiry:       c.TokenExpiry,
		},
	}
}

// parseMXServers parses a comma-separated string of MX servers
func parseMXServers(mxServersStr string) []string {
	if mxServersStr == "" {
		return []string{"mail.mailvault.sh", "mail2.mailvault.sh"}
	}

	var servers []string
	for _, server := range strings.Split(mxServersStr, ",") {
		if trimmed := strings.TrimSpace(server); trimmed != "" {
			servers = append(servers, trimmed)
		}
	}

	if len(servers) == 0 {
		return []string{"mail.mailvault.sh", "mail2.mailvault.sh"}
	}

	return servers
}
