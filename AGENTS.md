hi soup
keep shipping; keep CI green

# AGENTS

Owner: soup (s0up4200@pm.me)

## Collaboration

- Stay inside requested scope. Do not implement review-suggested/extra changes without explicit user approval.
- Treat other agent/CodeRabbit feedback as input to discuss, not automatic action.
- dashbrr is single-user self-hosted software. Prefer readable, maintainable code over paranoid guards for impossible states.
- Before you write logic that a published spec or format defines (Markdown, HTML, URLs, CSV, semver, cron, time zones), use a well-proven library for it. Name the library in the PR body. If hand-written logic of this kind needs a second fix for an edge case, replace it with the library.
- In final reports, state which checks ran and which were skipped. Do not claim complete while a required check is known failing.

## Rules and what enforces them

When soup corrects you, fix the mistake and add its rule here. If the rule is already here and nothing enforces it, the correction is a repeat: in the same change, enforce it with a structure, a type, or a check, and fill in the right column.

| Rule | Enforced by |
| --- | --- |
| Gate edit and remove controls on the `edits` field of a service configuration (`all`, `apiKey`, `none`). Only `models.AllowedEdits` decides it. | The settings response has no `discovered` field, so tsc fails on code that reads it. `TestServiceConfigResponseEdits` and `TestSettingsDiscoveredService`. |
| Put the URL placeholder, API key label, and API key help of a service type in `web/src/config/serviceForms.ts`. Both service forms read it. | `Record<ServiceType, ServiceForm>`: tsc fails when a type has no entry. |
| Add a service type in `internal/services/catalog/catalog.go`. The registry and the CLI read that table, and the web `ServiceType` union is generated from it. | `TestTypeScriptServiceTypes`, `TestPollerHasJobsForEveryServiceType`, and `TestEveryServiceTypeHasCLICommands` name each place a new type is missing. tsc fails on each `Record<ServiceType, …>` table. |
| Log a service URL only through `core.RedactURL`. | `make check-log-urls`, in CI. |
| Write web API URLs as relative paths (`api/...`), so they resolve under a base path. | ESLint `no-restricted-syntax` in `web/eslint.config.js`. |
| A PR with the `breaking change` label has `!` in its title. | `.github/workflows/pr-title.yml`. |
| Web unit tests pass. | `pnpm -C web test`, in CI. |
| Run the browser tests before you report on them. A launch error that names a missing executable means the browser build differs from the one the locked Playwright version needs. | `pnpm -C web test:browser` installs that Chromium build before it runs the tests. |
| Spec-defined logic (URLs, paths, Markdown, semver, cron, time zones) uses a proven library. | Judgment, no check: see Collaboration. |
| Update PR branches by merging develop. | Judgment, no check: see Git. |

## Git

- Conventional commits: `feat(scope):`, `fix(scope):`, etc. Keep commits focused; split backend/frontend when practical.
- Update PR branches by merging develop into them, never rebase/force-push. PRs are squash-merged, so rebase gains nothing and force-pushes break review history and contributors' local branches.
- Add the `breaking change` label whenever a PR renames or removes anything user-facing.

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

## Code Review Rules

These rules are for AI PR reviewers. The agent workflow rules in this file (scope, git, the full gate before a PR) are for coding agents. Do not apply them to PR authors.

- Report a defect only when the change causes a concrete wrong behavior. Name the trigger and the result for the user. If you cannot name both, omit the finding.
- Check the merge base. If `develop` already has the problem, still report it, but label it "already on develop" and do not call it a regression.
- When the PR body, a linked issue, an ADR in `docs/adr/`, or a code comment calls a behavior deliberate, respond to that reason. Report a design flaw only when you can say why the stated reason does not hold.
- Do not report what gofmt, golangci-lint, ESLint, or tsc already report. Do not ask for docstrings or other code comments. Report a stale comment.
- Read earlier review threads. Do not repeat a finding that was resolved or refuted, unless you have new evidence.
