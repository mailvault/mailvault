package webhook

import (
	"encoding/json"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// IncomingEmailEvent represents the webhook payload for received emails
type IncomingEmailEvent struct {
	// Event metadata
	EventType string    `json:"event_type"` // Always "email.received"
	EventID   uuid.UUID `json:"event_id"`   // Unique ID for this webhook event
	Timestamp time.Time `json:"timestamp"`  // When the email was received

	// Email metadata
	Email EmailMetadata `json:"email"`

	// Recipient information
	Recipient RecipientInfo `json:"recipient"`

	// Security and verification information
	Security SecurityInfo `json:"security"`

	// Domain information
	Domain DomainInfo `json:"domain"`
}

// EmailMetadata contains metadata about the received email
type EmailMetadata struct {
	ID               uuid.UUID `json:"id"`                         // Received email ID
	SequenceNumber   int       `json:"sequence_number"`            // Sequence number for this recipient
	FromAddress      string    `json:"from_address"`               // Sender email address
	Subject          string    `json:"subject"`                    // Email subject (empty string if no subject)
	EncryptedBody    string    `json:"encrypted_body"`             // Encrypted email body (base64 encoded)
	ReceivedAt       time.Time `json:"received_at"`                // When email was received
	Size             int       `json:"size,omitempty"`             // Email size in bytes
	MessageID        string    `json:"message_id,omitempty"`       // Email Message-ID header
	InReplyTo        string    `json:"in_reply_to,omitempty"`      // In-Reply-To header
	References       string    `json:"references,omitempty"`       // References header
	IsQuarantined    bool      `json:"is_quarantined"`             // Whether email was quarantined
}

// RecipientInfo contains information about the email recipient
type RecipientInfo struct {
	EmailAddress   string    `json:"email_address"`   // Full recipient email address
	LocalPart      string    `json:"local_part"`      // Local part (before @)
	DomainName     string    `json:"domain_name"`     // Domain name (after @)
	AddressID      uuid.UUID `json:"address_id"`      // Email address ID
	AutoCreated    bool      `json:"auto_created"`    // Whether address was auto-created for this email
}

// SecurityInfo contains security and verification information
type SecurityInfo struct {
	// Overall verification result
	VerificationAction string  `json:"verification_action"` // accept, reject, quarantine, temp_fail
	SpamScore          float64 `json:"spam_score"`           // Content spam score (0-1)
	ReputationScore    float64 `json:"reputation_score"`     // Sender reputation score (0-1)

	// SPF verification
	SPF SPFResult `json:"spf"`

	// DKIM verification
	DKIM DKIMResult `json:"dkim"`

	// DMARC verification
	DMARC DMARCResult `json:"dmarc"`

	// Sender information
	SenderIP string `json:"sender_ip,omitempty"` // IP address of sender
}

// SPFResult contains SPF verification results
type SPFResult struct {
	Result      string `json:"result"`               // pass, fail, softfail, neutral, none, temperror, permerror
	Domain      string `json:"domain,omitempty"`     // Domain checked
	Explanation string `json:"explanation,omitempty"` // SPF explanation if any
}

// DKIMResult contains DKIM verification results
type DKIMResult struct {
	Valid    bool   `json:"valid"`              // Whether DKIM signature is valid
	Domain   string `json:"domain,omitempty"`   // DKIM domain
	Selector string `json:"selector,omitempty"` // DKIM selector
	Error    string `json:"error,omitempty"`    // DKIM error message if any
}

// DMARCResult contains DMARC verification results
type DMARCResult struct {
	Result       string `json:"result"`                 // pass, fail, temperror, permerror
	Policy       string `json:"policy"`                 // none, quarantine, reject
	Disposition  string `json:"disposition,omitempty"`  // Action taken based on policy
	AlignmentSPF bool   `json:"alignment_spf"`          // SPF alignment result
	AlignmentDKIM bool  `json:"alignment_dkim"`         // DKIM alignment result
}

// DomainInfo contains information about the receiving domain
type DomainInfo struct {
	ID              uuid.UUID `json:"id"`               // Domain ID
	Name            string    `json:"name"`             // Domain name
	StorageEnabled  bool      `json:"storage_enabled"`  // Whether storage is enabled
	AutoCreateEmail bool      `json:"auto_create_email"` // Whether auto-creation is enabled
}

// NewIncomingEmailEvent creates a new incoming email webhook event
func NewIncomingEmailEvent(receivedEmail *entities.ReceivedEmail, domain *entities.Domain, emailAddress *entities.EmailAddress, verificationResult *verification.VerificationResult, autoCreated bool) *IncomingEmailEvent {
	event := &IncomingEmailEvent{
		EventType: "email.received",
		EventID:   uuid.Must(uuid.NewV4()),
		Timestamp: receivedEmail.ReceivedAt,
		Email: EmailMetadata{
			ID:             receivedEmail.ID,
			SequenceNumber: receivedEmail.SequenceNumber,
			FromAddress:    receivedEmail.FromAddress,
			Subject:        receivedEmail.GetSubjectString(),
			EncryptedBody:  receivedEmail.EncryptedBody,
			ReceivedAt:     receivedEmail.ReceivedAt,
			IsQuarantined:  false, // Will be set from verification result
		},
		Recipient: RecipientInfo{
			EmailAddress: receivedEmail.EmailAddress,
			DomainName:   receivedEmail.DomainName,
			AutoCreated:  autoCreated,
		},
		Domain: DomainInfo{
			ID:              domain.ID,
			Name:            domain.Domain,
			StorageEnabled:  domain.StorageEnabled,
			AutoCreateEmail: domain.AutoCreateAddress,
		},
	}

	// Set recipient info from email address
	if emailAddress != nil {
		event.Recipient.AddressID = emailAddress.ID
		event.Recipient.LocalPart = emailAddress.LocalPart
	}

	// Set security information from verification result
	if verificationResult != nil {
		event.Security = SecurityInfo{
			VerificationAction: verificationResult.Action.String(),
			SpamScore:          verificationResult.Content.SpamScore,
			ReputationScore:    verificationResult.Reputation.Score,
			SPF: SPFResult{
				Result:      verificationResult.SPF.Result.String(),
				Domain:      verificationResult.SPF.Mechanism, // Use mechanism as domain info
				Explanation: verificationResult.SPF.Error,     // Use error as explanation
			},
			DKIM: DKIMResult{
				Valid:    verificationResult.DKIM.Valid,
				Domain:   extractDKIMDomain(verificationResult.DKIM.Results),
				Selector: extractDKIMSelector(verificationResult.DKIM.Results),
				Error:    verificationResult.DKIM.Error,
			},
			DMARC: DMARCResult{
				Result:        verificationResult.DMARC.Result.String(),
				Policy:        verificationResult.DMARC.Policy,
				Disposition:   "", // Not available in DMARC result
				AlignmentSPF:  verificationResult.DMARC.SPFAlign,
				AlignmentDKIM: verificationResult.DMARC.DKIMAlign,
			},
		}

		// Set quarantine status
		event.Email.IsQuarantined = verificationResult.Action == verification.ActionQuarantine
	}

	return event
}

// ToJSON serializes the event to JSON
func (e *IncomingEmailEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// Validate validates the webhook event
func (e *IncomingEmailEvent) Validate() error {
	if e.EventType != "email.received" {
		return ErrInvalidEventType
	}

	if e.EventID == uuid.Nil {
		return ErrMissingEventID
	}

	if e.Email.ID == uuid.Nil {
		return ErrMissingEmailID
	}

	if e.Email.FromAddress == "" {
		return ErrMissingFromAddress
	}

	if e.Email.EncryptedBody == "" {
		return ErrMissingEncryptedBody
	}

	if e.Recipient.EmailAddress == "" {
		return ErrMissingRecipientAddress
	}

	if e.Recipient.AddressID == uuid.Nil {
		return ErrMissingRecipientAddressID
	}

	if e.Domain.ID == uuid.Nil {
		return ErrMissingDomainID
	}

	if e.Domain.Name == "" {
		return ErrMissingDomainName
	}

	return nil
}

// GetEmailSize calculates the approximate size of the email
func (e *IncomingEmailEvent) GetEmailSize() int {
	// Estimate size based on encrypted body length
	// This is an approximation since the actual email might be larger
	return len(e.Email.EncryptedBody)
}

// IsSpam returns true if the email is likely spam based on scores
func (e *IncomingEmailEvent) IsSpam() bool {
	// Consider spam if spam score > 0.7 or reputation score < 0.3
	return e.Security.SpamScore > 0.7 || e.Security.ReputationScore < 0.3
}

// IsSecure returns true if the email passed all security checks
func (e *IncomingEmailEvent) IsSecure() bool {
	return e.Security.VerificationAction == "accept" &&
		e.Security.SPF.Result == "pass" &&
		e.Security.DKIM.Valid &&
		e.Security.DMARC.Result == "pass"
}

// GetShortDescription returns a short description for logging
func (e *IncomingEmailEvent) GetShortDescription() string {
	return "Email from " + e.Email.FromAddress + " to " + e.Recipient.EmailAddress + " (" + e.Email.Subject + ")"
}

// extractDKIMDomain extracts the first DKIM domain from signature results
func extractDKIMDomain(results []verification.DKIMSignatureResult) string {
	if len(results) > 0 {
		return results[0].Domain
	}
	return ""
}

// extractDKIMSelector extracts the first DKIM selector from signature results
func extractDKIMSelector(results []verification.DKIMSignatureResult) string {
	if len(results) > 0 {
		return results[0].Selector
	}
	return ""
}