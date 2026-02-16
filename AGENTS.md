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
