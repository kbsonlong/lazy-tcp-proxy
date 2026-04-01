# Handle Port Conflicts Between Containers

**Date Added**: 2026-04-01
**Priority**: High
**Status**: Completed

## Problem Statement

When two or more containers are registered (via Docker events or discovery) with the same listen port, the second container silently overwrites the first in the proxy's target map. This means whichever container registered last wins, with no visibility into the conflict. The correct behaviour is to reject the conflicting registration and surface the error clearly.

## Functional Requirements

1. When `RegisterTarget` is called for a container that includes a listen port already registered by a **different** container, the entire registration for the new container must be rejected (no ports from that container are registered).
2. A red-text error must be logged identifying the conflicting port, the existing container name, and the new container name.
3. The conflict check applies to both the `Discover` path (initial scan) and the `WatchEvents` path (live Docker events).
4. If the same container re-registers its own ports (e.g. on a restart event), the existing update logic is retained (no conflict).

## User Experience Requirements

- The error message must be visually distinct (red ANSI text) so it stands out in log output.
- The message should include enough context to diagnose the conflict: conflicting port number, existing container name, incoming container name.

## Technical Requirements

- The conflict detection must be atomic with respect to the mutex already protecting `s.targets` in `ProxyServer.RegisterTarget`.
- No partial registration: if port A and port B are declared by the new container and only port A conflicts, the whole container registration is still rejected.
- No changes to the `TargetHandler` interface are needed; the check lives entirely inside `RegisterTarget` in `proxy/server.go`.

## Acceptance Criteria

- [x] When a second container declares a listen port already held by a different container, `RegisterTarget` logs a red error and returns without modifying `s.targets`.
- [x] The error log contains the conflicting port number and both container names.
- [x] When the same container re-registers (same `ContainerID`), no conflict error is raised and the target is updated as before.
- [x] The `Discover` path and `WatchEvents` path both pass through `RegisterTarget`, so both are covered automatically.

## Dependencies

- REQ-001 (Core TCP Proxy) — the `RegisterTarget` method being modified lives here.
- REQ-007 (Multi-Port Mappings) — containers may declare multiple ports; all must be checked.

## Implementation Notes

- Conflict check: before creating any listener, iterate all declared ports and check `s.targets[port]`; if any entry exists whose `ContainerID` differs from the incoming `info.ContainerID`, log the error and return immediately.
- ANSI red escape: `\033[31m … \033[0m` (consistent with existing colour usage in the codebase).
