# Go vs Spring Boot — Todo backend comparison

Both backends implement the **same REST contract** and talk to the **same
PostgreSQL schema**, so they are drop-in interchangeable in the Helm release
(only `backend.image.repository`/`tag` change). This document compares them.

Measured on GKE cluster `kubecourse` (e2-medium nodes) on 2026-09-07.

## Headline numbers

| Metric | Spring Boot | Go | Difference |
|--------|-------------|-----|------------|
| Memory (warm, `kubectl top`) | ~283 Mi | ~2–3 Mi | **~100× less** |
| CPU (warm, idle) | ~6 m | ~1 m | lower |
| Container image (uncompressed) | ~660 MB | ~4.6 MB | **~140× smaller** |
| Startup time | ~38 s | ~1 s | **~38× faster** |
| Full request cycle (avg, warm) | ~47.6 ms | ~15.6 ms | **~3× faster** |

## Detail

### Memory
- **Spring Boot ~283 Mi** (warm): JVM heap + metaspace + thread stacks +
  framework object graph. Configured requests were `448Mi` / limits `768Mi`.
- **Go ~2–3 Mi**: a statically-linked native binary; no VM, minimal runtime.
  Could run comfortably with requests around `32–64Mi`.

### Image size
- **Spring Boot ~660 MB**: JRE base image + Spring/dependency fat jar.
- **Go ~4.6 MB**: `distroless/static` base + a single static binary (CGO off,
  `-ldflags="-s -w"`). No shell, non-root.

### Startup time
- **Spring Boot ~38 s**: JVM start, classpath scan, Spring context, Flyway.
- **Go ~1 s**: `connect → migrate → listening` within the same second.

### Build
- **Spring Boot**: Maven builds a fat jar; multi-stage Docker (Maven → JRE).
- **Go**: `go build` a static binary; multi-stage Docker (golang → distroless).

## Functional parity

Identical from the client's perspective:

- Endpoints: `POST/GET/PUT/PATCH/DELETE /api/todos[...]`, `GET /actuator/health`.
- JSON field names (camelCase), status codes, and error shape
  (`timestamp/status/error/message/fieldErrors`).
- Validation: title required 1–255, description ≤2000.
- Same env vars (`DB_*`, `CORS_ALLOWED_ORIGINS`), port 8080, pod label
  `app: backend` — so Service, Ingress, ConfigMap, Secret, and probes are
  unchanged.
- Same PostgreSQL schema; Go uses `golang-migrate` (mirrors Flyway V1,
  idempotent) instead of Flyway.

## Differences / trade-offs

| Aspect | Spring Boot | Go |
|--------|-------------|-----|
| Prometheus metrics | `/actuator/prometheus` (Micrometer) | **not yet** — PodMonitoring gets 404 |
| OpenAPI/Swagger | springdoc (runtime UI) | swaggo annotations (codegen via `swag init`) |
| Ecosystem/DI | large, batteries-included | stdlib-first, minimal deps |
| Schema owner | Flyway (primary) | golang-migrate (idempotent, secondary) |
| Startup/footprint | heavier | very light |
| Team familiarity | (project default) | learning exercise |

## Live `kubectl top` output

### Pods — `kubectl top pods -n todo`

With **Go** backend deployed:
```
NAME                        CPU(cores)   MEMORY(bytes)
backend-6947c56bff-kw86j    1m           3Mi
frontend-7989b85d9f-v625m   1m           4Mi
postgres-0                  1m           29Mi
```

With **Spring Boot** backend (warm):
```
NAME                        CPU(cores)   MEMORY(bytes)
backend-86b9cd4f76-tc7fs    6m           283Mi
frontend-...                1m           4Mi
postgres-0                  1m           ~30Mi
```

The backend pod differs by **283Mi → 3Mi** memory between Spring and Go.
Frontend and postgres are unchanged (same tiers).

### Nodes — `kubectl top nodes`

After the swap to Go (backend node freed ~250Mi):
```
NAME                                       CPU    CPU%   MEMORY    MEM%
gke-kubecourse-medium-pool-...-i656        120m   12%    1352Mi    48%
gke-kubecourse-medium-pool-...-zpbf        122m   12%    1258Mi    44%
```

Before (Spring Boot on the same node was ~54% memory). The node hosting the
backend dropped roughly **54% → 44%** memory — directly visible as the JVM's
~250Mi being released. CPU is negligible for both at idle (~1–4m).

### Takeaway

At idle the entire app tier (Go backend + frontend + postgres) now uses well
under ~40Mi combined. This is the concrete argument for a `min: 1` autoscaling
floor and leaves ample room for additional workloads.



The Go backend's ~2 Mi footprint makes dropping the node pool to `min: 1`
autoscaling feasible (the app tier is now tiny). It also frees headroom for
future workloads (e.g. in-cluster Jenkins).

## API latency benchmark

100 cycles of **create → mark complete → get all → delete**, run from a pod
**inside the cluster** hitting the `backend` Service directly (bypasses the
internet/DNS filter, so this measures API + DB latency, not network round-trip).
Times in milliseconds.

### Go backend

| Operation      | n   | min  | avg  | median | p95  | max  |
|----------------|-----|------|------|--------|------|------|
| CREATE (POST)  | 100 | 2.9  | 4.1  | 3.8    | 5.3  | 13.0 |
| PATCH complete | 100 | 3.8  | 4.9  | 4.7    | 6.6  | 9.2  |
| GET all        | 100 | 2.0  | 2.8  | 2.6    | 4.0  | 8.1  |
| DELETE         | 100 | 3.1  | 3.8  | 3.6    | 4.6  | 15.4 |
| **Full cycle** | 100 | 12.4 | 15.6 | 14.8   | 19.3 | 29.7 |

### Spring Boot backend (warm)

| Operation      | n   | min  | avg  | median | p95  | max   |
|----------------|-----|------|------|--------|------|-------|
| CREATE (POST)  | 100 | 7.1  | 11.0 | 9.7    | 18.0 | 33.9  |
| PATCH complete | 100 | 8.5  | 13.6 | 11.9   | 28.3 | 41.3  |
| GET all        | 100 | 6.0  | 11.0 | 9.0    | 25.0 | 32.2  |
| DELETE         | 100 | 7.3  | 12.0 | 10.0   | 23.0 | 48.1  |
| **Full cycle** | 100 | 30.6 | 47.6 | 41.6   | 81.9 | 105.4 |

### Latency comparison

| | Spring Boot (avg) | Go (avg) | Ratio |
|---|---|---|---|
| CREATE | 11.0 ms | 4.1 ms | ~2.7× faster |
| PATCH  | 13.6 ms | 4.9 ms | ~2.8× faster |
| GET all| 11.0 ms | 2.8 ms | ~3.9× faster |
| DELETE | 12.0 ms | 3.8 ms | ~3.2× faster |
| Full cycle | 47.6 ms | 15.6 ms | ~3× faster |
| Full cycle p95 | 81.9 ms | 19.3 ms | ~4.2× faster |

Both were measured warm, in-cluster, same 100-cycle workload. Go was ~3× faster
per request and had noticeably tighter tail latency (lower p95/max). Likely
factors: Hibernate/JPA overhead per query vs. raw pgx, and JVM GC pauses showing
up in Spring's higher p95/max. Contrary to the common "warm JVM is competitive"
assumption, here Go led on steady-state latency as well as footprint.

## Follow-ups

- Add `prometheus/client_golang` to Go for metric parity with the Spring backend.


