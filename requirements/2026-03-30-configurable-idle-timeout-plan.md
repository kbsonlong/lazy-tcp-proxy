# Configurable Idle Timeout — Implementation Plan

**Requirement**: [2026-03-30-configurable-idle-timeout.md](2026-03-30-configurable-idle-timeout.md)
**Date**: 2026-03-30
**Status**: Implemented

## Implementation Steps

### Step 1 — `internal/proxy/server.go`: remove `idleTimeout` constant; add field to `ProxyServer`

Remove:
```go
const (
    dialRetries  = 30
    dialInterval = time.Second
    idleTimeout  = 2 * time.Minute
)
```

Replace with (no `idleTimeout` constant):
```go
const (
    dialRetries  = 30
    dialInterval = time.Second
)
```

Add `idleTimeout time.Duration` field to `ProxyServer`:
```go
type ProxyServer struct {
    docker      *docker.Manager
    mu          sync.RWMutex
    targets     map[int]*targetState
    idleTimeout time.Duration
}
```

### Step 2 — `internal/proxy/server.go`: update `NewServer` to accept `idleTimeout`

```go
func NewServer(d *docker.Manager, idleTimeout time.Duration) *ProxyServer {
    return &ProxyServer{
        docker:      d,
        targets:     make(map[int]*targetState),
        idleTimeout: idleTimeout,
    }
}
```

### Step 3 — `internal/proxy/server.go`: replace `idleTimeout` constant references with `s.idleTimeout`

Two locations:

a) `checkInactivity` idle-guard condition:
```go
if !ts.running || ts.activeConns.Load() > 0 || time.Since(ts.lastActive) < s.idleTimeout {
```

b) `handleConn` "idle timer started" log:
```go
log.Printf("proxy: last connection to %s closed; idle timer started (container will stop in ~%s if no new connections)",
    ts.info.ContainerName, s.idleTimeout)
```

### Step 4 — `main.go`: add `resolveIdleTimeout()` and pass value to `NewServer`

```go
const defaultIdleTimeout = 120 * time.Second

func resolveIdleTimeout() time.Duration {
    raw := os.Getenv("IDLE_TIMEOUT_SECS")
    if raw == "" {
        return defaultIdleTimeout
    }
    n, err := strconv.Atoi(raw)
    if err != nil || n <= 0 {
        log.Printf("IDLE_TIMEOUT_SECS=%q is invalid; using default %s", raw, defaultIdleTimeout)
        return defaultIdleTimeout
    }
    return time.Duration(n) * time.Second
}
```

In `main()`, resolve and log before creating the server:
```go
idleTimeout := resolveIdleTimeout()
log.Printf("idle timeout: %s (set IDLE_TIMEOUT_SECS to override)", idleTimeout)

srv := proxy.NewServer(mgr, idleTimeout)
```

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Steps 1–3 |
| `lazy-tcp-proxy/main.go` | Modify | Step 4 |
| `requirements/2026-03-30-configurable-idle-timeout.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## Risks & Open Questions

None. Identical pattern to `POLL_INTERVAL_SECS` / `resolvePollInterval()`.
