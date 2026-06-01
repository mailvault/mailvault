# Contributing to MailVault

Thanks for taking the time to contribute. MailVault is an open-source, self-hostable email backend; PRs that fix bugs, sharpen the SMTP/HTTP surface, or improve test coverage are all welcome.

## Quick start (development)

```bash
git clone https://github.com/mailvault/mailvault.git
cd mailvault
make setup            # installs migrate, moq, linters, test formatter, gosec, swag
make build            # builds service, smtpd, worker into ./build/
make test             # short tests with gotestfmt (skips dockertest integration suite)
```

For the SMTP daemon and the API service against a local Postgres, copy `.env.example` to `.env` and run `docker compose up -d`.

## Project layout

DDD / hexagonal:

```
cmd/<svc>/            entrypoint, config load, dependency wiring
domain/               entities + use cases (framework-agnostic)
app/                  HTTP handlers, SMTP server, workers
gateways/repository/  data access implementations (pg)
internal/             cross-cutting helpers
monitoring/           Prometheus + Grafana + AlertManager stack
```

Layering rule: `app` and `gateways` depend on `domain`; `domain` depends on nothing outward. Add ports as interfaces inside the consumer (usually a use case); implement them in `gateways`.

## Before opening a PR

```bash
make lint             # golangci-lint
make gosec            # security scan
make test-full        # full test suite with coverage
```

CI runs the same checks plus `govulncheck` and the dockertest pg-repository integration tests. Don't skip hooks (`--no-verify`, `--no-gpg-sign`); if a check fails, fix the underlying issue.

## Tests

- Table-driven subtests with `t.Run`.
- `github.com/stretchr/testify`: `require` for fatal preconditions, `assert` for checks.
- Mock interfaces with `github.com/matryer/moq` (`//go:generate moq ...`).
- Integration tests gated behind `-short` and run in a separate CI job.

## Commit + PR style

- Single-line commit subjects, conventional-commit prefixes (`feat:`, `fix:`, `chore:`, `docs:`, `test:`, `ci:`, `build:`, `refactor:`, `deps:`). Detail belongs in the PR body, not the commit.
- One logical change per PR. Describe the *why* and the user-visible effect.
- Reference issues with `Fixes #N` / `Closes #N` where applicable.

## Configuration

Configuration goes through `github.com/ardanlabs/conf/v3` — `app/service/config.go` and `app/smtpd/config.go` are the canonical examples. Don't introduce ad-hoc `os.Getenv` plumbing; extend the `Config` struct with `conf:"env:NAME,default:..."` tags.

## Extension interfaces

OSS exposes a small set of seams under `domain/extensions/` so deployments can layer quotas, metering, or custom transports without forking:

- `DomainLimiter` — gate domain creation
- `UsageTracker` — record domain / email / SMTP usage
- `Sender` — outbound mail transport (OSS default: `internal/smtprelay`)
- `auth.Provider` — auth provider plug-in (OSS default: `domain/auth/local`)

PRs that improve these interfaces are welcome; please open an issue first if you want to add a new seam.

## License

By contributing, you agree your contributions are licensed under the [MIT License](./LICENSE).
