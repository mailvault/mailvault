package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	domainUseCase "mailsafe/domain/domain"
	"mailsafe/domain/email"

	"github.com/ardanlabs/conf/v3"
	"github.com/emersion/go-smtp"
)

// Server wraps the SMTP server with our backend
type Server struct {
	server        *smtp.Server
	backend       *Backend
	config        Config
	logger        *slog.Logger
	domainUseCase *domainUseCase.UseCase
	emailUseCase  *email.UseCase
}

// NewServer creates a new SMTP server
func NewServer(domainUseCase *domainUseCase.UseCase, emailUseCase *email.UseCase, logger *slog.Logger) (*Server, error) {
	var cfg Config
	_, err := conf.Parse("", &cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing smtp config: %w", err)
	}

	backend := NewBackend(domainUseCase, emailUseCase, logger)

	smtpServer := smtp.NewServer(backend)
	smtpServer.Addr = cfg.Addr
	smtpServer.Domain = cfg.Domain
	smtpServer.ReadTimeout = 60 * time.Second
	smtpServer.WriteTimeout = 60 * time.Second
	smtpServer.MaxMessageBytes = 10 * 1024 * 1024 // 10MB
	smtpServer.MaxRecipients = 10
	smtpServer.AllowInsecureAuth = true

	// Configure TLS if provided
	if cfg.TLSConfig != nil {
		smtpServer.TLSConfig = cfg.TLSConfig
	} else if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, err
		}

		smtpServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   cfg.Domain,
		}
	}

	return &Server{
		server:        smtpServer,
		backend:       backend,
		config:        cfg,
		logger:        logger,
		domainUseCase: domainUseCase,
		emailUseCase:  emailUseCase,
	}, nil
}

// Start starts the SMTP server
func (s *Server) Start() error {
	s.logger.Info("Starting SMTP server",
		"addr", s.config.Addr,
		"domain", s.config.Domain,
		"tls", s.server.TLSConfig != nil,
	)

	return s.server.ListenAndServe()
}

// Stop gracefully stops the SMTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping SMTP server...")

	// The go-smtp server doesn't have a graceful shutdown method,
	// so we just close it directly
	err := s.server.Close()
	if err != nil {
		s.logger.Error("Error stopping SMTP server", "error", err)
	} else {
		s.logger.Info("SMTP server stopped")
	}

	return err
}
