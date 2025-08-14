package auth

import (
	"context"
	"fmt"
	"time"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// MockProvider is a simple in-memory provider for testing
type MockProvider struct {
	users map[string]*entities.User
	tokens map[string]*entities.User
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		users:  make(map[string]*entities.User),
		tokens: make(map[string]*entities.User),
	}
}

func (m *MockProvider) Provider() string {
	return "mock"
}

func (m *MockProvider) CreateUser(ctx context.Context, email, password string) (string, error) {
	// Check if user already exists
	if _, exists := m.users[email]; exists {
		return "", fmt.Errorf("user with email %s already exists", email)
	}

	// Create new user
	userID := uuid.Must(uuid.NewV4())
	now := time.Now()
	
	user := &entities.User{
		ID:             userID,
		Email:          email,
		AuthProvider:   "mock",
		AuthProviderID: userID.String(),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	m.users[email] = user
	return userID.String(), nil
}

func (m *MockProvider) Login(ctx context.Context, email, password string) (string, error) {
	user, exists := m.users[email]
	if !exists {
		return "", fmt.Errorf("user not found")
	}

	// Generate a simple token (just the user ID for mock)
	token := fmt.Sprintf("mock-token-%s", user.ID.String())
	m.tokens[token] = user
	
	return token, nil
}

func (m *MockProvider) ValidateToken(ctx context.Context, token string) (*entities.User, error) {
	user, exists := m.tokens[token]
	if !exists {
		return nil, fmt.Errorf("invalid token")
	}

	return user, nil
}

func (m *MockProvider) GetUserByID(ctx context.Context, id string) (*entities.User, error) {
	// Find user by ID
	for _, user := range m.users {
		if user.ID.String() == id {
			return user, nil
		}
	}
	
	return nil, fmt.Errorf("user not found")
}