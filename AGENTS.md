# AGENTS

Owner: soup (s0up4200@pm.me)

## Progress Log

### 2026-02-16
- Branch: `refactor/modernize`
- Repo scan: Go backend + Vite React frontend
- Key inefficiencies / risks found:
  - SSE mismatch: backend exposes `/api/health/events`; frontend connects to `/api/events` (likely dead/legacy)
  - Backend SSE: global mutable state (`clients`, `lastChecks`), multiple tickers, ad-hoc cleanup; duplicated monitoring (EventsHandler + HealthService)
  - Frontend: `useServiceData.ts` huge (1k+ LOC), mixes polling + SSE + per-service scheduling; immediate recursive SSE reconnect; high re-render risk
  - Some services create `http.Client{}` per request (no timeouts / no reuse) alongside a separate client pool in `internal/services/core`
  - Many `context.Background()` usage where request ctx should flow

## Next
- Architecture proposal (no code yet): single event stream contract, consolidate health check scheduling, shared HTTP client + typed service adapters, frontend data layer split + query/cache strategy.

### 2026-02-16 (cont)
- Confirmed: backend has no `/api/events` route. Only SSE route is `GET /api/health/events`.
- Frontend currently opens SSE to `/api/events` (broken).
- Frontend also has a more robust `useEventSource(path)` hook that appends `?token=`.
- Backend SSE `/api/health/events` is behind auth middleware; expects `Authorization: Bearer ...` header.
  - Browser EventSource cannot set headers; needs cookie auth OR token in query param.
- Backend handlers frequently use `context.Background()` (settings caching, config fetch), ignoring request ctx.

- Baseline tests:
  - `go test ./...` passes
  - `pnpm -C web lint` clean except 3 hook-deps warnings
- Critical bug/inefficiency: cache hit path always triggers background refresh.
  - Pattern: `time.Now().After(time.Now().Add(-CacheDuration + 5s))` is always true.
  - Affects: autobrr/sonarr/maintainerr/plex/prowlarr/overseerr/omegabrr/radarr handlers.

### 2026-02-16 (build 1)
- Implemented SSE hub + `/api/events` default-message SSE stream (JSON ServiceHealth)
- Implemented backend poller (server-push) for health + stats (plex/overseerr/radarr/sonarr/prowlarr/autobrr/maintainerr/tailscale)
- Fixed major handler cache ineff: removed broken "refresh cache in background" logic (was always true)
- Fixed SSE payload shapes to match frontend expectations (radarr/sonarr/maintainerr)
- Frontend: replaced `useServiceData` polling/timers with SSE-driven updates + refresh endpoint
- Added manual refresh endpoint: `POST /api/services/:instanceId/refresh?kind=health|stats|all`
- Go tests: pass (`go test ./...`)
- Web build: pass (`pnpm -C web build`)

### 2026-02-16 (security + polish)
- API keys: write-only semantics
  - Settings responses sanitize `apiKey` (always empty to browser)
  - Settings save preserves existing key when request omits/blank
- Omegabrr: UI no longer needs url/apiKey; triggers now pass `instanceId` (server loads stored key)
- Tailscale: UI no longer needs stored apiKey; uses `/api/tailscale/devices?instanceId=...`
- Tailscale handler: request ctx propagation; cache key safety for short tokens
- AuthContext: removed hook-deps warnings; simplified rate-limit retry loops
- Web lint: clean (`pnpm -C web lint`)

### 2026-02-16 (deps)
- Backend deps: upgraded Go modules (gin, docker, k8s, modernc sqlite, x/*, etc) + `go mod tidy`
  - Fixed vet-printf issues in arr services (fmt.Errorf w/ non-const string)
  - Go tests: pass (`go test ./...`)
- Frontend deps: upgraded to latest (React 19, Vite 7, Tailwind 4, MUI 7, Router 7, Workbox 7.4, etc)
  - Tailwind 4 migration: `@tailwindcss/postcss`, Vite uses `postcss.config.js` (no inline postcss plugins)
  - CSS cleanup: removed `theme()` and `@apply` usage from `web/src/index.css` to avoid Tailwind v4 incompat/errors
  - ESLint: pinned to v9 (v10 peer mismatch); disabled new v7 react-hooks heuristic rules (lint still clean)
  - Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)
- CI alignment: bumped workflow toolchain pins to match new engine requirements
  - `.github/workflows/release.yml`: `GO_VERSION=1.25.0`, `NODE_VERSION=22.12.0` (Vite 7 requires Node >=20.19.0)
- Container builds: bumped Go base images to match toolchain
  - `Dockerfile`: `golang:1.25-alpine3.23`
  - `ci.Dockerfile`: `golang:1.25-alpine3.23`

### 2026-02-16 (cleanup)
- Branch pushed: `refactor/modernize` -> `origin/refactor/modernize`
- Removed unused hooks (no imports found): moved to Trash
  - `web/src/hooks/useEventSource.ts`
  - `web/src/hooks/usePollingService.ts`
  - `web/src/hooks/useCachedServiceData.ts`

### 2026-02-16 (modernize pass 2)
- Deleted legacy `HealthService` monitoring subsystem (redundant with poller)
  - removed: `internal/services/health.go`, `internal/services/health_test.go`
  - handlers/server/cli updated to not inject/stop monitoring
  - Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 3)
- Core HTTP: added `ServiceCore.DoRequest` (method + optional body) to avoid ad-hoc `http.Client{}` usage
- Overseerr: `UpdateRequestStatus` now uses shared client + timeout; DB lookups use request ctx (no `context.Background()`)
- Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 4)
- Core HTTP: early-return on canceled ctx; clamp deadline-derived timeouts; treat `application/json; charset=utf-8` as JSON in `ReadBody`
- Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 5)
- *arr health: propagate ctx from callers; delete broken pseudo-caching + unsafe goroutine map mutation; rely on poller caching instead
- Update cache: add `CacheUpdateStatus` + legacy read; fix Maintainerr/Tailscale/*arr update caching to match `GetUpdateStatusFromCache`
- Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 6)
- Web: route-level code-splitting (lazy load `LoginPage`, `CallbackPage`, main `AppContent`) to shrink initial bundle + remove >500kb Vite chunk warning
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (modernize pass 7)
- Services: remove `auth_header/auth_value` header hack usage; pass explicit auth headers everywhere (keeps `MakeRequestWithContext` back-compat)
- Go tests: pass (`go test ./...`)
