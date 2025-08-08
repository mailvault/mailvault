package entities

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

type User struct {
	ID             uuid.UUID `json:"id" db:"id"`
	Email          string    `json:"email" db:"email"`
	AuthProvider   string    `json:"auth_provider" db:"auth_provider"`
	AuthProviderID string    `json:"auth_provider_id" db:"auth_provider_id"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

func (u *User) IsValid() bool {
	return u.Email != "" && u.AuthProvider != ""
}
