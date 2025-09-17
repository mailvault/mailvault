package smtp

import (
	"log/slog"
	"time"

	"mailvault/app/smtp/verification"
	domainUseCase "mailvault/domain/domain"
	"mailvault/domain/email"
	"mailvault/domain/smtp_stats"

	"github.com/emersion/go-smtp"
)

// Backend implements SMTP server backend
type Backend struct {
	domainUseCase    *domainUseCase.UseCase
	emailUseCase     *email.UseCase
	smtpStatsUseCase *smtp_stats.UseCase
	verifier         *verification.Verifier
	metrics          *SMTPMetrics
	logger           *slog.Logger
}

// NewBackend creates a new SMTP backend
func NewBackend(domainUseCase *domainUseCase.UseCase, emailUseCase *email.UseCase, smtpStatsUseCase *smtp_stats.UseCase, logger *slog.Logger) *Backend {
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
		verifier:         verifier,
		metrics:          metrics,
		logger:           logger,
	}
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
