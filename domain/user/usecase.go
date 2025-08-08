package user

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"mailsafe/domain/entities"

	"github.com/gofrs/uuid/v5"
)

type UseCase struct {
	repo Repository
}

func NewUseCase(repo Repository) *UseCase {
	return &UseCase{
		repo: repo,
	}
}

type CreateUserInput struct {
	Email          string `json:"email"`
	AuthProvider   string `json:"auth_provider"`
	AuthProviderID string `json:"auth_provider_id"`
}

func (uc *UseCase) CreateUser(ctx context.Context, req CreateUserInput) (*entities.User, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}

	if req.AuthProvider == "" {
		return nil, fmt.Errorf("auth provider is required")
	}

	// Check if user already exists
	existing, err := uc.repo.GetByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	user := &entities.User{
		ID:             uuid.Must(uuid.NewV4()),
		Email:          req.Email,
		AuthProvider:   req.AuthProvider,
		AuthProviderID: req.AuthProviderID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if !user.IsValid() {
		return nil, fmt.Errorf("invalid user data")
	}

	slog.Info("creating user", "user", user)

	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (uc *UseCase) GetUserByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("user ID is required")
	}

	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (uc *UseCase) GetUserByEmail(ctx context.Context, email string) (*entities.User, error) {
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}

func (uc *UseCase) GetOrCreateUserByAuthProvider(ctx context.Context, provider, providerID, email string) (*entities.User, error) {
	if provider == "" || providerID == "" || email == "" {
		return nil, fmt.Errorf("provider, provider ID, and email are required")
	}

	// Try to find existing user by auth provider
	user, err := uc.repo.GetByAuthProvider(ctx, provider, providerID)
	if err == nil && user != nil {
		return user, nil
	}

	// Try to find by email
	user, err = uc.repo.GetByEmail(ctx, email)
	if err == nil && user != nil {
		// Update user with auth provider info if it's different
		if user.AuthProvider != provider || user.AuthProviderID != providerID {
			user.AuthProvider = provider
			user.AuthProviderID = providerID
			user.UpdatedAt = time.Now()

			if err := uc.repo.Update(ctx, user); err != nil {
				return nil, fmt.Errorf("failed to update user auth provider: %w", err)
			}
		}
		return user, nil
	}

	// Create new user
	return uc.CreateUser(ctx, CreateUserInput{
		Email:          email,
		AuthProvider:   provider,
		AuthProviderID: providerID,
	})
}
