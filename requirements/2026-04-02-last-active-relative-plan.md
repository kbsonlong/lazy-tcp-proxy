# Last Active Default & Relative Time Field — Implementation Plan

**Requirement**: [2026-04-02-last-active-relative.md](2026-04-02-last-active-relative.md)
**Date**: 2026-04-02
**Status**: Implemented

## Implementation Steps

1. **`internal/proxy/server.go`** — add `startTime time.Time` field to `ProxyServer` struct.
2. **`internal/proxy/server.go`** — update `NewServer()` to accept a `startTime time.Time` parameter and assign it.
3. **`internal/proxy/server.go`** — add `LastActiveRelative string` field to `TargetSnapshot` with JSON tag `last_active_relative`, placed after `LastActive`.
4. **`internal/proxy/server.go`** — add pure helper function `relativeTime(t, now time.Time) string` (see key snippet below).
5. **`internal/proxy/server.go`** — update `Snapshot()`: when `ts.lastActive.IsZero()`, use `s.startTime` as the effective timestamp; always set `LastActive` (never nil); compute `LastActiveRelative` from the effective timestamp.
6. **`main.go`** — capture `startTime := time.Now()` at the top of `main()`, pass it to `proxy.NewServer(...)`.
7. **`internal/proxy/server_test.go`** — update `newTestServer()` to set a non-zero `startTime` (e.g. `time.Now().Add(-1 * time.Hour)`) so snapshot tests work predictably.
8. **`internal/proxy/server_test.go`** — update `TestSnapshot_NeverActiveMarshalsAsNil` to reflect the new behaviour: `LastActive` is no longer nil for a zero `lastActive`; it should equal the server's `startTime`.
9. **`internal/proxy/server_test.go`** — update `TestSnapshot_Fields` to assert `LastActiveRelative` is a non-empty string.
10. **`internal/proxy/server_test.go`** — add unit tests for `relativeTime` covering each threshold boundary (see test table below).

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `startTime` to `ProxyServer` and `NewServer()`; add `relativeTime` helper; add `LastActiveRelative` to snapshot; update `Snapshot()` logic |
| `lazy-tcp-proxy/main.go` | Modify | Capture `startTime` and pass to `NewServer()` |
| `lazy-tcp-proxy/internal/proxy/server_test.go` | Modify | Update existing snapshot tests; add `relativeTime` unit tests |

## API Contracts

`GET /status` — each entry gains one new field and `last_active` is never null:

```json
{
  "container_id": "abc123def456",
  "container_name": "my-service",
  "listen_port": 3000,
  "target_port": 3000,
  "running": false,
  "active_conns": 0,
  "last_active": "2026-04-02T08:00:00Z",
  "last_active_relative": "8 hours ago"
}
```

## Key Code Snippets

### `relativeTime` helper

```go
func relativeTime(t, now time.Time) string {
    d := now.Sub(t)
    switch {
    case d >= 365*24*time.Hour:
        return fmt.Sprintf("%d years ago", int(d.Hours()/24/365))
    case d >= 30*24*time.Hour:
        return fmt.Sprintf("%d months ago", int(d.Hours()/24/30))
    case d >= 24*time.Hour:
        return fmt.Sprintf("%d days ago", int(d.Hours()/24))
    case d >= time.Hour:
        return fmt.Sprintf("%d hours ago", int(d.Hours()))
    case d >= time.Minute:
        return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
    default:
        return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
    }
}
```

### Updated `Snapshot()` logic (changed lines only)

```go
effectiveLastActive := ts.lastActive
if effectiveLastActive.IsZero() {
    effectiveLastActive = s.startTime
}
t := effectiveLastActive
la = &t
// ...
LastActive:          la,
LastActiveRelative:  relativeTime(effectiveLastActive, time.Now()),
```

### Updated `NewServer()` signature

```go
func NewServer(ctx context.Context, d *docker.Manager, startTime time.Time, idleTimeout, pollInterval time.Duration) *ProxyServer {
    return &ProxyServer{
        // ...
        startTime: startTime,
    }
}
```

### Updated `main.go` call

```go
startTime := time.Now()
// ...
srv := proxy.NewServer(ctx, mgr, startTime, idleTimeout, tick)
```

## Unit Tests

| Test | Input (t relative to now) | Expected Output |
|------|--------------------------|-----------------|
| `TestRelativeTime_Seconds` | 10 seconds ago | `"10 seconds ago"` |
| `TestRelativeTime_Minutes` | 4 minutes ago | `"4 minutes ago"` |
| `TestRelativeTime_Hours` | 8 hours ago | `"8 hours ago"` |
| `TestRelativeTime_Days` | 3 days ago | `"3 days ago"` |
| `TestRelativeTime_Months` | 45 days ago | `"1 months ago"` |
| `TestRelativeTime_Years` | 400 days ago | `"1 years ago"` |
| `TestRelativeTime_MultiYear` | 800 days ago | `"2 years ago"` |
| `TestRelativeTime_BoundaryMinute` | exactly 60 seconds ago | `"1 minutes ago"` |
| `TestRelativeTime_BoundaryHour` | exactly 60 minutes ago | `"1 hours ago"` |

## Risks & Open Questions

- `TestSnapshot_NeverActiveMarshalsAsNil` directly asserts `nil` — must be updated to assert the startTime fallback instead.
- Month calculation uses fixed 30-day approximation (per requirement); no calendar arithmetic needed.
- `NewServer()` signature change affects only one call site (`main.go`) — no other callers exist.
