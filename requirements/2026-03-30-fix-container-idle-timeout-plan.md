# Fix Container Idle Timeout — Implementation Plan

**Requirement**: [2026-03-30-fix-container-idle-timeout.md](2026-03-30-fix-container-idle-timeout.md)
**Date**: 2026-03-30
**Status**: Implemented

## Implementation Steps

All changes are in `lazy-tcp-proxy/internal/proxy/server.go`.

### Step 1 — Fix `handleConn`: move `activeConns` tracking to cover the full connection lifecycle

Currently `activeConns.Add(1)` only fires after `EnsureRunning` + the upstream dial succeed
(potentially 30+ seconds after the connection was accepted).  During that window the
inactivity checker sees `activeConns == 0` and can stop the container.

**Change**: remove the existing block:
```go
ts.activeConns.Add(1)
defer func() {
    ts.activeConns.Add(-1)
    ts.lastActive = time.Now()
}()
```
and replace it with **two separate** declarations at the top of `handleConn` (immediately
after `defer conn.Close()`):

```go
// Track this connection immediately so the inactivity checker does not stop
// the container while we are starting it or dialling upstream.
ts.activeConns.Add(1)
defer ts.activeConns.Add(-1)
```

Then, **after** `defer upstream.Close()` (i.e., once we know the upstream connection is
established), add a separate defer that updates `lastActive`:

```go
defer func() { ts.lastActive = time.Now() }()
```

This keeps `lastActive` as "time of last successful proxy activity" while ensuring
`activeConns` guards the full setup + teardown window.

### Step 2 — Fix `checkInactivity`: reset `lastActive` after stopping a container

Currently `lastActive` is not updated when the checker stops a container, so the very
next 30-second tick still sees `allIdle = true` and calls `StopContainer` again.
Combined with Step 1's fix this is mostly harmless, but it still produces unnecessary
Docker API calls every 30 seconds for every stopped-but-registered container.

**Change**: extend the local `entry` struct inside `checkInactivity` to carry a slice of
`*targetState` pointers:

```go
type entry struct {
    containerID string
    name        string
    allIdle     bool
    states      []*targetState
}
```

Append each `ts` to `e.states` as the loop builds the map:

```go
e.states = append(e.states, ts)
```

After calling `StopContainer` successfully (nil error), reset `lastActive` on all
associated port mappings:

```go
if err := s.docker.StopContainer(ctx, e.containerID); err != nil {
    log.Printf("proxy: inactivity: error stopping %s: %v", e.name, err)
} else {
    now := time.Now()
    for _, ts := range e.states {
        ts.lastActive = now
    }
}
```

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Steps 1 and 2 |
| `requirements/2026-03-30-fix-container-idle-timeout.md` | Modify | Status → In Progress → Completed |
| `requirements/_index.md` | Modify | Status → In Progress → Completed |

## Key Code Snippets

### `handleConn` — before (current)

```go
// … after upstream dial succeeds …
defer upstream.Close()

ts.activeConns.Add(1)
defer func() {
    ts.activeConns.Add(-1)
    ts.lastActive = time.Now()
}()
```

### `handleConn` — after

```go
// Immediately after `defer conn.Close()`, at the top of the function:
ts.activeConns.Add(1)
defer ts.activeConns.Add(-1)

// … EnsureRunning, dial retry loop …

// After `defer upstream.Close()`:
defer func() { ts.lastActive = time.Now() }()
```

### `checkInactivity` entry struct — after

```go
type entry struct {
    containerID string
    name        string
    allIdle     bool
    states      []*targetState
}
// …
e.states = append(e.states, ts)
// …
if err := s.docker.StopContainer(ctx, e.containerID); err != nil {
    log.Printf("proxy: inactivity: error stopping %s: %v", e.name, err)
} else {
    now := time.Now()
    for _, ts := range e.states {
        ts.lastActive = now
    }
}
```

## Unit Tests

| Test | Scenario | Expected |
|------|----------|----------|
| Idle stop | Container registered, no connections, wait > 2 min | `StopContainer` called once |
| No double-stop | After stop, checker fires again within 2 min | `StopContainer` NOT called a second time |
| Active conn blocks stop | Checker fires while `activeConns > 0` | `StopContainer` not called |
| Setup window safe | Checker fires between connection accept and upstream dial | `StopContainer` not called (`activeConns == 1` from start of `handleConn`) |

## Risks & Open Questions

- `lastActive` is written from `handleConn` goroutines and `checkInactivity` without
  a mutex (pre-existing issue, not introduced here). The Go memory model does not
  guarantee visibility, but in practice the race only affects precision of the idle
  timer, not correctness. A future cleanup could wrap it in a mutex or use
  `sync/atomic` via a `UnixNano` int64. Not in scope for this fix.
