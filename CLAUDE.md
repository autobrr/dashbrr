# dashbrr — Claude Code Guide

dashbrr is a service dashboard for monitoring self-hosted services (Sonarr, Radarr, Autobrr, Prowlarr, etc.). It is a fork of [autobrr/dashbrr](https://github.com/autobrr/dashbrr).

**Repository:** `github.com/autobrr/dashbrr`

## Tech Stack

| Layer | Technologies |
|-------|--------------|
| Backend | Go 1.26, Chi router, GORM, SQLite/PostgreSQL |
| Frontend | React 19, TypeScript, Vite, pnpm, TailwindCSS, TanStack Query |
| Build | Makefile, Docker multi-stage |

## Dev Commands

```bash
# Backend
make build          # Build binary
make run            # Run backend (requires frontend built first)
go test ./...       # Run tests

# Frontend (from web/)
pnpm install
pnpm dev            # Dev server
pnpm build          # Production build
pnpm lint           # ESLint
pnpm typecheck      # TypeScript check

# Full stack
make dev            # Build frontend then start backend
docker compose -f docker-compose/docker-compose.dev.yml up
```

## Project Structure

```
cmd/dashbrr/        # Entrypoint
internal/           # Backend packages (models, handlers, services)
pkg/                # Shared utilities
web/                # React frontend (src/, public/)
web/src/components/ # UI components
web/src/contexts/   # React contexts
web/src/utils/      # API client and helpers
```

## Conventions

- Backend: standard Go idioms; errors wrapped with `fmt.Errorf("…: %w", err)`
- Frontend: named component exports, TanStack Query for server state
- All re-thrown errors must chain the original via `{ cause: err }`
- tsconfig lib is ES2022 — `Error.cause` is available
- Never commit secrets; use environment variables for all config
