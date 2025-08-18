package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type AccountType string

const (
	AccountTypeFreemium AccountType = "freemium"
	AccountTypeBasic    AccountType = "basic"
	AccountTypePro      AccountType = "pro"
	AccountTypePayAsGo  AccountType = "pay-as-go"
)

func (a AccountType) String() string {
	return string(a)
}

func (a AccountType) DomainLimit() int {
	switch a {
	case AccountTypeFreemium:
		return 1
	case AccountTypeBasic:
		return 3
	case AccountTypePro:
		return 10
	case AccountTypePayAsGo:
		return -1 // unlimited
	default:
		return 1 // default to freemium limit
	}
}

type User struct {
	ID             uuid.UUID   `json:"id" db:"id"`
	Email          string      `json:"email" db:"email"`
	AuthProvider   string      `json:"auth_provider" db:"auth_provider"`
	AuthProviderID string      `json:"auth_provider_id" db:"auth_provider_id"`
	AccountType    AccountType `json:"account_type" db:"account_type"`
	CreatedAt      time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at" db:"updated_at"`
}

func (u *User) IsValid() bool {
	return u.Email != "" && u.AuthProvider != ""
}
