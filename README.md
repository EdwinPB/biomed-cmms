# Biomed CMMS

Multi-tenant SaaS platform for biomedical equipment maintenance requests. Lets
hospital staff submit and track biomedical equipment service requests.

## Repository layout

| Path       | Description                              |
| ---------- | ---------------------------------------- |
| `apps/api` | Go backend — modular monolith            |
| `apps/web` | Next.js + React + TypeScript frontend    |
| `docs/adr` | Architecture Decision Records            |

## Architecture

- **Backend**: Go modular monolith (`apps/api`). Domains live as internal
  packages behind clear boundaries; extracted to services only when scaling
  demands it.
- **Frontend**: Next.js + React + TypeScript (`apps/web`).
- **Database**: PostgreSQL 16 via Docker Compose.

## Requirements

- Go 1.26+
- Node.js 24 (see `.nvmrc`)
- Docker (for PostgreSQL)

## Getting started

```sh
cp .env.example .env
make install     # Go modules + npm deps
make db-up       # start PostgreSQL
make migrate-up  # apply database migrations
make dev         # run API and web
```

## Database migrations

Migrations live in `apps/api/internal/migrations` and are embedded into the Go
binary via `embed`. Apply them with the `cmd/migrate` tool:

```sh
make migrate-up       # apply all pending migrations
make migrate-down     # roll back one migration
make migrate-status   # show current version
```

## Tests

Repository integration tests run against a local PostgreSQL test database
(`biomed_cmms_test`, created on demand from `TEST_DATABASE_URL`). They reuse the
docker-compose Postgres instance and skip automatically when it is unreachable:

```sh
make db-up    # ensure Postgres is running
make test
```

## Project status

**Sprint 2.3 — application layer.** Tenant use-case service (`CreateTenant`)
in place above the repository boundary. No business logic, auth, RFP, or API
endpoints implemented yet.
