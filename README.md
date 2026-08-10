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
make dev         # run API and web
```

## Project status

**Sprint 1 — repository foundation.** No business logic, auth, database models,
RFP, or API endpoints implemented yet.
