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

## Local development authentication

The web app authenticates through the API login flow (username/password + server
session cookie). `apps/web/.env.local` (gitignored) holds the API base URL:

```sh
cp apps/web/.env.example apps/web/.env.local
# set NEXT_PUBLIC_API_URL=http://localhost:8080
```

Then log in at http://localhost:3000/login with a tenant slug, email, and
password. Users are managed by an admin via the API (`POST /api/v1/users`).
A scripted production/demo seed (tenant + roles + data) is planned; until it
lands, reuse the existing local database or create the first admin directly in
PostgreSQL (bcrypt hash + `role = 'admin'`).

## Tests

Repository integration tests run against a local PostgreSQL test database
(`biomed_cmms_test`, created on demand from `TEST_DATABASE_URL`). They reuse the
docker-compose Postgres instance and skip automatically when it is unreachable:

```sh
make db-up    # ensure Postgres is running
make test
```

## Project status

**Sprint 8.1 — Deployment readiness.** Session/auth and admin user management
(login, logout, list/create/update users) are implemented with server-side
sessions. `GET /health` verifies database connectivity; every request is
logged (method, path, status, duration); `POST /api/v1/tenants` is admin-only;
the web production build requires `NEXT_PUBLIC_API_URL`. Deployment
(Docker/Caddy/CI) and the demo seed are the remaining sprints.
