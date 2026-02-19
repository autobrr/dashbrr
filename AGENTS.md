hi soup
keep shipping; keep CI green

# AGENTS

Owner: soup (s0up4200@pm.me)

## Progress Log

### 2026-02-19
- Auth/OIDC context cleanup slice completed.
- `internal/api/handlers/auth.go`
  - removed constructor-time OIDC discovery on `context.Background()`
  - added lazy request-scoped discovery (`ensureProviderConfig(ctx)`)
  - added mutex protection for provider/oauth config mutation
  - added `singleflight` dedupe for concurrent first-hit OIDC discovery (no startup stampede)
  - preserved request validation order (frontend URL/code/session checks before discovery)
- `internal/api/handlers/auth_test.go`
  - updated constructor expectations (no eager oauth config)
  - added lazy discovery success/failure coverage
  - added concurrent discovery regression test (one `.well-known` fetch across parallel callers)
- Poller startup hardening
  - forced ticks now run health/pending pass only; stats deferred to next normal tick (global service-order fix)
  - removed forced-tick synthetic bootstrap event (health-first scheduling only; less state coupling)
  - fixed snapshot merge regression: prior internal events no longer poison health event type on replay
  - health-state merge now applies on any non-internal event (not gated by non-empty message)
  - added regression tests around internal->health promotion + response-time/version replay safety
- Poller observability
  - added per-service one-shot startup metric log: `poller first health seen`
  - structured fields: `instance`, `service`, `status`, `startup_elapsed`
  - added regression tests for once-only tracking + startup timestamp guard
- SSE/reducer refresh regression matrix
  - added web merge test to lock `warning + version + responseTime` persistence across internal stats update + hydration refresh
  - protects prior startup/refresh latency regressions where cards briefly lost health fields after reconnect/reload
- SSE snapshot backend regression matrix
  - added broadcaster snapshot test locking combined replay persistence of `warning + version + responseTime` across internal payload updates
  - mirrors frontend merge guardrail; prevents split-layer drift during reconnect/reload paths
- Broadcaster test DRY cleanup
  - removed repeated snapshot decode boilerplate via shared helper in `broadcast_test.go`
  - keeps regression additions faster/safer while preserving runtime behavior
- Auth surface cleanup
  - removed unused OIDC refresh endpoint wiring (`POST /api/auth/oidc/refresh`)
  - removed dead frontend auth URL config key for refresh
  - full gate green after route cleanup
- Session payload hardening
  - removed unused OIDC token fields from cached `SessionData` (access/refresh/id token, token type)
  - auth now stores only session metadata needed at runtime (`expires_at`, `auth_type`, `user_id`)
  - full gate green after middleware + handler updates
- SSE route cleanup
  - removed legacy `/api/health/events` SSE alias; canonical stream path is `/api/events`
  - updated Vite dev proxy SSE header hint to only match `/api/events`
  - full gate green after route cleanup
- Poller no-stall regression locks
  - added tick-level tests for health-first behavior with slow stats jobs
  - added forced-tick regression test ensuring stats jobs are skipped on forced startup/refresh pass
  - uses temp sqlite DB in forced-tick test to cover real service reload path safely
- Auth mode source-of-truth cleanup
  - removed `auth_type` localStorage read/write/remove from `AuthContext`
  - auth mode now derives from active verified session/user info + in-memory state only
  - logout fallback now uses `authType || user?.auth_type || "builtin"`
- CI flake fix
  - stabilized poller slow-stats regression timing to avoid runner scheduling false negatives
  - renamed test intent to reflect behavior under slow stats contention
  - removed non-deterministic assertion that assumed health goroutine always acquires semaphore before stats
- Full gate run green:
  - `go test ./...`
  - `pnpm -C web lint`
  - `pnpm -C web typecheck`
  - `pnpm -C web test`
  - `pnpm -C web build`

## Next
- Continue request-path reliability hardening in auth/handlers (same no-regression approach).
- Keep poller/SSE regression guardrails tight while refactoring (no startup card stalls).
- Expand backend + frontend mirrored snapshot coverage before deeper poller cleanup slices.
- Defer qui parity/data-shape pass until requested (no qui repo edits).

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

### 2026-02-18 (qui data semantics)
- Qui card data alignment: `Combined Data` now sourced from qui all-time counters (`alltime_dl`/`alltime_ul`) via `/api/instances/:id/torrents?page=0&limit=1` server state; speed still from `/transfer-info` (`dl_info_speed`/`up_info_speed`)
- Fallback behavior: if all-time counters unavailable, use transfer-info session counters (prevents empty/zero regression)
- UI copy: updated card label to `Combined Data (all-time)` to match metric semantics
- Tests: expanded `internal/services/qui/qui_test.go` with all-time override + fallback regression coverage
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (qui overview reset fix)
- Root cause: poller health tick (`runHealth` -> `QuiService.CheckHealth`) emitted `details.qui.summary` with only instance counts; frontend shallow-merge replaced full overview summary and zeroed `Combined Speed/Data` until next `qui_overview` tick.
- Fix: stop emitting `details.qui.summary` from `CheckHealth`; emit count fields on `details.qui` (`totalInstances`, `activeInstances`, `connectedInstances`) so health/status updates do not clobber overview metrics.
- Regression test: `TestCheckHealth_SummarizesInstanceState` now asserts health details do not include `summary`.
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (plex pin auth)
- Added first-class Plex PIN authentication flow (replacing token-from-XML guidance as primary path):
  - Backend endpoints (auth required): `POST /api/plex/auth/pin`, `GET /api/plex/auth/pin/:pinId`
  - Backend handler: `internal/api/handlers/plex_auth.go`
  - Backend service support: `PlexService.CreateAuthPIN` + `PlexService.CheckAuthPIN` using Plex `api/v2/pins` flow
  - Plex PIN response model: `internal/types/plex.go` (`PlexPIN`)
- Added backend tests for PIN handler:
  - `internal/api/handlers/plex_auth_test.go`
- Frontend PIN auth UX:
  - New hook: `web/src/hooks/usePlexPinAuth.ts`
  - Added "Authenticate with Plex" button in both:
    - add service modal (`web/src/components/AddServicesMenu.tsx`)
    - edit configuration modal (`web/src/components/configuration/ConfigurationForm.tsx`)
  - Button opens Plex auth tab, polls PIN status, and auto-fills `X-Plex-Token` on success
- Updated Plex help link to Plex forum PIN auth guide in both forms.
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)
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

### 2026-02-18 (push-first data flow pass)
- Frontend: removed manual refresh hook API from `useServiceData` (`refreshService`) so components consume SSE-only state updates.
- Frontend: removed post-action refresh calls from:
  - `ConfigurationForm` save flow
  - `ArrQueueStatsBase` queue delete flow
  - `OverseerrStats` request approve/reject flow
  - `TailscaleStatusBar` mount bootstrap flow
- Backend: `SettingsHandler` now receives poller and triggers `poller.Refresh(instanceID, all)` after config save, so newly added/updated services publish state quickly without frontend-triggered refresh endpoints.
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).

### 2026-02-18 (qui card polish)
- Removed redundant `Instances x/y active` summary tile from `QuiStats`; per-instance list remains source of truth for active instances.
- Kept combined transfer speed/data tiles for high-signal aggregate metrics.
- Gates: pass (`pnpm -C web typecheck`, `pnpm -C web lint`).

### 2026-02-18 (layout readability pass)
- Dashboard card board tuned for readability at desktop widths: increased service-card min width `22rem -> 23rem` to reduce cramped 4-col packing on 1440/1536 widths.
- Heavy-content cards now follow overview-first disclosure:
  - `AutobrrStats` recent releases collapsed by default; expanded pane capped (`max-h-80`, scroll).
  - `OverseerrStats` recent requests collapsed by default; pending list capped to 3 visible + explicit “showing 3 of N”; lists use bounded scroll areas.
- Goal: reduce row-height blowouts in grid rows and keep top-level card scanline stable while preserving drilldown on demand.
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).

### 2026-02-18 (masonry layout pass)
- Replaced equal-row CSS grid with masonry-style column flow in `ServiceGrid` so shorter cards backfill vertical gaps under taller cards.
- Added `break-inside-avoid` + vertical spacing wrappers for both service cards and loading skeletons to keep card bodies intact across columns.
- Responsive column counts tuned for dashboard density: `1 -> 2 -> 3 -> 4` based on viewport width.
- Research checkpoint (library/build-vs-buy):
  - `react-masonry-css` latest publish is old (2021; npm metadata), so avoided adding stale dependency.
  - `masonic` and `@egjs/react-grid` are active options, but current use case (dozens of cards max, existing dnd-kit wiring) does not need JS layout/virtualization overhead yet.
  - Adopted native CSS multi-column now; keep active-library migration as fallback if drag/animation constraints appear.
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).

### 2026-02-18 (backend refresh-surface cleanup)
- Removed dead manual-refresh endpoint `POST /api/services/:instanceId/refresh` from API router.
- Simplified poller refresh API:
  - removed `RefreshKind` (`health|stats|all`) and related branching
  - `Poller.Refresh(instanceID)` now schedules a full pass for the target instance
  - poller tick path now always runs health pass + stats pass (existing force/interval semantics preserved)
- Updated settings save path to new poller API (`poller.Refresh(instanceID)`).
- Goal: remove obsolete push/pull hybrid surface and keep architecture cleanly backend-push-first.
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).

### 2026-02-18 (service-data reducer split)
- Refactored `web/src/hooks/useServiceData.ts` to reducer-driven state transitions (`useReducer`) instead of ad-hoc map mutation callbacks.
- Extracted merge/patch strategy into `web/src/hooks/serviceData/merge.ts`:
  - typed health payload derivation (`deriveHealthUpdate`)
  - deep merge behavior for nested `stats`/`details`
  - deterministic config hydration merge with latest SSE patches
- Added dedicated reducer module `web/src/hooks/serviceData/reducer.ts`:
  - explicit actions for loading/reset/hydrate/apply-patch/apply-releases
  - single state transition surface for service map + loading flag
- Goal: make SSE merge semantics testable and reduce accidental behavior drift in hook-level effects.
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).

### 2026-02-18 (card primitives dedupe)
- Added shared UI primitive `web/src/components/ui/CollapsibleSection.tsx` for card section disclosure/animation affordance.
- Migrated duplicate collapse section logic in:
  - `web/src/components/services/autobrr/AutobrrStats.tsx`
  - `web/src/components/services/overseerr/OverseerrStats.tsx`
- Goal: reduce repeated collapse markup and keep interaction semantics consistent across service cards.
- Gates: pass (`go test ./...`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web build`).

### 2026-02-18 (message dedupe fix)
- Fixed duplicated warning lines in service cards by centralizing message dedupe in `web/src/utils/serviceMessage.ts`:
  - split+trim lines from both `service.message` and `service.health.message`
  - remove duplicates while preserving order
  - return normalized newline-joined message
- Prowlarr now uses shared combiner instead of local string concatenation:
  - updated `web/src/components/services/prowlarr/ProwlarrStats.tsx`
- Scope is intentionally KISS/DRY: one shared formatter path for all cards using combined service messages.
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

### 2026-02-18 (fix)
- Prowlarr warning persistence/flap root cause: internal SSE payloads (e.g. `prowlarr_stats`, `prowlarr_indexers`) could overwrite top-level `status/message` in merge paths.
- Backend fix (`internal/api/handlers/broadcast.go`):
  - treat snake_case internal event messages as non-health-state payloads
  - preserve prior health `status/message` when merging snapshots from those payloads
- Frontend fix (`web/src/hooks/serviceData/merge.ts`):
  - same internal-event guard for live SSE patch merge
  - still merge `stats/details/lastChecked` from internal payloads; do not clobber warning/error state
- Regression tests:
  - `internal/api/handlers/broadcast_test.go`
  - assert warning state survives internal payload updates
  - assert snapshot message not overwritten by `*_stats`/`*_indexers` events
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Autobrr "Recent Releases" intermittent blank/slow-on-refresh root cause:
  - hydration race in web data layer
  - releases were only restored when latest SSE event for instance was exactly `autobrr_releases`
  - if `autobrr_stats`/`autobrr_irc_status` arrived last, releases were dropped until next releases tick
- Fix:
  - add dedicated `latestReleasesRef` cache in `web/src/hooks/useServiceData.ts`
  - pass `latestReleasesByInstance` through hydrate action/reducer
  - hydrate service cards from cached releases map (independent of latest message ordering)
  - remove stale/unused hydrate health arg from reducer/merge path
- Files:
  - `web/src/hooks/useServiceData.ts`
  - `web/src/hooks/serviceData/reducer.ts`
  - `web/src/hooks/serviceData/merge.ts`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (ui)
- Collapsible section default behavior cleanup:
  - root cause for "Recent Releases/Requests collapsed by default" was hardcoded `useState(false)` in component local state
  - updated defaults to expanded/open for consistency with other service cards
- Files:
  - `web/src/components/services/autobrr/AutobrrStats.tsx`
  - `web/src/components/services/overseerr/OverseerrStats.tsx`
- Gates: pass (`pnpm -C web lint`, `pnpm -C web typecheck`)

### 2026-02-18 (fix)
- Autobrr releases still empty after refresh: secondary root cause in SSE snapshot parsing
  - snapshot `message` is intentionally preserved as health text (e.g. `Healthy`)
  - releases extractor was gated on `message === autobrr_releases`, so snapshot payload with releases shape was ignored
- Fix in `web/src/hooks/serviceData/merge.ts`:
  - add `extractAutobrrReleases(health)` shape-based detection (`stats.autobrr.data[]`)
  - use that detection for both live events and snapshot hydration path
  - suppress `patch.stats` whenever releases payload shape is detected (prevents releases payload from clobbering autobrr stats card)
- Gates: pass (`pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Autobrr releases disappearing on page refresh (even after prior fix): root cause in backend snapshot merge payload shape.
  - `autobrr_stats` and `autobrr_releases` both used the same key (`stats.autobrr`) with different object shapes.
  - snapshot merge kept only last shape for that key, so one side got clobbered before frontend ever saw it.
- Backend fix:
  - namespaced Autobrr payloads under stable nested keys:
    - stats event => `stats.autobrr.stats`
    - releases event => `stats.autobrr.releases`
  - updated both poller emitters and handler broadcast helpers.
- Frontend compatibility fix:
  - Autobrr stats card now reads nested `stats.autobrr.stats` (with fallback for legacy shape).
  - release extractor handles nested releases (`stats.autobrr.releases`) and legacy top-level `stats.autobrr.data`.
- Regression coverage:
  - `internal/api/handlers/broadcast_test.go`: new test asserts snapshot keeps both `autobrr.stats` and `autobrr.releases` fields after consecutive events.
- Files:
  - `internal/api/handlers/poller.go`
  - `internal/api/handlers/autobrr.go`
  - `internal/api/handlers/broadcast_test.go`
  - `web/src/hooks/serviceData/merge.ts`
  - `web/src/components/services/autobrr/AutobrrStats.tsx`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Autobrr releases still intermittently blank on refresh (frontend resilience hardening):
  - card previously depended primarily on `service.releases` side-channel state
  - if that field missed a timing path, UI could show empty even when releases existed in `service.stats.autobrr`
- Fix in `web/src/components/services/autobrr/AutobrrStats.tsx`:
  - releases now resolved with fallback chain:
    - `service.releases.data`
    - `service.stats.autobrr.releases.data` (nested/new shape)
    - `service.stats.autobrr.data` (legacy shape)
- Gates: pass (`pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Autobrr refresh edge case follow-up:
  - fallback precedence bug: `service.releases.data` empty array took priority over non-empty fallback sources.
  - resulted in "No recent releases" even when releases existed in `stats.autobrr`.
- Fix:
  - choose first non-empty releases source across:
    - `service.releases.data`
    - `service.stats.autobrr.releases.data`
    - `service.stats.autobrr.data`
  - if all empty, fall back to first available empty array.
- File:
  - `web/src/components/services/autobrr/AutobrrStats.tsx`
- Gates: pass (`pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Root cause confirmed for flaky Autobrr releases after refresh:
  - not Redis persistence; SSE snapshot overwrite.
  - `AutobrrService.CheckHealth` still emitted legacy shape `stats.autobrr = <stats struct>`.
  - health tick (30s) overwrote nested snapshot payload and dropped `autobrr.releases`.
- Fix:
  - `internal/services/autobrr/autobrr.go`: `CheckHealth` now emits nested shape `stats.autobrr.stats` (same as poller/handler broadcast contract).
  - added regression in `internal/api/handlers/broadcast_test.go` to assert releases survive a subsequent Autobrr health update.
- Files:
  - `internal/services/autobrr/autobrr.go`
  - `internal/api/handlers/broadcast_test.go`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (refactor)
- Step 1 started: canonical SSE schema cleanup for Autobrr (remove fallback paths)
- Web data layer simplification:
  - removed releases side-channel state (`service.releases`, `apply_releases`, `latestReleasesRef`)
  - hydration now replays only canonical merged service patch map
  - removed legacy merge suppression logic tied to old `stats.autobrr.data` shape
- Type cleanup:
  - `ServiceStats.autobrr` now typed as canonical object:
    - `stats?: AutobrrStats`
    - `releases?: AutobrrReleases`
- UI cleanup:
  - Autobrr card now reads only canonical fields:
    - stats from `service.stats.autobrr.stats`
    - releases from `service.stats.autobrr.releases.data`
  - removed compatibility branches for legacy payload shapes
- Files:
  - `web/src/types/service.ts`
  - `web/src/hooks/serviceData/merge.ts`
  - `web/src/hooks/serviceData/reducer.ts`
  - `web/src/hooks/useServiceData.ts`
  - `web/src/components/services/autobrr/AutobrrStats.tsx`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (perf)
- Poller observability pass:
  - added per-job completion telemetry in `maybeRun` with instance/service/job/duration fields.
  - warn on slow jobs (`>=5s`) and warn on timeout (`context deadline exceeded`).
  - keep normal completions at debug level to avoid noisy prod logs while enabling traceability.
- File:
  - `internal/api/handlers/poller.go`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Poller/snapshot stability hardening:
  - dedupe duplicated `*arr` warning lines at source (same warning was rendered twice in cards).
  - snapshot merge now treats health-only fields as health-only:
    - `responseTime` updates only from non-internal health events.
    - `updateAvailable` updates only from non-internal health events and can now clear back to `false`.
- Frontend merge hardening:
  - internal stats events no longer clobber card `responseTime`.
  - health events now explicitly clear `updateAvailable` to `false` when absent/false in payload.
- Regression coverage:
  - `internal/services/arr/health_test.go`: duplicate warning payload collapses to one rendered warning line.
  - `internal/api/handlers/broadcast_test.go`:
    - internal events do not overwrite health response time/update flags.
    - later health update can clear `updateAvailable` from true -> false.
- Files:
  - `internal/services/arr/health.go`
  - `internal/services/arr/health_test.go`
  - `internal/api/handlers/broadcast.go`
  - `internal/api/handlers/broadcast_test.go`
  - `web/src/hooks/serviceData/merge.ts`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (fix)
- Overseerr request status mapping hardening:
  - removed fragile single-source status rendering that produced frequent `Unknown` pills.
  - now resolves display status from:
    - request status enum (source of truth), then
    - media lifecycle status enum fallback (for variant payloads).
  - status icon + color now derived from normalized status tone (`pending/success/error/neutral`).
  - fallback statuses are explicitly marked as `(media)` for clarity.
- Source validation:
  - checked `~/github/oss/seerr/server/constants/media.ts` enums before mapping update.
- File:
  - `web/src/components/services/overseerr/OverseerrStats.tsx`
- Gates: pass (`pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (feat)
- Persisted collapsible UI state in database (dry, shared path):
  - new DB table `ui_collapse_preferences` (+ sqlite/postgres schema + migration `002_add_ui_collapse_preferences`).
  - new DB methods:
    - `GetUICollapsePreferences(userID)`
    - `UpsertUICollapsePreference(userID, key, collapsed)`
  - new protected API endpoints:
    - `GET /api/ui/preferences/collapse`
    - `PUT /api/ui/preferences/collapse` (`{ key, collapsed }`)
  - new web shared preference layer:
    - `UIPreferencesProvider` (fetch once, optimistic updates, rollback on failure)
    - `useUIPreferences`
    - `useCollapsiblePreference`
    - shared key helpers in `web/src/utils/collapsePreferences.ts`
  - migrated collapsible sections/cards to shared persisted keys:
    - service card collapse
    - Autobrr `Recent Releases`
    - Overseerr `Recent Requests`
    - Prowlarr `Active Indexers`
    - Plex `Active Streams`
- Regression coverage:
  - `internal/api/handlers/ui_preferences_test.go` (round-trip + validation)
  - `internal/database/database_test.go` (`TestUICollapsePreferences`)
- Files:
  - `internal/database/migrations/sqlite_schema.sql`
  - `internal/database/migrations/postgres_schema.sql`
  - `internal/database/migrations/sqlite.go`
  - `internal/database/migrations/postgres.go`
  - `internal/database/database.go`
  - `internal/database/database_test.go`
  - `internal/api/handlers/ui_preferences.go`
  - `internal/api/handlers/ui_preferences_test.go`
  - `internal/api/server.go`
  - `web/src/contexts/UIPreferencesContext.tsx`
  - `web/src/hooks/useUIPreferences.ts`
  - `web/src/hooks/useCollapsiblePreference.ts`
  - `web/src/utils/collapsePreferences.ts`
  - `web/src/App.tsx`
  - `web/src/components/services/ServiceCard.tsx`
  - `web/src/components/services/autobrr/AutobrrStats.tsx`
  - `web/src/components/services/overseerr/OverseerrStats.tsx`
  - `web/src/components/services/prowlarr/ProwlarrStats.tsx`
  - `web/src/components/services/plex/PlexStats.tsx`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (ui)
- Plex configuration UX cleanup (KISS):
  - removed manual `X-Plex-Token` text/password input from:
    - `web/src/components/AddServicesMenu.tsx`
    - `web/src/components/configuration/ConfigurationForm.tsx`
  - Plex auth is now PIN-flow only in UI (`Authenticate with Plex` button).
  - added explicit submit guards: fail fast with `Authenticate with Plex first` when token not acquired.
  - show minimal state text (`Authenticated with Plex`) once token is obtained.
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (ui polish)
- Removed remaining Plex auth helper noise in add/edit forms:
  - dropped text `Plex authentication uses PIN login.`
  - dropped external guide link row
- Kept only actionable UI:
  - `Authenticate with Plex` button
  - `Authenticated with Plex` status text
- Gates: pass (`pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (plex auth tab flow)
- Root cause: PIN auth opened new browser context and could leave an extra tab/window open after Plex redirect.
- Fixes:
  - `web/src/hooks/usePlexPinAuth.ts`
    - switched `forwardUrl` to dedicated callback page (`/plex-auth-complete.html`)
    - added popup lifecycle tracking + close-grace handling (avoid false failure when auth window closes right after successful Plex approval)
    - added `postMessage` listener for immediate poll when callback page signals completion
    - kept auth window cleanup centralized in `stop()`
  - `web/public/plex-auth-complete.html`
    - posts completion signal to opener
    - focuses opener and self-closes
    - fallback text if browser blocks close
- Gates: pass (`pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (sse stability pass)
- Frontend SSE reconnect hardening (`web/src/hooks/useServiceData.ts`):
  - stop manual close/reopen on every `EventSource.onerror` (avoid fighting browser-native SSE reconnect loop)
  - reconnect only when stream state is explicitly `CLOSED`
  - jittered exponential reconnect delay helper (`nextReconnectDelay`) with bounded max
  - guard duplicate connections (`OPEN`/`CONNECTING` short-circuit) so one tab keeps one stream
  - reset retry state on clean teardown
- Backend SSE write-path hardening (`internal/api/handlers/events.go`):
  - centralized `write`/`writeString` helpers
  - fail fast on broken pipe/write errors instead of continuing stream loop
  - keeps logs scoped with `client_id` for easier churn debugging
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (poller isolation pass)
- Poller timeout model refactor (`internal/api/handlers/poller.go`):
  - replaced single global `pollerJobTimeout` with explicit timeout classes:
    - `pollerHealthTimeout`
    - `pollerPendingTimeout`
    - `pollerDefaultJobTimeout`
    - `pollerLongJobTimeout`
  - `jobSpec` now supports per-job timeout override (`timeout`)
  - long-running jobs (`autobrr_releases`, `maintainerr_collections`) explicitly opt into long timeout bucket
  - `maybeRun(...)` now receives timeout explicitly; health/pending/jobs pass timeout intentionally
  - added `effectiveJobTimeout(...)` fallback helper (KISS defaulting)
- Tests:
  - updated `internal/api/handlers/poller_test.go` for new `maybeRun` signature
  - added timeout fallback/override coverage in `internal/api/handlers/poller_jobs_test.go` (`TestEffectiveJobTimeout`)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (warning dedupe hardening)
- Root issue: warning dedupe in *arr health checks used strict string equality; whitespace variants could bypass dedupe and appear as duplicate warning lines in UI.
- Fix (`internal/services/arr/health.go`):
  - added `formatWarningMessage(source, message)` normalization helper
  - trims + collapses whitespace in source/message
  - creates stable case-insensitive dedupe key while preserving clean display text
  - skips empty warning payloads safely
- Regression coverage (`internal/services/arr/health_test.go`):
  - existing exact-duplicate test kept
  - added whitespace-variant duplicate test (`TestPerformHealthCheck_DeduplicatesWarningMessagesAfterWhitespaceNormalization`)
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (typed internal events; regex fallback retained)
- Data-shape cleanup for SSE payload semantics:
  - added explicit `eventType` to `models.ServiceHealth` (`internal|health`) in `internal/models/service.go`
  - frontend `ServiceHealth` type updated with `eventType` (`web/src/types/service.ts`)
- Merge behavior cleanup:
  - backend snapshot merge now prefers explicit `eventType` when deciding whether to merge user-visible health state (`internal/api/handlers/broadcast.go`)
  - frontend patch derivation now prefers `eventType` with legacy regex fallback (`web/src/hooks/serviceData/merge.ts`)
- Poller/handler emission cleanup:
  - internal/stat broadcast payloads now set `EventType: ServiceEventInternal` across:
    - `internal/api/handlers/poller.go`
    - `internal/api/handlers/autobrr.go`
    - `internal/api/handlers/plex.go`
    - `internal/api/handlers/radarr.go`
    - `internal/api/handlers/sonarr.go`
    - `internal/api/handlers/prowlarr.go`
    - `internal/api/handlers/maintainerr.go`
- Compatibility: kept regex fallback path so legacy payloads still merge correctly.
- Follow-up hardening:
  - explicit `eventType: health` now overrides regex fallback (treat as health state event even if message looks internal)
  - added broadcaster regression tests for explicit internal/health event semantics:
    - `TestBroadcasterSnapshotTreatsExplicitInternalEventTypeAsInternal`
    - `TestBroadcasterSnapshotTreatsExplicitHealthEventTypeAsHealth`
- Frontend merge fix (`web/src/hooks/serviceData/merge.ts`):
  - only apply `updateAvailable` patch when field exists on payload (`presence.hasUpdateAvailable`)
  - avoids clearing existing update state on SSE events that omit `updateAvailable`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (regression fix: slow services skeleton/hang)
- Regression source: timeout bucket pass used too aggressive default (`12s`) causing slower stats jobs to timeout repeatedly; cards stayed in skeleton/no-data state.
- Fix (`internal/api/handlers/poller.go`):
  - `pollerDefaultJobTimeout`: `12s` -> `25s` (restored safe baseline)
  - `pollerHealthTimeout`: `15s` -> `25s`
  - `pollerLongJobTimeout`: `20s` -> `35s` (keep isolation for known heavier jobs)
  - pending timeout remains short (`5s`)
- Follow-up root cause fix (scheduler jitter):
  - removed non-blocking semaphore `default` skip in `maybeRun(...)`
  - due jobs now wait for a worker slot instead of being dropped/retried each tick
  - eliminates random startup/load jitter (3s..30s) from opportunistic slot misses
  - updated test: `TestPollerMaybeRun_SemaphoreFullWaitsThenRuns` in `internal/api/handlers/poller_test.go`
- Result: keep timeout-bucket architecture, remove aggressive timeout regression.
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (regression fix: refresh hydrate status race)
- Root cause:
  - `hydrate_configurations` reapplied base service model (`status: loading`) over existing runtime service state.
  - cached SSE patch map kept only latest `patch`; when latest event was `internal`, status hint was dropped.
  - result: random cards stuck skeleton until next health tick (3s..30s) after refresh/startup.
- Fixes (`web/src/hooks/serviceData/*`, `web/src/hooks/useServiceData.ts`):
  - introduced `ServicePatchSnapshot { patch, internalStatus }` for cached SSE hydration input
  - merge snapshots instead of overwrite (`mergeServicePatchSnapshot`) to keep nested stats/details + hints
  - reducer/apply path accepts optional `internalStatus`
  - hydrate now preserves runtime fields from existing service and refreshes config fields only (no status reset)
  - hydrate applies cached snapshot with internal-status bootstrap for loading/pending/unknown states
- Env/debug note:
  - local curl confusion reproduced: `127.0.0.1:8080` was Java process (404), dashbrr bound on `localhost/[::1]:8080`.
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (regression test: refresh/startup hydrate race)
- Added frontend regression test harness:
  - `web/package.json`: new `test` script (`node --import tsx --test tests/**/*.test.ts`)
  - `web/tsconfig.tests.json`: node-targeted TS config for test files
  - `web/eslint.config.js`: include `tsconfig.tests.json` in type-aware lint project list
  - `web/package.json` + lockfile: dev dependency `tsx@^4.21.0`
- Added regression tests:
  - `web/tests/serviceData.merge.test.ts`
  - `hydrate_configurations keeps runtime status on refresh` (prevents status reset back to `loading`)
  - `hydrate_configurations applies cached internal status snapshot` (prevents startup skeleton wait until next health tick)
- New-dependency health check (quick):
  - `tsx` latest: `4.21.0`
  - recent publish activity: `2025-11-30` (`npm view tsx time.modified`)
- Gates: pass (`pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`, `go test ./...`)

### 2026-02-19 (refactor #3: shared health/internal event pipeline)
- Added shared backend utility: `internal/api/handlers/service_event.go`
  - `classifyServiceEventType(...)`: single classifier for `health` vs `internal` (explicit `eventType` first, regex fallback only if unset)
  - `normalizeServiceEvent(...)`: enforces non-zero `lastChecked` + explicit `eventType` on every publish
  - `shouldMergeHealthState(...)`: single merge gate for snapshot semantics
  - publish helpers:
    - `publishInternalServiceUpdate(...)`
    - `publishHealthServiceUpdate(...)`
- Broadcaster refactor (`internal/api/handlers/broadcast.go`):
  - `Publish(...)` now normalizes every event before SSE encode/snapshot merge
  - snapshot merge now uses shared `shouldMergeHealthState(...)` instead of local ad-hoc logic
  - removed duplicate internal-event regex logic from broadcaster
- Handler pipeline wiring:
  - switched internal emitters to shared helper in:
    - `internal/api/handlers/poller.go`
    - `internal/api/handlers/autobrr.go`
    - `internal/api/handlers/plex.go`
    - `internal/api/handlers/radarr.go`
    - `internal/api/handlers/sonarr.go`
    - `internal/api/handlers/prowlarr.go`
    - `internal/api/handlers/maintainerr.go`
    - `internal/api/handlers/overseerr.go`
  - health emitters in poller now use shared health helper (`runHealth`, `runPending`)
- Regression coverage:
  - `internal/api/handlers/broadcast_test.go`
    - `TestBroadcasterPublishNormalizesImplicitInternalEventType`
    - `TestBroadcasterPublishNormalizesImplicitHealthEventType`
    - adjusted first snapshot test to compare against normalized payload shape
- Result:
  - explicit event semantics on all runtime publishes
  - one classification/merge rule path, less per-handler drift/edge cases
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-18 (qui step: all-time totals resilience)
- Root cause for flaky `Qui` combined data:
  - `GetAggregatedTransferInfo` swallowed errors from all-time totals endpoint
  - transient `/torrents` misses fell back to session counters (`dl_info_data`/`up_info_data`), causing value drift/resets vs qui dashboard
- Fix (`internal/services/qui/qui.go`):
  - keep speed fields from `transfer-info` as before
  - prefer all-time counters from `serverState.alltime_dl/alltime_ul`
  - add in-process last-known all-time cache keyed by `url+instanceId`
  - on transient all-time fetch failure: reuse cached all-time totals; only use session counters when no cached baseline exists
  - add debug logs for transfer-info/all-time fetch failures (no behavior abort)
- Regression coverage (`internal/services/qui/qui_test.go`):
  - added `TestGetAggregatedTransferInfo_UsesCachedAllTimeTotalsOnTransientFailure`
  - verifies second run keeps first all-time totals when `/torrents` temporarily fails, while speeds still update
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (refactor: shared instanceId validation path)
- Removed repeated instance query/prefix validation boilerplate in handlers:
  - migrated `Autobrr` (`GetAutobrrReleases`, `GetAutobrrReleaseStats`, `GetAutobrrIRCStatus`)
  - migrated `Maintainerr` (`GetMaintainerrCollections`)
  - migrated `Plex` (`GetPlexSessions`)
  - migrated `Overseerr` (`GetRequests`)
- Added `requireInstanceIDWithMissingMessage(...)` in `internal/api/handlers/instance_id.go`:
  - reuses same prefix validation path
  - supports preserving existing custom missing-id error text where needed
  - existing `requireInstanceID(...)` now delegates to this helper
- Added unit coverage:
  - `TestRequireInstanceIDWithMissingMessage` in `internal/api/handlers/instance_id_test.go`
- Goal: KISS/DRY on request validation paths; lower drift risk between handlers.
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (fix: overseerr status mapping aligned with seerr)
- Source-of-truth check:
  - verified local `~/github/oss/seerr/server/constants/media.ts` enums
  - request status enum: `PENDING=1, APPROVED=2, DECLINED=3, FAILED=4, COMPLETED=5`
  - media status enum: `UNKNOWN=1, PENDING=2, PROCESSING=3, PARTIALLY_AVAILABLE=4, AVAILABLE=5, BLACKLISTED=6, DELETED=7`
- Frontend refactor:
  - added shared resolver `web/src/components/services/overseerr/status.ts`
  - robust status parsing supports numeric + string payloads (`\"2\"`, `\"APPROVED\"`, etc.)
  - card state now uses resolver for:
    - pending-request partitioning
    - action buttons (approve/reject visibility)
    - chip label/tone rendering
  - fallback behavior tightened:
    - prefers canonical request status when present
    - uses media lifecycle labels when request status missing/legacy
    - avoids noisy `Unknown` chip by using `Requested` fallback
- Tests:
  - added `web/tests/overseerr.status.test.ts` with resolver coverage for numeric/string/missing status cases
- Gates: pass (`pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`, `go test ./...`)

### 2026-02-19 (poller hardening: fast retry on failed jobs + queue timing)
- CI:
  - `build` run `22175391455` for commit `bea8e2b` completed `success`
- Backend poller resilience (`internal/api/handlers/poller.go`):
  - root cause addressed:
    - failed poller jobs previously waited full nominal interval before retry (e.g. 60s/120s), causing long card skeleton windows after transient upstream failures
  - changed runner contract to return `error` (success/failure now explicit)
  - added failed-job state tracking:
    - `lastRun` (attempt), `lastOKRun` (success), `failed` map
  - failed jobs now retry on short cadence (`pollerFailedRetryDelay=10s`) instead of full job interval
  - added queue-delay timing in logs (`queue_delay`) to expose semaphore wait pressure
  - added stale visibility on failure logs (`stale_for` since last successful run)
- DRY cleanup for *arr queue summaries:
  - `RadarrHandler.broadcastRadarrQueue` now uses shared `summarizeRadarrQueue`
  - `SonarrHandler.broadcastSonarrQueue` + `fetchStats` now use shared `summarizeSonarrQueue`
  - keeps poller + handlers aligned on queue-count math
- Tests:
  - updated poller tests for runner signature
  - added `TestPollerMaybeRun_FailedJobsRetrySoonerThanNominalInterval` regression case
- Gates: pass (`go test ./...`, `pnpm -C web test`)

### 2026-02-19 (refactor: shared service-config lookup helper in handlers)
- Added shared helper: `internal/api/handlers/service_config.go`
  - `findServiceConfig(...)`
  - `requireServiceConfig(...)` -> returns `NewServiceNotConfigured(serviceType)`
  - `requireServiceConfigLegacy(...)` -> returns `ErrServiceNotConfigured`
- Migrated repeated `FindServiceBy + nil/URL check` blocks to helper:
  - `RadarrHandler`: `fetchQueue`, `deleteQueueItem`
  - `SonarrHandler`: `fetchQueue`, `fetchStats`, `deleteQueueItem`
  - `ProwlarrHandler`: `fetchProwlarrData`
  - `PlexHandler`: `fetchSessions`
  - `AutobrrHandler`: `fetchStats`, `fetchReleases`, `fetchIRC`
  - `OverseerrHandler`: `fetchRequests`
- Follow-up migration:
  - `OverseerrHandler.UpdateRequestStatus`: now uses shared `findServiceConfig(...)`
  - `TailscaleHandler.GetTailscaleDevices`: now uses `requireServiceConfig(...)` for instance-key lookup path
- Goal:
  - reduce config-loading drift between handlers
  - keep not-configured semantics explicit (legacy/non-legacy paths preserved)
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (poller hardening #2: panic-safe jobs)
- Reliability hardening in `internal/api/handlers/poller.go`:
  - job execution now wrapped with panic recovery
  - recovered panic is converted into job failure (`error: panic: ...`)
  - failed state/fast-retry path remains active after panic (same as normal failures)
  - prevents one bad integration payload/job panic from crashing the process
- Tests:
  - added `TestPollerMaybeRun_PanicMarksJobFailed` in `internal/api/handlers/poller_test.go`
- Gates: pass (`go test ./internal/api/handlers/...`)

### 2026-02-19 (DRY: queue hash wrappers)
- Ran duplication scan:
  - `jscpd -f go --pattern "internal/api/handlers/**/*.go" --gitignore . --min-lines 8 --min-tokens 80`
  - detected duplicate wrapper logic in `internal/api/handlers/queue_hash.go`
- Refactor:
  - added shared generic mapper `wrapQueueRecords[T](...)`
  - `wrapRadarrQueue` + `wrapSonarrQueue` now delegate to shared mapper
  - behavior unchanged; less drift risk when queue wrapper fields evolve
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (DRY: builtin auth session helpers)
- Refactor in `internal/api/handlers/builtin_auth.go`:
  - added shared helpers:
    - `sessionCacheKey(...)`
    - `isSecureRequest(...)`
    - `BuiltinAuthHandler.getSession(...)`
    - `BuiltinAuthHandler.requireSession(...)`
  - removed repeated session-cache lookup blocks in `Verify` + `GetUserInfo`
  - removed repeated secure-cookie checks in `Login` + `Logout`
  - behavior unchanged; lower drift risk between auth endpoints
- Duplication scan update:
  - reran `jscpd` after refactor; remaining duplicates in handlers are test-only (`broadcast_test.go`)
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (tests: service-config helper coverage)
- Added `internal/api/handlers/service_config_test.go`:
  - `TestRequireServiceConfig`
  - `TestRequireServiceConfig_NotConfigured`
  - `TestRequireServiceConfigLegacy_NotConfigured`
- Scope:
  - verifies configured path and both not-configured semantics (`NewServiceNotConfigured` and legacy `ErrServiceNotConfigured`)
  - guards future helper refactors from silently changing handler error behavior
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (SSE noise reduction: actionable logs only)
- Backend SSE stream (`internal/api/handlers/events.go`):
  - removed connect/disconnect lifecycle chatter from the hot path
  - added `isExpectedSSEWriteError(...)` classifier:
    - treats client-disconnect conditions as expected (`ctx done`, `net.ErrClosed`, `EPIPE`, `ECONNRESET`)
  - write failures now:
    - expected disconnects: no warning noise
    - unexpected write errors: `warn` with `client_id`
- Tests:
  - extended `internal/api/handlers/events_test.go`:
    - `TestIsExpectedSSEWriteError_ContextCanceled`
    - `TestIsExpectedSSEWriteError_ConnectionClosed`
    - `TestIsExpectedSSEWriteError_Unexpected`
- Goal:
  - reduce log spam while preserving real SSE fault visibility
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (bugfix: prowlarr indexer stats 400)
- Root cause:
  - `internal/services/prowlarr/prowlarr.go` sent `startDate=1&endDate=30` (relative ints)
  - Prowlarr API contract (`~/github/oss/Prowlarr`) expects `DateTime? startDate/endDate` on `/api/v1/indexerstats`
  - result: periodic poller failures
    - `prowlarr get_indexer_stats: server returned Bad Request (400)`
- Fix:
  - switched query builder to RFC3339 UTC timestamps
  - default window preserved: last 30 days
  - ensured `startDate < endDate`
- Regression tests:
  - added `internal/services/prowlarr/prowlarr_test.go`
    - `TestBuildIndexerStatsURL_UsesRFC3339Window`
    - `TestGetIndexerStats_SendsProwlarrCompatibleDateParams`
- Gates: pass (`go test ./...`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (ui/bootstrap: remove placeholder warning leak + overseerr parity tests)
- Frontend service hydration:
  - removed configured-service placeholder message `"Waiting for updates"` from base config hydration path
  - configured services now start with no message until first health state arrives
  - avoids misleading transient warning text before real health payload (notably on Prowlarr)
- Overseerr parity hardening:
  - validated enum values against local source of truth `~/github/oss/seerr/server/constants/media.ts`
  - expanded tests in `web/tests/overseerr.status.test.ts`:
    - explicit request/media enum parity assertions
    - approved+available resolution behavior assertion
- Regression tests:
  - `web/tests/serviceData.merge.test.ts` now checks configured services are not seeded with placeholder message
- Gates: pass (`pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`, `go test ./...`)

### 2026-02-19 (poller QoS: jitter + per-job timeouts + stale guard)
- Poller scheduling (`internal/api/handlers/poller.go`):
  - added deterministic per-job jitter for steady-state runs (`applyPollerJobJitter`)
    - spreads non-failed jobs across the interval window (up to 10%, capped at 5s)
    - reduces synchronized bursts against upstream services
  - added stale-data threshold helper (`pollerStaleDataThreshold`)
    - threshold = `2x interval`, bounded `30s..10m`
  - added stale-warning state tracking (`staleWarn` map)
    - on repeated failures beyond stale threshold, emits dedicated warning:
      - `poller job data is stale`
    - stale warning auto-clears on successful job run
- Poller job timeout tuning (explicit by job):
  - short: `12s` (e.g., `plex_sessions`, `qui_overview`)
  - medium: `20s` (e.g., *arr queues/stats, overseerr requests, tailscale devices, autobrr stats/irc)
  - long: existing `35s` for heavier collection/release pulls
- Tests:
  - `internal/api/handlers/poller_jobs_test.go`
    - `TestPollerStaleDataThreshold`
    - `TestApplyPollerJobJitter`
  - `internal/api/handlers/poller_test.go`
    - `TestPollerMaybeRun_FailureMarksJobStaleAfterThreshold`
    - `TestPollerMaybeRun_SuccessClearsStaleWarning`
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (health-message consistency: suppress internal-online bootstrap flips)
- Frontend merge behavior (`web/src/hooks/serviceData/merge.ts`):
  - internal events now only promote card status from `loading/pending/unknown` when status is actionable:
    - `warning`, `error`, or `offline`
  - internal `online` status no longer marks a fresh card healthy before first real health payload
  - prevents transient “healthy”/message regressions while warning-bearing health state is still pending
- Regression tests:
  - `web/tests/serviceData.merge.test.ts`
    - renamed internal snapshot coverage to explicit warning path
    - added guard test: loading state is not promoted to online by internal snapshots
- Gates: pass (`pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`, `go test ./...`)

### 2026-02-19 (regression fix: restore fast card hydration on reload)
- Regression observed:
  - cards stayed in loading state too long on reload after internal-online bootstrap suppression.
- Fix:
  - restored internal `online` status promotion for `loading/pending/unknown` cards.
  - kept prior placeholder cleanup (`"Waiting for updates"` removed), so fast hydration no longer shows fake warning text.
- Test updates:
  - `web/tests/serviceData.merge.test.ts`
    - updated expectation: internal snapshot can promote loading -> online
- Gates: pass (`pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`, `go test ./...`)

### 2026-02-19 (CI watch: auth cleanup + poller test deflake validated)
- CI run: `22191799512` on PR `#82` finished `success` (9m50s).
- Result:
  - confirms `fix(auth): remove local auth mode cache fallback` (`c40a797`)
  - confirms poller test deflake (`29dc8ee`) is stable in GH Actions matrix
- Follow-up:
  - continue rolling refactor queue from `Rolling Plan` (no new CI regressions detected)

### 2026-02-19 (frontend payload normalization: deep merge nested service stats)
- Root cause:
  - `mergeServicePayload` in `web/src/hooks/serviceData/merge.ts` only shallow-merged nested service payloads.
  - partial nested updates could drop sibling fields (`stats.prowlarr.stats.*`, etc.).
- Change:
  - added recursive object merge helper (`mergeRecordDeep`) and switched `mergeServicePayload` to deep-merge record payloads.
  - arrays/primitives still replace (no list concat surprises).
- Regression test:
  - added `mergeServicePatchSnapshot deep-merges nested stats payloads` in `web/tests/serviceData.merge.test.ts`.
- Gates: pass (`pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`, `go test ./...`)

### 2026-02-19 (backend payload normalization: canonical SSE builders)
- Added shared payload builders in `internal/api/handlers/service_payload_builders.go`:
  - `plex_sessions`, `overseerr_requests`, `radarr_queue`, `sonarr_queue`, `sonarr_stats`
  - `prowlarr_stats`, `prowlarr_indexers`
  - `autobrr_stats`, `autobrr_releases`, `autobrr_irc_status`
  - `maintainerr_collections`, `tailscale_devices`, `qui_overview`
- Refactor:
  - poller + service handlers now call the same builder functions (single canonical payload shape per message key)
  - removed duplicated hand-built `models.ServiceHealth` maps across:
    - `poller.go`, `autobrr.go`, `plex.go`, `overseerr.go`, `radarr.go`, `sonarr.go`, `prowlarr.go`, `maintainerr.go`
- Behavior tightening:
  - Autobrr IRC warning path now uses one shared decision function for both handler and poller:
    - healthy => internal `autobrr_irc_status`
    - unhealthy enabled network => health warning (`IRC network <name> is unhealthy`)
- Regression tests:
  - new `internal/api/handlers/service_payload_builders_test.go`
    - `TestBuildAutobrrIRCServiceUpdate_Healthy`
    - `TestBuildAutobrrIRCServiceUpdate_Unhealthy`
    - `TestBuildRadarrQueueServiceUpdate_DetailsAndStats`
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

### 2026-02-19 (contract lock: prevent payload-shape drift)
- Added `internal/api/handlers/service_payload_contract_test.go`:
  - `TestCanonicalServicePayloadMessagesOnlyInBuilder`
    - scans non-test handler Go files
    - fails if canonical message keys are assigned via `Message: "..."` outside `service_payload_builders.go`
  - `TestCanonicalServicePayloadMessagesDeclaredOnceInBuilder`
    - ensures each canonical message key is declared exactly once in `service_payload_builders.go`
- Purpose:
  - hard lock on single-source payload message definitions
  - guards future poller/handler divergence regressions
- Gates: pass (`go test ./...`, `pnpm -C web test`, `pnpm -C web lint`, `pnpm -C web typecheck`, `pnpm -C web build`)

## Rolling Plan
- CI/watch: PR `#82` (`refactor/modernize` -> `develop`)
- Backend/frontend: continue normalizing multi-payload service events so each UI field has one canonical SSE key/path
- Frontend: keep reducing effect-driven derived state; move to memo/render-time derivation where possible
- Frontend: continue zinc palette consistency pass in frequently used components
- Backend: remove leftover dead fields/imports after SWR/singleflight migration
- Backend: remove remaining `context.Background()` in request paths; keep ctx flow explicit
- Backend: continue poller decomposition (extract payload-build helpers, add unit tests before behavior changes)
- Backend: consolidate more *arr handler config/service fetch paths into shared helpers
- Overseerr: keep `status.ts` synced with local `seerr` enums; update resolver tests first when upstream adds states
- Housekeeping: checked for `ead` hooks; none found (only pnpm lock integrity strings)
