package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mailvault/gateways/mailvault"

	"github.com/gofrs/uuid/v5"
)

type Client struct {
	sdk *mailvault.Client
}

func NewClient(baseURL string) *Client {
	return &Client{sdk: mailvault.NewClient(mailvault.ClientConfig{BaseURL: baseURL})}
}

func (c *Client) SetToken(token string) { c.sdk.SetAuthToken(token) }

// Type aliases to SDK types for reuse in CLI code
type (
	Domain              = mailvault.Domain
	WebhookConfig       = mailvault.WebhookConfig
	CreateDomainRequest = mailvault.CreateDomainRequest
	EmailAddress        = mailvault.EmailAddress
	CreateEmailRequest  = mailvault.CreateEmailRequest
	ReceivedEmail       = mailvault.ReceivedEmail
)

// Auth API methods
func (c *Client) Login(email, password string) (*mailvault.AuthResponse, error) {
	return c.sdk.Auth.Login(context.Background(), mailvault.LoginRequest{Email: email, Password: password})
}

func (c *Client) Register(email, password string) (*mailvault.AuthResponse, error) {
	return c.sdk.Auth.Register(context.Background(), mailvault.RegisterRequest{Email: email, Password: password})
}

func (c *Client) GetMe() (*mailvault.User, error) {
	return c.sdk.Auth.Me(context.Background())
}

// Domain API methods
func (c *Client) CreateDomain(req CreateDomainRequest) (*Domain, error) {
	return c.sdk.Domains.CreateDomain(context.Background(), req)
}
func (c *Client) ListDomains() ([]*Domain, error) {
	return c.sdk.Domains.GetDomains(context.Background())
}
func (c *Client) GetDomain(id string) (*Domain, error) {
	return c.sdk.Domains.GetDomain(context.Background(), id)
}
func (c *Client) DeleteDomain(id string) error {
	return c.sdk.Domains.DeleteDomain(context.Background(), id)
}

// Email API methods
func (c *Client) CreateEmailAddress(domainID string, req CreateEmailRequest) (*EmailAddress, error) {
	return c.sdk.Emails.CreateEmailAddress(context.Background(), domainID, req)
}
func (c *Client) ListEmailAddresses(domainID string) ([]*EmailAddress, error) {
	return c.sdk.Emails.GetEmailAddresses(context.Background(), domainID)
}
func (c *Client) GetEmailAddress(domainID, emailID string) (*EmailAddress, error) {
	return c.sdk.Emails.GetEmailAddress(context.Background(), domainID, emailID)
}
func (c *Client) DeleteEmailAddress(domainID, emailID string) error {
	return c.sdk.Emails.DeleteEmailAddress(context.Background(), domainID, emailID)
}

// Received Email API methods
func (c *Client) ListReceivedEmails(domainID, emailID string, limit, offset int) ([]*ReceivedEmail, error) {
	opts := &mailvault.GetReceivedEmailsOptions{Limit: &limit, Offset: &offset}
	resp, err := c.sdk.Emails.GetReceivedEmails(context.Background(), domainID, emailID, opts)
	if err != nil {
		return nil, err
	}
	var emails []*ReceivedEmail
	raw, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal received emails: %w", err)
	}
	if err := json.Unmarshal(raw, &emails); err != nil {
		return nil, fmt.Errorf("failed to unmarshal received emails: %w", err)
	}
	return emails, nil
}

func (c *Client) GetReceivedEmailByID(receivedEmailID string) (*ReceivedEmail, error) {
	return c.sdk.Emails.GetReceivedEmail(context.Background(), receivedEmailID)
}

// FindReceivedEmailByReference finds a received email by sequence number, short ID, or UUID
func (c *Client) FindReceivedEmailByReference(domainID, emailID, reference string) (*ReceivedEmail, error) {
	// Try UUID direct fetch
	if id, err := uuid.FromString(reference); err == nil {
		return c.GetReceivedEmailByID(id.String())
	}
	// Fallback to listing and searching
	emails, err := c.ListReceivedEmails(domainID, emailID, 1000, 0)
	if err != nil {
		return nil, err
	}
	// Build helper list
	var basic []ReceivedEmailBasic
	for _, e := range emails {
		basic = append(basic, ReceivedEmailBasic{
			ID:             parseUUIDString(e.ID),
			SequenceNumber: e.SequenceNumber,
			FromAddress:    e.FromAddress,
			Subject:        e.Subject,
			ReceivedAt:     e.ReceivedAt,
		})
	}
	withShort := generateEmailsWithShortID(basic)
	target := parseEmailReference(reference, withShort)
	if target == nil {
		return nil, fmt.Errorf("email not found with reference: %s", reference)
	}
	return c.GetReceivedEmailByID(target.String())
}

// Helper types and functions for CLI features

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

func parseUUIDString(s string) uuid.UUID {
	id, _ := uuid.FromString(s)
	return id
}

func shortID(id uuid.UUID) string {
	hash := sha256.Sum256([]byte(id.String()))
	return fmt.Sprintf("%x", hash)[:8]
}

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

func parseEmailReference(ref string, emails []EmailWithShortID) *uuid.UUID {
	if seqNum, err := strconv.Atoi(ref); err == nil {
		for _, email := range emails {
			if email.SequenceNumber == seqNum {
				return &email.ID
			}
		}
		return nil
	}
	if len(ref) == 8 {
		ref = strings.ToLower(ref)
		for _, email := range emails {
			if email.ShortID == ref {
				return &email.ID
			}
		}
		return nil
	}
	if id, err := uuid.FromString(ref); err == nil {
		for _, email := range emails {
			if email.ID == id {
				return &id
			}
		}
	}
	return nil
}

// Smart resolution helpers
func (c *Client) ResolveDomainReference(domainRef string) (*Domain, error) {
	if id, err := uuid.FromString(domainRef); err == nil {
		return c.GetDomain(id.String())
	}
	domains, err := c.ListDomains()
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}
	for _, domain := range domains {
		if domain.Domain == domainRef {
			return domain, nil
		}
	}
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
		fmt.Printf("Multiple domains match '%s':\n", domainRef)
		for i, domain := range matches {
			fmt.Printf("%d) %s\n", i+1, domain.Domain)
		}
		fmt.Printf("Please be more specific or use the full domain name.\n")
		return nil, fmt.Errorf("ambiguous domain reference: %s", domainRef)
	}
}

func (c *Client) ResolveEmailReference(domainRef, emailRef string) (*Domain, *EmailAddress, error) {
	var domain *Domain
	var err error
	if strings.Contains(domainRef, "@") && emailRef == "" {
		parts := strings.Split(domainRef, "@")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid email address format: %s", domainRef)
		}
		emailRef = parts[0]
		domainRef = parts[1]
	}
	domain, err = c.ResolveDomainReference(domainRef)
	if err != nil {
		return nil, nil, fmt.Errorf("domain resolution failed: %w", err)
	}
	if emailRef == "" {
		return domain, nil, fmt.Errorf("email reference is required")
	}
	if id, err := uuid.FromString(emailRef); err == nil {
		emailAddr, err := c.GetEmailAddress(domain.ID, id.String())
		if err != nil {
			return domain, nil, fmt.Errorf("email address not found: %w", err)
		}
		return domain, emailAddr, nil
	}
	emails, err := c.ListEmailAddresses(domain.ID)
	if err != nil {
		return domain, nil, fmt.Errorf("failed to list email addresses: %w", err)
	}
	if emailRef == "*" || emailRef == "catchall" {
		for _, email := range emails {
			if email.IsCatchAll {
				return domain, email, nil
			}
		}
		return domain, nil, fmt.Errorf("no catch-all address found for domain %s", domain.Domain)
	}
	for _, email := range emails {
		if email.LocalPart == emailRef {
			return domain, email, nil
		}
	}
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
