package mailvault

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client represents the MailVault SDK client
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
	apiKey     string

	// Services
	Auth    *AuthService
	Domains *DomainsService
	Emails  *EmailsService
	Send    *SendService
}

// ClientConfig holds configuration for the MailVault client
type ClientConfig struct {
	BaseURL    string
	HTTPClient *http.Client
	AuthToken  string
	APIKey     string
}

// NewClient creates a new MailVault SDK client
func NewClient(config ClientConfig) *Client {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.mailvault.sh"
	}

	c := &Client{
		baseURL:    config.BaseURL,
		httpClient: config.HTTPClient,
		authToken:  config.AuthToken,
		apiKey:     config.APIKey,
	}

	// Initialize services
	c.Auth = &AuthService{client: c}
	c.Domains = &DomainsService{client: c}
	c.Emails = &EmailsService{client: c}
	c.Send = &SendService{client: c}

	return c
}

// SetAuthToken sets the JWT authentication token for user endpoints
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// SetAPIKey sets the API key for domain-specific operations
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// WithAuth returns a shallow-copied client configured with the provided auth token
func (c *Client) WithAuth(token string) *Client {
	clone := *c
	clone.authToken = token
	return &clone
}

// WithAPIKey returns a shallow-copied client configured with the provided API key
func (c *Client) WithAPIKey(apiKey string) *Client {
	clone := *c
	clone.apiKey = apiKey
	return &clone
}

// doRequest performs an HTTP request with proper authentication
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}, useAPIKey bool) (*http.Response, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Set authentication
	if useAPIKey && c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	} else if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	return resp, nil
}

// parseResponse parses HTTP response into the target struct
func (c *Client) parseResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    errResp.Error,
		}
	}

	if target != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// buildQueryParams builds URL query parameters
func (c *Client) buildQueryParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}

	values := url.Values{}
	for key, value := range params {
		if value != nil {
			values.Add(key, fmt.Sprintf("%v", value))
		}
	}

	return "?" + values.Encode()
}
