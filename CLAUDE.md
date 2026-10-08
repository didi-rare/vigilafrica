# VigilAfrica — project instructions for Claude Code

Read this before touching anything. It is short because the long version is in
`CONTRIBUTING.md`, `docs/standards/`, and the OpenSpec records under `openspec/`.

## Governance gate (CI will fail without it)

- A PR that changes `api/internal/`, `api/cmd/` or `web/src/` MUST carry an
  OpenSpec record **in that PR's own diff**: either a proposal under
  `openspec/proposals/` or a change record under `openspec/changes/<id>/`.
  The sentinel (`api/cmd/sentinel`) diffs against `origin/development`.
- The only bypass is the `[trivial]` tag, and it must sit **on its own line in
  the commit message**, never in a PR title (the title check needs the subject
  to start with a letter). Mentioning the tag in prose does not count.
- The OpenSpec workflow is `/openspec-explore` → `/openspec-apply` →
  `/openspec-review` → `/openspec-archive` (project commands in
  `.claude/commands/`). A change record is `openspec/changes/<id>/proposal.md`
  + `spec.md` (or `specs/`) + `tasks.md`; tick tasks with the evidence, not
  just a checkbox.

## Branches and commits

- Feature and fix branches start from `development` and PR back to
  `development`. `main` is staging, `release` is production; promotions are
  merge-commit PRs (squashing loses CHANGELOG entries).
- Conventional commits (`feat(web):`, `fix(api):`, `chore:`, `docs(openspec):`,
  `refactor(web):`). Commit messages and PR bodies say what changed and why —
  no AI attribution or co-author trailers.
- Push your branch and stop. Do not open PRs to `main` or `release`.

## Verifying your work

- API (Go, from `api/`): `go vet ./...` and `go test -race ./...`. Integration
  tests need Docker: `go test -tags=integration ./internal/database/`. On a
  Windows host AppLocker blocks native test binaries — use
  `scripts/test-api.ps1` there instead.
- Web (from `web/`): `npm run lint`, `npm run type-check`,
  `npm run lint:styles`, `npm run test`, `npm run build`. Stylelint enforces
  design tokens for **colour only** (`web/.stylelintrc.json`); a new colour
  literal in component CSS fails CI — add a token to
  `web/src/styles/tokens.css`. Spacing, typography and z-index tokens are
  review-enforced (`developers-react.md` §7.5, §7.10, §7.11), not machine-checked.
- A pure-CSS change needs pixel evidence, not just green checks:
  `scripts/bench-dashboard-cls/README.md` has the two-arm (control build vs
  branch) protocol and the Playwright scripts for CLS, geometry and
  reduced-motion checks.
- `npm run web:build` regenerates `web/src/data/milestones.json`; don't commit
  that churn.
- The database is PostGIS (`postgis/postgis:15-3.4` in `docker-compose.yml`).
  Plain PostgreSQL will not run the migrations.

## Contracts and sources of truth

- OpenAPI: edit `openspec/specs/vigilafrica/openapi.yaml`, then
  `npm run sync:openapi`; never edit the copy under `api/internal/handlers/`.
- Any number that drives a decision ships with a runnable script in the same
  PR (`scripts/bench-*`), or it is an assertion, not evidence.
- Nothing in this repo deploys. Deploy keys, MaxMind credentials and VPS access
  are never needed for development work and must not be requested.
