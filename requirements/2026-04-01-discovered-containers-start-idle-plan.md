# Discovered/Registered Containers Start as Idle — Implementation Plan

**Requirement**: [2026-04-01-discovered-containers-start-idle.md](2026-04-01-discovered-containers-start-idle.md)
**Date**: 2026-04-01
**Status**: Implemented

## Implementation Steps

1. **`internal/docker/manager.go` — add `Running` field to `TargetInfo`**
   Add `Running bool` to the `TargetInfo` struct so callers can know whether
   the container was actually running at the time the info was built.

2. **`internal/docker/manager.go` — populate `Running` in `containerToTargetInfo`**
   After the existing `inspect` call, set `Running: inspect.State.Running` in the
   returned `TargetInfo`.

3. **`internal/proxy/server.go` — fix new-entry path in `RegisterTarget`**
   Change the new `targetState` literal from:
   ```go
   lastActive: time.Now(),
   running:    true,
   ```
   to:
   ```go
   lastActive: time.Time{},   // zero — immediately idle
   running:    info.Running,  // reflect actual container state
   ```

4. **`internal/proxy/server.go` — fix update path in `RegisterTarget`**
   Change:
   ```go
   existing.running = true
   ```
   to:
   ```go
   existing.running = info.Running
   ```

5. **Verify build** — run `go build ./...` from the module root.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Add `Running bool` to `TargetInfo`; set it from `inspect.State.Running` in `containerToTargetInfo` |
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | New-entry: `lastActive = time.Time{}`, `running = info.Running`; update-path: `running = info.Running` |

## Key Code Snippets

### `TargetInfo` (manager.go)
```go
type TargetInfo struct {
    ContainerID   string
    ContainerName string
    Ports         []PortMapping
    NetworkIDs    []string
    AllowList     []net.IPNet
    BlockList     []net.IPNet
    Running       bool   // true if the container was running at discovery time
}
```

### `containerToTargetInfo` return (manager.go)
```go
return TargetInfo{
    ContainerID:   containerID,
    ContainerName: name,
    Ports:         ports,
    NetworkIDs:    networkIDs,
    AllowList:     allowList,
    BlockList:     blockList,
    Running:       inspect.State.Running,
}, nil
```

### New `targetState` (server.go)
```go
ts := &targetState{
    info:       info,
    targetPort: m.TargetPort,
    listener:   ln,
    lastActive: time.Time{},   // zero — idle from the start
    running:    info.Running,
}
```

### Update path (server.go)
```go
existing.info = info
existing.targetPort = m.TargetPort
existing.running = info.Running   // was: true
existing.removed = false
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Running container registered | `info.Running = true`, no connections | Stopped after first idle check ≥ idleTimeout |
| Stopped container registered | `info.Running = false`, no connections | `StopContainer` never called; `running` stays false |
| Build check | `go build ./...` | Exits 0 |

## Risks & Open Questions

- **Update path for "start" events**: when a "start" Docker event fires and the
  container is genuinely running, `info.Running` will be `true` (confirmed by
  `containerToTargetInfo` inspecting the live state), so `existing.running = true`
  is still set correctly.
- **"create" events**: container just created, not yet running → `info.Running = false`
  → `existing.running = false`. Correct; idle checker will leave it alone until it
  starts.
- No API surface changes; `TargetInfo` is package-internal to the binary.
