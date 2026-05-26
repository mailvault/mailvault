// Package smtprelay submits outbound mail to a local (or user-configured) SMTP
// relay. It is the OSS default implementation of domain/email_sending.Sender —
// each deployment points it at the host's MTA (Postfix / sendmail / a smart
// host) and lets that MTA handle queueing, retry, DKIM, etc.
package smtprelay

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/mailvault/mailvault/domain/entities"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
)

// TLSMode controls how the client negotiates TLS with the relay.
type TLSMode string

const (
	TLSModeNone     TLSMode = "none"     // plain TCP (the default; fine for localhost:25)
	TLSModeSTARTTLS TLSMode = "starttls" // submit on 587 and upgrade with STARTTLS
	TLSModeImplicit TLSMode = "implicit" // submit on 465 with TLS from the first byte
)

// Config configures the relay connection.
type Config struct {
	// Addr is the relay address, e.g. "localhost:25" or "smtp.example.com:587".
	Addr string
	// Hostname is sent in the EHLO greeting.
	Hostname string
	// TLSMode controls TLS negotiation; default TLSModeNone.
	TLSMode TLSMode
	// Username/Password are used for PLAIN/LOGIN auth when non-empty.
	Username string
	Password string
	// InsecureSkipVerify skips TLS certificate verification (test/dev only).
	InsecureSkipVerify bool
}

// Sender is the SMTP-relay implementation of domain/email_sending.Sender.
type Sender struct {
	cfg    Config
	logger *slog.Logger
}

// New builds a Sender. Sensible defaults: empty Addr → localhost:25, empty
// Hostname → localhost.
func New(cfg Config, logger *slog.Logger) *Sender {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:25"
	}
	if cfg.Hostname == "" {
		cfg.Hostname = "localhost"
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = TLSModeNone
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Sender{cfg: cfg, logger: logger}
}

// Send submits msg to the configured relay. It returns the assigned message-id
// (echoes msg.MessageID if set, otherwise the relay's queue-id is not surfaced).
func (s *Sender) Send(ctx context.Context, msg *entities.SentEmail) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("nil message")
	}
	if msg.FromAddress == "" {
		return "", fmt.Errorf("from address is required")
	}
	recipients := append([]string{}, msg.ToAddresses...)
	recipients = append(recipients, msg.CCAddresses...)
	recipients = append(recipients, msg.BCCAddresses...)
	if len(recipients) == 0 {
		return "", fmt.Errorf("at least one recipient is required")
	}

	body := s.buildMessage(msg)

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := s.dial(dialCtx)
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", s.cfg.Addr, err)
	}

	var client *gosmtp.Client
	if s.cfg.TLSMode == TLSModeSTARTTLS {
		// NewClientStartTLS sends EHLO, then STARTTLS, then re-EHLO.
		client, err = gosmtp.NewClientStartTLS(conn, s.tlsConfig())
		if err != nil {
			_ = conn.Close()
			return "", fmt.Errorf("STARTTLS: %w", err)
		}
	} else {
		client = gosmtp.NewClient(conn)
	}
	client.CommandTimeout = 30 * time.Second
	client.SubmissionTimeout = 60 * time.Second
	defer func() { _ = client.Close() }()

	// For non-STARTTLS connections we still need to send EHLO ourselves;
	// NewClientStartTLS already did this above.
	if s.cfg.TLSMode != TLSModeSTARTTLS {
		if err := client.Hello(s.cfg.Hostname); err != nil {
			return "", fmt.Errorf("EHLO: %w", err)
		}
	}

	if s.cfg.Username != "" {
		if err := client.Auth(sasl.NewPlainClient("", s.cfg.Username, s.cfg.Password)); err != nil {
			return "", fmt.Errorf("AUTH: %w", err)
		}
	}

	if err := client.Mail(msg.FromAddress, nil); err != nil {
		return "", fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt, nil); err != nil {
			return "", fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("DATA: %w", err)
	}
	if _, err := wc.Write(body); err != nil {
		_ = wc.Close()
		return "", fmt.Errorf("writing message body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("closing DATA: %w", err)
	}

	if err := client.Quit(); err != nil {
		s.logger.Warn("QUIT failed", "error", err)
	}
	return msg.MessageID, nil
}

func (s *Sender) dial(ctx context.Context) (net.Conn, error) {
	d := &net.Dialer{}
	if s.cfg.TLSMode == TLSModeImplicit {
		return tls.DialWithDialer(d, "tcp", s.cfg.Addr, s.tlsConfig())
	}
	return d.DialContext(ctx, "tcp", s.cfg.Addr)
}

func (s *Sender) tlsConfig() *tls.Config {
	host := s.cfg.Addr
	if h, _, err := net.SplitHostPort(s.cfg.Addr); err == nil {
		host = h
	}
	return &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: s.cfg.InsecureSkipVerify,
	}
}

// buildMessage constructs the RFC 5322 message body. It uses msg.MessageID for
// the Message-ID header and falls back to a generated one when empty.
func (s *Sender) buildMessage(msg *entities.SentEmail) []byte {
	var buf bytes.Buffer
	now := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")

	fmt.Fprintf(&buf, "From: %s\r\n", msg.FromAddress)
	if len(msg.ToAddresses) > 0 {
		fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(quoteAddrList(msg.ToAddresses), ", "))
	}
	if len(msg.CCAddresses) > 0 {
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(quoteAddrList(msg.CCAddresses), ", "))
	}
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", now)
	if msg.MessageID != "" {
		fmt.Fprintf(&buf, "Message-ID: <%s>\r\n", msg.MessageID)
	}
	buf.WriteString("MIME-Version: 1.0\r\n")

	html := derefStr(msg.HTMLBody)
	text := derefStr(msg.TextBody)
	switch {
	case html != "" && text != "":
		boundary := "mv-bndy-" + msg.ID.String()
		fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
		fmt.Fprintf(&buf, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, text)
		fmt.Fprintf(&buf, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n", boundary, html)
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	case html != "":
		buf.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
		buf.WriteString(html)
	default:
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		buf.WriteString(text)
	}
	return buf.Bytes()
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func quoteAddrList(addrs []string) []string {
	out := make([]string, len(addrs))
	for i, a := range addrs {
		if parsed, err := mail.ParseAddress(a); err == nil {
			out[i] = parsed.String()
		} else {
			out[i] = a
		}
	}
	return out
}

