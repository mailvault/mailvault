package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type ReceivedEmail struct {
	ID               uuid.UUID             `json:"id" db:"id"`
	EmailAddressID   *uuid.UUID            `json:"email_address_id" db:"email_address_id"`
	SequenceNumber   int                   `json:"sequence_number" db:"sequence_number"`
	FromAddress      string                `json:"from_address" db:"from_address"`
	Subject          *string               `json:"subject" db:"subject"`
	EncryptedBody    string                `json:"encrypted_body" db:"encrypted_body"`
	DomainName       string                `json:"domain_name" db:"domain_name"`
	EmailAddress     string                `json:"email_address" db:"email_address"`
	ReceivedAt       time.Time             `json:"received_at" db:"received_at"`
	SMTPVerification *SMTPVerificationStat `json:"smtp_verification,omitempty" db:"-"`
}

func (r *ReceivedEmail) IsValid() bool {
	return r.FromAddress != "" && r.EncryptedBody != ""
}

func (r *ReceivedEmail) GetSubjectString() string {
	if r.Subject != nil {
		return *r.Subject
	}
	return ""
}
