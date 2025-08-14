package mailvault

import (
	"context"
	"fmt"
	"net/http"
)

// DomainsService provides domain-related API operations.
type DomainsService struct{ client *Client }

// CreateDomain creates a new domain for the authenticated user
func (s *DomainsService) CreateDomain(ctx context.Context, req CreateDomainRequest) (*Domain, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	resp, err := s.client.doRequest(ctx, http.MethodPost, "/api/v1/domains", req, false)
	if err != nil {
		return nil, fmt.Errorf("create domain request failed: %w", err)
	}

	var result Domain
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("create domain response parsing failed: %w", err)
	}

	return &result, nil
}

// GetDomains retrieves all domains belonging to the authenticated user
func (s *DomainsService) GetDomains(ctx context.Context) ([]*Domain, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	resp, err := s.client.doRequest(ctx, http.MethodGet, "/api/v1/domains", nil, false)
	if err != nil {
		return nil, fmt.Errorf("get domains request failed: %w", err)
	}

	var result []*Domain
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("get domains response parsing failed: %w", err)
	}

	return result, nil
}

// GetDomain retrieves a specific domain by its ID
func (s *DomainsService) GetDomain(ctx context.Context, domainID string) (*Domain, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s", domainID)
	resp, err := s.client.doRequest(ctx, http.MethodGet, endpoint, nil, false)
	if err != nil {
		return nil, fmt.Errorf("get domain request failed: %w", err)
	}

	var result Domain
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("get domain response parsing failed: %w", err)
	}

	return &result, nil
}

// UpdateDomain updates an existing domain
func (s *DomainsService) UpdateDomain(ctx context.Context, domainID string, req UpdateDomainRequest) (*Domain, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s", domainID)
	resp, err := s.client.doRequest(ctx, http.MethodPut, endpoint, req, false)
	if err != nil {
		return nil, fmt.Errorf("update domain request failed: %w", err)
	}

	var result Domain
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("update domain response parsing failed: %w", err)
	}

	return &result, nil
}

// DeleteDomain deletes a domain and all associated email addresses and received emails
func (s *DomainsService) DeleteDomain(ctx context.Context, domainID string) error {
	if s.client.authToken == "" {
		return fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s", domainID)
	resp, err := s.client.doRequest(ctx, http.MethodDelete, endpoint, nil, false)
	if err != nil {
		return fmt.Errorf("delete domain request failed: %w", err)
	}

	if err := s.client.parseResponse(resp, nil); err != nil {
		return fmt.Errorf("delete domain response parsing failed: %w", err)
	}

	return nil
}
