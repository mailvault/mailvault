package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type EmailAddress struct {
	ID               uuid.UUID `json:"id" db:"id"`
	DomainID         uuid.UUID `json:"domain_id" db:"domain_id"`
	LocalPart        string    `json:"local_part" db:"local_part"`
	IsCatchAll       bool      `json:"is_catch_all" db:"is_catch_all"`
	ForwardAddresses []string  `json:"forward_addresses" db:"forward_addresses"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

func (e *EmailAddress) IsValid() bool {
	return e.LocalPart != "" && e.DomainID != uuid.Nil
}

func (e *EmailAddress) GetFullAddress(domain string) string {
	return e.LocalPart + "@" + domain
}

func (e *EmailAddress) HasForwarding() bool {
	return len(e.ForwardAddresses) > 0
}
