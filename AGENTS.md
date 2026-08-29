hi soup
keep shipping; keep CI green

# AGENTS

Owner: soup (s0up4200@pm.me)

## Collaboration

- Stay inside requested scope. Do not implement review-suggested/extra changes without explicit user approval.
- Treat other agent/CodeRabbit feedback as input to discuss, not automatic action.
- dashbrr is single-user self-hosted software. Prefer readable, maintainable code over paranoid guards for impossible states.
- In final reports, state which checks ran and which were skipped. Do not claim complete while a required check is known failing.

## Git

- Conventional commits: `feat(scope):`, `fix(scope):`, etc. Keep commits focused; split backend/frontend when practical.
- Update PR branches by merging develop into them, never rebase/force-push. PRs are squash-merged, so rebase gains nothing and force-pushes break review history and contributors' local branches.
- Always label a PR when you open it. Run `gh label list` and pick from the existing labels; do not invent new ones. Add `breaking change` whenever the PR renames or removes anything user-facing.
- Never add AI advertising/attribution/co-author lines.

## Commands

Backend (Go):

- `go test -race -count=1 ./...`
- `make lint-backend` — golangci-lint
- `make precommit` — fmt + gofix + lint + type-check; run before final for code changes

Frontend (`web/`, pnpm):

- `pnpm -C web lint`
- `pnpm -C web typecheck`
- `pnpm -C web test`
- `pnpm -C web test:browser`
- `pnpm -C web build`

Full gate before a PR: the Go tests plus all five web commands above.

## Go

- Keep `gofmt` clean. Exports PascalCase, locals camelCase. Prefer explicit error handling.
- Avoid `map[string]interface{}`; use structs. Keep interfaces small (<=5 methods).
- No backward compatibility shims unless requested.
- Tests live beside code as `*_test.go`; prefer table-driven tests and existing fixtures.

## Layout

- `cmd/` — entrypoints
- `internal/` — Go backend: `api/` (handlers, middleware), `services/`, `types/`, `models/`
- `web/` — React + Vite frontend
- `docs/` — services matrix, k8s discovery example, CLI commands

## Gotchas

- CI backend lint runs golangci-lint with `--new-from-rev=HEAD~1`, so it lints only lines added in the newest commit. Re-adding or reverting old code exposes it to current lint rules. Oversized PR diffs (more than 20000 lines) cannot use `only-new-issues` because the GitHub diff API returns 406.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues. See `docs/agents/issue-tracker.md`.

### Triage labels

Use the five canonical triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

This repo uses a single-context domain layout. See `docs/agents/domain.md`.
