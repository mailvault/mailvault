package smtp

import (
	"log/slog"

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
	logger           *slog.Logger
}

// NewBackend creates a new SMTP backend
func NewBackend(domainUseCase *domainUseCase.UseCase, emailUseCase *email.UseCase, smtpStatsUseCase *smtp_stats.UseCase, logger *slog.Logger) *Backend {
	// Create verifier with default configuration
	verifierConfig := verification.DefaultConfig()
	verifier := verification.NewVerifier(verifierConfig, logger)

	return &Backend{
		domainUseCase:    domainUseCase,
		emailUseCase:     emailUseCase,
		smtpStatsUseCase: smtpStatsUseCase,
		verifier:         verifier,
		logger:           logger,
	}
}

// NewSession creates a new SMTP session
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		backend: b,
		conn:    c,
		logger:  b.logger,
	}, nil
}
