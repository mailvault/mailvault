package smtp

import (
	"log/slog"
	"time"

	"github.com/mailvault/mailvault/app/smtp/verification"
	domainUseCase "github.com/mailvault/mailvault/domain/domain"
	"github.com/mailvault/mailvault/domain/email"
	"github.com/mailvault/mailvault/domain/extensions"
	"github.com/mailvault/mailvault/domain/smtp_stats"

	"github.com/emersion/go-smtp"
)

// Backend implements SMTP server backend
type Backend struct {
	domainUseCase    *domainUseCase.UseCase
	emailUseCase     *email.UseCase
	smtpStatsUseCase *smtp_stats.UseCase
	usageTracker     extensions.UsageTracker
	verifier         *verification.Verifier
	metrics          *SMTPMetrics
	logger           *slog.Logger
}

// ForwardingConfig holds configuration for the email forwarding relay.
type ForwardingConfig struct {
	// RelayAddr is the SMTP relay address used to forward emails (e.g. "127.0.0.1:25").
	// Leave empty to disable forwarding.
	RelayAddr string
	// Hostname is the EHLO name sent to the relay. Defaults to the server domain.
	Hostname string
}

// NewBackend creates a new SMTP backend. Pass extensions.NoopUsageTracker{} in
// OSS builds; commercial overlays inject a billing-backed tracker.
func NewBackend(domainUseCase *domainUseCase.UseCase, emailUseCase *email.UseCase, smtpStatsUseCase *smtp_stats.UseCase, usageTracker extensions.UsageTracker, logger *slog.Logger) *Backend {
	// Create verifier with default configuration
	verifierConfig := verification.DefaultConfig()
	verifier := verification.NewVerifier(verifierConfig, logger)

	// Create metrics collector
	metricsConfig := DefaultSMTPMetricsConfig()
	metricsConfig.Logger = logger
	metrics := NewSMTPMetrics(metricsConfig)

	return &Backend{
		domainUseCase:    domainUseCase,
		emailUseCase:     emailUseCase,
		smtpStatsUseCase: smtpStatsUseCase,
		usageTracker:     usageTracker,
		verifier:         verifier,
		metrics:          metrics,
		logger:           logger,
	}
}

// ConfigureForwarding attaches a Forwarder to the email use case so that incoming
// emails with forwarding enabled are relayed after being stored.
func (b *Backend) ConfigureForwarding(cfg ForwardingConfig) {
	if cfg.RelayAddr == "" {
		b.logger.Info("Email forwarding disabled (no relay address configured)")
		return
	}
	forwarder := NewForwarder(cfg.RelayAddr, cfg.Hostname, b.logger)
	b.emailUseCase.SetEmailForwarder(forwarder)
	b.logger.Info("Email forwarding configured", "relay_addr", cfg.RelayAddr, "hostname", cfg.Hostname)
}

// NewSession creates a new SMTP session
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	// Record new connection
	remoteIP := b.metrics.GetRemoteIP(c.Conn())
	b.metrics.RecordConnection(remoteIP, "accepted")
	b.metrics.RecordConnectionStart()

	return &Session{
		backend:   b,
		conn:      c,
		logger:    b.logger,
		startTime: time.Now(),
		remoteIP:  remoteIP,
	}, nil
}

// GetMetrics returns the metrics collector for external access
func (b *Backend) GetMetrics() *SMTPMetrics {
	return b.metrics
}
