# Idle-Timeout Observability & Poll Interval — Implementation Plan

**Requirement**: [2026-03-30-idle-timeout-observability.md](2026-03-30-idle-timeout-observability.md)
**Date**: 2026-03-30
**Status**: Implemented

## Implementation Steps

### Step 1 — `internal/proxy/server.go`: convert `defer ts.activeConns.Add(-1)` to a logging func defer

Current code (from REQ-009):
```go
ts.activeConns.Add(1)
defer ts.activeConns.Add(-1)
```

Replace the plain-expression defer with a func that logs when the count reaches zero:
```go
ts.activeConns.Add(1)
defer func() {
    if ts.activeConns.Add(-1) == 0 {
        log.Printf("proxy: last connection to %s closed; idle timer started (container will stop in ~%s if no new connections)",
            ts.info.ContainerName, idleTimeout)
    }
}()
```

### Step 2 — `internal/proxy/server.go`: remove `inactivityTick` constant; add `tick` parameter to `RunInactivityChecker`

Remove:
```go
inactivityTick = 30 * time.Second
```

Change signature:
```go
func (s *ProxyServer) RunInactivityChecker(ctx context.Context, tick time.Duration) {
    ticker := time.NewTicker(tick)
    ...
```

### Step 3 — `main.go`: read `POLL_INTERVAL_SECS`, resolve tick, log it, pass to `RunInactivityChecker`

```go
const defaultPollInterval = 15 * time.Second

func resolvePollInterval() time.Duration {
    raw := os.Getenv("POLL_INTERVAL_SECS")
    if raw == "" {
        return defaultPollInterval
    }
    n, err := strconv.Atoi(raw)
    if err != nil || n <= 0 {
        log.Printf("POLL_INTERVAL_SECS=%q is invalid; using default %s", raw, defaultPollInterval)
        return defaultPollInterval
    }
    return time.Duration(n) * time.Second
}
```

In `main()`, after creating the server:
```go
tick := resolvePollInterval()
log.Printf("inactivity check interval: %s (set POLL_INTERVAL_SECS to override)", tick)
// ...
go func() {
    srv.RunInactivityChecker(ctx, tick)
}()
```

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Steps 1–2 |
| `lazy-tcp-proxy/main.go` | Modify | Step 3 |
| `requirements/2026-03-30-idle-timeout-observability.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## Key Code Snippets

See Implementation Steps above — all snippets are complete and directly usable.

## Unit Tests

| Test | Input | Expected |
|------|-------|----------|
| Log fires on last close | 1 connection opens then closes | "idle timer started" log emitted |
| Log silent with concurrent conns | 2 connections, one closes | no log emitted |
| Default poll interval | `POLL_INTERVAL_SECS` unset | 15 s tick |
| Custom poll interval | `POLL_INTERVAL_SECS=10` | 10 s tick |
| Invalid poll interval | `POLL_INTERVAL_SECS=abc` | 15 s tick + warning log |

## Risks & Open Questions

- None. Two files, small isolated changes.
