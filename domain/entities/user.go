package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type AccountType string

const (
	// Account types for role/permission management
	AccountTypeUser  AccountType = "user"
	AccountTypeOwner AccountType = "owner"
	AccountTypeAdmin AccountType = "admin"
)

type UserPlan string

const (
	// User plans for billing/feature management
	UserPlanFree    UserPlan = "free"
	UserPlanPro     UserPlan = "pro"
	UserPlanPremium UserPlan = "premium"
)

func (a AccountType) String() string {
	return string(a)
}

func (p UserPlan) String() string {
	return string(p)
}

func (p UserPlan) DomainLimit() int {
	switch p {
	case UserPlanFree:
		return 1
	case UserPlanPro:
		return 10
	case UserPlanPremium:
		return -1 // unlimited
	default:
		return 1 // default to free limit
	}
}

type User struct {
	ID             uuid.UUID   `json:"id" db:"id"`
	Email          string      `json:"email" db:"email"`
	AuthProvider   string      `json:"auth_provider" db:"auth_provider"`
	AuthProviderID string      `json:"auth_provider_id" db:"auth_provider_id"`
	AccountType    AccountType `json:"account_type" db:"account_type"`
	UserPlan       UserPlan    `json:"user_plan" db:"user_plan"`
	CreatedAt      time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at" db:"updated_at"`
}

func (u *User) IsValid() bool {
	return u.Email != "" && u.AuthProvider != ""
}

func (u *User) IsAdmin() bool {
	return u.AccountType == AccountTypeAdmin
}

func (u *User) GetDomainLimit() int {
	if u.AccountType == AccountTypeAdmin {
		return -1 // unlimited for admins
	}
	return u.UserPlan.DomainLimit()
}

type UserStats struct {
	DomainsCount int                    `json:"domains_count"`
	EmailsCount  int                    `json:"emails_count"`
	SMTPStats    []SMTPVerificationStat `json:"smtp_stats"`
}
