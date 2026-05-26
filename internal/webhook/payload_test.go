package webhook

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mailvault/mailvault/app/smtp/verification"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIncomingEmailEvent(t *testing.T) {
	// Create test data
	domain := &entities.Domain{
		ID:                uuid.Must(uuid.NewV4()),
		Domain:            "example.com",
		StorageEnabled:    true,
		AutoCreateAddress: false,
	}

	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		DomainID:  domain.ID,
		LocalPart: "test",
	}

	subject := "Test Email"
	receivedEmail := &entities.ReceivedEmail{
		ID:             uuid.Must(uuid.NewV4()),
		EmailAddressID: &emailAddress.ID,
		SequenceNumber: 1,
		FromAddress:    "sender@example.org",
		Subject:        &subject,
		EncryptedBody:  "encrypted-content",
		DomainName:     domain.Domain,
		EmailAddress:   "test@example.com",
		ReceivedAt:     time.Now(),
	}

	verificationResult := &verification.VerificationResult{
		Action: verification.ActionAccept,
		Content: verification.ContentResult{
			SpamScore: 0.1,
		},
		Reputation: verification.ReputationResult{
			Score: 0.9,
		},
		SPF: verification.SPFResult{
			Result:    verification.SPFPass,
			Mechanism: "example.org",
		},
		DKIM: verification.DKIMResult{
			Valid: true,
			Results: []verification.DKIMSignatureResult{
				{
					Domain:   "example.org",
					Selector: "default",
					Status:   verification.DKIMPass,
				},
			},
		},
		DMARC: verification.DMARCResult{
			Result:    verification.DMARCPass,
			Policy:    "none",
			SPFAlign:  true,
			DKIMAlign: true,
		},
	}

	// Test creating event
	event := NewIncomingEmailEvent(receivedEmail, domain, emailAddress, verificationResult, false)

	// Verify basic fields
	assert.Equal(t, "email.received", event.EventType)
	assert.NotEqual(t, uuid.Nil, event.EventID)
	assert.Equal(t, receivedEmail.ReceivedAt, event.Timestamp)

	// Verify email metadata
	assert.Equal(t, receivedEmail.ID, event.Email.ID)
	assert.Equal(t, receivedEmail.SequenceNumber, event.Email.SequenceNumber)
	assert.Equal(t, receivedEmail.FromAddress, event.Email.FromAddress)
	assert.Equal(t, "Test Email", event.Email.Subject)
	assert.Equal(t, receivedEmail.EncryptedBody, event.Email.EncryptedBody)
	assert.Equal(t, receivedEmail.ReceivedAt, event.Email.ReceivedAt)
	assert.False(t, event.Email.IsQuarantined)

	// Verify recipient info
	assert.Equal(t, receivedEmail.EmailAddress, event.Recipient.EmailAddress)
	assert.Equal(t, receivedEmail.DomainName, event.Recipient.DomainName)
	assert.Equal(t, emailAddress.ID, event.Recipient.AddressID)
	assert.Equal(t, emailAddress.LocalPart, event.Recipient.LocalPart)
	assert.False(t, event.Recipient.AutoCreated)

	// Verify security info
	assert.Equal(t, "accept", event.Security.VerificationAction)
	assert.Equal(t, 0.1, event.Security.SpamScore)
	assert.Equal(t, 0.9, event.Security.ReputationScore)
	assert.Equal(t, "pass", event.Security.SPF.Result)
	assert.Equal(t, "example.org", event.Security.SPF.Domain)
	assert.True(t, event.Security.DKIM.Valid)
	assert.Equal(t, "example.org", event.Security.DKIM.Domain)
	assert.Equal(t, "default", event.Security.DKIM.Selector)
	assert.Equal(t, "pass", event.Security.DMARC.Result)
	assert.Equal(t, "none", event.Security.DMARC.Policy)
	assert.True(t, event.Security.DMARC.AlignmentSPF)
	assert.True(t, event.Security.DMARC.AlignmentDKIM)

	// Verify domain info
	assert.Equal(t, domain.ID, event.Domain.ID)
	assert.Equal(t, domain.Domain, event.Domain.Name)
	assert.Equal(t, domain.StorageEnabled, event.Domain.StorageEnabled)
	assert.Equal(t, domain.AutoCreateAddress, event.Domain.AutoCreateEmail)
}

func TestNewIncomingEmailEvent_WithQuarantine(t *testing.T) {
	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
	}

	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "test",
	}

	receivedEmail := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "spam@bad.com",
		EncryptedBody: "encrypted-spam",
		ReceivedAt:    time.Now(),
	}

	verificationResult := &verification.VerificationResult{
		Action: verification.ActionQuarantine,
		Content: verification.ContentResult{
			SpamScore: 0.8,
		},
		Reputation: verification.ReputationResult{
			Score: 0.2,
		},
		SPF: verification.SPFResult{
			Result: verification.SPFPass,
		},
		DKIM: verification.DKIMResult{
			Valid: true,
		},
		DMARC: verification.DMARCResult{
			Result: verification.DMARCPass,
			Policy: "none",
		},
	}

	event := NewIncomingEmailEvent(receivedEmail, domain, emailAddress, verificationResult, true)

	assert.True(t, event.Email.IsQuarantined)
	assert.True(t, event.Recipient.AutoCreated)
	assert.Equal(t, "quarantine", event.Security.VerificationAction)
	assert.Equal(t, 0.8, event.Security.SpamScore)
	assert.Equal(t, 0.2, event.Security.ReputationScore)
}

func TestIncomingEmailEvent_Validate(t *testing.T) {
	validEvent := &IncomingEmailEvent{
		EventType: "email.received",
		EventID:   uuid.Must(uuid.NewV4()),
		Email: EmailMetadata{
			ID:            uuid.Must(uuid.NewV4()),
			FromAddress:   "test@example.com",
			EncryptedBody: "encrypted-content",
		},
		Recipient: RecipientInfo{
			EmailAddress: "recipient@example.com",
			AddressID:    uuid.Must(uuid.NewV4()),
		},
		Domain: DomainInfo{
			ID:   uuid.Must(uuid.NewV4()),
			Name: "example.com",
		},
	}

	// Valid event should pass
	assert.NoError(t, validEvent.Validate())

	// Test validation errors
	tests := []struct {
		name      string
		modify    func(*IncomingEmailEvent)
		expectErr error
	}{
		{
			name: "invalid event type",
			modify: func(e *IncomingEmailEvent) {
				e.EventType = "invalid"
			},
			expectErr: ErrInvalidEventType,
		},
		{
			name: "missing event ID",
			modify: func(e *IncomingEmailEvent) {
				e.EventID = uuid.Nil
			},
			expectErr: ErrMissingEventID,
		},
		{
			name: "missing email ID",
			modify: func(e *IncomingEmailEvent) {
				e.Email.ID = uuid.Nil
			},
			expectErr: ErrMissingEmailID,
		},
		{
			name: "missing from address",
			modify: func(e *IncomingEmailEvent) {
				e.Email.FromAddress = ""
			},
			expectErr: ErrMissingFromAddress,
		},
		{
			name: "missing encrypted body",
			modify: func(e *IncomingEmailEvent) {
				e.Email.EncryptedBody = ""
			},
			expectErr: ErrMissingEncryptedBody,
		},
		{
			name: "missing recipient address",
			modify: func(e *IncomingEmailEvent) {
				e.Recipient.EmailAddress = ""
			},
			expectErr: ErrMissingRecipientAddress,
		},
		{
			name: "missing recipient address ID",
			modify: func(e *IncomingEmailEvent) {
				e.Recipient.AddressID = uuid.Nil
			},
			expectErr: ErrMissingRecipientAddressID,
		},
		{
			name: "missing domain ID",
			modify: func(e *IncomingEmailEvent) {
				e.Domain.ID = uuid.Nil
			},
			expectErr: ErrMissingDomainID,
		},
		{
			name: "missing domain name",
			modify: func(e *IncomingEmailEvent) {
				e.Domain.Name = ""
			},
			expectErr: ErrMissingDomainName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the valid event
			eventCopy := *validEvent
			tt.modify(&eventCopy)

			err := eventCopy.Validate()
			assert.Error(t, err)
			assert.Equal(t, tt.expectErr, err)
		})
	}
}

func TestIncomingEmailEvent_ToJSON(t *testing.T) {
	event := &IncomingEmailEvent{
		EventType: "email.received",
		EventID:   uuid.Must(uuid.NewV4()),
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Email: EmailMetadata{
			ID:            uuid.Must(uuid.NewV4()),
			FromAddress:   "test@example.com",
			Subject:       "Test Email",
			EncryptedBody: "encrypted-content",
			ReceivedAt:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		Recipient: RecipientInfo{
			EmailAddress: "recipient@example.com",
			LocalPart:    "recipient",
			DomainName:   "example.com",
			AddressID:    uuid.Must(uuid.NewV4()),
		},
		Security: SecurityInfo{
			VerificationAction: "accept",
			SpamScore:          0.1,
			ReputationScore:    0.9,
		},
		Domain: DomainInfo{
			ID:   uuid.Must(uuid.NewV4()),
			Name: "example.com",
		},
	}

	jsonData, err := event.ToJSON()
	require.NoError(t, err)

	// Verify JSON is valid
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, "email.received", parsed["event_type"])
	assert.NotEmpty(t, parsed["event_id"])
	assert.NotEmpty(t, parsed["timestamp"])

	// Verify nested objects exist
	assert.NotNil(t, parsed["email"])
	assert.NotNil(t, parsed["recipient"])
	assert.NotNil(t, parsed["security"])
	assert.NotNil(t, parsed["domain"])
}

func TestIncomingEmailEvent_HelperMethods(t *testing.T) {
	event := &IncomingEmailEvent{
		Email: EmailMetadata{
			FromAddress: "test@example.com",
			Subject:     "Test Subject",
		},
		Recipient: RecipientInfo{
			EmailAddress: "recipient@example.com",
		},
		Security: SecurityInfo{
			VerificationAction: "accept",
			SpamScore:          0.3,
			ReputationScore:    0.8,
			SPF:                SPFResult{Result: "pass"},
			DKIM:               DKIMResult{Valid: true},
			DMARC:              DMARCResult{Result: "pass"},
		},
	}

	// Test GetEmailSize
	size := event.GetEmailSize()
	assert.Equal(t, 0, size) // Empty encrypted body

	event.Email.EncryptedBody = "test-content"
	size = event.GetEmailSize()
	assert.Equal(t, len("test-content"), size)

	// Test IsSpam
	assert.False(t, event.IsSpam()) // spam_score=0.3, reputation_score=0.8

	event.Security.SpamScore = 0.8
	assert.True(t, event.IsSpam()) // spam_score=0.8 > 0.7

	event.Security.SpamScore = 0.3
	event.Security.ReputationScore = 0.2
	assert.True(t, event.IsSpam()) // reputation_score=0.2 < 0.3

	// Test IsSecure
	event.Security.SpamScore = 0.1
	event.Security.ReputationScore = 0.9
	assert.True(t, event.IsSecure())

	event.Security.VerificationAction = "quarantine"
	assert.False(t, event.IsSecure())

	event.Security.VerificationAction = "accept"
	event.Security.SPF.Result = "fail"
	assert.False(t, event.IsSecure())

	// Test GetShortDescription
	desc := event.GetShortDescription()
	expected := "Email from test@example.com to recipient@example.com (Test Subject)"
	assert.Equal(t, expected, desc)
}

func TestIncomingEmailEvent_NoSubject(t *testing.T) {
	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
	}

	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "test",
	}

	// Create received email without subject
	receivedEmail := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "sender@example.org",
		Subject:       nil, // No subject
		EncryptedBody: "encrypted-content",
		ReceivedAt:    time.Now(),
	}

	event := NewIncomingEmailEvent(receivedEmail, domain, emailAddress, nil, false)

	// Should use empty string for missing subject
	assert.Equal(t, "", event.Email.Subject)
}

func TestIncomingEmailEvent_NilVerificationResult(t *testing.T) {
	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
	}

	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "test",
	}

	receivedEmail := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "sender@example.org",
		EncryptedBody: "encrypted-content",
		ReceivedAt:    time.Now(),
	}

	// Create event with nil verification result
	event := NewIncomingEmailEvent(receivedEmail, domain, emailAddress, nil, false)

	// Security info should have default values
	assert.Equal(t, "", event.Security.VerificationAction)
	assert.Equal(t, 0.0, event.Security.SpamScore)
	assert.Equal(t, 0.0, event.Security.ReputationScore)
	assert.Equal(t, "", event.Security.SPF.Result)
	assert.False(t, event.Security.DKIM.Valid)
	assert.Equal(t, "", event.Security.DMARC.Result)
	assert.False(t, event.Email.IsQuarantined)
}
