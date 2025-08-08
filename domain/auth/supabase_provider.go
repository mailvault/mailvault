package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"mailsafe/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/supabase-community/gotrue-go/types"
	"github.com/supabase-community/supabase-go"
)

type SupabaseProvider struct {
	client *supabase.Client
	url    string
	apiKey string
}

func NewSupabaseProvider(url, apiKey string) *SupabaseProvider {
	client, err := supabase.NewClient(url, apiKey, &supabase.ClientOptions{})
	if err != nil {
		// Log error but don't fail initialization
		// This allows the app to start even if Supabase is misconfigured
		slog.Error("failed to initialize Supabase client", "error", err)
	}

	return &SupabaseProvider{
		client: client,
		url:    url,
		apiKey: apiKey,
	}
}

func (s *SupabaseProvider) Provider() string {
	return "supabase"
}

func (s *SupabaseProvider) CreateUser(ctx context.Context, email, password string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("supabase client not initialized")
	}

	// Use the Auth client to signup user
	resp, err := s.client.Auth.Signup(types.SignupRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create user in Supabase: %w", err)
	}

	if resp.User.ID.String() == "" {
		return "", fmt.Errorf("no user ID received from Supabase")
	}

	return resp.User.ID.String(), nil
}

func (s *SupabaseProvider) Login(ctx context.Context, email, password string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("supabase client not initialized")
	}

	// Use the Auth client SignInWithEmailPassword method
	resp, err := s.client.Auth.SignInWithEmailPassword(email, password)
	if err != nil {
		return "", fmt.Errorf("failed to authenticate with Supabase: %w", err)
	}

	if resp.AccessToken == "" {
		return "", fmt.Errorf("no access token received from Supabase")
	}

	return resp.AccessToken, nil
}

func (s *SupabaseProvider) ValidateToken(ctx context.Context, token string) (*entities.User, error) {
	if s.client == nil {
		return nil, fmt.Errorf("supabase client not initialized")
	}

	// Update the auth session with the token
	session := types.Session{
		AccessToken: token,
	}
	s.client.UpdateAuthSession(session)

	// Get user information using the token
	userResp, err := s.client.Auth.GetUser()
	if err != nil {
		return nil, fmt.Errorf("failed to validate token with Supabase: %w", err)
	}

	if userResp == nil || userResp.ID.String() == "" {
		return nil, fmt.Errorf("invalid user from token")
	}

	// Convert Supabase user to our domain entity
	return s.convertSupabaseUser(&userResp.User), nil
}

func (s *SupabaseProvider) GetUserByID(ctx context.Context, id string) (*entities.User, error) {
	// For GetUserByID, we would need admin access or service role key
	// Since this is typically used for server-side operations, we'll return an error
	// indicating this method requires admin privileges or user data should be cached locally
	return nil, fmt.Errorf("GetUserByID requires admin access - consider caching user data locally")
}

// convertSupabaseUser converts a Supabase user to our domain entity
func (s *SupabaseProvider) convertSupabaseUser(supabaseUser *types.User) *entities.User {

	// Use the timestamps from Supabase user (they are time.Time, not pointers)
	createdAt := supabaseUser.CreatedAt
	updatedAt := supabaseUser.UpdatedAt

	// Store the Supabase user ID as auth provider ID
	authProviderID := supabaseUser.ID.String()

	return &entities.User{
		ID:             uuid.Nil,
		Email:          supabaseUser.Email,
		AuthProvider:   "supabase",
		AuthProviderID: authProviderID,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

// Helper method to check if Supabase is properly configured
func (s *SupabaseProvider) IsConfigured() bool {
	return s.client != nil &&
		s.url != "" &&
		s.apiKey != "" &&
		!strings.Contains(s.url, "your-project") // Common placeholder
}
