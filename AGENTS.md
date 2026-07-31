hi soup
keep shipping; keep CI green

# AGENTS

Owner: soup (s0up4200@pm.me)

## Git

- Do not rebase, squash, or force-push PR branches. Fix problems with a normal follow-up commit. PRs are squash-merged, so branch history does not matter.

## Commands

Backend (Go):

- `go test ./...`
- `make lint-backend` — golangci-lint
- `make precommit` — fmt + gofix + lint + type-check

Frontend (`web/`, pnpm):

- `pnpm -C web lint`
- `pnpm -C web typecheck`
- `pnpm -C web test`
- `pnpm -C web test:browser`
- `pnpm -C web build`

Full gate before a PR: `go test ./...` plus all five web commands above.

## Layout

- `cmd/` — entrypoints
- `internal/` — Go backend: `api/` (handlers, middleware), `services/`, `types/`, `models/`
- `web/` — React + Vite frontend
- `docs/` — services matrix, k8s discovery example, CLI commands

## Gotchas

- CI backend lint runs golangci-lint with `--new-from-rev=HEAD~1`, so it lints only lines added in the newest commit. Re-adding or reverting old code exposes it to current lint rules. Oversized PR diffs (more than 20000 lines) cannot use `only-new-issues` because the GitHub diff API returns 406.
