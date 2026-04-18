# Go Coding Standards — VigilAfrica

**Scope:** all Go code under `api/` in this repository.
**Audience:** contributors writing Go, and reviewers enforcing standards via `/openspec-review`.
**Status:** living document. Any contributor may open a PR proposing changes; maintainer approval merges.

Each rule has the shape: **statement → why → example (where useful)**. Rules are numbered (`§4.2`) so reviewers can cite them directly.

Cross-references:
- [ADR-007](../../openspec/specs/vigilafrica/decisions.md) — Go + stdlib `net/http` as the server runtime.
- [ADR-009](../../openspec/specs/vigilafrica/decisions.md) — No ORM; raw `pgx` queries in `internal/database/` only.

---

## Table of Contents

1. [Package Structure & Layout](#1-package-structure--layout)
2. [Configuration & Secrets](#2-configuration--secrets)
3. [Context Propagation](#3-context-propagation)
4. [Error Handling](#4-error-handling)
5. [Repository Pattern & DB Access](#5-repository-pattern--db-access)
6. [HTTP Handlers & Middleware](#6-http-handlers--middleware)
7. [Concurrency & Goroutine Lifecycle](#7-concurrency--goroutine-lifecycle)
8. [Logging & Observability](#8-logging--observability)
9. [Testing](#9-testing)
10. [Dependencies & Modules](#10-dependencies--modules)
11. [Migrations & SQL](#11-migrations--sql)
12. [Appendix — Decision Log](#appendix--decision-log)

---

## 1. Package Structure & Layout

> Cross-ref: ADR-007.

**§1.1 — Binaries live under `cmd/<name>/`; each has one `main.go`.**
*Why:* Multiple binaries (API server, one-shot ingestor) share internal packages without circular imports. Matches Go community convention.
Current layout: `cmd/server/main.go`, `cmd/ingest/main.go`. New binaries (e.g. `cmd/backfill/`) follow the same shape.

**§1.2 — All non-`main` code lives under `internal/`.**
*Why:* Go's compiler enforces `internal/` as import-private to the module. Prevents external consumers from depending on implementation details and keeps the public surface at zero.
❌ `api/pkg/database/` — exported to the world.
✅ `api/internal/database/` — module-private.

**§1.3 — One concern per package. Package name matches directory name and is a single lowercase noun.**
*Why:* `handlers`, `database`, `ingestor`, `normalizer` — readable at import sites (`database.NewRepository`, not `db.NewDB`).
❌ `package utils` / `package helpers` / `package common` — grab-bags rot fast.
✅ `package normalizer` — one job: EONET → internal model.

**§1.4 — No cyclic imports. Dependencies flow `cmd/` → `internal/<feature>` → `internal/database` + `internal/models`.**
*Why:* Models and the repository interface are leaf packages; feature packages (handlers, ingestor) depend on them, never the reverse. Cycles force premature abstraction.

**§1.5 — Shared types go in `internal/models/`. Do not redefine domain types in feature packages.**
*Why:* `models.Event`, `models.IngestionRun` are the single source of truth. Handlers, repo, and ingestor all import them.

---

## 2. Configuration & Secrets

**§2.1 — Read all configuration from environment variables via `os.Getenv`. No config files, no flags for runtime config.**
*Why:* 12-factor. Works identically in local dev, Docker, and the VPS deployment. Env vars are the only surface ops needs to manage.

**§2.2 — Required env vars must fail fast at startup with `log.Fatal`. Never fall back to a default for secrets or DB URLs.**
*Why:* A silent default means the wrong database or a missing API key ships to prod unnoticed.
❌
```go
dbURL := os.Getenv("DATABASE_URL")
if dbURL == "" {
    dbURL = "postgres://localhost/vigilafrica"  // silent fallback
}
```
✅ (from `api/cmd/ingest/main.go`)
```go
dbURL := os.Getenv("DATABASE_URL")
if dbURL == "" {
    log.Fatal("DATABASE_URL is not set")
}
```

**§2.3 — Non-secret operational defaults (ports, timeouts, poll intervals) may have safe fallbacks. Document the default in code.**
*Why:* `PORT=8080` is fine to default; `RESEND_API_KEY` is not.
```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080" // default
}
```

**§2.4 — Never hardcode secrets, API keys, URLs with credentials, or email addresses. Never commit `.env`.**
*Why:* Git history is forever. `.env.example` is the canonical template; `.env` is gitignored.

**§2.5 — Env var names are `SCREAMING_SNAKE_CASE` and scoped by feature prefix where ambiguous.**
*Why:* `RESEND_API_KEY`, `ALERT_EMAIL_TO`, `DATABASE_URL`, `INGEST_POLL_INTERVAL`. Prefix disambiguates when multiple subsystems have similar concepts.

**§2.6 — Read env vars once at startup into a typed config struct or local vars in `main`. Do not call `os.Getenv` from deep in the call stack.**
*Why:* Makes dependencies explicit, testable, and auditable. Handlers and the repository should receive their config via constructor args, not read env directly.

---

## 3. Context Propagation

**§3.1 — `context.Context` is the first parameter of every function that does I/O, blocks, or calls another ctx-aware function. Name it `ctx`.**
*Why:* Cancellation and deadlines must flow end-to-end. A function that can't be cancelled poisons every caller.
✅ `func (r *pgRepo) ListEvents(ctx context.Context, filters EventFilters) (...)`

**§3.2 — Never store `context.Context` in a struct field. Pass it explicitly.**
*Why:* Contexts are request-scoped; struct fields are long-lived. Storing ctx leaks request state across requests and defeats cancellation.
❌
```go
type Handler struct { ctx context.Context; repo Repository }
```
✅ Pass `ctx` through method args; store only dependencies (`repo`, `logger`) on the struct.

**§3.3 — HTTP handlers must propagate `r.Context()` into every downstream call.**
*Why:* When the client disconnects, the request context cancels — DB queries and outbound HTTP should unwind immediately.
✅ (from `api/internal/handlers/events.go`)
```go
events, total, err := h.repo.ListEvents(r.Context(), filters)
```
❌ `h.repo.ListEvents(context.Background(), filters)` — ignores the request lifecycle.

**§3.4 — Background work (scheduler, ingestor) owns its own root context derived from `context.Background()`, wired to `os.Interrupt` / `SIGTERM` via `signal.NotifyContext` or manual `cancel()`.**
*Why:* Graceful shutdown requires a cancellation signal the scheduler actually listens to.
✅ (from `api/cmd/ingest/main.go`)
```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
go func() { <-sigCh; cancel() }()
```

**§3.5 — Apply `context.WithTimeout` at the boundary of external calls (outbound HTTP, long DB queries). Do not set a timeout on the handler's root context — let the HTTP server's timeouts govern.**
*Why:* Timeouts should be as close to the call they bound as possible. Nested timeouts compose correctly; redundant outer timeouts mask the real cause of cancellations.

**§3.6 — Check `ctx.Err()` before starting expensive work inside loops.**
*Why:* In a loop over N countries or N events, cancellation should abort between iterations rather than complete all work before returning.
```go
for _, country := range DefaultCountries {
    if err := ctx.Err(); err != nil { return err }
    // ...
}
```

---

## 4. Error Handling

**§4.1 — Wrap errors with `fmt.Errorf("...: %w", err)`. Never use `%s` or `%v` on an error you're re-returning.**
*Why:* `%w` preserves the chain so `errors.Is` and `errors.As` work upstream. `%s` flattens it to a string and breaks sentinel checks.
❌ `return fmt.Errorf("query failed: %s", err.Error())`
✅ (from `api/internal/database/queries.go`)
```go
return nil, 0, fmt.Errorf("failed to get events count: %w", err)
```

**§4.2 — Wrap with context that names *what* failed, not *where*. The stack already knows the function.**
*Why:* "failed to scan event row" is useful; "error in ListEvents" is noise the stack trace already tells you.

**§4.3 — Check sentinel errors with `errors.Is`, never `==`.**
*Why:* Wrapped errors break `==` comparison. `errors.Is` walks the chain.
✅ (from `api/internal/handlers/events.go`)
```go
if errors.Is(err, pgx.ErrNoRows) {
    respondWithError(w, http.StatusNotFound, "event not found")
    return
}
```

**§4.4 — Never `panic` in handlers, repository methods, or ingestion code. Return an error.**
*Why:* A panic in a handler crashes the goroutine and, without recovery middleware, the whole process. Errors are the project's single control-flow primitive for failure.
Acceptable panic: `init()`-time invariants that indicate a build-time bug (e.g. regex compilation).

**§4.5 — Handlers must not leak internal error text to clients. Log the real error; return a sanitised message.**
*Why:* Internal errors (SQL syntax, pgx codes, file paths) are attack surface and noise.
```go
if err != nil {
    slog.Error("list events failed", "err", err)
    respondWithError(w, http.StatusInternalServerError, "internal server error")
    return
}
```

**§4.6 — Use `respondWithError(w, code, message)` for every client error response. Do not hand-roll JSON error bodies.**
*Why:* Consistent `{"error": "..."}` shape across the API; one place to change if the error envelope evolves.

**§4.7 — Do not ignore errors with `_`. If an error is genuinely safe to drop, comment why.**
*Why:* Ignored errors hide failures that look fine in dev and page at 3am in prod.
❌ `_ = json.NewEncoder(w).Encode(resp)` without comment.
✅ Either handle the error, or comment the reason it's safe to drop.

**§4.8 — Define sentinel errors as package-level `var Err... = errors.New(...)`. Do not return string-literal errors for conditions callers may need to branch on.**
*Why:* Enables `errors.Is` at call sites. `errors.New("not found")` in three different places is three different errors to the type system.

---

## 5. Repository Pattern & DB Access

> Cross-ref: ADR-009.

**§5.1 — All SQL lives in `internal/database/`. Handlers, ingestor, and normalizer call repository methods; they never import `pgx` directly or write SQL.**
*Why:* Single place to audit queries, index usage, and PostGIS calls. Handlers stay focused on HTTP concerns.
❌ `pool.Query(ctx, "SELECT ...")` inside a handler.
✅ `h.repo.ListEvents(r.Context(), filters)`.

**§5.2 — Define the repository surface as a Go `interface`. Handlers and the ingestor depend on the interface, not the concrete `pgRepo`.**
*Why:* Enables swapping implementations for tests and composition. The interface is the contract; `pgRepo` is one implementation.

**§5.3 — User-supplied values go through `$N` parameter placeholders. Never string-concatenate or `fmt.Sprintf` user values into SQL.**
*Why:* SQL injection. `fmt.Sprintf` is only acceptable for building the placeholder *index* or the `WHERE` clause structure — never for the values themselves.
✅ (from `api/internal/database/queries.go`)
```go
conditions = append(conditions, fmt.Sprintf("category = $%d", argID))
args = append(args, filters.Category)
argID++
```
The `$1` is formatted into the SQL; the value goes through `args`.
❌ `fmt.Sprintf("WHERE category = '%s'", filters.Category)` — injection.

**§5.4 — `pool.Query` must be followed by `defer rows.Close()` on the next line, and `rows.Err()` must be checked after the loop.**
*Why:* Leaked rows hold a connection. `rows.Err()` surfaces iteration errors that `rows.Next()` hides.
```go
rows, err := r.pool.Query(ctx, query, args...)
if err != nil { return nil, 0, fmt.Errorf("failed to query events: %w", err) }
defer rows.Close()
for rows.Next() { ... }
if err := rows.Err(); err != nil { return nil, 0, fmt.Errorf("rows iteration error: %w", err) }
```

**§5.5 — `QueryRow` callers must handle `pgx.ErrNoRows` explicitly. Translate it to a domain error (`nil, nil` for "optional", a sentinel for "required but missing") before returning.**
*Why:* Callers shouldn't know `pgx.ErrNoRows` exists; they should know "no last run" or "event not found". Prefer `errors.Is(err, pgx.ErrNoRows)` per §4.3; update legacy sites opportunistically.

**§5.6 — Return typed slices, not `*sql.Rows` or `any`. Scan inside the repository.**
*Why:* The repository owns the row → struct mapping. Leaking `rows` to callers couples them to pgx and the query shape.

**§5.7 — Return an allocated empty slice (`make([]T, 0)`), never `nil`, when the result is empty.**
*Why:* JSON-encodes to `[]` instead of `null`; callers can range without nil checks.

**§5.8 — Transactions use `pool.BeginTx` with explicit `Commit`/`Rollback`. `defer tx.Rollback(ctx)` immediately after `Begin` — rollback after commit is a no-op and the deferred call protects the error paths.**
*Why:* A forgotten rollback holds locks until the connection is reaped.

**§5.9 — `COUNT(*)` + data query is the standard pagination pattern when total is needed. Reuse the same `whereClause` and `args` for both.**
*Why:* Single source of truth for filter logic.

---

## 6. HTTP Handlers & Middleware

> Cross-ref: ADR-007.

**§6.1 — Handlers use the stdlib signature `func(w http.ResponseWriter, r *http.Request)`. No Gin, Echo, Fiber, or Chi.**
*Why:* ADR-007. Go 1.22+ `http.ServeMux` supports method routing and path params — frameworks add dependencies without paying for themselves at this scale.

**§6.2 — Path parameters are read via `r.PathValue("id")`. Query params via `r.URL.Query().Get("key")`.**
*Why:* Stdlib idioms; no router coupling.
```go
idStr := r.PathValue("id")
```

**§6.3 — Handlers live on a struct that holds dependencies. Construct with `NewXHandler(repo, ...)` in `main`.**
*Why:* Explicit dependency injection; testable without globals.
```go
type EventHandler struct { repo database.Repository }
```

**§6.4 — Validate and parse inputs before calling the repository. Return `400` with a specific message for invalid input; `500` only for unexpected failures.**
*Why:* Cheap to validate, expensive to query. A clear 400 helps callers more than a generic 500.
```go
if cat != "floods" && cat != "wildfires" {
    respondWithError(w, http.StatusBadRequest, "invalid category: valid values: floods, wildfires")
    return
}
```

**§6.5 — Set `Content-Type: application/json` *before* `WriteHeader`, and `WriteHeader` *before* the body. Once the body is written, headers are frozen.**
*Why:* Silently dropped headers are one of the most common stdlib `net/http` bugs.
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
json.NewEncoder(w).Encode(response)
```

**§6.6 — Use `respondWithError` for every error response. Do not mix bare `http.Error` calls with JSON responses.**
*Why:* `http.Error` returns `text/plain`; the rest of the API returns JSON. Consistency matters for clients.

**§6.7 — Middleware is `func(http.Handler) http.Handler`. Compose in `main` in the order: recovery → logging → CORS → rate-limit → auth → handler.**
*Why:* Recovery outermost so panics in any later middleware are caught. Logging sees the request even if rate-limit rejects. Auth after rate-limit so unauthenticated floods are cheap to reject.

**§6.8 — Never block in a handler on an unbounded operation. Wrap long calls in `context.WithTimeout` derived from `r.Context()`.**
*Why:* A slow downstream (EONET fetch, slow query) shouldn't hold an HTTP goroutine indefinitely.

**§6.9 — Do not read `r.Body` more than once without `io.ReadAll` + reassignment. Close it when done.**
*Why:* `Body` is a stream. Frameworks often hide this; stdlib doesn't.

**§6.10 — Register routes in `main` (or a dedicated `routes.go`), not inside handler packages.**
*Why:* One place to audit the public API surface. Matches the `cmd/server/main.go` pattern.

---

## 7. Concurrency & Goroutine Lifecycle

**§7.1 — Every goroutine has a known owner responsible for its lifecycle. No `go f()` without an answer to "who cancels it and who waits for it?"**
*Why:* Goroutine leaks are invisible until the process OOMs. Ownership = a `context.Context` that cancels it plus a `sync.WaitGroup` or channel that signals completion.

**§7.2 — Background goroutines receive a `context.Context` and return when `ctx.Done()` fires.**
*Why:* Graceful shutdown depends on every goroutine honouring cancellation.
```go
go func() {
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C: s.runAllCountries(ctx)
        }
    }
}()
```

**§7.3 — Signal handling (`os.Interrupt`, `SIGTERM`) belongs in `main` only. Library code never calls `signal.Notify`.**
*Why:* Two packages fighting over signals produces unpredictable shutdowns.

**§7.4 — Use `sync.WaitGroup` to wait for goroutines on shutdown. `wg.Add` before `go`, `defer wg.Done` first line of the goroutine.**
*Why:* `Add` after `go` races with `Wait`. `defer wg.Done` first ensures it runs even on panic.
❌
```go
go func() { doWork(); wg.Done() }()
wg.Add(1)
```
✅
```go
wg.Add(1)
go func() { defer wg.Done(); doWork() }()
```

**§7.5 — Channels have an explicit owner: one writer, many readers; or many writers, one reader. The owner closes the channel; readers never close.**
*Why:* Closing a channel from a reader (or a second writer) panics. Single-owner discipline makes closure safe.

**§7.6 — Channel direction belongs in function signatures: `<-chan T` for read-only, `chan<- T` for write-only.**
*Why:* Compiler-enforced contracts beat comments.
✅ `func consume(events <-chan Event)` — can't accidentally send.

**§7.7 — Shared mutable state is protected by `sync.Mutex` (or `sync.RWMutex` for read-heavy). Lock acquisition and release are on paired lines with `defer`.**
*Why:* Paired `defer` runs on every return path including panics. A forgotten `Unlock` deadlocks the next caller.
```go
m.mu.Lock()
defer m.mu.Unlock()
```

**§7.8 — Prefer channels for coordination (signalling, pipelines) and mutexes for protection (shared state). Do not use a channel where a mutex fits, or vice versa.**
*Why:* "Share memory by communicating" applies to data flow; a counter doesn't need a channel.

**§7.9 — Every package with goroutines or shared state must be tested with `-race` in CI. Failing race detector = failing build.**
*Why:* Race conditions are silent in dev and corrupting in prod. The detector is cheap; skipping it is not.

**§7.10 — The scheduler's `runAllCountries` loop checks `ctx.Err()` between countries and logs errors per-country without aborting siblings.**
*Why:* One country's ingestion failure should not block the others. See §3.6.

**§7.11 — No global state for concurrency primitives. Mutexes, channels, and wait groups live on the owning struct.**
*Why:* Package-level `sync.Mutex` is untestable and usually hides a missing abstraction.

---

## 8. Logging & Observability

**§8.1 — Use `log/slog` for all new logging. Do not use `log.Printf` or `fmt.Println` in `internal/`.**
*Why:* Structured logs are queryable; unstructured strings aren't. Legacy `log.Printf` in `cmd/ingest/main.go` is grandfathered — new code uses slog.

**§8.2 — Log as key-value pairs, not formatted strings.**
*Why:* `slog` fields become JSON keys; grep becomes filter.
❌ `slog.Info(fmt.Sprintf("ingested %d events for %s", n, country))`
✅ `slog.Info("ingestion complete", "country", country, "events", n)`

**§8.3 — Use levels with intent: `Debug` (local-only noise), `Info` (normal operations), `Warn` (recoverable anomaly), `Error` (failed operation a human should see).**
*Why:* If everything is Info, nothing is. Alerting depends on levels meaning something.

**§8.4 — Every error log includes the error under the `err` key. Use `slog.Error("what failed", "err", err, ...)`.**
*Why:* Consistent key makes it searchable across the fleet.

**§8.5 — Do not log secrets, full request bodies, full DB URLs, or PII. Log identifiers (IDs, counts, countries) instead.**
*Why:* Logs often ship to third-party aggregators. `postgres://user:pass@host` in a log is a credential leak.

**§8.6 — Library code receives a `*slog.Logger` via constructor or ctx, not via `slog.Default()`.**
*Why:* `slog.Default()` is global state; inject the logger so tests can assert output and main can scope attributes per-subsystem.
```go
type EventHandler struct { repo database.Repository; log *slog.Logger }
```

**§8.7 — Request handlers log at most once per request outcome (success at Debug, failure at Error). Middleware handles the access log.**
*Why:* Double-logging (middleware + handler) doubles volume and cost without adding signal.

**§8.8 — Long-running operations (ingestion run, scheduler tick) log start and end with duration. Use a consistent phrase pair: "ingestion started" / "ingestion complete".**
*Why:* Duration telemetry falls out of log pairs for free; consistency lets ops dashboards match them.

**§8.9 — Include a correlation key when one is available: `run_id`, `event_id`, `country`. Pass via logger context.**
*Why:* Joins otherwise disparate log lines.
```go
runLog := log.With("run_id", runID, "country", country.Code)
runLog.Info("ingestion started")
```

**§8.10 — Handler JSON output is not a logging channel. Do not return internals in error bodies; log them (§4.5) and return a sanitised message.**
*Why:* Clients are not the audit trail.

---

## 9. Testing

**§9.1 — Test files sit next to the code they test as `foo_test.go`. Package is `package foo` for white-box tests or `package foo_test` for black-box.**
*Why:* Go convention. Black-box (`_test`) is preferred for testing exported behaviour; white-box for unexported helpers.

**§9.2 — Use table-driven tests for any function with more than one input case.**
*Why:* One loop, N cases; adding a case is one struct literal. Shared setup runs once.
```go
tests := []struct {
    name    string
    input   string
    wantErr bool
}{
    {"valid category", "floods", false},
    {"invalid category", "earthquakes", true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

**§9.3 — Subtests use `t.Run(name, ...)` with a descriptive name. No `Test1`, `Test2`.**
*Why:* Names appear in `go test -run` and failure output. Readable names = faster debugging.

**§9.4 — Assertions use `testify/require` (fatal) for preconditions, `testify/assert` (non-fatal) for independent checks.**
*Why:* `require` stops the test when continuing is pointless (nil pointer dereference incoming). `assert` lets multiple independent checks report together.

**§9.5 — Mocks are hand-written or generated; never stub by modifying globals. Tests depend on the repository `interface`, not `pgRepo`.**
*Why:* Modifying globals bleeds state across tests. Interface substitution is the project's mocking seam (§5.2).

**§9.6 — Integration tests hit a real Postgres + PostGIS. Do not mock the database.**
*Why:* Mocked DB tests have passed while a migration broke prod. Integration tests catch schema, trigger, and PostGIS behaviour mocks miss. Use `docker-compose up -d` or `testcontainers-go` for the test DB.

**§9.7 — Integration tests are tagged `//go:build integration` and gated behind a `make test-integration` target.**
*Why:* Unit tests stay fast (`go test ./...` in seconds); integration tests run separately in CI.
```go
//go:build integration
package database_test
```

**§9.8 — Every package with goroutines or shared state runs under `-race` in CI.**
*Why:* Repeats §7.9 because it's a testing rule too. Race detector is mandatory for concurrent code.

**§9.9 — Table rows must not share mutable state. If the test mutates, capture the row variable: `tt := tt` before `t.Run`.**
*Why:* Classic loop-variable capture bug. Go 1.22+ fixes this for `for` loops; pin `tt` anyway for 1.21 compatibility and parallel subtests.

**§9.10 — `t.Cleanup` is preferred over `defer` for teardown that must run after subtests.**
*Why:* `defer` runs when the enclosing function returns, which may be before parallel subtests finish. `t.Cleanup` runs at the right time.

**§9.11 — Tests do not depend on wall-clock time, random ports, or network egress. Inject clocks; use `httptest.Server` for HTTP; use `net.Listen(":0")` for OS-assigned ports.**
*Why:* Flaky tests erode trust in CI faster than missing coverage.

**§9.12 — Benchmark hot paths (ingestion loop, event list query assembly) with `testing.B`. Commit benchmark results alongside significant perf changes.**
*Why:* Optimisation without measurement is superstition. `go test -bench` is free.

---

## 10. Dependencies & Modules

**§10.1 — Stdlib first. Reach for a third-party dependency only when the stdlib equivalent is materially worse or doesn't exist.**
*Why:* Every dependency is a supply-chain surface, a version to track, and a decision future contributors must understand. `net/http`, `log/slog`, `database/sql`, `encoding/json` cover most needs.

**§10.2 — New dependencies require an ADR or a line in an OpenSpec change record justifying the choice. Include: what problem it solves, what was considered, why stdlib isn't sufficient.**
*Why:* Governance. Today's convenient dep is tomorrow's unmaintained abandonware. `go.mod` is a contract, not a scratchpad.
Cross-ref: ADR-009 (pgx chosen over `database/sql` + driver with explicit rationale).

**§10.3 — Pin exact versions in `go.mod`. Do not use `latest` or floating pseudo-versions without a recorded reason.**
*Why:* Reproducible builds. `go mod tidy` + committed `go.sum` is the baseline.

**§10.4 — Current approved non-stdlib dependencies: `jackc/pgx/v5`, `google/uuid`, `golang-migrate/migrate/v4`, `resend/resend-go/v2`, `oschwald/maxminddb-golang`. Adding to this list requires §10.2.**
*Why:* One place to audit the external surface. Keep this list up to date when deps change.

**§10.5 — `go mod tidy` runs clean before every commit that touches `.go` files. `go.sum` is committed.**
*Why:* Divergent `go.sum` breaks other contributors' builds and CI.

**§10.6 — Do not vendor (`go mod vendor`) unless there's a specific airgap or build-reproducibility requirement. VigilAfrica does not vendor.**
*Why:* Vendoring multiplies repo size and makes dependency updates noisier than they need to be.

**§10.7 — Prefer libraries over frameworks. A framework that owns your `main` (Echo, Fiber, Buffalo) is a one-way door; a library you call is reversible.**
*Why:* ADR-007 codified this for HTTP. Apply the principle to future choices: ORM vs query builder, framework vs router, etc.

**§10.8 — Security-sensitive deps (TLS, auth, crypto, database drivers) require at minimum a monthly `go list -u -m all` review. Subscribe to the repo's security advisories.**
*Why:* CVEs in `pgx` or `net/http` matter; CVEs in a logging formatter usually don't.

**§10.9 — Do not import `internal/` packages from other modules. If code needs to be shared across repos, that's an ADR conversation about extracting a module, not an `internal/` bypass.**
*Why:* `internal/` is Go's hardest boundary; respecting it keeps the public API of this module at zero.

**§10.10 — `go.mod`'s `go` directive matches the project's declared Go version (currently `1.26`). Bumping it is a separate PR.**
*Why:* Language version changes are semantic; they deserve their own review and change record.

---

## 11. Migrations & SQL

**§11.1 — Migrations live in `api/db/migrations/` and are numbered sequentially: `NNNNNN_description.up.sql` and `NNNNNN_description.down.sql`. Six-digit zero-padded prefix.**
*Why:* `golang-migrate` orders by numeric prefix. Zero-padding keeps lexical sort matching numeric sort. Current sequence: `000001` … `000006`.

**§11.2 — Every `up.sql` has a matching `down.sql`. If a migration genuinely cannot be reversed (e.g. data transformation), `down.sql` contains `-- no-op: irreversible, see header` with justification in the `up.sql` header comment.**
*Why:* Rollback is a production-incident tool. "We can't roll back" is a decision, not an accident; it should be explicit.

**§11.3 — Migrations run automatically on API server startup. Manual migration runs are only for operational recovery.**
*Why:* Reduces dev setup friction and prevents schema drift. See `CONTRIBUTING.md`.

**§11.4 — Migrations are idempotent where safe. Use `CREATE TABLE IF NOT EXISTS`, `DO $$ BEGIN ... EXCEPTION WHEN duplicate_object THEN NULL; END $$`, and `INSERT ... ON CONFLICT DO NOTHING` for seed data.**
*Why:* Re-running a migration after partial failure should not wedge the database. Data-seeding migrations (`000005_admin_boundary_data`) rely on this.

**§11.5 — Migrations do not depend on data from earlier migrations beyond what their `up.sql` creates. Each migration is self-contained in its schema dependencies.**
*Why:* Out-of-order application during debugging shouldn't corrupt state. Data seeds can depend on schema from earlier numbered migrations; schema changes should not depend on seed data.

**§11.6 — One logical change per migration. Do not bundle a new table, a trigger fix, and a seed update into one file.**
*Why:* Smaller migrations are easier to review, roll back, and bisect. The history is a ledger.

**§11.7 — Never edit a migration that has been merged to `main`. Write a new migration that supersedes it.**
*Why:* Other environments have already applied the old file; editing it creates divergent schemas that `golang-migrate`'s hash check will flag — or worse, silently miss.

**§11.8 — SQL in migrations uses lowercase identifiers and `snake_case` for tables, columns, and indexes.**
*Why:* Postgres lowercases unquoted identifiers. Consistent casing avoids surprise quoting.

**§11.9 — Spatial columns use `geometry(Point, 4326)` or `geometry(Polygon, 4326)` — always SRID 4326 (WGS84). Distance calculations cast to `::geography`.**
*Why:* Mixed SRIDs fail silently or at query time. EPSG:4326 is the project's coordinate system; casting to geography gives spherical distance in metres.

**§11.10 — Spatial indexes are `GIST` on the geometry column. Create them in the same migration as the column.**
*Why:* ST_Intersects and ST_DWithin need GIST to avoid sequential scans on large tables.
```sql
CREATE INDEX idx_events_geom ON events USING GIST (geom);
```

**§11.11 — Trigger functions use `CREATE OR REPLACE FUNCTION` and are accompanied by a `CREATE TRIGGER ... IF NOT EXISTS` (or a drop-then-create guarded by `DO $$`).**
*Why:* Triggers evolve; replacement without dropping the trigger definition is the normal edit path. See `api/db/migrations/000006_fix_enrichment_trigger.up.sql`.

**§11.12 — Migration headers include a one-line purpose and the target milestone. Production-quality vs prototype-quality data is labelled in the header.**
*Why:* `000005_admin_boundary_data.up.sql` uses simplified rectangular boundaries — the header says so, making the future HDX upgrade obvious.

---

## Appendix — Decision Log

Decisions made during the brainstorming session that produced this document.

| # | Decision | Alternatives | Rationale |
|---|---|---|---|
| 1 | Document serves both enforcement and onboarding | Two separate docs | Same rules for both audiences; one file avoids drift |
| 2 | Cover all 8 originally-proposed areas | Subset | Full coverage from v1; gaps create ambiguity in review |
| 3 | Rule + rationale + code example per rule | Rule-only, or rule + long explanation | Examples drawn from actual repo make rules concrete and citable |
| 4 | Cross-reference ADRs where they exist; stand alone otherwise | ADR-only, or no cross-refs | ADRs capture *why the decision was made*; rules capture *how to apply it* |
| 5 | Living document — contributor PRs, maintainer approves | Maintainer-only edits; frozen releases | Keeps the doc current with codebase evolution |
| 6 | Flat sections with numbered rules (§N.M) | Grouped by severity; grouped by lifecycle phase | Citable in reviews; scannable for onboarding |
| 7 | Added Context Propagation and Concurrency as dedicated sections | Fold into Handlers/Repository | /golang-pro review flagged these as foundational for a repo with schedulers and goroutines |
| 8 | Configuration & Secrets as its own section (not folded into §1) | Fold into Package Structure | Secret handling warrants dedicated visibility |
| 9 | Order: Context before Error Handling; Concurrency after Handlers | Original 8-section order | Context is foundational; concurrency builds on ctx + handlers |
