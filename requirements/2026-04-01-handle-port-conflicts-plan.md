# Handle Port Conflicts Between Containers — Implementation Plan

**Requirement**: [2026-04-01-handle-port-conflicts.md](2026-04-01-handle-port-conflicts.md)
**Date**: 2026-04-01
**Status**: Implemented

## Implementation Steps

1. **Open `lazy-tcp-proxy/internal/proxy/server.go`** — this is the only file that needs changing.
2. **In `RegisterTarget`, before the existing loop that creates/updates listeners**, add a pre-flight conflict check:
   - Iterate over `info.Ports`.
   - For each `m.ListenPort`, check `s.targets[m.ListenPort]`.
   - If an entry exists **and** its `ContainerID` differs from `info.ContainerID`, log a red error message and `return` immediately (no ports are registered for this container).
3. **Leave the existing update path untouched** — the check only fires when `ContainerID` differs, so same-container re-registration continues to update as before.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add port-conflict pre-flight check at the top of `RegisterTarget` |

## Key Code Snippets

```go
// RegisterTarget adds or updates a target. One listener is created per port mapping.
func (s *ProxyServer) RegisterTarget(info docker.TargetInfo) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Pre-flight: reject the entire registration if any declared port is already
    // held by a different container.
    for _, m := range info.Ports {
        if existing, ok := s.targets[m.ListenPort]; ok && existing.info.ContainerID != info.ContainerID {
            log.Printf("\033[31mproxy: port conflict on port %d: already registered by \033[33m%s\033[31m, ignoring \033[33m%s\033[31m\033[0m",
                m.ListenPort, existing.info.ContainerName, info.ContainerName)
            return
        }
    }

    // ... existing loop unchanged ...
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Conflict: different container, same port | Register container A on port 8080, then register container B on port 8080 | B is not added to `targets`; red log emitted |
| No conflict: same container re-registers | Register container A on port 8080 twice | Target updated, no error log |
| No conflict: different containers, different ports | Register A on 8080, B on 9090 | Both registered successfully |
| Partial conflict: new container has two ports, one conflicts | Register A on 8080; register B on ports 8080 and 9090 | B is fully rejected (neither port registered) |

## Risks & Open Questions

- None. The change is small, entirely within the existing mutex, and does not affect any other paths.
