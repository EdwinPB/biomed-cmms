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

## Local development identity

The API authenticates requests via dev-only identity headers. The web app reads
them from `apps/web/.env.local` (gitignored), so copy `apps/web/.env.example`
there and fill in a real tenant/user. Tenant is created through the existing API
endpoint; there is no user endpoint yet, so create the user in SQL:

```sh
# API running:  curl -s -X POST localhost:8080/api/v1/tenants -H 'Content-Type: application/json' \
#               -d '{"slug":"local-dev","name":"Local Development"}'
# Then capture the returned tenant id and run:
docker exec -i biomed-cmms-postgres psql -U biomed -d biomed_cmms -c \
  "INSERT INTO users (tenant_id, email, password_hash, full_name)
   VALUES ('<tenant-id>', 'dev@local.test', 'dev-only', 'Dev User');"
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

**Sprint 6.3 — Equipment selection.** New tenant-scoped `GET /api/v1/equipment`
endpoint (repo `ListByTenant` already existed; added minimal `equipment/service`
pass-through + HTTP route). `/requests/new` now uses an equipment selector
rendering human-readable "Name — AssetTag" options and submits the real UUID.
No equipment CRUD, filtering, pagination, or auth yet.
