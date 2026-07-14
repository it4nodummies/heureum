# Contributing to Heureum

Thanks for your interest in contributing to Heureum! This document covers what you need to
get set up locally, the quality gates every change must pass, and the conventions we use for
commits and pull requests.

By participating in this project you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).

## License note

Heureum is licensed under the [GNU AGPL v3.0](LICENSE). By submitting a contribution (a pull
request, patch, or any other content intended for inclusion in the project), you agree that
your contribution is provided under the same AGPL-3.0 license.

## Prerequisites

- **Go 1.25** (see `go.mod`)
- **Node 22** for the frontend (`frontend-next/`)
- SQLite is enough for local development; Postgres/MySQL and Redis are optional and only
  needed to exercise those code paths

## Running the project locally

```bash
# 1. seed a local SQLite database with demo users, a project and sample issues
APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed

# 2. start the API server
APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/server

# 3. start the frontend (Next.js App Router, served under /app)
cd frontend-next && npm install && npm run dev
```

Open `http://localhost:3000/app` and sign in with the demo account:

- Email: `admin@example.com`
- Password: `admin-demo-123`

## The three-level quality gate

Every change must pass all three of the following before it is considered mergeable. Run
them locally before opening a pull request — CI enforces the same gates.

1. **Backend build, vet and tests**

   ```bash
   go build ./... && go vet ./... && go test ./...
   ```

2. **Frontend build and end-to-end tests**

   ```bash
   cd frontend-next && npm run build && npx playwright test
   ```

3. **API contract freshness gate**

   ```bash
   go run ./cmd/gapreport
   ```

   This regenerates `docs/contracts/gap-report.md` from the current route table. The
   command must leave the file with **no diff** against what's committed — if it does,
   commit the regenerated file as part of your change. This keeps the documented Jira API
   compatibility surface honest and up to date.

If you change anything in the public API surface (routes, request/response shapes, status
codes), make sure it stays conformant to the contract tests in `internal/contract/`. Those
tests compare Heureum's behavior against the official Jira Cloud v3 / Agile 1.0 OpenAPI
specs and are the source of truth for compatibility — update them alongside your change, and
re-run the gap report gate above.

## Commit style

We use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages,
e.g.:

```
feat(frontend): add sprint burndown chart
fix(api): correct pagination cursor for issue search
docs: update CONTRIBUTING with contract test guidance
```

Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`, `build`, `ci`.

## Branching and pull requests

- Branch off `master` for your work; use a short, descriptive branch name
  (e.g. `feat/sprint-burndown`, `fix/issue-search-pagination`).
- Keep pull requests focused on a single change where possible.
- Describe what changed and why in the PR description; link any related issues.
- Make sure the three-level gate above passes before requesting review.
- Squash or clean up noisy WIP commits before merge where practical.

## Design and planning docs

Implementation plans for larger pieces of work live under
[`docs/superpowers/plans/`](docs/superpowers/plans/). If you're proposing a substantial
change, consider writing a short plan there first so reviewers have context on the intended
approach before diving into the diff.

## Reporting bugs and requesting features

Please open a GitHub issue with:

- A clear description of the problem or request
- Steps to reproduce (for bugs), including Go/Node versions and DB driver in use
- Expected vs. actual behavior

For security vulnerabilities, do **not** open a public issue — see
[SECURITY.md](SECURITY.md) instead.
