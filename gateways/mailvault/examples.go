package mailvault

import (
	"context"
	"fmt"
	"log"
)

// Example demonstrates how to use the MailVault SDK
func Example() {
	ctx := context.Background()

	// Create a new client
	client := NewClient(ClientConfig{
		BaseURL: "https://api.mailvault.sh", // or your local API URL
	})

	// Example 1: User Registration and Authentication
	fmt.Println("=== User Registration Example ===")
	authResp, err := client.Auth.Register(ctx, RegisterRequest{
		Email:    "user@example.com",
		Password: "securepassword123",
	})
	if err != nil {
		log.Printf("Registration failed: %v", err)
		return
	}
	fmt.Printf("User registered successfully: %s\n", authResp.User.Email)

	// Example 2: Login (alternative to registration)
	fmt.Println("\n=== User Login Example ===")
	loginResp, err := client.Auth.Login(ctx, LoginRequest{
		Email:    "user@example.com",
		Password: "securepassword123",
	})
	if err != nil {
		log.Printf("Login failed: %v", err)
		return
	}
	fmt.Printf("User logged in successfully: %s\n", loginResp.User.Email)

	// Example 3: Get current user info
	fmt.Println("\n=== Get User Info Example ===")
	user, err := client.Auth.Me(ctx)
	if err != nil {
		log.Printf("Get user info failed: %v", err)
		return
	}
	fmt.Printf("Current user: %s (ID: %s)\n", user.Email, user.ID)

	// Example 4: Create a domain
	fmt.Println("\n=== Create Domain Example ===")
	storageEnabled := true
	domain, err := client.Domains.CreateDomain(ctx, CreateDomainRequest{
		Domain:    "example.com",
		PublicKey: "-----BEGIN PUBLIC KEY-----\nYour-PGP-Public-Key-Here\n-----END PUBLIC KEY-----",
		WebhookConfig: &WebhookConfig{
			URL:     "https://your-app.com/webhook",
			Secret:  "webhook-secret",
			Enabled: true,
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		},
		StorageEnabled: &storageEnabled,
	})
	if err != nil {
		log.Printf("Create domain failed: %v", err)
		return
	}
	fmt.Printf("Domain created: %s (API Key: %s)\n", domain.Domain, domain.APIKey)

	// Example 5: Get all domains
	fmt.Println("\n=== Get Domains Example ===")
	domains, err := client.Domains.GetDomains(ctx)
	if err != nil {
		log.Printf("Get domains failed: %v", err)
		return
	}
	for _, d := range domains {
		fmt.Printf("Domain: %s, Verified: %t\n", d.Domain, d.Verified)
	}

	// Example 6: Create email address
	fmt.Println("\n=== Create Email Address Example ===")
	email, err := client.Emails.CreateEmailAddress(ctx, domain.ID, CreateEmailRequest{
		LocalPart: "hello",
		ForwardAddresses: []string{
			"forward1@external.com",
			"forward2@external.com",
		},
		IsCatchAll: false,
	})
	if err != nil {
		log.Printf("Create email address failed: %v", err)
		return
	}
	fmt.Printf("Email address created: %s\n", email.FullAddress)

	// Example 7: Get email addresses for domain
	fmt.Println("\n=== Get Email Addresses Example ===")
	emailAddresses, err := client.Emails.GetEmailAddresses(ctx, domain.ID)
	if err != nil {
		log.Printf("Get email addresses failed: %v", err)
		return
	}
	for _, ea := range emailAddresses {
		fmt.Printf("Email: %s, Catch-all: %t, Forwards: %v\n",
			ea.FullAddress, ea.IsCatchAll, ea.ForwardAddresses)
	}

	// Example 8: Send email using domain API key
	fmt.Println("\n=== Send Email Example ===")

	// Create a new client with domain API key for sending emails
	sendClient := NewClientForDomain("https://api.mailvault.sh", domain.APIKey)

	sendResp, err := sendClient.Send.SendEmail(ctx, SendEmailRequest{
		From:     "hello@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test Email from MailVault SDK",
		TextBody: "This is a test email sent using the MailVault SDK.",
		HTMLBody: "<h1>Test Email</h1><p>This is a test email sent using the <strong>MailVault SDK</strong>.</p>",
	})
	if err != nil {
		log.Printf("Send email failed: %v", err)
		return
	}
	fmt.Printf("Email queued for delivery: %s (Status: %s)\n", sendResp.MessageID, sendResp.Status)

	// Example 9: Get received emails (with pagination)
	fmt.Println("\n=== Get Received Emails Example ===")
	limit := 10
	offset := 0
	receivedEmails, err := client.Emails.GetReceivedEmails(ctx, domain.ID, email.ID, &GetReceivedEmailsOptions{
		Limit:  &limit,
		Offset: &offset,
	})
	if err != nil {
		log.Printf("Get received emails failed: %v", err)
		return
	}
	fmt.Printf("Found %d received emails (showing %d)\n",
		receivedEmails.Pagination.Total, len(receivedEmails.Data.([]*ReceivedEmail)))

	// Example 10: Health check
	fmt.Println("\n=== Health Check Example ===")
	health, err := client.Auth.Health(ctx)
	if err != nil {
		log.Printf("Health check failed: %v", err)
		return
	}
	fmt.Printf("API Status: %s\n", health.Status)
}

// ExampleSendingOnly demonstrates how to use the SDK only for sending emails
func ExampleSendingOnly() {
	ctx := context.Background()

	// Create a client configured only for sending emails
	client := NewClientForDomain("https://api.mailvault.sh", "your-domain-api-key")

	// Send email
	resp, err := client.Send.SendEmail(ctx, SendEmailRequest{
		From:     "noreply@yourdomain.com",
		To:       []string{"customer@example.com"},
		CC:       []string{"support@yourdomain.com"},
		Subject:  "Welcome to Our Service",
		TextBody: "Welcome! Thanks for signing up.",
		HTMLBody: "<h1>Welcome!</h1><p>Thanks for signing up for our service.</p>",
	})

	if err != nil {
		log.Printf("Failed to send email: %v", err)
		return
	}

	fmt.Printf("Email sent successfully. Message ID: %s\n", resp.MessageID)
}

// ExampleErrorHandling demonstrates error handling
func ExampleErrorHandling() {
	ctx := context.Background()
	client := NewClient(ClientConfig{})

	// This will fail due to missing auth token
	_, err := client.Domains.GetDomains(ctx)
	if err != nil {
		// Check if it's an API error
		if apiErr, ok := err.(*APIError); ok {
			fmt.Printf("API Error: %d - %s\n", apiErr.StatusCode, apiErr.Message)
		} else {
			fmt.Printf("Other error: %v\n", err)
		}
	}
}
