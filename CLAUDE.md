# MailSafe Development Guide for Claude

## Project Overview
MailSafe (mailsafe.sh) is an open-source, developer-focused email service providing private, encrypted email services where users point their own domains to create secure email addresses. This is a complete email platform with web frontend, APIs, and SMTP server, designed for easy integration by developers similar to Resend and inbox.new.

## Current Status
- **Status**: Core development completed, ready for SMTP server implementation
- **Location**: `/home/guilhermebr/code/guilhermebr/privatemail` (will be renamed to mailsafe)
- **Domain**: mailsafe.sh
- **Technology**: Go-based email service with SSR frontend

## Architecture Overview

### Core Services
1. **Web Frontend** (`cmd/web`) - Server-side rendered with Templ + HTMX + Tailwind
2. **API Backend** (`cmd/service`) - REST APIs with OpenAPI/Swagger
3. **Admin Panel** (`cmd/admin`) - Management interface
4. **SMTP Server** (`cmd/smtpd`) - Email receiving/sending using github.com/emersion/go-smtp

### Key Features
- **User Authentication**: Email/password + OAuth2 (Google, Github)
- **Domain Management**: Users add domains with public keys for encryption
- **Email Addresses**: Create emails per domain with webhook/catch-all/forwarding options
- **API Access**: Send emails via API using domain API_KEY
- **Security**: End-to-end encryption for all received emails

## Technology Stack

### Backend (Go)
```go
// Core Dependencies
github.com/guilhermebr/gox/logger     // Logging utilities
github.com/guilhermebr/gox/postgres   // Database utilities  
github.com/supabase/auth              // Supabase Auth provider integration
github.com/emersion/go-smtp           // SMTP server
github.com/go-chi/chi/v5              // HTTP router
github.com/jackc/pgx/v5               // PostgreSQL driver
```

### Frontend
- **Server Side Rendering**: Go with Templ templates
- **Interactivity**: HTMX for dynamic behavior
- **Styling**: Tailwind CSS
- **Build**: Standard Go build process

### Infrastructure
- **Database**: PostgreSQL with migrations
- **Authentication**: Supabase (recommended) or Firebase
- **Observability**: OpenTelemetry with tracing and structured logging
- **Documentation**: OpenAPI/Swagger for APIs

## Project Structure (DDD Pattern from go-template)
```
mailsafe/
├── cmd/
│   ├── web/           # Frontend server (Templ + HTMX)
│   ├── service/       # API backend server
│   ├── admin/         # Admin panel server
│   └── smtpd/         # SMTP server daemon (minimal main.go)
├── domain/
│   ├── entities/      # Core business entities
│   ├── user/          # User management use cases
│   ├── domain/        # Domain management use cases  
│   ├── email/         # Email management use cases
│   └── auth/          # Authentication use cases
├── internal/
│   ├── api/           # HTTP handlers and routing
│   ├── config/        # Configuration management
│   ├── smtp/          # SMTP server implementation
│   │   ├── server.go  # SMTP server wrapper
│   │   ├── backend.go # SMTP backend implementation
│   │   └── session.go # SMTP session handling
│   └── repository/    # Data persistence layer
│       └── pg/        # PostgreSQL implementations
├── migrations/        # Database migrations
├── web/              # Static assets and templates
└── docs/             # API documentation
```

## Database Schema Design

### Core Entities
```sql
-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    auth_provider VARCHAR(50) NOT NULL, -- 'supabase', 'firebase', 'email'
    auth_provider_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Domains table  
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    domain VARCHAR(255) UNIQUE NOT NULL,
    public_key TEXT NOT NULL, -- For email encryption
    api_key VARCHAR(255) UNIQUE NOT NULL, -- For API access
    verified BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Email addresses table
CREATE TABLE email_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID REFERENCES domains(id) ON DELETE CASCADE,
    local_part VARCHAR(255) NOT NULL, -- part before @
    webhook_url VARCHAR(500),
    is_catch_all BOOLEAN DEFAULT false,
    forward_addresses TEXT[], -- JSON array of forward emails
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(domain_id, local_part)
);

-- Received emails table
CREATE TABLE received_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_address_id UUID REFERENCES email_addresses(id),
    from_address VARCHAR(255) NOT NULL,
    subject VARCHAR(500),
    encrypted_body TEXT NOT NULL, -- Encrypted with domain public key
    received_at TIMESTAMPTZ DEFAULT now()
);
```

## Authentication Strategy

### Recommended: Supabase Auth ✅ IMPLEMENTED
- **Pros**: Free tier, PostgreSQL included, simple setup, good Go support
- **Implementation**: HTTP-based client using Supabase Auth REST API
- **Features**: Email/password + OAuth2 (Google, GitHub, etc.)
- **Configuration**: Set `AUTH_PROVIDER=supabase` with `SUPABASE_URL` and `SUPABASE_API_KEY`

### Implementation Pattern
```go
// Abstract auth interface
type AuthProvider interface {
    Provider() string
    CreateUser(ctx context.Context, user User) (string, error)
    Login(ctx context.Context, email, password string) (string, error)
    ValidateToken(ctx context.Context, token string) (*User, error)
}

// Use factory pattern to switch providers
func NewAuthProvider(config AuthConfig) AuthProvider {
    switch config.Provider {
    case "supabase":
        return supabase.NewAuth(config.Supabase)
    case "firebase": 
        return firebase.NewAuth(config.Firebase)
    default:
        return basic.NewAuth(config.Database)
    }
}
```

## Development Workflow

### Getting Started
1. **Clone go-template structure**: Copy from `/home/guilhermebr/code/guilhermebr/go-template`
2. **Update dependencies**: Use gox libraries from `/home/guilhermebr/code/guilhermebr/gox`
3. **Study sasmail**: Reference `/home/guilhermebr/code/guilhermebr/receive/sasmail` for SMTP patterns

### Commands to Run
```bash
# Development
make build        # Build service and smtpd binaries
./build/service   # Start API backend server  
./build/smtpd     # Start SMTP server

# Testing
go test ./...                    # Run all tests
go test -race ./...             # Run with race detection  
go test -coverprofile=cover.out ./...  # Generate coverage

# Database
make migration/up    # Run database migrations
make migration/down  # Rollback migrations

# Code Quality
make lint         # Run linter
make gosec        # Run security analysis
make test-full    # Run comprehensive tests

# Environment
cp .env.example .env  # Copy environment template
# Edit .env with your database and service configuration
```

## Security Requirements

### Email Encryption
- **All received emails MUST be encrypted** using domain public key
- **Keys stored securely** with proper access controls  
- **No plaintext email storage** in database

### API Security
- **Domain API keys** for sending email via API
- **Rate limiting** on all endpoints
- **Input validation** to prevent injection attacks
- **HTTPS only** for all web traffic

### Authentication Security
- **Secure token handling** with proper expiration
- **OAuth2 state verification** to prevent CSRF
- **Password requirements** if using email/password auth

## API Design Patterns

### RESTful Endpoints
```go
// Domain management
POST   /api/v1/domains              # Create domain
GET    /api/v1/domains              # List user domains  
GET    /api/v1/domains/{id}         # Get domain details
PUT    /api/v1/domains/{id}         # Update domain
DELETE /api/v1/domains/{id}         # Delete domain

// Email address management  
POST   /api/v1/domains/{id}/emails  # Create email address
GET    /api/v1/domains/{id}/emails  # List domain emails
PUT    /api/v1/domains/{id}/emails/{email_id}  # Update email
DELETE /api/v1/domains/{id}/emails/{email_id}  # Delete email

// Email sending (with API key auth)
POST   /api/v1/send                 # Send email using domain API key
```

### Error Handling
```go
// Standardized error responses
type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details any    `json:"details,omitempty"`
}
```

## Frontend Development (HTMX + Templ)

### Page Structure
```go
// Templ templates for each page
templ Dashboard(user *User, domains []*Domain) {
    @layout.Base("Dashboard") {
        <div hx-get="/dashboard/domains" hx-trigger="load">
            // Domain list component
        </div>
    }
}

// HTMX endpoints for dynamic updates
func (h *Handlers) DashboardDomains(w http.ResponseWriter, r *http.Request) {
    // Return partial HTML for HTMX swap
}
```

### Key Pages
- **Landing Page**: Marketing site with signup
- **Dashboard**: Domain and email management  
- **Settings**: User preferences and billing
- **Documentation**: API docs and guides

## SMTP Server Implementation

### Using github.com/emersion/go-smtp
```go
// SMTP server setup
type SMTPServer struct {
    emailService *email.Service
    logger       logger.Logger
}

func (s *SMTPServer) Login(username, password string) error {
    // Implement SMTP authentication
}

func (s *SMTPServer) Data(from string, to []string, data []byte) error {
    // Process incoming email
    // 1. Parse recipients  
    // 2. Find matching email addresses
    // 3. Encrypt email content
    // 4. Store in database
    // 5. Trigger webhooks if configured
    // 6. Forward if configured
}
```

## Observability & Monitoring

### OpenTelemetry Integration
```go
// Tracing setup
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

// Add tracing to critical paths
func (s *EmailService) CreateEmailAddress(ctx context.Context, req CreateEmailRequest) error {
    ctx, span := otel.Tracer("privatemail").Start(ctx, "EmailService.CreateEmailAddress")
    defer span.End()
    
    // Implementation with structured logging
    s.logger.Info(ctx, "creating email address", 
        "domain_id", req.DomainID,
        "local_part", req.LocalPart)
}
```

### Logging Strategy
- **Structured logging** using gox/logger
- **No sensitive data** in logs (emails, passwords, keys)
- **Request tracing** with correlation IDs
- **Performance metrics** for critical operations

## Testing Strategy

### Test Categories
1. **Unit Tests**: Business logic in domain layer
2. **Integration Tests**: Database operations and external APIs  
3. **E2E Tests**: Full user flows via HTTP endpoints
4. **SMTP Tests**: Email receiving and processing flows

### Test Utilities
```go
// Use existing testutils from gox
import "github.com/guilhermebr/gox/testutils"

func TestCreateDomain(t *testing.T) {
    // Setup test database
    db := testutils.NewTestDB(t)
    defer db.Close()
    
    // Test implementation
}
```

## Deployment & Infrastructure

### Docker Setup
```dockerfile
# Multi-stage build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o privatemail-web ./cmd/web

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/privatemail-web /
CMD ["./privatemail-web"]
```

### Environment Variables
```bash
# Database
DATABASE_URL=postgres://user:pass@localhost/privatemail

# Auth Provider  
AUTH_PROVIDER=supabase
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_KEY=your-anon-key

# SMTP
SMTP_HOST=0.0.0.0
SMTP_PORT=25
SMTP_TLS=true

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
```

## Business Model & Scaling

### Pricing Tiers
- **Starter**: $5/month - 1 domain, 1K emails
- **Professional**: $15/month - 3 domains, 10K emails  
- **Business**: $35/month - 10 domains, 50K emails
- **Enterprise**: Custom pricing

### Success Metrics
- **Technical**: 99.9% uptime, <100ms API response time
- **Business**: 1000 users in 6 months, $10K MRR in year 1
- **Security**: Zero data breaches, SOC2 compliance

## Important Notes for Claude

### Code Patterns
- **Follow DDD**: Use domain/entities, domain/usecase pattern from go-template
- **Minimize dependencies**: Prefer stdlib over external packages
- **Security first**: Always encrypt sensitive data, validate inputs
- **Testing**: Write tests for all business logic

### Reference Projects
- **Structure**: `/home/guilhermebr/code/guilhermebr/go-template`
- **Libraries**: `/home/guilhermebr/code/guilhermebr/gox`  
- **SMTP Inspiration**: `/home/guilhermebr/code/guilhermebr/receive/sasmail`

### Development Priorities
1. **Phase 1**: Core APIs and basic web frontend
2. **Phase 2**: SMTP server and email processing
3. **Phase 3**: Admin panel and monitoring
4. **Phase 4**: Advanced features and optimization

Remember: This handles sensitive email data - security, privacy, and reliability are paramount. Always encrypt emails at rest, validate all inputs, and follow secure coding practices.