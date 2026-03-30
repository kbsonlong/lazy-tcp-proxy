# Improved Idle-Timeout Observability and Configurable Poll Interval

**Date Added**: 2026-03-30
**Priority**: Medium
**Status**: Planned

## Problem Statement

Two related gaps in the current proxy:

1. **No log entry when the idle timer starts.** When the last active connection to a container closes, `activeConns` drops to zero and the 2-minute idle countdown begins — but nothing is logged. Operators cannot tell from logs when the countdown started, making it hard to understand whether a late (or missed) shutdown is due to the idle timeout not triggering or a connection staying open longer than expected.

2. **Poll interval is a hard-coded constant (30 s).** The inactivity checker tick cannot be tuned without recompiling. A shorter default (15 s) and an environment variable would let operators trade off CPU/API overhead against shutdown responsiveness.

## Functional Requirements

1. When `activeConns` for a container drops to zero (i.e. the last in-flight connection — including setup attempts — has finished), log a message that makes clear the idle countdown has started and states the timeout duration. Example:
   ```
   proxy: last connection to whoami closed; idle timer started (container will stop in ~2m0s if no new connections)
   ```
2. The inactivity-checker poll interval is controlled by the environment variable `POLL_INTERVAL_SECS`. Accepted values are positive integers (seconds). If the variable is absent, empty, zero, or non-numeric, the proxy falls back to **15 seconds** and logs a warning if the value was present but invalid.
3. The resolved poll interval is logged at startup.

## Technical Requirements

- `POLL_INTERVAL_SECS` is read in `main.go` before starting the inactivity-checker goroutine.
- `RunInactivityChecker` gains a `tick time.Duration` parameter; `main.go` passes the resolved value. The `inactivityTick` constant is removed.
- The "idle timer started" log is emitted inside `handleConn` when `ts.activeConns.Add(-1)` returns 0. The existing `defer ts.activeConns.Add(-1)` (plain expression defer) is converted to a `defer func()` so the return value can be inspected.

## Acceptance Criteria

- [ ] Log line appears when the last connection to a container closes; it includes the container name and the idle-timeout duration.
- [ ] No log line appears when a connection closes but others remain active for the same container.
- [ ] `POLL_INTERVAL_SECS=10` causes the checker to tick every 10 s (verifiable via logs).
- [ ] Omitting `POLL_INTERVAL_SECS` causes the checker to tick every 15 s.
- [ ] An invalid value (e.g. `POLL_INTERVAL_SECS=abc`) falls back to 15 s and logs a warning.
- [ ] `go build ./...` passes.

## Dependencies

- REQ-009 — modifies `internal/proxy/server.go` and `main.go`.
