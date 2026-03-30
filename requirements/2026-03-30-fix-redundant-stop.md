# Fix Redundant Container Stop Calls

**Date Added**: 2026-03-30
**Priority**: High
**Status**: Completed

## Problem Statement

After a container is stopped by the inactivity checker, the checker continues calling
`StopContainer` every 2 minutes indefinitely. The REQ-009 fix reset `lastActive` after
each stop to back off from every 30 s to every 2 min, but it did not eliminate the
re-stops entirely.

Root cause: the checker has no knowledge of whether a container is already stopped.
It re-evaluates every tick and sees `activeConns == 0` and `lastActive` old enough →
fires `StopContainer` again.

Observed behaviour:
```
20:48:31  docker: stopping container cf57005d4aa9 (idle timeout)  ← correct
20:48:32  docker: event: container stopped: whoami (still registered)
20:50:46  docker: stopping container cf57005d4aa9 (idle timeout)  ← redundant
20:53:01  docker: stopping container cf57005d4aa9 (idle timeout)  ← redundant
20:55:16  docker: stopping container cf57005d4aa9 (idle timeout)  ← redundant
```

## Functional Requirements

1. Once a container is stopped (by the checker or externally), the checker must not
   call `StopContainer` again until the container has started again.
2. When a container starts (on-demand via `EnsureRunning` or externally), the checker
   resumes normal idle-timeout evaluation.

## Technical Requirements

### 1. Add `running bool` to `targetState`

```go
type targetState struct {
    // ...
    running     bool
}
```

Initialise to `true` for new targets in `RegisterTarget`.
Set to `true` for existing targets in `RegisterTarget` (container restarted).

### 2. Add `ContainerStopped(containerID string)` to `TargetHandler` interface

```go
type TargetHandler interface {
    RegisterTarget(info TargetInfo)
    RemoveTarget(containerID string)
    ContainerStopped(containerID string)
}
```

### 3. Implement `ContainerStopped` on `ProxyServer`

Sets `ts.running = false` for all port mappings belonging to the container.

### 4. Call `handler.ContainerStopped` on "die" events in `WatchEvents`

Replaces the current log-only handler for `"die"`.

### 5. Update `checkInactivity`

- Add `!ts.running` to the idle-guard condition so stopped containers are skipped:
  ```go
  if !ts.running || ts.activeConns.Load() > 0 || time.Since(ts.lastActive) < idleTimeout {
      e.allIdle = false
  }
  ```
- After `StopContainer` succeeds, set `ts.running = false` for all states of that
  container (handles the brief window before the "die" event arrives).
- Remove the `lastActive = time.Now()` reset introduced in REQ-009 — it is no longer
  needed now that `running` prevents re-stops.

## Acceptance Criteria

- [x] After the checker stops a container, no further `StopContainer` calls are made for that container until it starts again.
- [x] If the container is started on-demand (new connection) and then goes idle again, it is stopped once more after 2 minutes.
- [x] If the container is stopped externally (e.g. `docker stop`), the checker also stops issuing stop calls.
- [x] `go build ./...` passes.

## Dependencies

- REQ-008 (Keep Stopped Containers Registered) — extends the "die" event handling.
- REQ-009 (Fix Container Idle Timeout) — removes the `lastActive` reset workaround.
