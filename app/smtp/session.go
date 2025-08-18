package smtp

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

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

	// Parse email headers to extract subject and from address
	subject, fromAddr := s.parseEmailHeaders(string(body))

	// Encrypt the email body using the domain's public key
	encryptedBody, err := s.encryptEmailBody(body, domain.PublicKey)
	if err != nil {
		s.logger.Error("Failed to encrypt email body", "domain", domainName, "error", err)
		return &smtp.SMTPError{Code: 451, Message: "Temporary failure processing email"}
	}

	// Process the incoming email with encrypted body
	err = s.backend.emailUseCase.ProcessIncomingEmail(context.Background(), email.ProcessIncomingEmailInput{
		EmailAddressID: emailAddress.ID,
		FromAddress:    fromAddr,
		Subject:        subject,
		Body:           encryptedBody,
		DomainID:       domain.ID,
	})

	if err != nil {
		s.logger.Error("Failed to process incoming email", "error", err)
		return err
	}

	s.logger.Info("Email processed successfully", "recipient", recipient, "from", fromAddr, "subject", subject)
	return nil
}

// parseEmailHeaders extracts basic information from email headers
func (s *Session) parseEmailHeaders(body string) (subject, from string) {
	lines := strings.SplitSeq(body, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
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
