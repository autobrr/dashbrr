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
- Keep PR `#82` green; monitor/retry CI until latest run green.
- Continue small, low-risk refactors on large web files (`ServiceCard`, auth/form flows, status/render helpers).
- Triage default-branch Dependabot/security backlog (separate from PR code health work).

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
  - Affects: autobrr/sonarr/maintainerr/plex/prowlarr/overseerr/radarr handlers.

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

### 2026-02-17
- Removed deprecated omegabrr integration (backend routes/handlers/services/commands; frontend templates/types/UI; docs/env vars)
- Gates: `go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`
- Web cleanup: removed unused `ConfigurationForm` prop (`serviceName`)
- Backend cleanup: init fetch goroutines use `context.WithoutCancel(ctx)` (no request-cancel bleed)
- Auth: builtin auth handlers now use `c.Request.Context()` for cache ops (avoid passing `*gin.Context`); logout uses `getSessionToken`
- Web/auth: drop token-in-URL/localStorage flow; cookie-only bootstrap; removed unused SPA `/auth/callback` route
- Web: removed unused local cache utility (`web/src/utils/cache.ts`)
- Web: removed unused `validateServiceConfig` legacy helper from `web/src/contexts/useConfiguration.ts`
- Web/auth: `frontendUrl` for OIDC endpoints now uses `window.location.origin` (supports backend proxy dev mode)
- Web: removed unused `web/src/components/auth/CallbackPage.tsx` + route
- OIDC: added `/api/auth/oidc/callback` alias; updated default/example `OIDC_REDIRECT_URL` (legacy `/api/auth/callback` kept)
- Web: only unregister service-worker on 401 in dev (prod keeps PWA registered)
- Web/tailscale: remove axios-style error parsing; align with fetch-based api client errors + simplify UI states
- Web/tailscale: render "Add Tailscale" affordance when not configured; hook up `onConfigOpen` from `AppContent`
- Auth/OIDC: add GET `/api/auth/oidc/logout` and switch frontend to navigation-based logout (fetch cannot follow provider redirects)
- Web/deps: removed unused `axios` + `lodash` (+ types). `pnpm audit --prod` clean; `pnpm audit` still flags dev-only `ajv@6` via eslint toolchain.
- Go/cache: added shared typed SWR helper `FetchWithSWRCache` + tests; migrated Sonarr handler off `interface{}`+convert to typed cache ops

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

### 2026-02-18
- API cache: SWR helper supports optional `singleflight` stampede protection (cache-miss only) + unit test
- Handlers: removed redundant `.sf.Do` wrappers around SWR cache fetches (Autobrr/Plex/Overseerr/Maintainerr)
- Handlers: replaced brittle `"service not configured"` string matching with sentinel `ErrServiceNotConfigured` + `errors.Is`
- Handlers: add `ServiceNotConfiguredError` wrapper (keeps old messages) + migrate Prowlarr off `err.Error()==...`
- Handlers: migrate Sonarr/Radarr "not configured" errors to `NewServiceNotConfigured(...)`
- Tailscale handler: switch devices endpoint to shared `FetchWithSWRCache` (drop manual stale/cache refresh goroutines)
- Handlers: add `DeleteSWRCacheKeys` helper; remove repeated `Delete(key)` + `Delete(key+":stale")` blocks
- Sonarr/Radarr handlers: return 404 for `ErrServiceNotConfigured` (instead of generic 500)
- Tailscale handler: return 404 for `ErrServiceNotConfigured`
- Web/auth: remove dead access/id token localStorage + unused `AuthResponse` type (cookie-only)
- Web/*arr: de-dupe Sonarr/Radarr queue stats UI into shared `ArrQueueStatsBase`; remove unused message re-export files
- Web/messages: delete duplicated `{Autobrr,General,Overseerr,Plex}Message` components; use shared `ArrMessage` + new `combineServiceMessage` helper; remove dead Overseerr localStorage write
- Web/http: centralize error-body parsing into `web/src/utils/http.ts` and reuse in `AuthContext` + api client
- Web/overseerr: optimistic request status updates now use status overrides map (avoid stale localRequests array)
- Web/plex: stable playback key for timers + React keys (avoid collisions; less rerender churn)
- Web/api client: removed request queue; simpler fetch wrapper; 401 redirect guard no longer resets (prevents cascades)
- Omegabrr: confirmed fully removed (no code references remain; only this doc notes history)
- CI: docker metadata action now passed explicit `github-token` (fix intermittent "Bad credentials" on PR docker jobs)
- Web/login: removed effect-driven password validation state; now derived with `useMemo` + requirement map render loop (smaller, no derived-state effect)
- Web/add-services: fixed odd import path for modal; replaced large service switch logic with typed config maps; grouped+filtered categories via memo
- Web/status-indicator: moved static parsing constants out of render; unified status display map; switched gray fallbacks to zinc for consistency
- API/sonarr: removed dead `singleflight` field/import left after SWR migration
- Web/login: registration-status check now uses shared `api` client (consistent timeout/error handling)
- Web/add-services: swapped remaining gray utility classes to zinc equivalents (theme consistency with login/dashboard)
- Web/login: guarded async registration-status effect with cancel flag to prevent setState after unmount
- CI: added `pull-requests: read` workflow permission after docker metadata step still reported `Bad credentials` on PR runs
- Web/service-card: replaced service-type switch with renderer map + extracted last-checked formatter + zinc class consistency pass
- API/prowlarr: removed dead `singleflight` field/import; consolidated repeated error->status mapping into `statusFromProwlarrError`
- API/auth: removed dead OIDC discovery struct; parse discovery JSON via decoder stream (drop read-all alloc)
- Web/service-grid: replaced prev-ref diffing with deterministic merge (preserve dragged order, append new services by saved order); extracted localStorage order helpers
- Web/auth-context: deduped 429 retry loops into shared `fetchWith429Retry` helper (verify + userinfo paths)
- Web/auth-context: deduped builtin login/register POST boilerplate with `submitAuthForm` helper
- Web/arr-queue: extracted reusable queue-option listbox UI in `ArrQueueStatsBase`; fixed blocklist option value mismatch (`block`/`blacklist` -> `blocklist`/`blocklistAndSearch`)
- Web/auth-context: hardened retry-after parsing (`Retry-After` NaN/negative fallback) + removed unreachable extra fetch in retry helper
- Web/arr-queue: zinc palette alignment pass in `ArrQueueStatsBase` (removed remaining gray-* classes)
- Prowlarr backend: added `ProwlarrService.GetIndexers` and switched handler + poller to shared implementation (removed duplicated HTTP decode logic)
- Prowlarr backend: replaced remaining TODO in indexer stats window with explicit constants + clarified default (last 30 days)
- Poller: extracted pure aggregation helpers from run paths (`countTranscodingSessions`, `summarizeRadarrQueue`, `summarizeSonarrQueue`, `countOnlineTailscaleDevices`) to reduce inline loop noise
- Poller tests: added `internal/api/handlers/poller_stats_test.go` coverage for queue/device/transcode aggregations
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)
- Auth/oidc: hardened discovery fetch to fail fast on non-200 status (even with JSON body) and on missing required endpoints
- Auth tests: expanded `TestGetProviderEndpoints` for non-200 + malformed discovery payload cases
- Auth/oidc: logout redirect URL now built via `net/url` query encoding (`buildLogoutURL`) to avoid malformed `returnTo` when frontend URL has query params/spaces
- Auth tests: added `TestBuildLogoutURL` regression coverage for encoded logout redirect query
- Handlers/dedupe: unified online-device counters across poller + tailscale handler (`countOnlineDevices`) and removed duplicate local helper
- Plex handler: removed transcode session slice allocation in broadcast path; now uses shared `countTranscodingSessions` counter helper
- Maintainerr: introduced sentinel errors (`ErrURLRequired`, `ErrAPIKeyRequired`) for `get_collections` validation
- Maintainerr handler: replaced brittle string equality with `errors.Is(...)` for sentinel validation errors
- Tests: added `internal/api/handlers/maintainerr_error_test.go` covering status/message mapping for validation, upstream auth, timeout paths
- Arr handlers: extracted shared queue-delete query parsing + error response helper into `internal/api/handlers/queue_delete.go` (used by Sonarr + Radarr)
- Arr handlers: Sonarr/Radarr `DeleteQueueItem` now call shared `handleQueueDeleteError` (dedupe retry failure handling path)
- Tests: added `internal/api/handlers/queue_delete_test.go` for query-flag parsing + error mapping statuses (404/502/500)
- Sonarr service: `DeleteQueueItem` now returns `*arr.ErrArr` (not `ErrSonarr`) so handler upstream-status normalization path actually triggers
- Sonarr tests: added `internal/services/sonarr/sonarr_test.go` regression coverage for delete validation errors returning `arr.ErrArr`
- Sonarr service: removed local `makeRequest` passthrough; calls `arr.MakeArrRequest` directly
- Sonarr service: `GetSystemStatus` now delegates to shared `arr.GetArrSystemStatus` (drops duplicated status/version parse/cache logic)
- Arr service core: added shared `arr.DeleteQueueItem(...)` helper for queue-delete request/validation/log/error behavior
- Radarr/Sonarr services: `DeleteQueueItem` now delegate to shared `arr.DeleteQueueItem` (removed duplicated HTTP/delete/error codepaths)
- Arr tests: expanded `internal/services/arr/queue_test.go` with delete validation + upstream message/status mapping coverage
- Poller scheduling: non-blocking semaphore acquisition in `maybeRun` (no queued waiters), `lastRun` now stamped at actual job start, and upstream concurrency raised `4 -> 8` to reduce startup starvation from slow services.
- Autobrr releases: bounded fetch now uses `/api/release?limit=5&offset=0` with dedicated 8s timeout to avoid long-running release pulls delaying other updates.
- Tests: added `internal/services/autobrr/autobrr_test.go` (release query params/header regression) and `TestPollerMaybeRun_SemaphoreFullSkipsWithoutMarkingLastRun` in `internal/api/handlers/poller_test.go`.
- Qui card: replaced DHT/cross-seed display with practical transfer metrics (combined speed + combined data, plus down/up breakdown) and kept per-instance speed rows for active qBittorrent instances.
- Poller: removed qui cross-seed polling job (`qui_cross_seed`) to cut noisy/unused upstream calls.
- Tests: added `TestNewPoller_QuiJobsAreOverviewOnly` to lock job list to `qui_overview`.
- Gates: pass (`go test ./internal/api/handlers`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).
- UI responsiveness: replaced masonry-style service layout with true CSS grid breakpoints in `ServiceGrid` and switched dnd-kit to `rectSortingStrategy` (grid-aware drag ordering).
- UI interaction: removed UA sniffing for drag sensors; now pointer+touch+keyboard sensors with activation constraints (less accidental drags on touch, better cross-device behavior).
- UI card polish: service actions visible on small screens (no hover dependency), motion-safe hover scaling, tighter small-screen spacing, and top-level app content width cap (`max-w-[1800px]`) with wrap-safe header controls.
- Card internals: autobrr + qui stats grids now collapse to 1 column on small screens (`grid-cols-1 sm:grid-cols-2`) to prevent cramped 2-col rendering.
- Web/messages: `ArrMessage` now renders message boxes only for actionable states (`warning|error|offline`), not healthy/online noise
- Web/messages: filtered machine event keys (`*_queue`, `plex_sessions`, etc.) from rendered message content to avoid useless green/yellow boxes
- Web/SSE: deduped reconnect scheduling in `useServiceData` (single pending reconnect timer + stale-connection guard) to avoid parallel EventSource reconnect storms
- Vite proxy: set `/api` `timeout` + `proxyTimeout` to `0` for dev/preview, preventing SSE stream timeouts through proxy
- Arr health: update checks now run only on cache miss; update-check errors now cache fallback status for 10m (prevents repeated slow/canceled `/api/v3/update` probes every health tick)
- Arr tests: added `internal/services/arr/health_test.go` coverage for cache-hit skip, async cache fill, and error fallback-cache behavior
- Core cache API: added `GetUpdateStatusFromCacheWithFound` to distinguish cache misses from cached `false`
- Arr queue plumbing: added shared `arr.BuildQueueURL` + `arr.FetchQueueBody` helper (URL/build/request/status/read validation)
- Radarr/Sonarr: `getQueueRecords` now delegate queue HTTP path to shared ARR helper (less duplicated API-v3 queue fetch logic)
- Arr queue tests: expanded `internal/services/arr/queue_test.go` with shared queue URL builder + queue fetch validation/status/success cases
- Arr queue plumbing (pass 2): added generic `arr.FetchQueueRecords[T]` helper (typed records decode + shared parse error mapping)
- Radarr/Sonarr: removed remaining per-service queue JSON decode blocks; both now use `FetchQueueRecords[T]`
- Arr queue tests: added typed decode coverage for `FetchQueueRecords` parse error + success cases
- Arr API versioning: added `GetArrSystemStatusWithVersion` + `CheckArrForUpdatesWithVersion` helpers (default wrappers still v3)
- Prowlarr service: switched system-status/update-check calls to API `v1` endpoints (fixes mismatch with shared v3 defaults)
- Arr common tests: added coverage for versioned endpoint pathing (`/api/v1/system/status`, `/api/v1/update`) and default-v3 fallback
- SSE/events: added hub `SubscriberCount()` and switched connect/disconnect logs to debug with `client_id` + subscriber count (reduces noisy INFO churn)
- SSE/events: stream now emits `retry: 5000` directive so browser reconnects back off to 5s on disconnects
- SSE tests: added `internal/sse/hub_test.go` for subscribe/unsubscribe lifecycle and close cleanup subscriber-count behavior
- API/arr handlers: added shared `handleArrFetchError(...)` for not-configured/upstream-status/internal error mapping
- Sonarr/Radarr handlers: queue/stats fetch endpoints now use shared ARR fetch-error responder (removed duplicated error branches)
- API tests: added `internal/api/handlers/arr_handler_test.go` coverage for 404 not-configured, upstream-status normalization, and 500 fallback
- Web/SSE service merge: `useServiceData` now tracks optional-field presence from SSE payloads and only overwrites `version|updateAvailable|responseTime` when keys are present
- Web/SSE hydration: added `latestPatchRef` replay map so config-hydration merge uses last precise patch (fixes version flicker/disappear between health vs stats events)
- Auth middleware: extracted shared auth internals (`bypassSessionData`, bearer-token parser, dual-key session loader) to reduce RequireAuth/OptionalAuth duplication

### 2026-02-18 (sse async hardening)
- SSE stability: disabled global HTTP server `WriteTimeout` for streaming responses (`internal/api/server.go`); avoids forced stream teardown every ~15s.
- SSE bootstrap: added broadcaster snapshot cache + replay on connect (`internal/api/handlers/broadcast.go`, `internal/api/handlers/events.go`) so new/reconnected clients get immediate latest service state instead of waiting next poller interval.
- ARR noise reduction: suppress benign canceled/deadline update-check logs in async update checker (`internal/services/arr/health.go`).
- Frontend no-polling pass: removed `TailscaleStatusBar` interval/API fetch loop; now consumes SSE-fed `useServiceData` only and triggers one-shot backend refresh (`web/src/components/services/TailscaleStatusBar.tsx`).
- Types: added typed tailscale device/details shapes to `ServiceStats/ServiceDetails` and shared modal typing (`web/src/types/service.ts`, `web/src/components/services/TailscaleDeviceModal.tsx`).
- Tests: added `internal/api/handlers/broadcast_test.go` to lock snapshot replay behavior (latest-per-service + deterministic ordering).
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`).
- Commits: `5747629`, `225063a`.

### 2026-02-18 (ui responsive pass 2)
- Service board layout: switched from fixed breakpoint columns to fluid CSS grid `auto-fit/minmax` in `ServiceGrid` (adapts by available width, avoids forced 2-col feel on mid widths).
- Drag layout: kept dnd-kit `rectSortingStrategy` (grid-aware ordering) with same sensor setup; no UA sniffing.
- Card responsiveness: `ServiceCard` spacing now keyed to container queries (`@container`, `@md:*`) so header/body padding adapts to card width.
- Header responsiveness: `ServiceHeader` title sizing/alignment now container-query aware; service action controls stay visible on small cards while still hover-revealing on larger cards.
- Top-row UX polish: `AddServicesMenu` wrapper now `w-full` on mobile and `auto` on larger screens; logout/status colors aligned to zinc palette.
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).

## Next
- Run full gate: `go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`.
- Live verify: `/api/events` stays connected (no periodic disconnect churn), service cards leave loading quickly after connect/reconnect.
- SSE root-cause fix: `useServiceData` moved behind singleton `ServiceDataProvider`; multiple component hook calls now share one data/SSE instance
- App wiring: `web/src/App.tsx` now wraps routes with `ServiceDataProvider` (inside auth/config providers) so only one `/api/events` connection is created app-wide
- SSE middleware fix: auth middleware no longer replaces downstream request context with a 5s timeout context; timeout now used only for cache/session lookup
- Middleware tests: added `internal/api/middleware/auth_test.go` to lock no-deadline propagation for `RequireAuth` and `OptionalAuth`
- Arr health async: update-check goroutine now uses detached context (`context.WithoutCancel`) + skips noisy canceled logs, avoiding immediate request-scoped cancellation

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

### 2026-02-18 (qui integration)
- Backend: added new `qui` service integration (`internal/services/qui`) with health checks (`/health` + `/api/instances` auth), instances list, transfer-info fetch, cross-seed automation status fetch.
- Backend poller: added `qui_overview` + `qui_cross_seed` jobs; publishes per-instance connectivity, aggregate transfer stats, and cross-seed scheduler/run status over SSE.
- Backend CLI: added `dashbrr service qui {add|remove|list}` command set; wired service registry + global health command/service registration imports.
- Types/tests: added `internal/types/qui.go`; added `internal/services/qui/qui_test.go`, poller status helper tests, and registry coverage for `qui` creator.
- Frontend: added `qui` service type/template/category/config-help + `QuiStats` card renderer in `ServiceCard`.
- Frontend card content: active/connected instance counts, aggregate up/down speeds, transfer totals, per-instance live speeds, cross-seed automation run metadata.
- Docs: updated `README.md` supported services, `docs/commands.md` service command list, and `docs/config_management.md` env var list.

## Next
- Live-verify against real `qui` instance:
  - check add/config flow (`/api/health/qui` validation with `X-API-Key`)
  - confirm `qui` card fields populate quickly via SSE (no long skeleton hangs)
- If needed: add follow-up job for per-instance `app-info` (qB version badges) once baseline stability is confirmed.

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

### 2026-02-17 (cleanup)
- Web/api: remove unused `getEventSourceUrl()` export from `web/src/utils/api.ts`
- Web gate: pass (`pnpm -C web lint`)

### 2026-02-17 (cleanup)
- Web: remove unused PWA/`virtual:pwa-register` type declarations from `web/src/vite-env.d.ts`
- Web gate: pass (`pnpm -C web typecheck`, `pnpm -C web lint`)

### 2026-02-17 (refactor)
- Web/api: parse successful responses via `content-type` (use `response.json()` when JSON; tolerate empty/204); avoid brittle `JSON.parse(text)`
- Web gate: pass (`pnpm -C web lint`)

### 2026-02-17 (chore)
- Remove deprecated Omegabrr end-to-end: API handlers/routes, service registry/CLI, UI templates/components/types, cache TTLs, docs/README
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-17 (fix)
- API: CORS now supports explicit origin allowlist + credentialed requests (cookies/SSE); new config/env knobs (`cors_origins`, `DASHBRR__CORS_ORIGINS`, etc)
- Web: SSE uses `EventSource(..., { withCredentials: true })` when supported

### 2026-02-17 (refactor)
- Auth/oidc: session cookie is now a server-generated stable session id (no longer provider access token); callback redirect no longer leaks tokens in URL
- Auth/web: cookie-first auth bootstrap (no longer requires `localStorage.access_token`); `ConfigurationContext` no longer gates on stored token
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`)

### 2026-02-17 (fix)
- Auth/oidc: preserve existing refresh token if provider omits it during refresh

### 2026-02-17 (cleanup)
- Web/auth: remove unused `AUTH_URLS.oidc.callback` (no matching backend route)

### 2026-02-17 (security)
- Auth/oidc: store nonce in state; validate `id_token` nonce claim on callback (adds unit test)
- Auth/oidc: typed state payload (avoid map/type-assert footguns)

### 2026-02-17 (refactor)
- Go/models: add `ServiceTypeFromInstanceID`; use it in poller + service manager + discovery display (less string-split duplication)

### 2026-02-17 (refactor)
- Web/api: stop sending `Authorization` from `localStorage` by default (cookie-first sessions, less XSS blast radius)

### 2026-02-17 (refactor)
- API/cache: add typed SWR cache helper + tests; migrate Sonarr + Radarr + Prowlarr + Plex + Maintainerr + Overseerr + Autobrr handlers to shared helper (drop local cache funcs + `SafeStructConvert`)

### 2026-02-17 (chore)
- Go deps: `go get -u ./...` + `go mod tidy`; `go test ./...` pass

### 2026-02-17 (chore)
- Web deps: `pnpm -C web up`; gate pass (`pnpm -C web typecheck`, `lint`, `build`)

### 2026-02-17 (refactor)
- CLI: dedupe `dashbrr service <type> {list,add,remove}` commands via shared CRUD helpers; move `getNextInstanceID` + URL validation into common utils

### 2026-02-17 (cleanup)
- Go: remove unused `internal/utils/type_conversion.go` (no remaining callers); `go test ./...` pass

### 2026-02-17 (refactor)
- Arr services: dedupe Sonarr/Radarr queue delete URL construction + error message parsing into `internal/services/arr`; add unit test

### 2026-02-17 (refactor)
- Tailscale handler: background refresh now uses `context.WithoutCancel(requestCtx)` (no `context.Background()`); keeps refresh independent but preserves request-scoped values

### 2026-02-17 (fix)
- Arr client: `MakeArrRequest` now consistently uses the timeout-wrapped context when building the request

### 2026-02-17 (refactor)
- Discovery: centralize label/env parsing for Docker/K8s/config-file imports; remove `strings.Title`; add unit tests

### 2026-02-18 (fix)
- SSE stream lifecycle hardening:
  - server `WriteTimeout` disabled for long-lived streams
  - SSE snapshot replay on connect (latest payload per service)
  - async ARR update-check cancellation/deadline noise suppressed
- Frontend Tailscale status switched to backend-driven SSE state (removed local polling loop)
- Added snapshot regression tests (`internal/api/handlers/broadcast_test.go`)
- Commits: `5747629`, `225063a`, `2792ca2`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (refactor)
- ARR service error-model consolidation:
  - Sonarr service migrated from local `ErrSonarr` to shared `arr.ErrArr`
  - Prowlarr service migrated from local `ErrProwlarr` to shared `arr.ErrArr`
  - Prowlarr `GetSystemStatus` now uses shared `arr.GetArrSystemStatus`
- Prowlarr handler status mapping simplified to shared `arr.ErrArr` path
- Added handler regression tests for prowlarr error->status mapping (`internal/api/handlers/prowlarr_error_test.go`)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (refactor)
- ARR queue fetch typing cleanup:
  - Sonarr/Radarr now use typed internal queue fetch helpers
  - `GetQueueForHealth` no longer round-trips through `interface{}` + type assertion
  - Removes panic-prone cast path and keeps compatibility on exported `GetQueue`
- Commit: `6772df6`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (refactor)
- Handler input validation dedupe:
  - added shared `requireInstanceID(...)` helper for `instanceId` query validation (`internal/api/handlers/instance_id.go`)
  - migrated Sonarr/Radarr/Prowlarr handlers to helper (consistent error/status/log behavior)
  - Radarr queue-delete now validates service prefix consistently via shared helper
- Added helper unit tests (`internal/api/handlers/instance_id_test.go`)
- Commit: `5de2fbc`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (refactor)
- ARR queue delete follow-up dedupe:
  - added shared `refreshQueueAfterDelete(...)` helper (`internal/api/handlers/queue_delete.go`)
  - Sonarr/Radarr delete handlers now share cache-clear + SWR refetch + SSE broadcast flow
- Added helper coverage in `internal/api/handlers/queue_delete_test.go`
- Commit: `bcc7091`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Diagnosed variable hard-refresh load times as frontend hydration race:
  - SSE snapshot events could arrive before `ConfigurationContext` populated service map
  - dropped early events left some services in `loading` until next poll interval (appears instant or "forever")
- Fix: `useServiceData` now stores latest SSE health per instance and reapplies it during config hydration (`latestHealthRef` + merge-on-config path).
- Auth bypass for troubleshooting:
  - new env flag `DASHBRR_AUTH_BYPASS=true`
  - middleware short-circuits auth checks and injects synthetic session context
  - `/api/auth/config` exposes `bypass` boolean
  - frontend `AuthProvider` auto-authenticates when bypass is enabled
- Added bypass env unit test (`internal/api/middleware/auth_bypass_test.go`)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Diagnosed Autobrr stats showing zeros as frontend SSE merge clobber:
  - backend emits separate `autobrr_stats` and `autobrr_releases` events under same `stats.autobrr` key
  - shallow merge caused last event to overwrite prior payload shape
- Fix in `web/src/hooks/useServiceData.ts`:
  - added typed nested merge helper for `stats`/`details` service payload maps
  - keep Autobrr releases in `service.releases`; ignore `stats` write on `autobrr_releases` patch
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (refactor)
- Poller: split Autobrr monolithic stats job into independent jobs:
  - `autobrr_stats`
  - `autobrr_irc_status`
  - `autobrr_releases`
- Removes intra-job head-of-line blocking (one slow Autobrr endpoint no longer stalls other Autobrr SSE payloads)
- Added regression coverage for job registration (`internal/api/handlers/poller_jobs_test.go`)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (perf)
- Overseerr request path latency cleanup (`internal/services/overseerr/overseerr.go`):
  - removed N+1 title lookup calls to Sonarr/Radarr for each request item
  - removed redundant marshal/unmarshal roundtrip for each result row
  - now uses Overseerr `/api/v1/request?take=10` payload directly
- Added regression test (`internal/services/overseerr/overseerr_test.go`) asserting single upstream call (no per-item fanout)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Web/Prowlarr: stop gating indexer rendering on both `indexers` and `stats` payloads in `ProwlarrStats`
- Initial UI now unblocks as soon as indexers payload lands (stats can arrive later without keeping skeleton state)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (refactor)
- Poller: split Prowlarr monolithic job into independent payload jobs:
  - `prowlarr_stats`
  - `prowlarr_indexers`
- Removes intra-job coupling so one Prowlarr endpoint no longer blocks the other SSE payload
- Added regression coverage in `internal/api/handlers/poller_jobs_test.go`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Version flicker on hard refresh: root cause in SSE snapshot replay (`Broadcaster`) storing only the last event per service
  - when last event was stats/details payload (no `version`), refresh bootstrap lost version until next health event
- Fix:
  - `internal/api/handlers/broadcast.go` now stores merged per-service health snapshot state (status/message/timestamps + deep-merged stats/details)
  - preserves known `version` across partial payload updates used for snapshot replay
- Added regression coverage:
  - `internal/api/handlers/broadcast_test.go` for version preservation across partial updates
  - `internal/api/handlers/broadcast_test.go` for nested stats merge in snapshot replay
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Poller scheduling now prioritizes health pass before stats pass in each tick (`internal/api/handlers/poller.go`)
  - health/version events get queued first for all services
  - avoids stats jobs starving version-bearing health updates on cold start
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (audit)
- Audited branch `refactor/services-health-checks` against `refactor/modernize`:
  - branch head is older (`204fa4e`, 2025-02-16) and behind modernize
  - architecture there (global client maps, legacy monitor loops, removed SSE handler path) is mostly superseded by current poller+hub design
- Idea retained/applied from audit direction:
  - keep version-first experience by prioritizing health before stats per tick (`fix(api): prioritize health checks ahead of stats jobs`)
- Decision:
  - no direct code cherry-pick from `refactor/services-health-checks`; continue incremental modernization on current branch

### 2026-02-18 (fix)
- Overseerr title regression fix after perf pass (`internal/services/overseerr/overseerr.go`):
  - source-of-truth checked against `~/github/oss/seerr` (request list payload can omit resolved titles; Seerr fetches media details separately)
  - added best-effort title enrichment for missing-title rows via Overseerr metadata endpoints:
    - movie: `/api/v1/movie/:tmdbId`
    - tv: `/api/v1/tv/:tmdbId`
  - enrichment is bounded/async:
    - worker limit (`4`) via `errgroup`
    - per-lookup timeout (`3s`)
    - dedupe by `baseURL+mediaType+tmdbId`
    - in-process TTL cache (`30m`) for repeated rows/refreshes
  - keeps prior perf gains: no Sonarr/Radarr N+1 fanout
- Added regression coverage (`internal/services/overseerr/overseerr_test.go`):
  - missing titles get enriched from Overseerr movie/tv endpoints
  - repeat `GetRequests` calls reuse title cache (no duplicate metadata lookup)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Overseerr request status labels in UI (`web/src/components/services/overseerr/OverseerrStats.tsx`):
  - aligned with Seerr enum in `~/github/oss/seerr/server/constants/media.ts`
  - added support for `FAILED` (`4`) and `COMPLETED` (`5`) statuses
  - remove misleading generic `Unknown` for valid statuses; unknown numeric values now render as `Unknown (<code>)`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)
## Rolling Plan
- CI/watch: PR `#82` (`refactor/modernize` -> `develop`)
- Backend/frontend: continue normalizing multi-payload service events so each UI field has one canonical SSE key/path
- Frontend: keep reducing effect-driven derived state; move to memo/render-time derivation where possible
- Frontend: continue zinc palette consistency pass in frequently used components
- Backend: remove leftover dead fields/imports after SWR/singleflight migration
- Backend: remove remaining `context.Background()` in request paths; keep ctx flow explicit
- Backend: continue poller decomposition (extract payload-build helpers, add unit tests before behavior changes)
- Overseerr: follow-up on status-label mapping (`Unknown`) when upstream payload uses newer states
- Housekeeping: checked for `ead` hooks; none found (only pnpm lock integrity strings)
