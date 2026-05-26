# MailVault

<p align="center">
  <img src="./docs/assets/mailvault_go.png" alt="MailVault" width="320">
</p>

**Open-source, self-hostable email infrastructure for developers.**

MailVault is a developer-focused email service: point your own domains at it to create encrypted email addresses, receive mail over SMTP, query everything over a REST API, and send mail back out through a local SMTP relay. Built in Go with a strict DDD layout, pluggable auth, and a small set of extension interfaces for deployments that want to layer quotas / metering / custom transports on top.

---

## Highlights

- **Self-hostable.** One Go module, one Postgres, one binary per role. No managed services required.
- **Bring your own domains.** Each domain holds an X25519 public key; received mail is encrypted at rest with it.
- **Inbound SMTP daemon.** Listens on 25 / 465 / 587, supports `off` / STARTTLS / implicit TLS, runs as a separate binary (`cmd/smtpd`).
- **Outbound via local SMTP relay.** `POST /api/v1/send` persists the message and hands it to the configured relay (default `localhost:25`) — let your host's MTA own queueing, retry, and DKIM.
- **Webhook delivery.** Per-domain webhook configurations with signing, retries, audit log, and health checks.
- **Inbound forwarding.** Optionally forward received mail to external addresses.
- **Pluggable authentication.** Ships with a built-in `local` provider (users table + bcrypt + JWT/HS256). Deployments can register additional providers in their own `cmd/service/auth.go`.
- **Extension seams.** `DomainLimiter`, `UsageTracker`, and `Sender` interfaces let overlays bolt on quotas, metering, or custom outbound transports without forking.
- **Observability.** Prometheus metrics on `:8080`, SMTP-verification stats, an admin viewer for stats + users, plus a turnkey Grafana + AlertManager stack in `monitoring/`.

---

## Quick start

### Option A — Docker Compose (fastest)

```bash
git clone https://github.com/mailvault/mailvault.git
cd mailvault
cp .env.example .env          # edit DATABASE_*, AUTH_SECRET_KEY, SMTP_DOMAIN

docker compose up -d          # postgres + api + smtpd
docker compose exec api ./service --help   # sanity check
```

API on http://localhost:3000, metrics on http://localhost:8080/metrics, inbound SMTP on `:25` and `:587`.

### Option B — Build from source

```bash
git clone https://github.com/mailvault/mailvault.git
cd mailvault
cp .env.example .env
make setup            # install dev tools (moq, swag, golangci-lint, govulncheck)
make build            # produces build/service, build/smtpd, build/worker
make migration/up     # apply DB migrations

./build/service &     # API server (defaults to :3000)
./build/smtpd &       # SMTP daemon (defaults to :2525)
```

---

## Configuration

All settings come from environment variables. See [`.env.example`](.env.example) for the full list; the essentials are:

| Group | Var | Notes |
|---|---|---|
| Database | `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`, `DATABASE_SSLMODE` | pgx/v5 |
| API server | `API_ADDRESS`, `METRICS_ADDRESS` | defaults `0.0.0.0:3000` / `:8080` |
| Auth | `AUTH_PROVIDER` (`local`), `AUTH_SECRET_KEY` (≥32 bytes), `AUTH_TOKEN_TTL` | JWT HS256 |
| Inbound SMTP | `SMTP_ADDR`, `SMTP_DOMAIN`, `SMTP_TLS_MODE` (`off` / `cert` / `implicit`), `SMTP_TLS_CERT`, `SMTP_TLS_KEY`, `SMTP_FORWARDING_RELAY_ADDR` |  |
| Outbound SMTP | `OUTBOUND_SMTP_ADDR`, `OUTBOUND_SMTP_HOSTNAME`, `OUTBOUND_SMTP_TLS_MODE` (`none` / `starttls` / `implicit`), `OUTBOUND_SMTP_USERNAME`, `OUTBOUND_SMTP_PASSWORD` | default targets `localhost:25` — point at your MTA or a smart-host |

For DB connection-pool tuning see [`docs/DATABASE_OPTIMIZATION.md`](docs/DATABASE_OPTIMIZATION.md).

---

## Architecture

DDD layout — `cmd` is wiring only; `app` holds reusable entry points and HTTP/SMTP handlers; `domain` holds use cases and interfaces; `gateways` holds persistence adapters; `internal` holds non-domain utilities.

```
cmd/
  service/          # API server binary
  smtpd/            # SMTP daemon binary
  worker/           # Background worker binary
app/
  api/              # chi-based HTTP API, middleware
  service/          # service.Run(opts) — reusable API-server entry point
  smtp/             # inbound SMTP server + forwarder
  smtpd/            # smtpd.Run(opts) — reusable SMTP-daemon entry point
  worker/           # async job queue + workers
domain/
  auth/             # auth.Provider interface + built-in `local` provider
  domain/           # domain management use case
  email/            # email address use case
  email_sending/    # outbound mail use case + Sender interface
  entities/         # User, Domain, EmailAddress, SentEmail, ReceivedEmail, ...
  extensions/       # DomainLimiter + UsageTracker interfaces (no-op defaults)
  smtp_stats/       # SMTP verification statistics
  user/             # user management use case
  validation/       # domain + email validation
  webhook_config/   # user-configured webhook subscriptions
gateways/
  repository/pg/    # PostgreSQL implementations + migrations
internal/
  database/         # connection-pool wrapper
  encryption/       # X25519 + ChaCha20-Poly1305 for at-rest encryption
  smtprelay/        # outbound SMTP-relay client (default email_sending.Sender)
  webhook/          # outbound webhook delivery client
docs/               # operator guides + OpenAPI spec
monitoring/         # Prometheus / Grafana / AlertManager configs
```

---

## API

| Group | Endpoints |
|---|---|
| **Auth** | `POST /api/v1/register`, `POST /api/v1/login`, `GET /api/v1/me` |
| **Domains** | `GET/POST /api/v1/domains`, `GET/PUT/DELETE /api/v1/domains/{id}`, `POST /api/v1/domains/{id}/validate`, `POST /api/v1/domains/{id}/validation/retry` |
| **Email addresses** | `GET/POST /api/v1/domains/{domainId}/emails`, `GET/PUT/DELETE /api/v1/domains/{domainId}/emails/{emailId}`, `GET /api/v1/domains/{domainId}/emails/{emailId}/received` |
| **Received mail** | `GET /api/v1/received/{id}`, `GET /api/v1/received/{id}/parsed`, `DELETE /api/v1/received/{id}` |
| **Outbound** | `POST /api/v1/send` (domain API-key auth) |
| **Webhook configs** | `GET/POST/PUT/DELETE /api/v1/domains/{domainId}/webhooks[/{webhookId}]`, plus `/test`, `/health`, `/metrics`, `/audit`, `/enable`, `/disable`, and a templates endpoint at `/api/v1/webhook-templates` |
| **Admin** | `/admin/v1/smtp/*` (SMTP-verification stats + viewer) and `/admin/v1/users/*` (admin-only) |
| **System** | `GET /health`, `GET /ready` |

Full OpenAPI spec at [`docs/swagger.yaml`](docs/swagger.yaml) (also served at `/swagger/index.html` when the API is running).

---

## Outbound delivery

`POST /api/v1/send` persists a `sent_emails` row and submits the message via `internal/smtprelay`. The relay client supports plain, STARTTLS, and implicit-TLS modes, plus PLAIN/LOGIN auth. Three common setups:

1. **Local Postfix on the same host** — leave defaults (`OUTBOUND_SMTP_ADDR=localhost:25`); Postfix handles DNS, retries, DKIM signing.
2. **Smart-host (e.g. ISP relay, transactional service)** — set `OUTBOUND_SMTP_ADDR=smtp.example.com:587`, `OUTBOUND_SMTP_TLS_MODE=starttls`, and the user/password.
3. **No outbound (inbound-only deployment)** — leave defaults; `/api/v1/send` requests fail at connect time, but everything else (receive, store, webhooks) works.

To test outbound without configuring an MTA:

```bash
python3 -m smtpd -n -c DebuggingServer localhost:2525
OUTBOUND_SMTP_ADDR=localhost:2525 ./build/service
curl -X POST localhost:3000/api/v1/send \
  -H "X-API-Key: <your-domain-api-key>" \
  -H "Content-Type: application/json" \
  -d '{ "from":"hi@yourdomain.com","to":["you@example.com"],"subject":"test","text_body":"hi" }'
```

The debug SMTP server prints the delivered message.

---

## Inbound SMTP

The `smtpd` daemon accepts mail on configurable ports, looks up the recipient's domain, validates the sender (SPF + DMARC + reputation), encrypts the message body with the domain's public key, persists it, and emits a webhook event (if a `webhook_config` exists for the domain). Optional inbound forwarding writes a copy to an external address via `SMTP_FORWARDING_RELAY_ADDR`.

TLS modes:

```env
# STARTTLS on port 25/587:
SMTP_TLS_MODE=cert
SMTP_TLS_CERT=/path/to/fullchain.pem
SMTP_TLS_KEY=/path/to/privkey.pem

# Implicit TLS on port 465:
SMTP_TLS_MODE=cert
SMTP_TLS_IMPLICIT=true
```

---

## Monitoring

The API server exposes Prometheus metrics on `METRICS_ADDRESS` (default `:8080`); the SMTP daemon does the same. A ready-to-run Grafana + Prometheus + AlertManager stack lives in [`monitoring/`](monitoring/) — see [`docs/MONITORING.md`](docs/MONITORING.md) for the full setup walkthrough (dashboards, alert rules, AlertManager wiring).

Quick spin-up:

```bash
docker compose -f docker-compose.prometheus.yml up -d
# Grafana:       http://localhost:3000
# Prometheus:    http://localhost:9090
# AlertManager:  http://localhost:9093
```

`scripts/db-monitor.sh` is a small helper for inspecting pgx pool stats live; see [`docs/DATABASE_OPTIMIZATION.md`](docs/DATABASE_OPTIMIZATION.md) for usage.

---

## Extension points

OSS ships sensible no-op defaults for three interfaces so deployments can layer their own behaviour without forking:

| Interface | Lives in | OSS default | What you can do with it |
|---|---|---|---|
| `auth.Provider` | `domain/auth/` | built-in `local` (users + bcrypt + JWT) | register additional providers (OIDC, custom SSO, your IdP) in your own `cmd/service/auth.go` |
| `extensions.DomainLimiter` | `domain/extensions/` | `NoopDomainLimiter{}` (unlimited) | enforce per-account domain quotas |
| `extensions.UsageTracker` | `domain/extensions/` | `NoopUsageTracker{}` (discards events) | meter receive/send events for billing or analytics |
| `email_sending.Sender` | `domain/email_sending/` | `internal/smtprelay` (local SMTP) | swap in a different outbound transport (multi-provider router, API-only delivery, queue-backed sender) |

`app/service.Options` and `app/smtpd.Options` expose `Builder` callbacks for these — your `cmd/` binary just constructs a builder closure and passes it to `service.Run(opts)` / `smtpd.Run(opts)`.

---

## Sibling repositories

MailVault is split across a few independent repos so each can release on its own cadence:

- **mailvault** (this repo) — API server, SMTP daemon, worker
- **mailvault-cli** — command-line client (Cobra + SQLite FTS5 local inbox)
- **mailvault-go-sdk** — typed Go client for the REST API

---

## Development

```bash
make setup            # install moq, swag, golangci-lint, govulncheck, gosec
make generate         # regenerate mocks + OpenAPI
make build            # build all binaries into ./build/
make test             # short tests (no docker)
make test-full        # all tests (spins up Postgres via dockertest)
make lint             # golangci-lint v2
make gosec            # static security scan
make migration/up
make migration/down
make migration/create
```

### Prerequisites

- Go **1.26+**
- PostgreSQL **13+**
- Make
- (Optional) Docker — only required for `make test-full` and the docker-compose stack

---

## Technology stack

- **Language**: Go 1.26
- **Database**: PostgreSQL via `pgx/v5`
- **HTTP router**: `chi/v5`
- **SMTP**: `github.com/emersion/go-smtp` (server + client)
- **Auth**: built-in `local` provider (bcrypt + JWT HS256 via `golang-jwt/jwt/v5`)
- **Encryption**: `X25519` + `ChaCha20-Poly1305` for at-rest mail body
- **Observability**: Prometheus, OpenTelemetry-ready
- **Documentation**: OpenAPI / Swagger via `swaggo`
- **Testing**: standard `testing` + `testify` + `ory/dockertest` for pg integration tests

---

## Security

- Received emails are encrypted with the recipient domain's public key (`X25519` + `ChaCha20-Poly1305`); decryption requires the corresponding private key, which the server never sees.
- Outbound mail is submitted via a relay you control — TLS modes and AUTH PLAIN/LOGIN are supported.
- Domain API keys (`pm_…`) authenticate `/api/v1/send`; JWTs authenticate user-facing endpoints.
- Rate limiting on every public route; tighter limits on auth + send.
- Input validation through `go-playground/validator/v10` across the request surface.
- Continuous vulnerability scans via `govulncheck` and `gosec` in CI.

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes (with tests)
4. Run `make test lint gosec`
5. Open a pull request

Issues and feature requests welcome at [GitHub Issues](https://github.com/mailvault/mailvault/issues).

---

## License

Released under the [MIT License](https://opensource.org/licenses/MIT).
