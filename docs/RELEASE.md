# Publishing Heureum

Owner: `it4nodummies` — this matches the Go module path `github.com/it4nodummies/heureum` and the container image names `ghcr.io/it4nodummies/heureum-{api,worker,frontend}`.

These steps are performed by the maintainer. Nothing here runs automatically.

## 1. Create the GitHub repository

Create an **empty** public repo on GitHub named `it4nodummies/heureum` (no README/license/gitignore — the repo already has them).

## 2. Push the code

```bash
cd /path/to/heureum
git remote add github git@github.com:it4nodummies/heureum.git
git push github <current-branch>:main
```

On GitHub:
- Set the default branch to `main`.
- Confirm the repo card shows **AGPL-3.0** as the license.
- Add topics (e.g. `jira-alternative`, `issue-tracker`, `agile`, `go`, `nextjs`, `self-hosted`).
- Optionally enable Discussions (the issue-template `config.yml` links to it).

## 3. Cut the 1.0.0 release

Set the `[1.0.0]` date in `CHANGELOG.md` first, then tag. Pushing the tag triggers `.github/workflows/release.yml`, which builds and pushes the three images to GHCR.

```bash
git tag -a v1.0.0 -m "Heureum 1.0.0"
git push github v1.0.0
```

## 4. Publish the GitHub Release

Create a GitHub Release from the `v1.0.0` tag and paste the `## [1.0.0]` section from `CHANGELOG.md` as the release notes.

## 5. Verify the images

After the release workflow finishes, confirm the packages exist and are public:

- `ghcr.io/it4nodummies/heureum-api:1.0.0` (and `:latest`, `:1.0`)
- `ghcr.io/it4nodummies/heureum-worker:1.0.0`
- `ghcr.io/it4nodummies/heureum-frontend:1.0.0`

(GHCR packages default to private; make them public in the package settings if you want anonymous pulls.)

## Pre-publication checklist

- [ ] Three-level gate green: `go build ./... && go vet ./... && go test ./...`; `cd frontend-next && npm run build && npx playwright test`; `go run ./cmd/gapreport` leaves no diff.
- [ ] `helm lint deploy/helm/heureum` passes (run where Helm is installed — it was not available in the build environment during Round 10).
- [ ] `docker compose -f deploy/docker/docker-compose.yml config` and `...prod.yml config` parse (run where Docker is installed).
- [ ] `CHANGELOG.md` `[1.0.0]` date set.
- [ ] Confirm no Atlassian/Jira trademarked assets remain (branding is text "Heureum"; the `/rest/api/3` + `/rest/agile/1.0` + JQL surfaces are deliberate API-compatibility and are documented as such with the trademark disclaimer in `README.md`).

## Known limitation to disclose

Authorization is **enforced server-side on mutations** (403 by role/global-admin). However, **reads are not yet permission-gated**: any authenticated user can read any project's data. Read-side authorization is a planned post-1.0 enhancement (see `SECURITY.md`).
