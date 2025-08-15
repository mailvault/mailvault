# MailVault Go SDK

The official Go SDK for MailVault - a developer-focused email service providing private, encrypted email services.

## Features

- **User Authentication**: Register, login, and user management
- **Domain Management**: Create and manage domains with encryption keys
- **Email Address Management**: Configure email addresses with forwarding and catch-all options
- **Email Sending**: Send emails via API using domain API keys
- **Received Email Access**: Access received emails through secure endpoints
- **Webhook Support**: Configure webhooks for real-time email notifications
- **Comprehensive Error Handling**: Detailed error responses with status codes

## Installation

```bash
go get github.com/guilhermebr/mailvault/gateways/mailvault
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/guilhermebr/mailvault/gateways/mailvault"
)

func main() {
    ctx := context.Background()
    
    // Create client
    client := mailvault.NewClient(mailvault.ClientConfig{
        BaseURL: "https://api.mailvault.sh",
    })
    
    // Register user
    auth, err := client.Register(ctx, mailvault.RegisterRequest{
        Email:    "user@example.com",
        Password: "securepassword123",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("User registered: %s", auth.User.Email)
}
```

### Sending Emails Only

If you only need to send emails and already have a domain API key:

```go
package main

import (
    "context"
    "log"
    
    "github.com/guilhermebr/mailvault/gateways/mailvault"
)

func main() {
    ctx := context.Background()
    
    // Create client with domain API key
    client := mailvault.NewClientForDomain("https://api.mailvault.sh", "your-api-key")
    
    // Send email
    resp, err := client.SendEmail(ctx, mailvault.SendEmailRequest{
        From:     "noreply@yourdomain.com",
        To:       []string{"customer@example.com"},
        Subject:  "Welcome!",
        TextBody: "Welcome to our service!",
        HTMLBody: "<h1>Welcome!</h1><p>Thanks for joining us.</p>",
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Email sent: %s", resp.MessageID)
}
```

## API Reference

### Client Configuration

```go
type ClientConfig struct {
    BaseURL    string        // API base URL (default: https://api.mailvault.sh)
    HTTPClient *http.Client  // Custom HTTP client (optional)
    AuthToken  string        // JWT token for user authentication (optional)
    APIKey     string        // Domain API key for sending emails (optional)
}
```

### Authentication

#### Register
```go
auth, err := client.Register(ctx, mailvault.RegisterRequest{
    Email:    "user@example.com",
    Password: "password123",
})
```

#### Login
```go
auth, err := client.Login(ctx, mailvault.LoginRequest{
    Email:    "user@example.com", 
    Password: "password123",
})
```

#### Get Current User
```go
user, err := client.Me(ctx)
```

### Domain Management

#### Create Domain
```go
domain, err := client.CreateDomain(ctx, mailvault.CreateDomainRequest{
    Domain:    "example.com",
    PublicKey: "-----BEGIN PUBLIC KEY-----...",
    WebhookConfig: &mailvault.WebhookConfig{
        URL:     "https://yourapp.com/webhook",
        Secret:  "webhook-secret",
        Enabled: true,
    },
})
```

#### Get Domains
```go
domains, err := client.GetDomains(ctx)
```

#### Update Domain
```go
verified := true
domain, err := client.UpdateDomain(ctx, domainID, mailvault.UpdateDomainRequest{
    Verified: &verified,
})
```

#### Delete Domain
```go
err := client.DeleteDomain(ctx, domainID)
```

### Email Address Management

#### Create Email Address
```go
email, err := client.CreateEmailAddress(ctx, domainID, mailvault.CreateEmailRequest{
    LocalPart: "hello",
    ForwardAddresses: []string{"forward@external.com"},
    IsCatchAll: false,
})
```

#### Get Email Addresses
```go
emails, err := client.GetEmailAddresses(ctx, domainID)
```

#### Update Email Address
```go
catchAll := true
email, err := client.UpdateEmailAddress(ctx, domainID, emailID, mailvault.UpdateEmailRequest{
    IsCatchAll: &catchAll,
})
```

#### Delete Email Address
```go
err := client.DeleteEmailAddress(ctx, domainID, emailID)
```

### Email Sending

#### Send Email
```go
resp, err := client.SendEmail(ctx, mailvault.SendEmailRequest{
    From:     "sender@yourdomain.com",
    To:       []string{"recipient@example.com"},
    CC:       []string{"cc@example.com"},
    BCC:      []string{"bcc@example.com"},
    Subject:  "Subject",
    TextBody: "Plain text body",
    HTMLBody: "<p>HTML body</p>",
})
```

### Received Emails

#### Get Received Emails (Paginated)
```go
limit := 10
offset := 0
result, err := client.GetReceivedEmails(ctx, domainID, emailID, &mailvault.GetReceivedEmailsOptions{
    Limit:  &limit,
    Offset: &offset,
})
```

#### Get Specific Received Email
```go
email, err := client.GetReceivedEmail(ctx, receivedEmailID)
```

### Health Check

```go
health, err := client.Health(ctx)
```

## Error Handling

The SDK provides structured error handling:

```go
domains, err := client.GetDomains(ctx)
if err != nil {
    if apiErr, ok := err.(*mailvault.APIError); ok {
        // Handle API error
        log.Printf("API Error %d: %s", apiErr.StatusCode, apiErr.Message)
    } else {
        // Handle other errors (network, parsing, etc.)
        log.Printf("Error: %v", err)
    }
}
```

### Common Error Status Codes

- `400` - Bad Request (validation errors, invalid data)
- `401` - Unauthorized (missing/invalid auth token or API key)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found (resource doesn't exist)
- `500` - Internal Server Error

## Authentication Types

### JWT Authentication (User Operations)
Used for domain management, email address configuration, and accessing received emails:

```go
client := mailvault.NewClient(mailvault.ClientConfig{
    BaseURL:   "https://api.mailvault.sh",
    AuthToken: "your-jwt-token",
})
```

### API Key Authentication (Email Sending)
Used for sending emails:

```go
client := mailvault.NewClientForDomain("https://api.mailvault.sh", "your-domain-api-key")
```

## Examples

See `examples.go` for comprehensive usage examples including:

- User registration and authentication
- Domain and email management
- Sending emails
- Handling received emails
- Error handling patterns

## Configuration

### Environment Variables

You can configure the SDK using environment variables:

```bash
export MAILVAULT_API_URL="https://api.mailvault.sh"
export MAILVAULT_AUTH_TOKEN="your-jwt-token" 
export MAILVAULT_API_KEY="your-domain-api-key"
```

### Custom HTTP Client

```go
httpClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
    },
}

client := mailvault.NewClient(mailvault.ClientConfig{
    HTTPClient: httpClient,
})
```

## Contributing

This SDK is part of the MailVault project. See the main repository for contributing guidelines.

## License

MIT License - see the main MailVault repository for details.