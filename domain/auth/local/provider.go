// Package local implements the OSS built-in auth provider for MailVault.
//
// Users are persisted in the main `users` table (auth_provider = "local") and
// their bcrypt password hashes live in `local_credentials`. Sessions are
// stateless JWTs signed with a configured secret key.
package local

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mailvault/mailvault/domain/auth"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	goxjwt "github.com/guilhermebr/gox/jwt"
	"golang.org/x/crypto/bcrypt"
)

const ProviderName = "local"

// CredentialsRepo persists password hashes keyed by user.ID.
type CredentialsRepo interface {
	Create(ctx context.Context, userID uuid.UUID, passwordHash string) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Credentials, error)
	Delete(ctx context.Context, userID uuid.UUID) error
}

// Credentials is a row in the local_credentials table.
type Credentials struct {
	UserID         uuid.UUID
	PasswordHash   string
	EmailConfirmed bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UserStore is the subset of the user repository required by this provider.
// It is satisfied by user.Repository directly.
type UserStore interface {
	Create(ctx context.Context, u *entities.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.User, error)
	GetByEmail(ctx context.Context, email string) (*entities.User, error)
}

// Provider is the OSS built-in auth provider.
type Provider struct {
	creds CredentialsRepo
	users UserStore
	jwt   goxjwt.Service
}

// NewProvider builds the local auth provider. secretKey signs JWTs and must
// be at least 32 bytes. tokenTTL controls token lifetime (e.g. 24h).
func NewProvider(creds CredentialsRepo, users UserStore, secretKey string, tokenTTL time.Duration) (*Provider, error) {
	if len(secretKey) < 32 {
		return nil, fmt.Errorf("local auth: secretKey must be at least 32 bytes (got %d)", len(secretKey))
	}
	if tokenTTL <= 0 {
		return nil, fmt.Errorf("local auth: tokenTTL must be positive")
	}
	return &Provider{
		creds: creds,
		users: users,
		jwt:   goxjwt.NewService(secretKey, "mailvault-local", tokenTTL.String()),
	}, nil
}

// Provider returns the provider name. Implements auth.Provider.
func (p *Provider) Provider() string { return ProviderName }

// CreateUser registers a new local user: creates the users row, hashes the
// password into local_credentials, and returns an immediately-usable JWT.
func (p *Provider) CreateUser(ctx context.Context, email, password string) (*auth.CreateUserResponse, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	if existing, err := p.users.GetByEmail(ctx, email); err == nil && existing != nil {
		return nil, fmt.Errorf("user already exists")
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("checking existing user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	userID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()
	user := &entities.User{
		ID:             userID,
		Email:          email,
		AuthProvider:   ProviderName,
		AuthProviderID: userID.String(),
		AccountType:    entities.AccountTypeUser,
		UserPlan:       entities.UserPlanFree,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := p.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	if err := p.creds.Create(ctx, userID, string(hash)); err != nil {
		// Best-effort: the user row exists but creds don't. Log via caller; we
		// surface the error so the caller can decide whether to roll back.
		return nil, fmt.Errorf("storing credentials: %w", err)
	}

	token, err := p.signToken(userID, email)
	if err != nil {
		return nil, fmt.Errorf("signing token: %w", err)
	}

	return &auth.CreateUserResponse{
		UserID:          userID.String(),
		RequiresConfirm: false,
		AccessToken:     token,
	}, nil
}

// Login verifies the password and returns a JWT.
func (p *Provider) Login(ctx context.Context, email, password string) (string, error) {
	email = normalizeEmail(email)
	user, err := p.users.GetByEmail(ctx, email)
	if err != nil || user == nil {
		// Deliberately generic to avoid user enumeration.
		return "", fmt.Errorf("invalid email or password")
	}
	if user.AuthProvider != ProviderName {
		return "", fmt.Errorf("invalid email or password")
	}

	creds, err := p.creds.GetByUserID(ctx, user.ID)
	if err != nil || creds == nil {
		return "", fmt.Errorf("invalid email or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(creds.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid email or password")
	}

	return p.signToken(user.ID, user.Email)
}

// ValidateToken parses and verifies a JWT; returns the corresponding user.
func (p *Provider) ValidateToken(ctx context.Context, tokenStr string) (*entities.User, error) {
	claims, err := p.parseToken(tokenStr)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.FromString(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid token subject: %w", err)
	}
	user, err := p.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// GetUserByID fetches a user by their UUID string.
func (p *Provider) GetUserByID(ctx context.Context, id string) (*entities.User, error) {
	userID, err := uuid.FromString(id)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}
	return p.users.GetByID(ctx, userID)
}

// ResendConfirmation is a no-op for the local provider since accounts are
// auto-confirmed at creation.
func (p *Provider) ResendConfirmation(_ context.Context, _ string) error {
	return nil
}

// ConfirmEmail is a no-op for the local provider; it returns an empty token
// to match the auth.Provider contract.
func (p *Provider) ConfirmEmail(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (p *Provider) signToken(userID uuid.UUID, email string) (string, error) {
	return p.jwt.GenerateToken(userID.String(), email, "")
}

func (p *Provider) parseToken(tokenStr string) (*goxjwt.Claims, error) {
	return p.jwt.ValidateToken(tokenStr)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
