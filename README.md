# MailVault

![MailVault Logo](./docs/assets/mailvault_go.png)

**Open-source, self-hostable email infrastructure for developers.**

MailVault is a developer-focused email service providing private, encrypted email infrastructure where users point their own domains at the server to create secure email addresses. It exposes a REST API for integration, accepts inbound mail over SMTP, and delivers outbound mail through a local SMTP relay (the host's MTA or a configured smart-host).

## Features

- **Domain Management**: Point your own domains to create branded email addresses
- **End-to-End Encryption**: Received emails are encrypted with the domain's public key
- **Webhook Integration**: Real-time notifications to user-configured URLs when mail arrives, with configurable signing, retries, and audit logs
- **Email Forwarding**: Forward inbound mail to external addresses or catch-all destinations
- **API Sending**: `POST /api/v1/send` accepts authenticated requests and hands them off to the configured outbound SMTP relay
- **SMTP Server**: Full inbound SMTP daemon (port 25 / 465 / 587) with optional STARTTLS
- **Flexible Storage**: Store received emails in the database, deliver via webhook, or both
- **Pluggable Authentication**: Built-in local users with bcrypt + JWT by default; the provider interface lets deployments swap in their own (OIDC, Supabase, etc.)

## Quick Start

### 1. Setup and Installation

```bash
git clone https://github.com/mailvault/mailvault.git
cd mailvault
cp .env.example .env
```

### 2. Configure Environment

Edit your `.env` file:

```env
# Database
DATABASE_HOST=localhost
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=mailvault

# API server
API_ADDRESS=0.0.0.0:3000

# Inbound SMTP daemon
SMTP_ADDR=:25
SMTP_DOMAIN=mail.yourdomain.com

# Outbound SMTP relay (a local Postfix on localhost:25 is the default;
# point this at a smart-host if you don't run an MTA on the same machine).
OUTBOUND_SMTP_ADDR=localhost:25
OUTBOUND_SMTP_HOSTNAME=mail.yourdomain.com
OUTBOUND_SMTP_TLS_MODE=none           # none|starttls|implicit
OUTBOUND_SMTP_USERNAME=
OUTBOUND_SMTP_PASSWORD=

# Auth (built-in local provider)
AUTH_PROVIDER=local
AUTH_SECRET_KEY=replace_with_32_random_bytes_or_more
AUTH_TOKEN_TTL=24h
```

### 3. Build and Run

```bash
# Install dev tooling and build all binaries
make setup
make build

# Run database migrations
make migration/up

# Start the API server
./build/service

# In another terminal, start the SMTP daemon
./build/smtpd
```

## Architecture

MailVault follows Domain-Driven Design (DDD):

```
mailvault/
├── cmd/
│   ├── service/         # API backend (HTTP)
│   └── smtpd/           # SMTP daemon (inbound mail)
├── app/
│   ├── api/             # HTTP handlers and routing
│   ├── service/         # Reusable service.Run entry point
│   ├── smtp/            # SMTP server, forwarder
│   ├── smtpd/           # Reusable smtpd.Run entry point
│   └── worker/          # Async worker / queue
├── domain/
│   ├── entities/        # Core entities (User, Domain, EmailAddress, ...)
│   ├── extensions/      # Pluggable extension interfaces (DomainLimiter, UsageTracker)
│   ├── auth/            # Provider interface + built-in local provider
│   ├── domain/          # Domain management use case
│   ├── email/           # Email address use case
│   ├── email_sending/   # Outbound mail use case (Sender interface)
│   ├── webhook_config/  # User-configured webhook subscriptions
│   ├── smtp_stats/      # SMTP verification statistics
│   ├── user/            # User management use case
│   └── validation/      # Domain & email validation
├── gateways/
│   └── repository/pg/   # PostgreSQL implementations + migrations
├── internal/
│   ├── database/        # Connection pool
│   ├── encryption/      # X25519 / ChaCha20-Poly1305 for at-rest mail
│   ├── smtprelay/       # Outbound SMTP-relay sender (the default email_sending.Sender)
│   ├── utils/
│   └── webhook/         # User-webhook delivery client
└── docs/                # OpenAPI/Swagger + monitoring / database guides
```

## API Endpoints

### Authentication
- `POST /api/v1/register` — User registration
- `POST /api/v1/login` — User login
- `GET /api/v1/me` — Get current user

### Domain Management
- `GET /api/v1/domains` — List user domains
- `POST /api/v1/domains` — Create domain
- `GET /api/v1/domains/{id}` — Get domain details
- `PUT /api/v1/domains/{id}` — Update domain
- `DELETE /api/v1/domains/{id}` — Delete domain

### Email Addresses
- `POST /api/v1/domains/{domainId}/emails` — Create email address
- `GET /api/v1/domains/{domainId}/emails` — List domain emails
- `GET /api/v1/domains/{domainId}/emails/{emailId}` — Get email details
- `PUT /api/v1/domains/{domainId}/emails/{emailId}` — Update email
- `DELETE /api/v1/domains/{domainId}/emails/{emailId}` — Delete email
- `GET /api/v1/domains/{domainId}/emails/{emailId}/received` — Get received emails

### Received Emails
- `GET /api/v1/received/{receivedEmailId}` — Get received email by ID
- `DELETE /api/v1/received/{receivedEmailId}` — Delete received email

### Email Sending
- `POST /api/v1/send` — Submit email via API (domain API key auth) — handed off to the local SMTP relay

### Webhook Configurations
- `POST /api/v1/domains/{domainId}/webhooks` — Create webhook config
- `GET /api/v1/domains/{domainId}/webhooks` — List webhook configs
- `GET /api/v1/domains/{domainId}/webhooks/{webhookId}` — Get config
- `PUT /api/v1/domains/{domainId}/webhooks/{webhookId}` — Update config
- `DELETE /api/v1/domains/{domainId}/webhooks/{webhookId}` — Delete config
- `POST /api/v1/domains/{domainId}/webhooks/{webhookId}/test` — Test delivery
- `GET /api/v1/domains/{domainId}/webhooks/{webhookId}/health` — Health
- `GET /api/v1/domains/{domainId}/webhooks/{webhookId}/audit` — Audit log

### System
- `GET /health` — Liveness + dependency check
- `GET /ready` — Kubernetes-style readiness

The full OpenAPI specification is generated at [`docs/swagger.yaml`](docs/swagger.yaml).

## SMTP Server

The inbound SMTP daemon (`cmd/smtpd`) handles incoming mail with:

- **TLS Support**: Configurable modes (`off`, `cert`, `implicit`)
- **Email Processing**: Automatic encryption with the domain's public key
- **Webhook Delivery**: Real-time notifications to user-configured URLs
- **Email Forwarding**: Optional forward to external addresses

### TLS Configuration

```env
# Certificate files (port 587 + STARTTLS, or port 465 implicit)
SMTP_TLS_MODE=cert
SMTP_TLS_CERT=/certs/fullchain.pem
SMTP_TLS_KEY=/certs/privkey.pem

# Implicit TLS on port 465
SMTP_TLS_IMPLICIT=true
```

## Outbound Mail

Outbound `POST /api/v1/send` requests are persisted to `sent_emails` and then submitted to the SMTP relay configured by `OUTBOUND_SMTP_*`. The default works for any host running a local MTA (Postfix, sendmail) on `localhost:25`. Pointing at a smart-host (with `OUTBOUND_SMTP_TLS_MODE=starttls`, `OUTBOUND_SMTP_USERNAME`, `OUTBOUND_SMTP_PASSWORD`) is supported.

## Development

### Prerequisites

- Go 1.26+
- PostgreSQL 13+
- Make

### Development Commands

```bash
make setup            # Install dev tooling
make generate         # Generate mocks + swagger
make test             # Short tests (no docker)
make test-full        # All tests
make lint             # golangci-lint
make gosec            # Security scan
make migration/up
make migration/down
make migration/create
```

## Technology Stack

- **Language**: Go 1.26
- **Database**: PostgreSQL via pgx/v5
- **HTTP Router**: chi/v5
- **SMTP**: github.com/emersion/go-smtp (server + client)
- **Auth**: built-in local provider (bcrypt + JWT/HS256)
- **Documentation**: OpenAPI/Swagger via swaggo
- **Testing**: standard testing + testify + dockertest

## Security

- Received emails are encrypted using the domain's public key (X25519 + ChaCha20-Poly1305)
- Domain API keys authenticate email-sending requests
- JWT tokens authenticate user operations
- Rate limiting on public endpoints
- Input validation across the API surface

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint gosec`
6. Submit a pull request

## License

[MIT](https://opensource.org/licenses/MIT).

## Support

- Issues: [GitHub Issues](https://github.com/mailvault/mailvault/issues)
