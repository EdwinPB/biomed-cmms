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

## Demo seed

`apps/api/cmd/seed` inserts an idempotent demo dataset (1 tenant, 3 users,
10 equipment, service requests across all statuses, request events, and 2
RFPs). Run it against the production Compose stack with:

```sh
docker compose -f docker-compose.prod.yml --env-file .env --profile tools run --rm seed
```

Credentials (tenant slug `demo`, password `DemoPass!123`):

| email | role |
| --- | --- |
| admin@demo.test | admin |
| requester@demo.test | requester |
| biomedic@demo.test | biomedic |

Override the password with `SEED_DEMO_PASSWORD` in the environment. The seed is
repeatable (no duplicates) and atomic (single transaction). It is a DEMO seed
for local and demo environments only — do not apply it as-is to a real customer
database. It is never run automatically.

## Tests

Repository integration tests run against a local PostgreSQL test database
(`biomed_cmms_test`, created on demand from `TEST_DATABASE_URL`). They reuse the
docker-compose Postgres instance and skip automatically when it is unreachable:

```sh
make db-up    # ensure Postgres is running
make test
```

## Project status

**Sprint 8.4 — Deployment + demo seed.** Docker Compose production stack
(Postgres + API + web + Caddy), Dockerfiles for API/web, DB-aware `/health`,
and an idempotent demo seed (`cmd/seed`) runnable as a one-off Compose service
(`docker compose --profile tools run --rm seed`). See "Demo seed" above for
credentials. Deployment to a real server and CI remain.
