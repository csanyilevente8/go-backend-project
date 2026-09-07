# Go vs Spring Boot — Todo backend comparison

Both backends implement the **same REST contract** and talk to the **same
PostgreSQL schema**, so they are drop-in interchangeable in the Helm release
(only `backend.image.repository`/`tag` change). This document compares them.

Measured on GKE cluster `kubecourse` (e2-medium nodes) on 2026-09-07.

## Headline numbers

| Metric | Spring Boot | Go | Difference |
|--------|-------------|-----|------------|
| Memory (idle, `kubectl top`) | ~258 Mi | ~2 Mi | **~130× less** |
| CPU (idle) | ~4 m | ~1 m | lower |
| Container image (uncompressed) | ~660 MB | ~4.6 MB | **~140× smaller** |
| Startup time | ~38 s | ~1 s | **~38× faster** |

## Detail

### Memory
- **Spring Boot ~258 Mi**: JVM heap + metaspace + thread stacks + framework
  object graph. Configured requests were `448Mi` / limits `768Mi` to be safe.
- **Go ~2 Mi**: a statically-linked native binary; no VM, minimal runtime. Could
  run comfortably with requests around `32–64Mi`.

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

## Implication for the cluster

The Go backend's ~2 Mi footprint makes dropping the node pool to `min: 1`
autoscaling feasible (the app tier is now tiny). It also frees headroom for
future workloads (e.g. in-cluster Jenkins).

## Deployment (Option 1 — image swap)

```bash
# to Go
helm upgrade todo oci://europe-central2-docker.pkg.dev/PROJECT/kubecourse/charts/todo \
  -n todo --reset-then-reuse-values \
  --set backend.image.repository=go-backend --set backend.image.tag=<sha> --wait

# back to Spring
helm upgrade todo oci://.../charts/todo -n todo --reset-then-reuse-values \
  --set backend.image.repository=backend --set backend.image.tag=2609514 --wait
```

## Follow-ups

- Add `prometheus/client_golang` to Go for metric parity with the Spring backend.
- Re-measure image size from the registry (`gcloud artifacts docker images list`)
  for the compressed on-registry size.
