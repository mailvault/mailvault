package auth

import (
	"context"

	"mailvault/domain/entities"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/provider.go . Provider
type Provider interface {
	Provider() string
	CreateUser(ctx context.Context, email, password string) (string, error)
	Login(ctx context.Context, email, password string) (string, error)
	ValidateToken(ctx context.Context, token string) (*entities.User, error)
	GetUserByID(ctx context.Context, id string) (*entities.User, error)
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string         `json:"token"`
	User  *entities.User `json:"user"`
}
