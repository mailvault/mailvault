package auth

import (
	"context"
	"fmt"

	"privatemail/domain/entities"
)

type SupabaseProvider struct {
	url    string
	apiKey string
}

func NewSupabaseProvider(url, apiKey string) *SupabaseProvider {
	return &SupabaseProvider{
		url:    url,
		apiKey: apiKey,
	}
}

func (s *SupabaseProvider) Provider() string {
	return "supabase"
}

func (s *SupabaseProvider) CreateUser(ctx context.Context, email, password string) (string, error) {
	// TODO: Implement Supabase user creation
	// This would use the Supabase Auth API to create a user
	return "", fmt.Errorf("not implemented")
}

func (s *SupabaseProvider) Login(ctx context.Context, email, password string) (string, error) {
	// TODO: Implement Supabase login
	// This would use the Supabase Auth API to authenticate
	return "", fmt.Errorf("not implemented")
}

func (s *SupabaseProvider) ValidateToken(ctx context.Context, token string) (*entities.User, error) {
	// TODO: Implement Supabase token validation
	// This would validate the JWT token with Supabase
	return nil, fmt.Errorf("not implemented")
}

func (s *SupabaseProvider) GetUserByID(ctx context.Context, id string) (*entities.User, error) {
	// TODO: Implement Supabase get user by ID
	// This would fetch user data from Supabase
	return nil, fmt.Errorf("not implemented")
}