# Discovered/Registered Containers Start as Idle

**Date Added**: 2026-04-01
**Priority**: High
**Status**: Completed

## Problem Statement

When lazy-tcp-proxy discovers containers at startup (via `Discover()`) or receives
"create"/"start" Docker events, `RegisterTarget` always sets `running = true` and
`lastActive = time.Now()`. This means:

1. Running containers discovered at startup are given a fresh idle-timeout window,
   so they will not be stopped until at least one full `idleTimeout` *after
   registration* — even if they have never served a connection.
2. Stopped containers discovered at startup are incorrectly marked `running = true`,
   causing the idle checker to call `StopContainer` on an already-stopped container
   every `idleTimeout` window.

## Functional Requirements

1. Every container registered for the first time (new `targetState`) must start with
   `lastActive` at the zero value (`time.Time{}`), making it immediately eligible for
   idle shutdown on the very first inactivity-checker tick.
2. The `running` flag must reflect the container's **actual** running state at the
   time of registration, not always `true`.
3. The update path in `RegisterTarget` (existing port mapping) must also honour the
   actual running state passed in via `TargetInfo`.

## User Experience Requirements

- Containers started alongside the proxy in a docker-compose file should be
  automatically stopped after the configured idle timeout with no manual intervention.

## Technical Requirements

- Add `Running bool` to `docker.TargetInfo`.
- Populate it from `inspect.State.Running` inside `containerToTargetInfo`.
- In `RegisterTarget` (new entry): `running = info.Running`, `lastActive = time.Time{}`.
- In `RegisterTarget` (update path): `existing.running = info.Running`.

## Acceptance Criteria

- [x] A container that is running when first registered is stopped after `idleTimeout`
      with no connections, even if the timeout elapses shortly after startup.
- [x] A container that is stopped when first registered does NOT trigger a
      `StopContainer` call.
- [x] `go build ./...` passes.

## Dependencies

- REQ-001 (Core TCP Proxy) — modifies `internal/proxy/server.go`.
- REQ-009 (Fix Container Idle Timeout) — idle-checker logic.
- REQ-010 (Idle-Timeout Observability) — related observability work.
