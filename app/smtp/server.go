package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
)

// Server wraps the SMTP server with our backend
type Server struct {
	server  *smtp.Server
	backend *Backend
	config  Config
	logger  *slog.Logger
}

// NewServer creates a new SMTP server
func NewServer(cfg Config, backend *Backend, logger *slog.Logger) (*Server, error) {
	smtpServer := smtp.NewServer(backend)
	smtpServer.Addr = cfg.Addr
	smtpServer.Domain = cfg.Domain
	smtpServer.ReadTimeout = 60 * time.Second
	smtpServer.WriteTimeout = 60 * time.Second
	smtpServer.MaxMessageBytes = 10 * 1024 * 1024 // 10MB
	smtpServer.MaxRecipients = 10
	smtpServer.AllowInsecureAuth = cfg.TLSMode == TLSModeOff
	//smtpServer.EnableREQUIRETLS = cfg.TLSMode == TLSModeCert

	// Route internal server errors to slog
	smtpServer.ErrorLog = &smtpSlogAdapter{logger: logger.With("component", "smtp_server")}
	// If debug enabled, capture protocol dialog
	if cfg.Debug {
		smtpServer.Debug = &slogWriter{logger: logger.With("component", "smtp_protocol")}
	}

	// Configure TLS based on mode
	tlsConfig, err := setupTLS(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("setting up TLS: %w", err)
	}

	if tlsConfig != nil {
		smtpServer.TLSConfig = tlsConfig
		cfg.TLSConfig = tlsConfig
	}

	return &Server{
		server:  smtpServer,
		backend: backend,
		config:  cfg,
		logger:  logger,
	}, nil
}

// setupTLS configures TLS based on the specified mode
func setupTLS(cfg Config, logger *slog.Logger) (*tls.Config, error) {
	switch cfg.TLSMode {
	case TLSModeOff:
		logger.Info("TLS disabled")
		return nil, nil

	case TLSModeCert:
		tlsConfig, err := setupCertTLS(cfg, logger)
		return tlsConfig, err

	default:
		return nil, fmt.Errorf("unknown TLS mode: %s", cfg.TLSMode)
	}
}

// setupCertTLS configures TLS using certificate files
func setupCertTLS(cfg Config, logger *slog.Logger) (*tls.Config, error) {
	if cfg.TLSCert == "" || cfg.TLSKey == "" {
		return nil, fmt.Errorf("TLS certificate and key files must be specified when using cert mode")
	}

	cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("loading TLS certificate: %w", err)
	}

	logger.Info("TLS configured with certificate files",
		"cert", cfg.TLSCert,
		"key", cfg.TLSKey,
		"domain", cfg.Domain)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   cfg.Domain,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// Start starts the SMTP server
func (s *Server) Start() error {
	s.logger.Info("Starting SMTP server",
		"addr", s.config.Addr,
		"domain", s.config.Domain,
		"tls_mode", s.config.TLSMode,
		"tls_enabled", s.server.TLSConfig != nil,
	)

	// When TLS mode is cert: choose implicit TLS or plaintext with STARTTLS
	if s.config.TLSMode == TLSModeCert {
		if s.config.TLSImplicit {
			s.logger.Info("Starting implicit TLS listener", "addr", s.server.Addr)
			listener, err := tls.Listen("tcp", s.server.Addr, s.server.TLSConfig)
			if err != nil {
				return fmt.Errorf("failed to start implicit TLS listener: %w", err)
			}
			return s.server.Serve(listener)
		}
		s.logger.Info("Starting plaintext listener with STARTTLS", "addr", s.server.Addr)
		// In this mode, go-smtp offers STARTTLS using s.server.TLSConfig
		return s.server.ListenAndServe()
	}

	// For TLS mode off, use regular listener
	return s.server.ListenAndServe()
}

// Stop gracefully stops the SMTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping SMTP server...")

	var lastErr error

	err := s.server.Shutdown(ctx)
	if err != nil {
		s.logger.Error("error stopping SMTP server",
			slog.String("error", err.Error()),
		)
		lastErr = err
	} else {
		s.logger.Info("SMTP server stopped")
	}

	return lastErr
}

// smtpSlogAdapter adapts slog to go-smtp Logger interface
type smtpSlogAdapter struct{ logger *slog.Logger }

func (a *smtpSlogAdapter) Printf(format string, v ...interface{}) {
	a.logger.Error(fmt.Sprintf(format, v...))
}
func (a *smtpSlogAdapter) Println(v ...interface{}) { a.logger.Error(fmt.Sprint(v...)) }

// slogWriter writes debug bytes to slog as debug lines
type slogWriter struct{ logger *slog.Logger }

func (w *slogWriter) Write(p []byte) (int, error) {
	// Trim and split to avoid giant single-line entries
	msg := strings.TrimSpace(string(p))
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		w.logger.Debug("SMTP", "line", line)
	}
	return len(p), nil
}
