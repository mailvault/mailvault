package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type EmailAddress struct {
	ID                uuid.UUID `json:"id" db:"id"`
	DomainID          uuid.UUID `json:"domain_id" db:"domain_id"`
	LocalPart         string    `json:"local_part" db:"local_part"`
	ForwardAddresses  []string  `json:"forward_addresses" db:"forward_addresses"`
	ForwardingEnabled bool      `json:"forwarding_enabled" db:"forwarding_enabled"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

func (e *EmailAddress) IsValid() bool {
	return e.LocalPart != "" && e.DomainID != uuid.Nil
}

func (e *EmailAddress) GetFullAddress(domain string) string {
	return e.LocalPart + "@" + domain
}

func (e *EmailAddress) HasForwarding() bool {
	return e.ForwardingEnabled && len(e.ForwardAddresses) > 0
}
