# Fix Dependabot Security Alerts — Implementation Plan

**Requirement**: [2026-03-31-fix-dependabot-security-alerts.md](2026-03-31-fix-dependabot-security-alerts.md)
**Date**: 2026-03-31
**Status**: Implemented

## Deviations from Plan

- `github.com/docker/docker` was upgraded to `v27.5.1` (not v28.5.2 as planned). v28.5.2 transitively requires `golang.org/x/time@v0.15.0` which mandates Go 1.25, and the Go 1.25 toolchain could not be downloaded in this environment. v27.5.1 fixes all listed CVEs and avoids the toolchain constraint.
- `golang.org/x/time` was explicitly pinned to `v0.9.0` to prevent the MVS resolver from selecting v0.15.0 (which requires Go 1.25). This is a safe, older version with no known vulnerabilities.

## Implementation Steps

1. Mark REQ-019 as "In Progress" in the requirement file and `_index.md`.

2. Upgrade `github.com/docker/docker` to v28.5.2:
   ```
   cd lazy-tcp-proxy && go get github.com/docker/docker@v28.5.2
   ```

3. Upgrade all `go.opentelemetry.io/otel/*` packages to v1.40.0:
   ```
   go get go.opentelemetry.io/otel@v1.40.0
   go get go.opentelemetry.io/otel/sdk@v1.40.0
   go get go.opentelemetry.io/otel/trace@v1.40.0
   go get go.opentelemetry.io/otel/metric@v1.40.0
   go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.21.0
   go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.46.1
   ```
   (The contrib and exporter packages are pinned — only the core sdk/otel/trace/metric are flagged; upgrade those and let tidy sort the rest.)

4. Run `go mod tidy` to remove unused entries and ensure `go.sum` is consistent.

5. Run `go build ./...` to confirm the binary still compiles.

6. Commit `go.mod` and `go.sum` changes.

7. Push to `claude/reduce-proxy-memory-p7iks`.

8. Mark REQ-019 Completed, plan Implemented; commit and push.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/go.mod` | Modify | Bump docker/docker to v28.5.2, otel/* to v1.40.0 |
| `lazy-tcp-proxy/go.sum` | Modify | Updated hashes for new dependency versions |
| `requirements/2026-03-31-fix-dependabot-security-alerts.md` | Modify | Status → In Progress → Completed |
| `requirements/2026-03-31-fix-dependabot-security-alerts-plan.md` | Modify | Status → Implemented |
| `requirements/_index.md` | Modify | Status → In Progress → Completed |

## Risks & Open Questions

- docker/docker v28 may introduce new transitive dependencies or remove old ones; `go mod tidy` handles this automatically.
- If the docker client API has breaking changes between v25 and v28, `go build` will fail and we will need to update call sites in `manager.go`. This is unlikely for the subset of the API used (ContainerList, ContainerInspect, ContainerStart, ContainerStop, NetworkConnect, NetworkDisconnect, Events).
- The `otelhttp` contrib package pins its own otel dependency; upgrading only the core otel packages should be safe as Go module minimum version selection will pick the highest required version.
