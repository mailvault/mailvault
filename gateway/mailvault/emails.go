package mailvault

import (
	"context"
	"fmt"
	"net/http"
)

// EmailsService provides email-address and received-email operations.
type EmailsService struct{ client *Client }

// CreateEmailAddress creates a new email address for a specific domain
func (s *EmailsService) CreateEmailAddress(ctx context.Context, domainID string, req CreateEmailRequest) (*EmailAddress, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s/emails", domainID)
	resp, err := s.client.doRequest(ctx, http.MethodPost, endpoint, req, false)
	if err != nil {
		return nil, fmt.Errorf("create email address request failed: %w", err)
	}

	var result EmailAddress
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("create email address response parsing failed: %w", err)
	}

	return &result, nil
}

// GetEmailAddresses retrieves all email addresses configured for a specific domain
func (s *EmailsService) GetEmailAddresses(ctx context.Context, domainID string) ([]*EmailAddress, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s/emails", domainID)
	resp, err := s.client.doRequest(ctx, http.MethodGet, endpoint, nil, false)
	if err != nil {
		return nil, fmt.Errorf("get email addresses request failed: %w", err)
	}

	var result []*EmailAddress
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("get email addresses response parsing failed: %w", err)
	}

	return result, nil
}

// GetEmailAddress retrieves a specific email address by its ID
func (s *EmailsService) GetEmailAddress(ctx context.Context, domainID, emailID string) (*EmailAddress, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s/emails/%s", domainID, emailID)
	resp, err := s.client.doRequest(ctx, http.MethodGet, endpoint, nil, false)
	if err != nil {
		return nil, fmt.Errorf("get email address request failed: %w", err)
	}

	var result EmailAddress
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("get email address response parsing failed: %w", err)
	}

	return &result, nil
}

// UpdateEmailAddress updates email address settings such as catch-all and forwarding configuration
func (s *EmailsService) UpdateEmailAddress(ctx context.Context, domainID, emailID string, req UpdateEmailRequest) (*EmailAddress, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s/emails/%s", domainID, emailID)
	resp, err := s.client.doRequest(ctx, http.MethodPut, endpoint, req, false)
	if err != nil {
		return nil, fmt.Errorf("update email address request failed: %w", err)
	}

	var result EmailAddress
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("update email address response parsing failed: %w", err)
	}

	return &result, nil
}

// DeleteEmailAddress deletes an email address and all associated received emails
func (s *EmailsService) DeleteEmailAddress(ctx context.Context, domainID, emailID string) error {
	if s.client.authToken == "" {
		return fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s/emails/%s", domainID, emailID)
	resp, err := s.client.doRequest(ctx, http.MethodDelete, endpoint, nil, false)
	if err != nil {
		return fmt.Errorf("delete email address request failed: %w", err)
	}

	if err := s.client.parseResponse(resp, nil); err != nil {
		return fmt.Errorf("delete email address response parsing failed: %w", err)
	}

	return nil
}

// GetReceivedEmails retrieves received emails for a specific email address with pagination
func (s *EmailsService) GetReceivedEmails(ctx context.Context, domainID, emailID string, options *GetReceivedEmailsOptions) (*PaginatedResponse, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/%s/emails/%s/received", domainID, emailID)

	// Add query parameters
	if options != nil {
		params := make(map[string]interface{})
		if options.Limit != nil {
			params["limit"] = *options.Limit
		}
		if options.Offset != nil {
			params["offset"] = *options.Offset
		}
		endpoint += s.client.buildQueryParams(params)
	}

	resp, err := s.client.doRequest(ctx, http.MethodGet, endpoint, nil, false)
	if err != nil {
		return nil, fmt.Errorf("get received emails request failed: %w", err)
	}

	var result PaginatedResponse
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("get received emails response parsing failed: %w", err)
	}

	return &result, nil
}

// GetReceivedEmail retrieves a specific received email by its ID
func (s *EmailsService) GetReceivedEmail(ctx context.Context, receivedEmailID string) (*ReceivedEmail, error) {
	if s.client.authToken == "" {
		return nil, fmt.Errorf("authentication token required")
	}

	endpoint := fmt.Sprintf("/api/v1/domains/received/%s", receivedEmailID)
	resp, err := s.client.doRequest(ctx, http.MethodGet, endpoint, nil, false)
	if err != nil {
		return nil, fmt.Errorf("get received email request failed: %w", err)
	}

	var result ReceivedEmail
	if err := s.client.parseResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("get received email response parsing failed: %w", err)
	}

	return &result, nil
}
