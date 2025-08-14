package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) doRequest(method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	if verbose {
		fmt.Printf("→ %s %s\n", method, path)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if verbose {
		fmt.Printf("← %d %s\n", resp.StatusCode, resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errorResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error != "" {
			return fmt.Errorf("API error: %s", errorResp.Error)
		}
		return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// Auth API methods
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Provider string `json:"auth_provider"`
}

func (c *Client) Login(email, password string) (*LoginResponse, error) {
	var resp LoginResponse
	err := c.doRequest("POST", "/api/v1/auth/login", LoginRequest{
		Email:    email,
		Password: password,
	}, &resp)
	return &resp, err
}

func (c *Client) Register(email, password string) (*RegisterResponse, error) {
	var resp RegisterResponse
	err := c.doRequest("POST", "/api/v1/auth/register", RegisterRequest{
		Email:    email,
		Password: password,
	}, &resp)
	return &resp, err
}

func (c *Client) GetMe() (*User, error) {
	var resp User
	err := c.doRequest("GET", "/api/v1/auth/me", nil, &resp)
	return &resp, err
}

// Domain API methods
type Domain struct {
	ID             string            `json:"id"`
	Domain         string            `json:"domain"`
	PublicKey      string            `json:"public_key"`
	APIKey         string            `json:"api_key"`
	Verified       bool              `json:"verified"`
	WebhookConfig  *WebhookConfig    `json:"webhook_config,omitempty"`
	StorageEnabled bool              `json:"storage_enabled"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
}

type WebhookConfig struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

type CreateDomainRequest struct {
	Domain         string         `json:"domain"`
	PublicKey      string         `json:"public_key"`
	WebhookConfig  *WebhookConfig `json:"webhook_config,omitempty"`
	StorageEnabled *bool          `json:"storage_enabled,omitempty"`
}

func (c *Client) CreateDomain(req CreateDomainRequest) (*Domain, error) {
	var resp Domain
	err := c.doRequest("POST", "/api/v1/domains", req, &resp)
	return &resp, err
}

func (c *Client) ListDomains() ([]*Domain, error) {
	var resp []*Domain
	err := c.doRequest("GET", "/api/v1/domains", nil, &resp)
	return resp, err
}

func (c *Client) GetDomain(id string) (*Domain, error) {
	var resp Domain
	err := c.doRequest("GET", "/api/v1/domains/"+id, nil, &resp)
	return &resp, err
}

func (c *Client) DeleteDomain(id string) error {
	return c.doRequest("DELETE", "/api/v1/domains/"+id, nil, nil)
}

// Email API methods
type EmailAddress struct {
	ID               string   `json:"id"`
	LocalPart        string   `json:"local_part"`
	IsCatchAll       bool     `json:"is_catch_all"`
	ForwardAddresses []string `json:"forward_addresses,omitempty"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type CreateEmailRequest struct {
	LocalPart        string   `json:"local_part"`
	IsCatchAll       bool     `json:"is_catch_all"`
	ForwardAddresses []string `json:"forward_addresses,omitempty"`
}

func (c *Client) CreateEmailAddress(domainID string, req CreateEmailRequest) (*EmailAddress, error) {
	var resp EmailAddress
	err := c.doRequest("POST", fmt.Sprintf("/api/v1/domains/%s/emails", domainID), req, &resp)
	return &resp, err
}

func (c *Client) ListEmailAddresses(domainID string) ([]*EmailAddress, error) {
	var resp []*EmailAddress
	err := c.doRequest("GET", fmt.Sprintf("/api/v1/domains/%s/emails", domainID), nil, &resp)
	return resp, err
}

func (c *Client) DeleteEmailAddress(domainID, emailID string) error {
	return c.doRequest("DELETE", fmt.Sprintf("/api/v1/domains/%s/emails/%s", domainID, emailID), nil, nil)
}

// Received Email API methods
type ReceivedEmail struct {
	ID             string `json:"id"`
	SequenceNumber int    `json:"sequence_number"`
	FromAddress    string `json:"from_address"`
	Subject        string `json:"subject"`
	Body           string `json:"encrypted_body"`
	ReceivedAt     string `json:"received_at"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data       []*ReceivedEmail `json:"data"`
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Total  int `json:"total,omitempty"`
	} `json:"pagination"`
}

func (c *Client) ListReceivedEmails(domainID, emailID string, limit, offset int) ([]*ReceivedEmail, error) {
	var resp PaginatedResponse
	path := fmt.Sprintf("/api/v1/domains/%s/emails/%s/received?limit=%d&offset=%d", 
		domainID, emailID, limit, offset)
	err := c.doRequest("GET", path, nil, &resp)
	return resp.Data, err
}

// GetReceivedEmailByID gets a specific received email by its UUID
func (c *Client) GetReceivedEmailByID(receivedEmailID string) (*ReceivedEmail, error) {
	var resp ReceivedEmail
	path := fmt.Sprintf("/api/v1/domains/received/%s", receivedEmailID)
	err := c.doRequest("GET", path, nil, &resp)
	return &resp, err
}

// FindReceivedEmailByReference finds a received email by sequence number, short ID, or UUID
func (c *Client) FindReceivedEmailByReference(domainID, emailID, reference string) (*ReceivedEmail, error) {
	// First, try to parse as UUID and get directly if possible
	if id, err := uuid.FromString(reference); err == nil {
		// It's a valid UUID, try to get it directly
		return c.GetReceivedEmailByID(id.String())
	}

	// For sequence numbers and short IDs, we still need to get the list to find the UUID
	emails, err := c.ListReceivedEmails(domainID, emailID, 1000, 0)
	if err != nil {
		return nil, err
	}

	// Convert to utils format for parsing
	var basicEmails []ReceivedEmailBasic
	for _, email := range emails {
		basicEmails = append(basicEmails, ReceivedEmailBasic{
			ID:             parseUUIDString(email.ID),
			SequenceNumber: email.SequenceNumber,
			FromAddress:    email.FromAddress,
			Subject:        email.Subject,
			ReceivedAt:     email.ReceivedAt,
		})
	}

	emailsWithShortID := generateEmailsWithShortID(basicEmails)
	targetUUID := parseEmailReference(reference, emailsWithShortID)
	if targetUUID == nil {
		return nil, fmt.Errorf("email not found with reference: %s", reference)
	}

	// Get the full email using the direct endpoint
	return c.GetReceivedEmailByID(targetUUID.String())
}

// Helper functions for email reference parsing
type ReceivedEmailBasic struct {
	ID             uuid.UUID
	SequenceNumber int
	FromAddress    string
	Subject        string
	ReceivedAt     string
}

type EmailWithShortID struct {
	ID             uuid.UUID
	SequenceNumber int
	ShortID        string
	FromAddress    string
	Subject        string
	ReceivedAt     string
}

// parseUUIDString safely parses a UUID string
func parseUUIDString(s string) uuid.UUID {
	id, _ := uuid.FromString(s)
	return id
}

// shortID generates a short 8-character ID from a UUID for display purposes
func shortID(id uuid.UUID) string {
	hash := sha256.Sum256([]byte(id.String()))
	return fmt.Sprintf("%x", hash)[:8]
}

// generateEmailsWithShortID creates EmailWithShortID slice for CLI usage
func generateEmailsWithShortID(emails []ReceivedEmailBasic) []EmailWithShortID {
	result := make([]EmailWithShortID, len(emails))
	for i, email := range emails {
		result[i] = EmailWithShortID{
			ID:             email.ID,
			SequenceNumber: email.SequenceNumber,
			ShortID:        shortID(email.ID),
			FromAddress:    email.FromAddress,
			Subject:        email.Subject,
			ReceivedAt:     email.ReceivedAt,
		}
	}
	return result
}

// parseEmailReference parses an email reference which can be:
// - A sequence number (integer)  
// - A short ID (8-character hex string)
// - A full UUID
// Returns the UUID if found, otherwise returns nil
func parseEmailReference(ref string, emails []EmailWithShortID) *uuid.UUID {
	// Try parsing as sequence number
	if seqNum, err := strconv.Atoi(ref); err == nil {
		for _, email := range emails {
			if email.SequenceNumber == seqNum {
				return &email.ID
			}
		}
		return nil
	}

	// Try parsing as short ID (8 characters)
	if len(ref) == 8 {
		ref = strings.ToLower(ref)
		for _, email := range emails {
			if email.ShortID == ref {
				return &email.ID
			}
		}
		return nil
	}

	// Try parsing as full UUID
	if id, err := uuid.FromString(ref); err == nil {
		for _, email := range emails {
			if email.ID == id {
				return &id
			}
		}
	}

	return nil
}

// Smart resolution functions for user-friendly CLI inputs

// ResolveDomainReference resolves a domain reference to a Domain object
// Accepts: domain name, partial domain name, or UUID
func (c *Client) ResolveDomainReference(domainRef string) (*Domain, error) {
	// Try parsing as UUID first
	if id, err := uuid.FromString(domainRef); err == nil {
		return c.GetDomain(id.String())
	}

	// Get all domains and search by name
	domains, err := c.ListDomains()
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	// Exact match first
	for _, domain := range domains {
		if domain.Domain == domainRef {
			return domain, nil
		}
	}

	// Partial match if no exact match
	var matches []*Domain
	for _, domain := range domains {
		if strings.Contains(domain.Domain, domainRef) {
			matches = append(matches, domain)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("domain not found: %s", domainRef)
	case 1:
		return matches[0], nil
	default:
		// Multiple matches - show options
		fmt.Printf("Multiple domains match '%s':\n", domainRef)
		for i, domain := range matches {
			fmt.Printf("%d) %s\n", i+1, domain.Domain)
		}
		fmt.Printf("Please be more specific or use the full domain name.\n")
		return nil, fmt.Errorf("ambiguous domain reference: %s", domainRef)
	}
}

// ResolveEmailReference resolves an email reference to EmailAddress and Domain
// Accepts: 
// - "local@domain.com" (full email address format)
// - domain UUID + email UUID (backwards compatibility)  
// - domain name + local part
// - domain name + "*" for catch-all
func (c *Client) ResolveEmailReference(domainRef, emailRef string) (*Domain, *EmailAddress, error) {
	var domain *Domain
	var err error

	// Check if domainRef contains @ (full email address format)
	if strings.Contains(domainRef, "@") && emailRef == "" {
		// Parse full email address
		parts := strings.Split(domainRef, "@")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid email address format: %s", domainRef)
		}
		emailRef = parts[0]
		domainRef = parts[1]
	}

	// Resolve domain
	domain, err = c.ResolveDomainReference(domainRef)
	if err != nil {
		return nil, nil, fmt.Errorf("domain resolution failed: %w", err)
	}

	// If emailRef is empty, we can't resolve the email
	if emailRef == "" {
		return domain, nil, fmt.Errorf("email reference is required")
	}

	// Try parsing emailRef as UUID first (backwards compatibility)
	if id, err := uuid.FromString(emailRef); err == nil {
		emailAddr, err := c.GetEmailAddress(domain.ID, id.String())
		if err != nil {
			return domain, nil, fmt.Errorf("email address not found: %w", err)
		}
		return domain, emailAddr, nil
	}

	// Get all email addresses for the domain
	emails, err := c.ListEmailAddresses(domain.ID)
	if err != nil {
		return domain, nil, fmt.Errorf("failed to list email addresses: %w", err)
	}

	// Handle special cases
	if emailRef == "*" || emailRef == "catchall" {
		// Find catch-all address
		for _, email := range emails {
			if email.IsCatchAll {
				return domain, email, nil
			}
		}
		return domain, nil, fmt.Errorf("no catch-all address found for domain %s", domain.Domain)
	}

	// Search by local part
	for _, email := range emails {
		if email.LocalPart == emailRef {
			return domain, email, nil
		}
	}

	// If not found, show available options
	if len(emails) > 0 {
		fmt.Printf("Email address '%s' not found in domain '%s'.\n", emailRef, domain.Domain)
		fmt.Printf("Available addresses:\n")
		for _, email := range emails {
			if email.IsCatchAll {
				fmt.Printf("  * (catch-all)\n")
			} else {
				fmt.Printf("  %s\n", email.LocalPart)
			}
		}
	}

	return domain, nil, fmt.Errorf("email address not found: %s@%s", emailRef, domain.Domain)
}

// GetEmailAddress gets a specific email address by domain ID and email ID
func (c *Client) GetEmailAddress(domainID, emailID string) (*EmailAddress, error) {
	var resp EmailAddress
	path := fmt.Sprintf("/api/v1/domains/%s/emails/%s", domainID, emailID)
	err := c.doRequest("GET", path, nil, &resp)
	return &resp, err
}