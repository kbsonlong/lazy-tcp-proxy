# Per-Container Idle Timeout Label Override

**Date Added**: 2026-04-03
**Priority**: Medium
**Status**: Completed

## Problem Statement

The idle-shutdown period is currently a single global value (`IDLE_TIMEOUT_SECS` env var,
default 120 s). Operators cannot tune it per container — e.g. a dev database may need a
short timeout while a stateful service needs a longer one. Additionally, the global timeout
deliberately rejects `0` as invalid; operators may want an "immediate shutdown" mode where
a container is stopped as soon as its last connection closes.

## Functional Requirements

1. A container may declare the label `lazy-tcp-proxy.idle-timeout-secs` with a non-negative
   integer value (seconds). This overrides `IDLE_TIMEOUT_SECS` for that container only.
2. If the label value is absent, empty, or non-numeric the global timeout is used (no
   warning needed — absence is normal). If the value is present but invalid (e.g. negative
   or non-integer) a warning is logged and the global timeout is used.
3. A value of `0` — whether from the label or from `IDLE_TIMEOUT_SECS` — means
   **"stop the container immediately once its last connection closes"** (i.e. the inactivity
   checker stops it on the next poll tick after activeConns reaches 0).
4. `IDLE_TIMEOUT_SECS=0` is now a valid global setting (previously treated as invalid/
   defaulting to 120 s). The startup log must clearly indicate the "immediate shutdown"
   behaviour when `0` is configured.
5. The "idle timer started" log in `handleConn` must reflect the effective (per-container
   or global) timeout value, and must say "container will stop immediately if no new
   connections" when the effective timeout is `0`.

## User Experience Requirements

- Container authors set the label once in their `docker-compose.yml` / Dockerfile; no
  proxy restart is needed because labels are read on container registration.
- Operators can still set `IDLE_TIMEOUT_SECS` as the global default; the label only
  overrides for containers that declare it.

## Technical Requirements

- `docker.TargetInfo` gains `IdleTimeout *time.Duration`. A `nil` pointer means
  "use global default"; a non-nil pointer (including `0`) means "use this value".
- `manager.containerToTargetInfo()` parses `lazy-tcp-proxy.idle-timeout-secs`. Valid values
  are non-negative integers (≥ 0). Invalid/absent values leave `IdleTimeout` as `nil`.
- `proxy.targetState` and `proxy.udpListenerState` each gain an `idleTimeout *time.Duration`
  field copied from `TargetInfo.IdleTimeout` at registration time.
- `proxy.checkInactivity()` resolves the effective timeout per container:
  `effectiveTimeout(ts.idleTimeout, s.idleTimeout)` — returns `*ts.idleTimeout` if
  non-nil, else `s.idleTimeout`.
- `main.go` `resolveIdleTimeout()` must accept `0` as a valid value (drop the existing
  "zero → default" fallback). Update the docstring / log accordingly.
- No changes to `RunInactivityChecker` signature or poll interval logic.

## Acceptance Criteria

- [x] A container with `lazy-tcp-proxy.idle-timeout-secs=30` is stopped after ~30 s of
      inactivity regardless of the global `IDLE_TIMEOUT_SECS`.
- [x] A container without the label uses the global `IDLE_TIMEOUT_SECS`.
- [x] A container with `lazy-tcp-proxy.idle-timeout-secs=0` is stopped on the next poll
      tick after its last connection closes (immediate shutdown).
- [x] `IDLE_TIMEOUT_SECS=0` makes all containers (without a label override) use immediate
      shutdown; the startup log reflects this.
- [x] An invalid label value (e.g. `lazy-tcp-proxy.idle-timeout-secs=abc` or `-5`) logs a
      warning and falls back to the global timeout.
- [x] The "idle timer started" log shows the correct effective timeout; it says "immediately"
      when the effective timeout is `0`.
- [x] `README.md` Container Label Configuration table includes `lazy-tcp-proxy.idle-timeout-secs`; `IDLE_TIMEOUT_SECS` description notes `0` = immediate shutdown.
- [x] `go build ./...` and `go test ./...` pass.

## Dependencies

- REQ-013 — extends `IDLE_TIMEOUT_SECS` handling; changes `resolveIdleTimeout()` semantics
  for `0`.
- REQ-001, REQ-007 — modifies `docker.TargetInfo`, `targetState`, and `checkInactivity`.

## Implementation Notes

The `checkInactivity` idle condition already works correctly for `timeout == 0`:
`time.Since(lastActive) < 0` is always `false` (durations are non-negative), so no changes
to the comparison logic are needed — only the value being compared changes.
