# SQLite → PostgreSQL Migration Design

**Date:** 2026-05-20  
**Status:** Approved

## Context

The project uses SQLite for persistence. The only consumer is the `reminders` feature, which is incomplete and not yet used in production. Existing data can be discarded. A PostgreSQL instance is now available in the homelab, and the goal is to replace SQLite with it without changing any service behaviour.

## Scope

- Replace `internal/tech/sqlite/` with `internal/tech/postgres/`
- Swap driver and migration tooling to PostgreSQL equivalents
- Regenerate sqlc with `engine: postgresql`
- Update config env var from `SQLITE_PATH` to `DATABASE_URL`
- Update tests to use `testcontainers-go` instead of `:memory:`
- Update deployment config (docker-compose)

Out of scope: schema design changes, new features, changes to domain or HTTP layers.

## Architecture

The package boundary and exported API stay identical:

```
app.go
  └── postgres.Open(cfg.Database)  → *sql.DB
  └── postgres.NewRemindersRepo(db) → *RemindersRepo
         └── implements reminders.Repository
```

`reminders_repo.go` and `conversion.go` are unchanged. Only the import path changes from `home-go/internal/tech/sqlite` to `home-go/internal/tech/postgres`.

## Components

### `internal/tech/postgres/database.go`

- Driver: `github.com/jackc/pgx/v5/stdlib` (registered as `"pgx"`)
- `sql.Open("pgx", cfg.DSN)` — accepts a standard PostgreSQL DSN or URL
- Remove `db.SetMaxOpenConns(1)` (SQLite single-writer constraint, not needed for PG)
- Migration driver: `github.com/golang-migrate/migrate/v4/database/pgx/v5`
- Embedded migrations and `golang-migrate` error handling unchanged

### `internal/tech/postgres/migrations/`

Rename existing migration files. Schema changes from SQLite:

| Column | SQLite type | PostgreSQL type |
|---|---|---|
| `trigger_at`, `next_run_at`, `recur_every_seconds`, `valid_until`, `last_fired_at`, `created_at`, `updated_at`, `acked_at` | `INTEGER` | `BIGINT` |
| `requires_ack` | `INTEGER NOT NULL DEFAULT 0` | `BIGINT NOT NULL DEFAULT 0` |

Rationale: Go model stores all these as `int64`; keeping `BIGINT` means no change to `conversion.go` or `reminders_repo.go`.

All other DDL is compatible as-is: `CHECK` constraints, FK `ON DELETE CASCADE`, `CREATE INDEX IF NOT EXISTS`, `CREATE TABLE IF NOT EXISTS`.

Down migration: unchanged.

### `internal/tech/postgres/sql/reminders.sql`

SQL query changes:

| Change | SQLite | PostgreSQL |
|---|---|---|
| Positional placeholders | `?` | `$1`, `$2`, … `$N` |
| Ignore-on-conflict insert | `INSERT OR IGNORE INTO` | `INSERT INTO … ON CONFLICT DO NOTHING` |

All other query logic is identical.

### `sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "internal/tech/postgres/migrations/*.up.sql"
    queries: "internal/tech/postgres/sql/reminders.sql"
    gen:
      go:
        package: "sqlc"
        out: "internal/tech/postgres/sqlc"
        sql_package: "database/sql"
        emit_interface: true
        emit_json_tags: true
```

`sqlc generate` is re-run to produce `$N`-style generated code.

### `internal/config/config.go`

```go
type DatabaseConfig struct {
    DSN string  // was: Path string
}
```

Env var: `DATABASE_URL` (was `SQLITE_PATH`).  
No default value — the app will fail to connect if unset (same behaviour as before when `SQLITE_PATH` was left empty and the driver attempted to open it).

### `internal/app.go`

Import path changed: `home-go/internal/tech/sqlite` → `home-go/internal/tech/postgres`.  
No other changes.

### `ops/deploy/docker-compose.yaml`

- Remove `SQLITE_PATH` environment override and the `data:` named volume
- Remove volume mount (`volumes: - data:/data`)
- Add `DATABASE_URL` to the service's `environment` block (value sourced from `.env`)

### `env.example`

Replace:
```
# SQLITE_PATH=./home_go.db
```
With:
```
# DATABASE_URL=postgres://user:password@host:5432/dbname?sslmode=disable
```

### `go.mod`

Remove:
- `modernc.org/sqlite`
- `modernc.org/libc`, `modernc.org/mathutil`, `modernc.org/memory` (indirect deps of sqlite)
- `github.com/golang-migrate/migrate/v4/database/sqlite` (migrate driver)

Add:
- `github.com/jackc/pgx/v5`
- `github.com/testcontainers/testcontainers-go`
- `github.com/testcontainers/testcontainers-go/modules/postgres`

The `github.com/golang-migrate/migrate/v4` core stays; only the database sub-driver changes.

## Tests

`internal/tech/postgres/reminders_repo_test.go` — all 8 existing test cases kept verbatim. Only `openDB` changes:

```go
func openDB(t *testing.T) *reminders.Repository {
    t.Helper()
    ctx := context.Background()
    pgC, err := tcpostgres.Run(ctx, "postgres:16-alpine",
        tcpostgres.WithDatabase("testdb"),
        tcpostgres.WithUsername("test"),
        tcpostgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).WithStartupTimeout(30*time.Second),
        ),
    )
    // ... error handling, t.Cleanup, open + migrate
}
```

Test run: `go test ./internal/tech/postgres/...` — pulls `postgres:16-alpine` if not cached, runs migrations, exercises all cases.

No other test files are affected.

## Error Handling

- `Open` returns `(*sql.DB, error)` — same signature
- Migration errors bubble up unchanged
- `sql.ErrNoRows` → `reminders.ErrNotFound` mapping in `reminders_repo.go` unchanged
- No new error paths introduced

## Acceptance Criteria

1. `go build ./...` succeeds with no SQLite imports
2. `go test ./internal/tech/postgres/...` passes (all 8 test cases green)
3. `go test ./...` (all other tests) unaffected
4. Application starts and connects to a PostgreSQL instance via `DATABASE_URL`
5. `reminders.Repository` interface fully satisfied by the new `RemindersRepo`
6. No `modernc.org/sqlite` dependency in `go.mod`
