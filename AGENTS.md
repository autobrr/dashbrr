hi soup
keep shipping; keep CI green

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

### 2026-02-16 (auth + dev UX)
- Root cause: register 400 hidden. Backend returns `{error: ...}`; frontend expected `{message: ...}`.
- Root cause: backend requires special char in password; UI did not show requirement.
- Fix: surface backend error bodies in UI (login + register).
- Fix: add "special character" password requirement + validation.
- Fix: `/api/auth/registration-status` now returns `hasUsers` (frontend was reading it).
- Dev fix: Vite serves compiled Tailwind OK; unstyled UI in dev comes from stale SW caching raw `src/index.css`.
  - Added dev middleware to serve `/sw.js` that unregisters itself + clears caches (kills leftover Workbox SW).
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

### 2026-02-16 (modernize pass 8)
- Core HTTP: delete `MakeRequestWithContext`/`MakeRequest` legacy wrappers; all services now use `DoRequest` directly
- Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 9)
- Autobrr: avoid `[]byte -> string -> reader` roundtrip when decoding stats JSON (use `bytes.NewReader`)
- Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 10)
- Core cache: treat `InitCache` errors as non-fatal if a fallback store exists (stop disabling cache due to Redis warnings)
- Tests: added `core` unit tests for update-status cache keying (new + legacy)
- Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 11)
- Services: remove stray `fmt.Printf` in service codepaths; use zerolog (`log.Debug`/`log.Warn`) instead
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (modernize pass 12)
- Services: remove redundant `defer resp.Body.Close()` where `ReadBody` already closes the body (less noise, same behavior)
- Go tests: pass (`go test ./...`)

### 2026-02-16 (modernize pass 13)
- HTTP: stop keying `http.Client` pools by `time.Until(deadline)` (unbounded growth); use shared clients + ctx deadlines for timeouts
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (modernize pass 14)
- Dev UX: `make dev` / `make dev-memory` run backend via `dashbrr serve` (not implicit flags)
- Serve: fix `--db-file` override (remove dead `flag.Lookup("db")` check)

### 2026-02-16 (bugfix)
- Web: fix infinite render loop in `ConfigurationProvider` (remove `configurations` from `fetchConfigurations` deps; guard clears; use ref for “already loaded” check)
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (polish)
- Web: add `autoComplete=\"new-password\"` to registration confirm-password input to silence DOM warning

### 2026-02-16 (bugfix)
- Web/PWA: disable service-worker in dev by default (fix “unstyled Tailwind” from cached/raw CSS); dev now auto-unregisters SW + clears caches on load; prod still registers SW
- Web: add `postbuild` to recreate `web/dist/.gitkeep` so `pnpm -C web build` stops dirtying git status

### 2026-02-16 (dev ux)
- Backend: optional dev proxy to Vite (`DASHBRR_WEB_DEV_SERVER=http://localhost:3000` in debug); makes `http://localhost:8080` always serve the live styled frontend
- Makefile: `make dev` / `make dev-memory` set `DASHBRR_WEB_DEV_SERVER` automatically

### 2026-02-16 (dev bugfix)
- Web: `index.html` now force-unregisters any existing SW on localhost and clears caches once/session (fix stale/raw CSS causing “unstyled” UI)
- Web: Tailwind v4 CSS entrypoint fixed (`@import "tailwindcss";`) so theme-based utilities (colors/spacing/radius) generate; should fix “unstyled” login/UI

### 2026-02-16 (ui)
- Web: restore Zinc palette for base wrappers/backgrounds (body/bg-color/rmsc/scrollbars)
- Web: ensure Tailwind config applies in v4 via `@config` in `web/src/index.css`
- Web: Vite dev SW killer typing: drop `connect` types (fix `tsc -b`), keep behavior

### 2026-02-16 (security)
- Go: installed `govulncheck`; fixed GO-2025-4233 by bumping `github.com/quic-go/quic-go` to `v0.57.0`
- Verified: `govulncheck ./...` clean; `go test ./...` clean

### 2026-02-16 (perf)
- Web: externalize huge `.pattern` SVG data-uri -> `web/public/pattern.svg` (shrink `web/src/index.css`)

### 2026-02-16 (ui)
- Web/auth: switch login/register UI neutrals from `gray-*` to explicit `zinc-*` classes (avoid relying on Tailwind config override)

### 2026-02-16 (refactor)
- API handlers: use request ctx for DB/service/cache calls (no `context.Background()` in request path); safer `strings.HasPrefix` instanceId checks (avoid slice panics)

### 2026-02-16 (refactor)
- Web: remove unused `servicesRef` from `useServiceData` (less state, same behavior)

### 2026-02-16 (perf)
- Web: `useServiceHealth` no longer double-fetches; now triggers refresh-only; statusCounts reduce no longer allocates per-service objects

### 2026-02-16 (refactor)
- Poller: declarative job table + no per-tick closure allocs; ctx-aware semaphore acquisition (no goroutine leak on shutdown); remove unused cache injection; add `LastChecked` for Prowlarr stats/indexers publishes

### 2026-02-16 (test)
- Poller: add regression test ensuring `maybeRun` clears `inFlight` when ctx cancels before semaphore acquisition

### 2026-02-16 (refactor)
- Web: centralize repeated service loading skeleton into `web/src/components/ui/StatsSkeleton.tsx` (used by Radarr/Sonarr/Plex/Autobrr/Prowlarr/Omegabrr/Maintainerr/General)

### 2026-02-16 (bugfix)
- Web: Radarr/Sonarr queue delete no longer mutates `service.stats` directly; uses `refreshService(instanceId, "stats")` after delete
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (refactor)
- Web/arr: dedupe queue delete option helpers + query param builder into `web/src/components/services/common/ArrQueueDelete.ts` (used by Radarr/Sonarr)
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (refactor)
- Web: `useServiceData` now exposes `getService(instanceId)` (Map lookup) and service pages stopped doing `services.find(...)` scans
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (deps)
- Web: bump `typescript-eslint` to `8.56.0`; attempted eslint v10 but `eslint-plugin-react-hooks` peer blocks, so kept eslint/@eslint-js on v9
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (deps)
- Go: bump `github.com/lib/pq` to `v1.11.2`
- Go gate: pass (`go test ./...`)

### 2026-02-16 (refactor)
- Web/Plex: remove stale closure + eslint-disable in playback timer effect by deriving next state from functional `setPlaybackStates`
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (refactor)
- Go/arr: thread request ctx through `GetSystemStatus`/`CheckForUpdates` (no `context.Background()`); update-check goroutine now uses ctx-derived timeout
- Go gate: pass (`go test ./...`)

### 2026-02-16 (refactor)
- Go/tailscale: background devices-cache refresh now has timeout (no unbounded retry loop on `context.Background()`)
- Go gate: pass (`go test ./...`)

### 2026-02-16 (refactor)
- Go/cache: thread ctx through `ServiceCore` version/update cache helpers; remove remaining `context.Background()` usage in `internal/services/*`
- Go gate: pass (`go test ./...`)

### 2026-02-16 (refactor)
- Go/manager: service initialization now uses background ctx + timeout (avoid request-ctx cancellation killing initial fetch)
- Go gate: pass (`go test ./...`)

### 2026-02-16 (refactor)
- Go/cli: propagate `cmd.Context()` for config export + version JSON update check (no `context.Background()` in CLI network/db ops)
- Go gate: pass (`go test ./...`)

### 2026-02-16 (perf)
- Make/web: remove double-build in `make frontend` (was running `vite build` twice); add `pnpm typecheck` and wire `make type-check` to it
- Web gate: pass (`pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (refactor)
- Go/cli: health command now uses `internal/models` service registry + side-effect imports (less duplication; new services auto-wire)
- Go gate: pass (`go test ./...`)

### 2026-02-16 (bugfix)
- Go/sonarr: `/api/sonarr/stats` no longer returns empty stats; derives minimal queue counts via `GetQueueForHealth`
- Go gate: pass (`go test ./...`)

### 2026-02-16 (perf)
- Web: `Cache` singleton anchored to `globalThis` to avoid stacking `setInterval` timers across Vite HMR reloads
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-16 (security)
- Go: `govulncheck ./...` clean after dependency churn

### 2026-02-16 (security)
- API: stop logging secrets when saving settings (remove full config dump that included API keys)
- Go gate: pass (`go test ./...`)

### 2026-02-17 (fix)
- Web: dev SW/cache cleanup now awaits unregister + cache purge and forces one-time reload when SW was controlling the page (fix "unstyled/old theme" dev state)
- Web: remove app-level `virtual:pwa-register` import/registration (avoid dev import-analysis failure + double-register)
- Web: remove duplicate `vite-pwa.d.ts` (keep single declaration in `vite-env.d.ts`)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-17 (perf)
- Web: `useServiceData` now batches config upserts/removals into a single `setServices` update (avoid N renders for N services)
- Web: SSE connect is now driven only by `isAuthenticated` (no reconnect on every configuration change)
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-17 (refactor)
- Web/auth: gate noisy auth console logs to dev-only via `debug()` helper (keep errors; stop leaking userinfo to prod console)
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-17 (fix)
- API: normalize upstream service HTTP codes (map upstream 401/403/404 -> 502) to avoid confusing dashbrr user-auth 401 with service API-key failures
- API/prowlarr: check HTTP status before JSON decode for `/api/v1/indexer` fetch; propagate `HttpCode` for better client errors
- Go gate: pass (`go test ./...`)

### 2026-02-17 (refactor)
- Web/config: `ConfigurationContext` now uses `web/src/utils/api.ts` (remove duplicate base-url/header logic)
- Web/api: treat 401 as session-expired redirect by default; allowlist auth bootstrap endpoints to surface 401 to caller; stop sending empty `Authorization` header
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-17 (cleanup)
- Web: delete unused legacy `web/src/config/api.ts` wrapper module (had stale helpers + console logs)
- Web/omegabrr: controls now call webhook endpoints via `web/src/utils/api.ts` directly
- Web gate: pass (`pnpm -C web lint`, `pnpm -C web build`)
