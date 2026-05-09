package smtp

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

// Forwarder relays incoming emails to configured forward addresses using SMTP.
// It implements the email.EmailForwarder interface.
type Forwarder struct {
	// smtpAddr is the address of the SMTP relay to use for forwarding.
	// Defaults to "localhost:25" if empty.
	smtpAddr string
	// hostname is the EHLO name used when connecting to the relay.
	hostname string
	logger   *slog.Logger
}

// NewForwarder creates a Forwarder that relays mail via smtpAddr (e.g. "127.0.0.1:25").
// hostname is used in the EHLO greeting and as the sender domain in forwarded messages.
func NewForwarder(smtpAddr, hostname string, logger *slog.Logger) *Forwarder {
	if smtpAddr == "" {
		smtpAddr = "localhost:25"
	}
	if hostname == "" {
		hostname = "localhost"
	}
	return &Forwarder{
		smtpAddr: smtpAddr,
		hostname: hostname,
		logger:   logger,
	}
}

// ForwardEmail connects to the configured SMTP relay and sends the forwarded message
// to each address in forwardTo. Each delivery failure is logged but does not prevent
// delivery to remaining recipients.
func (f *Forwarder) ForwardEmail(ctx context.Context, fromAddr, originalRecipient, subject string, forwardTo []string) error {
	if len(forwardTo) == 0 {
		return nil
	}

	// Build the forwarded message body with forwarding headers.
	msgBytes := f.buildForwardMessage(fromAddr, originalRecipient, subject, forwardTo)

	var errs []string
	for _, recipient := range forwardTo {
		if err := f.sendToRecipient(ctx, fromAddr, recipient, msgBytes); err != nil {
			f.logger.Warn("Email forwarding failed",
				"from", fromAddr,
				"original_recipient", originalRecipient,
				"forward_to", recipient,
				"error", err,
			)
			errs = append(errs, fmt.Sprintf("%s: %v", recipient, err))
		} else {
			f.logger.Info("Email forwarded successfully",
				"from", fromAddr,
				"original_recipient", originalRecipient,
				"forward_to", recipient,
			)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("forwarding errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// sendToRecipient dials the SMTP relay and delivers a single forwarded message.
func (f *Forwarder) sendToRecipient(ctx context.Context, fromAddr, recipient string, msgBytes []byte) error {
	// Respect context cancellation when dialling.
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", f.smtpAddr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", f.smtpAddr, err)
	}

	client := gosmtp.NewClient(conn)
	client.CommandTimeout = 30 * time.Second
	client.SubmissionTimeout = 60 * time.Second

	// Send EHLO.
	if err := client.Hello(f.hostname); err != nil {
		_ = client.Close()
		return fmt.Errorf("EHLO: %w", err)
	}

	// MAIL FROM – use a bounce address so bounces return to us, not the original sender.
	bounceFrom := "mailvault-forward@" + f.hostname
	if err := client.Mail(bounceFrom, nil); err != nil {
		_ = client.Close()
		return fmt.Errorf("MAIL FROM: %w", err)
	}

	// RCPT TO.
	if err := client.Rcpt(recipient, nil); err != nil {
		_ = client.Close()
		return fmt.Errorf("RCPT TO %s: %w", recipient, err)
	}

	// DATA.
	wc, err := client.Data()
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("DATA: %w", err)
	}

	if _, err := wc.Write(msgBytes); err != nil {
		_ = wc.Close()
		_ = client.Close()
		return fmt.Errorf("writing message: %w", err)
	}

	if err := wc.Close(); err != nil {
		_ = client.Close()
		return fmt.Errorf("closing DATA: %w", err)
	}

	return client.Quit()
}

// buildForwardMessage constructs a minimal RFC-5322 message with forwarding headers.
// It does not attempt to parse or modify the original encrypted body; instead it
// composes a plain notification message so that the forward recipient knows an email
// was received and can retrieve it via the API.
func (f *Forwarder) buildForwardMessage(fromAddr, originalRecipient, subject string, forwardTo []string) []byte {
	var buf bytes.Buffer

	now := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")

	buf.WriteString(fmt.Sprintf("From: MailVault Forwarder <mailvault-forward@%s>\r\n", f.hostname))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(forwardTo, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: Fwd: %s\r\n", subject))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", now))
	buf.WriteString(fmt.Sprintf("X-Forwarded-For: %s\r\n", originalRecipient))
	buf.WriteString("X-Forwarded-By: MailVault\r\n")
	buf.WriteString("X-Original-From: " + fromAddr + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(fmt.Sprintf("This is an automated notification from MailVault.\r\n\r\n"))
	buf.WriteString(fmt.Sprintf("A new email has been received at %s.\r\n\r\n", originalRecipient))
	buf.WriteString(fmt.Sprintf("  From:    %s\r\n", fromAddr))
	buf.WriteString(fmt.Sprintf("  Subject: %s\r\n\r\n", subject))
	buf.WriteString("The original message is stored encrypted in your MailVault inbox.\r\n")
	buf.WriteString("Log in to the MailVault dashboard or use the CLI to read it.\r\n")

	return buf.Bytes()
}
