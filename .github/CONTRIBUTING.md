# Contributing to dashbrr

Thanks for taking interest in contribution! We welcome anyone who wants to contribute.

If you have an idea for a bigger feature or a change then we are happy to discuss it before you start working on it.
It is usually a good idea to make sure it aligns with the project and is a good fit.
Open an issue or ask on [Discord](https://discord.gg/WQ2eUycxyT).

This document is a guide to help you through the process of contributing to dashbrr.

## Become a contributor

* Code: new features, bug fixes, improvements
* Report bugs

## Dependencies

Make sure you have the following dependencies installed before setting up your developer environment:

- [Git](https://git-scm.com/)
- [Go](https://golang.org/dl/) (see [go.mod](../go.mod#L3) for exact version)
- [Node.js](https://nodejs.org) (we usually use the latest Node LTS version)
- [pnpm](https://pnpm.io/installation)
- [prek](https://github.com/prefix-dev/prek) (optional, for pre-commit hooks)

## How to contribute

- **Clone:** Clone the repository or work from a [fork](https://github.com/autobrr/dashbrr/fork).
- **Branching:** Create a new branch for your changes. Use a descriptive name for easy understanding.
  - Checkout a new branch for your fix or feature `git checkout -b fix/service-card-issue`
- **Coding:** Ensure your code is well-commented for clarity. With go use `go fmt`
- **Commit Guidelines:** We appreciate the use of [Conventional Commit Guidelines](https://www.conventionalcommits.org/en/v1.0.0/#summary) when writing your commits.
  - Examples: `fix(oidc): guard session TTL`, `feat(services): add uptime kuma card`
  - There is no need for force pushing or rebasing. We squash commits on merge to keep the history clean and manageable.
- **Pull Requests:** Open a pull request with a clear description of your changes. Reference any related issues.
  - Mark it as Draft if it's still in progress.
- **Code Review:** Be open to feedback during the code review process.

## Development environment

The backend is written in Go and the frontend is written in TypeScript using React.

Clone the project and change dir:

```shell
git clone github.com/YOURNAME/dashbrr && cd dashbrr
```

## Frontend

First install the web dependencies:

```shell
cd web && pnpm install
```

Run the project:

```shell
pnpm dev
```

This should make the frontend available at [http://localhost:5173](http://localhost:5173). It's setup to proxy `/api` to the backend at [http://localhost:8080](http://localhost:8080).

To build the frontend, run:

```shell
pnpm -C web build
```

## Backend

Install Go dependencies:

```shell
go mod tidy
```

Run the project:

```shell
go run cmd/dashbrr/main.go
```

This uses the default `config.toml` and runs the API on [http://localhost:8080](http://localhost:8080).

To build the backend, run:

```shell
make backend
```

You can also build the frontend and the backend at once with:

```shell
make all
```

## Tests

Run backend tests:

```shell
go test ./...
```

Run frontend checks:

```shell
pnpm -C web lint
pnpm -C web typecheck
pnpm -C web test
```
