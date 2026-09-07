# go-backend-project

A functional Go port of the Spring Boot Todo backend. It implements the **same
REST contract** and connects to the **same PostgreSQL schema**, so the Angular
frontend and the database tier require no changes — this backend is a drop-in
alternative for experimentation and resource comparison.


## Why this exists

To compare a Go implementation against the JVM/Spring Boot one on the same
cluster (image size, memory, CPU, startup time). The two tiers around it
(Angular frontend, PostgreSQL) are unchanged.

## Tech

- Go (net/http with Go 1.22+ pattern routing — no web framework)
- `pgx/v5` (pgxpool) for PostgreSQL
- `golang-migrate` for schema migrations (embedded SQL, run on startup)
- `google/uuid` for ids
- `swaggo/swag` annotations for OpenAPI/Swagger (codegen; see below)
- Testcontainers + `testing` for tests
- No ORM. Migrations live in `internal/db/migrations/` and are applied on
  startup (like Flyway). The initial migration mirrors the Spring Flyway V1 and
  uses `CREATE TABLE IF NOT EXISTS`, so it is safe against a DB whose `todos`
  table already exists.

## Project layout

```
cmd/server/main.go            entrypoint (graceful shutdown)
internal/db/db.go             pgx pool from env vars
internal/model/todo.go        structs matching the DTO/error contract
internal/repository/          SQL data access (todos table)
internal/httpapi/             handlers + validation + CORS + router
```

## REST contract (identical to the Spring backend)

Base path `/api/todos`:

| Method | Path                        | Success | Notes |
|--------|-----------------------------|---------|-------|
| POST   | `/api/todos`                | 201     | title required 1–255, description ≤2000 |
| GET    | `/api/todos`                | 200     | JSON array |
| GET    | `/api/todos/{id}`           | 200/404 | |
| PUT    | `/api/todos/{id}`           | 200/404 | |
| PATCH  | `/api/todos/{id}/complete`  | 200/404 | body `{"completed": true}` |
| DELETE | `/api/todos/{id}`           | 204/404 | |

Health: `GET /actuator/health` → `{"status":"UP"}` (same path so the K8s probes
and Ingress are unchanged).

Validation errors return the same shape:
```json
{"timestamp":"...","status":400,"error":"Validation failed",
 "message":"Invalid request","fieldErrors":{"title":"Title must not be empty"}}
```

JSON uses the same camelCase field names (`createdAt`, `updatedAt`).

## Configuration (same env vars as Spring)

| Variable               | Default                 | Notes |
|------------------------|-------------------------|-------|
| `DB_HOST`              | `localhost`             | |
| `DB_PORT`              | `5432`                  | |
| `DB_NAME`              | `todo`                  | |
| `DB_USERNAME`          | `todo`                  | |
| `DB_PASSWORD`          | **(required)**          | no default — app fails fast if unset |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:4200` | comma-separated |
| `SERVER_PORT`          | `8080`                  | |

## Run locally

Requires the shared PostgreSQL (from the workspace root: `./start-db.sh`).

```bash
export DB_PASSWORD=todo
go run ./cmd/server
# http://localhost:8080
```

## Build / test

```bash
go build ./...
go test -short ./...    # unit + handler tests (no Docker)
go test ./...           # + Testcontainers integration test (needs Docker)
```

## OpenAPI / Swagger

Handlers are annotated with `swaggo/swag` comments. Generate the spec + UI
assets:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o internal/docs
```

Then serve the UI by importing the generated `internal/docs` package and adding
an `http-swagger` route in `internal/httpapi/router.go`. (Not wired in by
default so the project builds without running codegen first.)

## Schema ownership (important)

Both this app (golang-migrate) and the Spring backend (Flyway) can create the
same `todos` table. They keep **separate** migration-tracking tables
(`schema_migrations` vs `flyway_schema_history`). While both run against the
same database:

- Keep the DDL identical across both migration sets.
- Treat **Flyway (Spring) as the primary schema owner**; this app's migration
  is idempotent (`IF NOT EXISTS`) so it never conflicts.
- If Go ever becomes the sole backend, it can own the schema outright via its
  own migrations.

---

## Resource baseline — Spring Boot backend (for comparison)

Measured on GKE (cluster `kubecourse`) on 2026-09-07, so we can later compare
the Go backend against it. Update this section once the Go backend runs on the
cluster.

### Spring Boot (current, live)

| Metric | Value |
|--------|-------|
| Actual memory (idle, `kubectl top`) | **~258 Mi** |
| Actual CPU (idle) | ~4 m |
| Configured requests | cpu 150m, memory 448Mi |
| Configured limits | cpu 1, memory 768Mi |
| Container image (uncompressed) | ~660 MB (JRE + fat jar) |
| Startup time | ~38 s (JVM + Flyway) observed in-cluster |

### Go backend (measured on GKE, 2026-09-07, chart image swap)

| Metric | Value |
|--------|-------|
| Actual memory (idle, `kubectl top`) | **~2 Mi** |
| Actual CPU (idle) | ~1 m |
| Container image (uncompressed, local build) | ~4.6 MB (distroless static) |
| Startup time | **~1 s** (connect → migrate → listening within the same second) |

### Result

| Metric | Spring Boot | Go | Difference |
|--------|-------------|----|-----------|
| Memory (idle) | ~258 Mi | ~2 Mi | ~130× less |
| Startup | ~38 s | ~1 s | ~38× faster |
| Image (uncompressed) | ~660 MB | ~4.6 MB | ~140× smaller |

Deployed via Option 1 (image swap in the existing Helm release): only
`backend.image.repository/tag` were overridden — Service, Ingress, ConfigMap,
Secret, PVC, Postgres, and frontend were untouched, because the Go app matches
the same REST contract, env vars, port (8080), pod label (`app: backend`), and
`/actuator/health` path.

Caveat: the Go app does not (yet) expose a Prometheus metrics endpoint, so the
`PodMonitoring` scrape of `/actuator/prometheus` returns 404 while Go is
deployed. Add `prometheus/client_golang` for metric parity if desired.

Rollback to Spring Boot:
```bash
helm upgrade todo oci://.../charts/todo -n todo --reset-then-reuse-values \
  --set backend.image.repository=backend --set backend.image.tag=2609514 --wait
```
