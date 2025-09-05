package smtp

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/email"
	"mailvault/domain/entities"
	"mailvault/internal/encryption"

	"github.com/emersion/go-smtp"
	"github.com/gofrs/uuid/v5"
)

// Session represents an SMTP session
type Session struct {
	backend *Backend
	conn    *smtp.Conn
	logger  *slog.Logger
	from    string
	to      []string
}

// AuthPlain handles PLAIN authentication (optional)
func (s *Session) AuthPlain(username, password string) error {
	// For now, we don't require authentication for receiving emails
	// This could be extended to support authenticated relaying
	return nil
}

// Mail sets the return path for the email
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.logger.Info("SMTP Mail from", "from", from)
	s.from = from
	return nil
}

// Rcpt sets a recipient for the email
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.logger.Info("SMTP Rcpt to", "to", to)
	s.to = append(s.to, to)
	return nil
}

// Data receives the email data
func (s *Session) Data(r io.Reader) error {
	// Read the entire email body
	body, err := io.ReadAll(r)
	if err != nil {
		s.logger.Error("Failed to read email body", "error", err)
		return err
	}

	s.logger.Info("SMTP Data received", "from", s.from, "to", s.to, "size", len(body))

	// Process each recipient
	for _, recipient := range s.to {
		err := s.processEmail(recipient, body)
		if err != nil {
			s.logger.Error("Failed to process email", "recipient", recipient, "error", err)
			// Continue processing other recipients even if one fails
		}
	}

	return nil
}

// processEmail handles incoming email for a specific recipient
func (s *Session) processEmail(recipient string, body []byte) error {
	// Extract domain from recipient email
	parts := strings.Split(recipient, "@")
	if len(parts) != 2 {
		return &smtp.SMTPError{Code: 550, Message: "Invalid recipient address"}
	}

	localPart := parts[0]
	domainName := parts[1]

	// Get domain configuration
	domain, err := s.backend.domainUseCase.GetDomainByName(context.Background(), domainName)
	if err != nil {
		s.logger.Error("Domain not found", "domain", domainName, "error", err)
		return &smtp.SMTPError{Code: 550, Message: "Domain not configured"}
	}

	// Check if domain is verified
	if !domain.Verified {
		s.logger.Warn("Email rejected: domain not verified", "domain", domainName)
		return &smtp.SMTPError{Code: 550, Message: "Domain not verified"}
	}

	// Parse email headers to extract subject and from address
	subject, fromAddr := s.parseEmailHeaders(string(body))

	// Create email context for verification
	emailCtx := verification.EmailContext{
		From:       fromAddr,
		To:         []string{recipient},
		Subject:    subject,
		Body:       body,
		Headers:    s.extractHeaders(body),
		SenderIP:   s.getSenderIP(),
		ReceivedAt: time.Now(),
	}

	// Perform spam verification
	verificationResult := s.backend.verifier.VerifyEmail(context.Background(), emailCtx)

	// Build Authentication-Results header and attach to the raw message for storage/forwarding
	authResHeader := verification.BuildAuthResultsHeader("mailvault", emailCtx, verificationResult)
	body = prependHeader(body, authResHeader)

	s.logger.Info("Email verification completed",
		"recipient", recipient,
		"from", fromAddr,
		"action", verificationResult.Action.String(),
		"spf", verificationResult.SPF.Result.String(),
		"dkim_valid", verificationResult.DKIM.Valid,
		"dmarc", verificationResult.DMARC.Result.String(),
		"reputation_score", verificationResult.Reputation.Score,
		"content_score", verificationResult.Content.SpamScore,
	)

	// Handle verification result
	switch verificationResult.Action {
	case verification.ActionReject:
		s.logger.Warn("Email rejected by spam filter",
			"recipient", recipient,
			"reason", s.backend.verifier.GetVerificationSummary(verificationResult))
		return &smtp.SMTPError{Code: 550, Message: "Email rejected by spam filter"}

	case verification.ActionTempFail:
		s.logger.Warn("Email temporarily rejected",
			"recipient", recipient,
			"reason", s.backend.verifier.GetVerificationSummary(verificationResult))
		return &smtp.SMTPError{Code: 451, Message: "Temporary failure - try again later"}

	case verification.ActionQuarantine:
		s.logger.Info("Email quarantined",
			"recipient", recipient,
			"reason", s.backend.verifier.GetVerificationSummary(verificationResult))
		// Continue processing but mark as quarantined

	case verification.ActionAccept:
		s.logger.Debug("Email accepted", "recipient", recipient)
		// Continue normal processing
	}

	// Try to find specific email address first
	emailAddress, err := s.backend.emailUseCase.GetEmailAddressByAddress(context.Background(), recipient)
	if err != nil {
		s.logger.Info("Email address not found, checking domain auto-creation policy", "email", recipient, "auto_create_enabled", domain.AutoCreateAddress)

		// Check if domain allows auto-creation of email addresses
		if !domain.AutoCreateAddress {
			s.logger.Warn("Email rejected: address not found and auto-creation disabled", "email", recipient, "domain", domainName)
			return &smtp.SMTPError{Code: 550, Message: "Email address not found"}
		}

		// Auto-create new email address since domain allows it
		emailAddress = &entities.EmailAddress{
			ID:               uuid.Must(uuid.NewV4()),
			DomainID:         domain.ID,
			LocalPart:        localPart,
			ForwardAddresses: []string{},
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err = s.backend.emailUseCase.CreateEmailAddress(context.Background(), emailAddress)
		if err != nil {
			s.logger.Error("Failed to auto-create email address", "email", recipient, "error", err)
			return &smtp.SMTPError{Code: 451, Message: "Temporary failure creating email address"}
		}

		s.logger.Info("Auto-created email address", "email", recipient, "domain", domainName)
	}

	// Encrypt the email body using the domain's public key
	encryptedBody, err := s.encryptEmailBody(body, domain.PublicKey)
	if err != nil {
		s.logger.Error("Failed to encrypt email body", "domain", domainName, "error", err)
		return &smtp.SMTPError{Code: 451, Message: "Temporary failure processing email"}
	}

	// Process the incoming email with encrypted body and verification results
	err = s.backend.emailUseCase.ProcessIncomingEmail(context.Background(), email.ProcessIncomingEmailInput{
		EmailAddressID:      emailAddress.ID,
		FromAddress:         fromAddr,
		Subject:             subject,
		Body:                encryptedBody,
		DomainID:            domain.ID,
		VerificationResults: &verificationResult,
		IsQuarantined:       verificationResult.Action == verification.ActionQuarantine,
	})

	if err != nil {
		s.logger.Error("Failed to process incoming email", "error", err)
		return err
	}

	// Record verification statistics (async, don't fail email processing if stats fail)
	go func() {
		statsCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		err := s.backend.smtpStatsUseCase.RecordVerificationResult(
			statsCtx,
			domain.ID,
			emailAddress.ID,
			s.getSenderIP(),
			fromAddr,
			&verificationResult,
			verificationResult.Action,
		)
		if err != nil {
			s.logger.Warn("Failed to record SMTP verification statistics", 
				"error", err,
				"recipient", recipient,
				"from", fromAddr)
		}
	}()

	s.logger.Info("Email processed successfully", "recipient", recipient, "from", fromAddr, "subject", subject)
	return nil
}

// parseEmailHeaders extracts basic information from email headers
func (s *Session) parseEmailHeaders(body string) (subject, from string) {
	lines := strings.SplitSeq(body, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		slog.Info("parseEmailHeaders", "line", line)
		if line == "" {
			break // End of headers
		}

		if strings.HasPrefix(strings.ToLower(line), "subject:") {
			subject = strings.TrimSpace(line[8:]) // Remove "Subject: "
		} else if strings.HasPrefix(strings.ToLower(line), "from:") {
			from = strings.TrimSpace(line[5:]) // Remove "From: "
		}
	}

	if subject == "" {
		subject = "(No Subject)"
	}
	if from == "" {
		from = "(Unknown Sender)"
	}

	return subject, from
}

// encryptEmailBody encrypts the email body using the domain's public key
func (s *Session) encryptEmailBody(body []byte, domainPublicKey string) (string, error) {
	// Parse the domain's public key
	publicKeyBytes, err := encryption.ParsePublicKey(domainPublicKey)
	if err != nil {
		return "", err
	}

	// Encrypt the email body
	encryptedData, err := encryption.Encrypt(body, publicKeyBytes)
	if err != nil {
		return "", err
	}

	// Serialize the encrypted data for storage
	serializedData, err := encryptedData.Serialize()
	if err != nil {
		return "", err
	}

	return serializedData, nil
}

// Reset resets the session state
func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

// Logout ends the session
func (s *Session) Logout() error {
	return nil
}

// extractHeaders extracts headers from email body for verification
func (s *Session) extractHeaders(body []byte) []verification.Header {
	var headers []verification.Header
	bodyStr := string(body)

	// Find headers section (before first empty line)
	lines := strings.Split(bodyStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}

		if colonIdx := strings.Index(line, ":"); colonIdx != -1 {
			name := strings.TrimSpace(line[:colonIdx])
			value := strings.TrimSpace(line[colonIdx+1:])
			headers = append(headers, verification.Header{
				Name:  name,
				Value: value,
			})
		}
	}

	return headers
}

// prependHeader adds a header line at the top of the message, preserving CRLF separation.
func prependHeader(body []byte, headerLine string) []byte {
	// Ensure header line ends with CRLF
	if !strings.HasSuffix(headerLine, "\r\n") {
		headerLine += "\r\n"
	}

	// Insert before the blank line separating headers and body
	// Find first occurrence of CRLFCRLF or \n\n
	if idx := strings.Index(string(body), "\r\n\r\n"); idx != -1 {
		// Place header before the first blank line
		return append([]byte(headerLine), body...)
	}

	if idx := strings.Index(string(body), "\n\n"); idx != -1 {
		return append([]byte(headerLine), body...)
	}

	// If no header/body delimiter found, just prefix
	return append([]byte(headerLine), body...)
}

// getSenderIP extracts sender IP from SMTP connection
func (s *Session) getSenderIP() net.IP {
	if s.conn == nil {
		return nil
	}

	// Get remote address from connection
	remoteAddr := s.conn.Conn().RemoteAddr()
	if remoteAddr == nil {
		return nil
	}

	// Parse IP from address
	addrStr := remoteAddr.String()
	if host, _, err := net.SplitHostPort(addrStr); err == nil {
		return net.ParseIP(host)
	}

	return net.ParseIP(addrStr)
}
