---
id: fix-drop-libpq-migration-driver
status: proposed
branch: fix/drop-libpq-migrate-driver
---

# Proposal: Drop `lib/pq` by Moving Migrations onto the pgx Driver (fix-drop-libpq-migration-driver)

## Why

`build-and-test` began failing on **2026-08-18** on a documentation-only PR (#250). It is not
environmental. `govulncheck` reports **five reachable advisories** against `github.com/lib/pq@v1.10.9`:

| ID | Summary |
|---|---|
| GO-2026-6172 | Backend frame lengths cause pre-validation memory exhaustion |
| GO-2026-6171 | Malformed `RowDescription` / `DataRow` messages cause panics |
| GO-2026-6170 | Malformed backend frame length causes panic |
| GO-2026-6168 | Unbounded iteration count causes CPU denial of service in `lib/pq/scram` |
| GO-2026-6166 | GSS authentication completes without mutual proof |

⚠️ **Every one of them reports `Fixed in: N/A`.** There is no version to bump to. This gate is
therefore not a transient red — it blocks **every future PR in the repository** until `lib/pq` leaves
the build graph. Suppression is the only other option, and `reference_go_toolchain_pin` records the
standing rule: bump, never suppress.

## What is actually exposed

Narrower than the advisory count suggests, and this should not be overstated:

- The application's runtime queries **never touch `lib/pq`**. `pgRepo` holds a `pgxpool.Pool`
  (`jackc/pgx/v5`), and no file in the repository imports `lib/pq` — it is an `// indirect` entry.
- `lib/pq` is reachable through exactly one edge: the blank import
  `_ "github.com/golang-migrate/migrate/v4/database/postgres"` in
  [`api/internal/database/db.go`](../../api/internal/database/db.go), whose driver is built on it.
  Every govulncheck trace passes through `migrate.NewWithSourceInstance` or `migrate.Migrate.Up`.
- So the vulnerable code runs **at startup only**, against our own Postgres container, on a private
  Docker network, with our own credentials. All five advisories are server→client attacks. Real-world
  exploitability here is **low**.

The reason to act is not imminent compromise — it is that a permanently red security gate stops all
delivery, and that an unmaintained driver with no upstream fix will keep accruing advisories.

## What Changes

Swap the migration driver for the pgx one that golang-migrate already ships. Verified against the
module source at `migrate/v4@v4.19.1/database/pgx/v5/pgx.go`, not assumed:

1. Blank-import `_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"` instead of
   `database/postgres`. That driver imports `jackc/pgx/v5/stdlib` — the module the application
   already depends on — and no `lib/pq`.
2. Rewrite the DSN **scheme only** for the migrate call, via a new `migrateDSN` helper. golang-migrate
   dispatches on URL scheme and the pgx/v5 driver registers itself as **`pgx5`** (`pgx.go:28`), so a
   `postgres://` URL would resolve to the driver this package no longer imports and fail at run time
   with an unknown-driver error. Credentials, host, database and query parameters carry over
   untouched, and the application pool keeps the original URL.
3. `go mod tidy` drops `lib/pq` from `go.mod`.

⚠️ **This is a runtime-critical path with a compile-time-invisible failure mode.** If the scheme
rewrite is wrong the API does not start at all — migrations run before the pool opens. It must not be
merged on a green unit suite alone.

## Verification

- `go build ./...` and `go vet ./...` clean.
- `lib/pq` absent from `go.mod` **and** `go list -deps ./...` returns **0** matches — two independent
  checks, because go.mod alone would not prove it left the build graph.
- `govulncheck ./...` reports **0 module vulnerabilities**. (Locally 7 stdlib advisories remain; they
  are exactly the set CI's pinned Go **1.26.6** fixes, and the local toolchain is 1.26.5.)
- **The load-bearing one:** CI's `Run Database Integration Tests` step
  (`go test -tags=integration ./internal/database/`) calls `database.NewRepository` against a real
  Postgres via testcontainers, so it executes the rewritten DSN and the migrations for real. That step
  passing — not the unit suite — is what clears this change.
- Staging deploy afterwards: `/health` must report `ok`. A failed migration surfaces there as a
  container that will not start.

## Impact

- **Affected:** `api/internal/database/db.go`, `api/go.mod`, `api/go.sum`.
- **Not affected:** all runtime queries, the schema, the migration files themselves.
- **Risk:** startup-only, and caught by the integration step before merge and by staging after it.
