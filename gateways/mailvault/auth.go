package mailvault

import (
	"context"
	"fmt"
	"net/http"
)

// AuthService provides auth-related API operations.
type AuthService struct{ client *Client }

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	resp, err := s.client.doRequest(ctx, http.MethodPost, "/api/v1/auth/register", req, false)
	if err != nil {
		return nil, fmt.Errorf("register request failed: %w", err)
	}

	var result AuthResponse
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("register response parsing failed: %w", err)
	}

	// Automatically set the auth token from successful registration
	s.client.SetAuthToken(result.Token)

	return &result, nil
}

// Login authenticates a user with email and password
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	resp, err := s.client.doRequest(ctx, http.MethodPost, "/api/v1/auth/login", req, false)
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}

	var result AuthResponse
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("login response parsing failed: %w", err)
	}

	// Automatically set the auth token from successful login
	s.client.SetAuthToken(result.Token)

	return &result, nil
}

// Me returns information about the currently authenticated user
func (s *AuthService) Me(ctx context.Context) (*User, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	resp, err := s.client.doRequest(ctx, http.MethodGet, "/api/v1/auth/me", nil, false)
	if err != nil {
		return nil, fmt.Errorf("me request failed: %w", err)
	}

	var result User
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("me response parsing failed: %w", err)
	}

	return &result, nil
}

// Health checks if the API is running and healthy
func (s *AuthService) Health(ctx context.Context) (*HealthStatus, error) {
	resp, err := s.client.doRequest(ctx, http.MethodGet, "/health", nil, false)
	if err != nil {
		return nil, fmt.Errorf("health request failed: %w", err)
	}

	var result HealthStatus
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("health response parsing failed: %w", err)
	}

	return &result, nil
}
