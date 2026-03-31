# Container Name in Start/Stop Log Messages — Implementation Plan

**Requirement**: [2026-03-31-container-name-in-start-stop-logs.md](2026-03-31-container-name-in-start-stop-logs.md)
**Date**: 2026-03-31
**Status**: Implemented

## Implementation Steps

1. `docker/manager.go` — `EnsureRunning`: after the existing `ContainerInspect` call, derive `name := strings.TrimPrefix(inspect.Name, "/")` and use it in both log lines instead of `containerID[:12]`.
2. `docker/manager.go` — `StopContainer`: add `containerName string` parameter; replace `containerID[:12]` with `containerName` in both log lines.
3. `proxy/server.go` — `checkInactivity`: update `StopContainer` call to pass `e.name` as the new third argument.
4. Run `go build ./...` to confirm no compilation errors.
5. Commit and push.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Use `inspect.Name` in `EnsureRunning`; add `containerName` param to `StopContainer` |
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Pass `e.name` to updated `StopContainer` call |

## API Contracts

N/A

## Data Models

N/A

## Key Code Snippets

```go
// EnsureRunning — name derived from existing inspect
name := strings.TrimPrefix(inspect.Name, "/")
log.Printf("docker: starting container \033[33m%s\033[0m", name)

// StopContainer — new signature
func (m *Manager) StopContainer(ctx context.Context, containerID string, containerName string) error {
    log.Printf("docker: stopping container \033[33m%s\033[0m (idle timeout)", containerName)
```

## Unit Tests

No automated tests for log output; acceptance criteria verified by visual inspection and successful `go build`.

## Risks & Open Questions

None — `StopContainer` has a single call site so the signature change is safe.

## Deviations from Plan

None.
