package smtp

import (
	"log/slog"

	"mailsafe/domain/email"
	domainUseCase "mailsafe/domain/domain"

	"github.com/emersion/go-smtp"
)

// Backend implements SMTP server backend
type Backend struct {
	domainUseCase *domainUseCase.UseCase
	emailUseCase  *email.UseCase
	logger        *slog.Logger
}

// NewBackend creates a new SMTP backend
func NewBackend(domainUseCase *domainUseCase.UseCase, emailUseCase *email.UseCase, logger *slog.Logger) *Backend {
	return &Backend{
		domainUseCase: domainUseCase,
		emailUseCase:  emailUseCase,
		logger:        logger,
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