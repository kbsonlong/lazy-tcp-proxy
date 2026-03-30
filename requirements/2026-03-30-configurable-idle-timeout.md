# Configurable Idle Timeout via IDLE_TIMEOUT_SECS

**Date Added**: 2026-03-30
**Priority**: Medium
**Status**: Completed

## Problem Statement

The idle timeout (how long a container must be inactive before being stopped) is
hard-coded as `idleTimeout = 2 * time.Minute` in `internal/proxy/server.go`. Operators
cannot tune it without recompiling.

## Functional Requirements

1. The idle timeout is controlled by the environment variable `IDLE_TIMEOUT_SECS`.
   Accepted values are positive integers (seconds).
2. If the variable is absent, empty, zero, or non-numeric the proxy falls back to
   **120 seconds** and logs a warning if the value was present but invalid.
3. The resolved timeout is logged at startup.

## Technical Requirements

- `IDLE_TIMEOUT_SECS` is read in `main.go` alongside `POLL_INTERVAL_SECS`, following
  the same pattern as `resolvePollInterval()`.
- A `resolveIdleTimeout() time.Duration` helper is added to `main.go`.
- The `idleTimeout` constant in `server.go` is removed; `RunInactivityChecker` gains
  an `idleTimeout time.Duration` parameter and passes it through to `checkInactivity`.
- `checkInactivity` receives `idleTimeout` as a parameter (or via the server struct)
  and uses it in the `time.Since(ts.lastActive) < idleTimeout` comparison.
- The "idle timer started" log message in `handleConn` (REQ-010) must continue to
  display the actual configured timeout, not a hard-coded value.

### Preferred approach — store on `ProxyServer`

Add `idleTimeout time.Duration` to `ProxyServer`. Set it in `NewServer`. Pass the
value from `main.go` via a new `NewServer(d *docker.Manager, idleTimeout time.Duration)`
signature (or a separate setter). The constant in `server.go` is removed.

## Acceptance Criteria

- [x] `IDLE_TIMEOUT_SECS=60` causes containers to be stopped after 60 seconds of inactivity.
- [x] Omitting `IDLE_TIMEOUT_SECS` defaults to 120 seconds.
- [x] An invalid value (e.g. `IDLE_TIMEOUT_SECS=abc`) falls back to 120 s with a warning log.
- [x] The startup log reflects the configured timeout.
- [x] The "idle timer started" log in `handleConn` shows the correct configured timeout.
- [x] `go build ./...` passes.

## Dependencies

- REQ-001 — modifies `internal/proxy/server.go` and `main.go`.
- REQ-010 — same pattern as `POLL_INTERVAL_SECS`; "idle timer started" log must use the dynamic value.
