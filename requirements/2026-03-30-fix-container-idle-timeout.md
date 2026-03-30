# Fix Container Idle Timeout — Containers Not Stopping After 2 Minutes

**Date Added**: 2026-03-30
**Priority**: High
**Status**: Completed

## Problem Statement

Since REQ-008 (keep stopped containers registered), containers no longer reliably stop after 2 minutes of idle time. Two bugs were introduced:

1. **Race condition between connection setup and the inactivity checker**: `activeConns` is only incremented *after* the upstream dial succeeds (inside `handleConn`). The window between accepting a connection and successfully dialling the upstream (which includes `EnsureRunning` plus up to 30 retry seconds) leaves `activeConns == 0`. If the inactivity checker fires during this window it sees the container as idle, stops it, and causes the in-flight connection to fail.

2. **Redundant stop calls every 30 seconds**: After REQ-008 a container stays in the `targets` map after a "die" event. Because `lastActive` is never updated when the checker stops a container, every subsequent 30-second tick still sees `time.Since(lastActive) >= idleTimeout`, sets `allIdle = true`, and calls `StopContainer` again. Combined with bug #1, this means every 30-second window is an opportunity to kill a container that was just started by a new incoming connection.

## Functional Requirements

1. The inactivity checker must not stop a container while any connection to it is in any stage of setup or active proxying.
2. After the checker stops a container, it must not attempt to stop it again for at least 2 minutes (to allow time for the container to remain stopped or for a new connection to stabilise before re-evaluation).
3. Containers with no active connections and no activity for 2 minutes must still be stopped.

## Technical Requirements

### Fix 1 — move `activeConns` tracking to cover the full connection lifecycle

In `handleConn` in `internal/proxy/server.go`:
- Move `ts.activeConns.Add(1)` to the very start of the function (immediately after `defer conn.Close()`).
- Add a corresponding `defer ts.activeConns.Add(-1)` directly below it, so the counter is decremented even if the function returns early (failed `EnsureRunning`, exhausted dial retries, etc.).
- Keep `ts.lastActive = time.Now()` in a *separate* defer that is only registered after the upstream connection is successfully established, so `lastActive` continues to reflect actual successful proxy activity.

### Fix 2 — reset `lastActive` after stopping a container

In `checkInactivity` in `internal/proxy/server.go`:
- Extend the `entry` struct used for the per-container grouping to also hold a slice of the associated `*targetState` pointers.
- After a successful `StopContainer` call, update `ts.lastActive = time.Now()` for every port mapping of that container.  This resets the 2-minute idle clock, preventing the checker from firing `StopContainer` again for at least 2 minutes.

## Acceptance Criteria

- [x] A container with no connections is stopped approximately 2 minutes after the last connection closes (or after initial registration if no connections are ever made).
- [x] Connecting to a proxy port for a stopped container successfully starts the container and proxies the connection, even if the connection arrives immediately after the idle timeout has fired.
- [x] The checker does not call `StopContainer` more than once per 2-minute window for the same container.
- [x] `go build ./...` passes.

## Dependencies

- REQ-001 (Core TCP Proxy) — modifies `internal/proxy/server.go`.
- REQ-007 (Multi-Port Mappings) — idle checker grouping logic is extended.
- REQ-008 (Keep Stopped Containers Registered) — root cause of the regression.
