# MailVault

![MailVault Logo](./mailvault_go.png)

**Open-source email infrastructure for developers with multi-provider support.**

MailVault is a developer-focused email service providing private, encrypted email infrastructure where users point their own domains to create secure email addresses. This complete email platform includes REST APIs, CLI client, SMTP server, and multi-provider email delivery with automatic failover, designed for easy integration by developers.

## Features

### Core Email Infrastructure
- **Domain Management**: Point your own domains to create branded email addresses
- **End-to-End Encryption**: All received emails are encrypted with domain public keys
- **Webhook Integration**: Real-time notifications for received emails with configurable webhook settings
- **Email Forwarding**: Forward emails to external addresses or catch-all configurations
- **API Access**: Send emails via REST API using domain API keys
- **CLI Interface**: Complete command-line interface for all email operations
- **SMTP Server**: Full SMTP server for receiving and processing emails
- **Flexible Storage**: Store emails in database or process them via webhooks only
- **Authentication**: Supabase Auth integration with JWT tokens

### Multi-Provider Email Delivery 🆕
- **Multiple Email Providers**: Support for Resend, SendGrid, AWS SES, Postmark, and Mailgun
- **Automatic Failover**: Seamless switching between providers when one fails
- **Load Balancing**: Intelligent routing based on provider health and priority
- **Health Monitoring**: Real-time provider health checks with automatic recovery
- **Rate Limiting**: Per-provider rate limiting to prevent quota exhaustion
- **Delivery Tracking**: Webhook-based delivery status updates from all providers
- **Smart Routing**: Priority-based routing with configurable retry logic

## Quick Start

### 1. Setup and Installation

```bash
git clone https://github.com/guilhermebr/mailvault.git
cd mailvault
cp .env.example .env
```

### 2. Configure Environment

Edit your `.env` file:

```env
# Database Configuration
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=mailvault

# Authentication (Supabase recommended)
AUTH_PROVIDER=supabase

# API Server
API_ADDRESS=:8080

# SMTP Server Configuration
SMTP_ADDR=:25
SMTP_DOMAIN=mail.yourdomain.com

# Email Providers (configure as needed)
RESEND_API_KEY=re_your_api_key_here
SENDGRID_API_KEY=SG.your_api_key_here
AWS_ACCESS_KEY_ID=your_aws_access_key
AWS_SECRET_ACCESS_KEY=your_aws_secret_key
AWS_REGION=us-east-1
POSTMARK_API_KEY=your_postmark_api_key
MAILGUN_API_KEY=your_mailgun_api_key
MAILGUN_DOMAIN=your_mailgun_domain

# Webhook Security
WEBHOOK_SECRET_KEY=your_webhook_secret_for_hmac_verification
```

### 3. Build and Run

```bash
# Install required tools and build all services
make setup
make build

# Run database migrations
make migration/up

# Start API server (in one terminal)
./build/service

# Start SMTP server (in another terminal)
./build/smtpd

# Use CLI interface
./build/cli --help
```

## Architecture

MailVault follows Domain-Driven Design (DDD) principles:

```
mailvault/
├── cmd/                    # Application entry points
│   ├── service/           # API backend server
│   ├── smtpd/             # SMTP server daemon
│   └── cli/               # CLI client application
├── app/                   # Application layer
│   ├── api/               # HTTP handlers and routing
│   ├── smtp/              # SMTP server implementation
│   └── cli/               # CLI command implementations
├── domain/                # Business logic and entities
│   ├── entities/          # Core business entities
│   ├── user/              # User management use cases
│   ├── domain/            # Domain management use cases
│   ├── email/             # Email management use cases
│   └── auth/              # Authentication providers
├── gateways/              # Infrastructure layer
│   └── repository/        # Data persistence
│       └── pg/            # PostgreSQL implementations
└── docs/                  # API documentation
```

## CLI Interface

MailVault provides a comprehensive command-line interface:

### Authentication

```bash
# Login to your account
./build/cli auth login

# Register new account
./build/cli auth register

# View current user
./build/cli user info
```

### Domain Management

```bash
# List all domains
./build/cli domain list

# Create a new domain
./build/cli domain create \
  --domain example.com \
  --public-key "$(cat public_key.pem)" \
  --webhook-url https://api.myapp.com/webhooks/email \
  --storage=true

# Show domain details
./build/cli domain show <domain-id>

# Delete a domain
./build/cli domain delete <domain-id>
```

### Email Address Management

```bash
# List email addresses for a domain
./build/cli email list <domain-id>

# Create a new email address
./build/cli email create \
  --domain <domain-id> \
  --address hello \
  --forward user@gmail.com,backup@proton.me

# Create a catch-all address
./build/cli email create \
  --domain <domain-id> \
  --catch-all \
  --forward admin@mycompany.com

# Delete an email address
./build/cli email delete <domain-id> <email-id>
```

### Inbox Management

```bash
# View received emails (smart resolution with domain names)
./build/cli inbox list example.com hello
./build/cli inbox list hello@example.com

# Show specific email by sequence number or short ID
./build/cli inbox show example.com hello 1
./build/cli inbox show hello@example.com a1b2c3d4

# Interactive mode when no reference provided
./build/cli inbox show example.com hello
```

## API Endpoints

### Authentication
- `POST /api/v1/register` - User registration
- `POST /api/v1/login` - User login
- `GET /api/v1/me` - Get current user

### Domain Management
- `GET /api/v1/domains` - List user domains
- `POST /api/v1/domains` - Create domain
- `GET /api/v1/domains/{id}` - Get domain details
- `PUT /api/v1/domains/{id}` - Update domain
- `DELETE /api/v1/domains/{id}` - Delete domain

### Email Addresses
- `POST /api/v1/domains/{domainId}/emails` - Create email address
- `GET /api/v1/domains/{domainId}/emails` - List domain emails
- `GET /api/v1/domains/{domainId}/emails/{emailId}` - Get email details
- `PUT /api/v1/domains/{domainId}/emails/{emailId}` - Update email
- `DELETE /api/v1/domains/{domainId}/emails/{emailId}` - Delete email
- `GET /api/v1/domains/{domainId}/emails/{emailId}/received` - Get received emails

### Received Emails
- `GET /api/v1/received/{receivedEmailId}` - Get received email by ID
- `DELETE /api/v1/received/{receivedEmailId}` - Delete received email

### Email Sending
- `POST /api/v1/send` - Send email via API (API key auth)

### Provider Management
- `GET /api/v1/domains/{domainId}/providers` - List email providers for domain
- `POST /api/v1/domains/{domainId}/providers` - Create new email provider
- `GET /api/v1/domains/{domainId}/providers/{providerId}` - Get provider details
- `PATCH /api/v1/domains/{domainId}/providers/{providerId}` - Update provider configuration
- `DELETE /api/v1/domains/{domainId}/providers/{providerId}` - Delete provider
- `POST /api/v1/domains/{domainId}/providers/{providerId}/test` - Test provider configuration
- `GET /api/v1/domains/{domainId}/providers/{providerId}/health` - Get provider health status
- `GET /api/v1/domains/{domainId}/providers/{providerId}/stats` - Get provider statistics

### Webhooks
- `POST /api/v1/webhooks/resend` - Resend delivery status webhooks
- `POST /api/v1/webhooks/sendgrid` - SendGrid delivery status webhooks
- `POST /api/v1/webhooks/aws-ses` - AWS SES delivery status webhooks
- `POST /api/v1/webhooks/postmark` - Postmark delivery status webhooks
- `POST /api/v1/webhooks/mailgun` - Mailgun delivery status webhooks

### System
- `GET /health` - Health check

## SMTP Server

The SMTP server handles incoming emails with:

- **TLS Support**: Configurable TLS modes (disabled, certificate files, implicit TLS)
- **Email Processing**: Automatic encryption with domain public keys
- **Webhook Delivery**: Real-time notifications to configured webhook URLs
- **Email Forwarding**: Forward to external email addresses
- **Storage Options**: Store in database or webhook-only processing

### TLS Configuration

```env
# Use certificate files
SMTP_TLS_MODE=cert
SMTP_TLS_CERT=/certs/fullchain.pem
SMTP_TLS_KEY=/certs/privkey.pem

# For implicit TLS (port 465)
SMTP_TLS_IMPLICIT=true
```

## Development

### Prerequisites

- Go 1.24+
- PostgreSQL 13+
- Make

### Development Commands

```bash
# Install development tools
make setup

# Generate code and templates
make generate

# Run tests
make test
make test-full

# Code quality
make lint
make gosec

# Database migrations
make migration/create
make migration/up
make migration/down
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
make coverage

# Run security analysis
make gosec
```

## Technology Stack

- **Language**: Go 1.24
- **Database**: PostgreSQL with pgx driver
- **HTTP Router**: Chi v5
- **CLI Framework**: Cobra
- **SMTP**: github.com/emersion/go-smtp
- **Authentication**: Supabase Auth integration
- **Email Providers**: Official Go SDKs for Resend, SendGrid, AWS SES, Postmark, Mailgun
- **Documentation**: OpenAPI/Swagger
- **Testing**: Standard Go testing with testify

## External Dependencies

- **MailVault Go SDK**: External SDK for API communication
- **gox libraries**: Logging and PostgreSQL utilities
- **Supabase**: Authentication provider
- **Provider SDKs**: Official Go SDKs for email service providers

## Documentation

### Getting Started
- **[Quick Start Guide](docs/QUICK_START.md)** - Get up and running in under 10 minutes
- **[Migration Guide](docs/MIGRATION.md)** - Migrate from single-provider to multi-provider system

### Multi-Provider System
- **[Email Providers Guide](docs/EMAIL_PROVIDERS.md)** - Complete guide to configuring and managing email providers
- **[Provider API Reference](docs/PROVIDER_API.md)** - Complete API documentation for provider management
- **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** - Common issues and debugging techniques

### Technical Documentation
- **[Monitoring Guide](docs/MONITORING.md)** - System monitoring and observability
- **[Database Optimization](docs/DATABASE_OPTIMIZATION.md)** - Database performance optimization
- **[OpenAPI/Swagger](docs/swagger.yaml)** - Complete API specification

## Security

- All received emails are encrypted using domain public keys
- API key authentication for email sending
- JWT token authentication for user operations
- Input validation and sanitization
- Rate limiting on API endpoints
- Secure token handling and storage

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run lints and tests: `make test lint gosec`
6. Submit a pull request

## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT).

## Support

- Issues: [GitHub Issues](https://github.com/mailvault/mailvault/issues)