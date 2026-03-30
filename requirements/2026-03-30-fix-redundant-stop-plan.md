# Fix Redundant Container Stop Calls — Implementation Plan

**Requirement**: [2026-03-30-fix-redundant-stop.md](2026-03-30-fix-redundant-stop.md)
**Date**: 2026-03-30
**Status**: Approved

## Implementation Steps

### Step 1 — `internal/proxy/server.go`: add `running bool` to `targetState`

```go
type targetState struct {
    info        docker.TargetInfo
    targetPort  int
    listener    net.Listener
    lastActive  time.Time
    activeConns atomic.Int32
    running     bool   // ← add
    removed     bool
}
```

### Step 2 — `internal/proxy/server.go`: initialise and maintain `running` in `RegisterTarget`

For **new** targets, set `running: true` in the struct literal:
```go
ts := &targetState{
    info:       info,
    targetPort: m.TargetPort,
    listener:   ln,
    lastActive: time.Now(),
    running:    true,   // ← add
}
```

For **existing** targets (container restarted), set `existing.running = true`:
```go
if existing, ok := s.targets[m.ListenPort]; ok {
    existing.info = info
    existing.targetPort = m.TargetPort
    existing.removed = false
    existing.running = true   // ← add
    // ...
}
```

### Step 3 — `internal/proxy/server.go`: add `ContainerStopped` method

```go
// ContainerStopped marks all port mappings for the given container as stopped
// so the inactivity checker does not issue further stop calls.
func (s *ProxyServer) ContainerStopped(containerID string) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, ts := range s.targets {
        if ts.info.ContainerID == containerID {
            ts.running = false
        }
    }
}
```

### Step 4 — `internal/proxy/server.go`: update `checkInactivity`

a) Add `!ts.running` to the idle-guard so stopped containers are skipped:
```go
if !ts.running || ts.activeConns.Load() > 0 || time.Since(ts.lastActive) < idleTimeout {
    e.allIdle = false
}
```

b) After `StopContainer` succeeds, mark all states as not running (handles the window
before the "die" event arrives) and **remove** the `lastActive` reset (no longer needed):
```go
if err := s.docker.StopContainer(ctx, e.containerID); err != nil {
    log.Printf("proxy: inactivity: error stopping %s: %v", e.name, err)
} else {
    for _, ts := range e.states {
        ts.running = false
    }
}
```

### Step 5 — `internal/docker/manager.go`: add `ContainerStopped` to `TargetHandler`

```go
type TargetHandler interface {
    RegisterTarget(info TargetInfo)
    RemoveTarget(containerID string)
    ContainerStopped(containerID string)   // ← add
}
```

### Step 6 — `internal/docker/manager.go`: call `ContainerStopped` on "die" events

Replace the current log-only "die" case:
```go
case "die":
    name := msg.Actor.Attributes["name"]
    log.Printf("docker: event: container stopped: %s (still registered)", name)
    handler.ContainerStopped(msg.Actor.ID)   // ← add
```

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Steps 1–4 |
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Steps 5–6 |
| `requirements/2026-03-30-fix-redundant-stop.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## Risks & Open Questions

- `ts.running` is read in `checkInactivity` (after snapshot, no lock) and written in
  `ContainerStopped` and `RegisterTarget` (under `s.mu.RLock`/`Lock`). This is the
  same pre-existing data-race pattern as `ts.lastActive`. In the worst case a single
  checker tick fires a redundant stop, which is idempotent. Acceptable for now.
