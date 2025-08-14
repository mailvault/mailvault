package mailvault

import (
	"context"
	"fmt"
	"net/http"
)

// SendService provides email sending operations.
type SendService struct{ client *Client }

// SendEmail sends an email using domain API key authentication.
// The from address must belong to the authenticated domain
func (s *SendService) SendEmail(ctx context.Context, req SendEmailRequest) (*SendEmailResponse, error) {
	if s.client.apiKey == "" {
		return nil, fmt.Errorf("API key required for sending emails")
	}

	resp, err := s.client.doRequest(ctx, http.MethodPost, "/api/v1/send", req, true)
	if err != nil {
		return nil, fmt.Errorf("send email request failed: %w", err)
	}

	var result SendEmailResponse
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("send email response parsing failed: %w", err)
	}

	return &result, nil
}

// NewClientForDomain creates a new client configured with a domain API key for sending emails
func NewClientForDomain(baseURL, apiKey string) *Client {
	config := ClientConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewClient(config)
}
