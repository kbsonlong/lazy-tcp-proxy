# Docker Dependency Cascade

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: Planned

## Problem Statement

When a managed "hub" container (e.g. `selenium-hub`) starts or stops due to
lazy-tcp-proxy's idle lifecycle management, its downstream dependents
(e.g. `chromium`, `firefox`) are not automatically started or stopped.  This
leaves orphaned running nodes when the hub is idle, and unready nodes when the
hub is first woken by traffic.

## Functional Requirements

1. **Dependency discovery** — When a managed container is registered, read its
   `com.docker.compose.depends_on` label and record which upstream containers
   it depends on.
2. **Reverse-map** — Build and maintain a map of
   `upstreamContainerName → []downstreamContainerID` from all registered
   containers.
3. **Start cascade** — When a managed upstream container starts (Docker `start`
   event), immediately start all registered downstream containers that declare
   a dependency on it.
4. **Stop cascade** — When a managed upstream container stops (Docker `die`
   event or idle-timeout), immediately stop all registered downstream
   containers that declare a dependency on it.
5. **Managed-only** — Cascade only applies when the upstream container is
   itself a managed container (has `lazy-tcp-proxy.enabled=true`).  Events
   for unmanaged containers are ignored.
6. **No-op on correct state** — Starting an already-running dependent or
   stopping an already-stopped dependent is a silent no-op (Docker handles
   this gracefully; existing `EnsureRunning` / `StopContainer` behaviour is
   preserved).
7. **Hub is a full managed container** — The upstream hub container is proxied
   exactly like any other managed container, including idle-timeout stop.
   Idle stop of the hub cascades a stop to all dependents.

## User Experience Requirements

- **Zero extra configuration** — the `depends_on` relationship already exists
  in `docker-compose.yml` and is reflected in the
  `com.docker.compose.depends_on` container label.  No new labels are needed.
- Log lines should clearly indicate cascade actions, naming both the trigger
  container and the downstream container being started/stopped.

## Technical Requirements

- Read `com.docker.compose.depends_on` label during `containerToTargetInfo`.
  Label format: `svc1:condition:required[,svc2:condition:required]`
  — only the service name (first field) is needed.
- The reverse dependency map lives in the proxy `Server` (or a new
  `DependencyMap` type), protected by a mutex.
- Cascade start is triggered from the `ContainerStarted` notification path
  (after `EnsureRunning` succeeds) and from the Docker `start` event in
  `WatchEvents`.
- Cascade stop is triggered from the `StopContainer` call path (after stop
  succeeds) and from the Docker `die` event in `WatchEvents`.
- Cascade is **not** recursive in v1 (no transitive chains).

## Acceptance Criteria

- [ ] Registering a container with `com.docker.compose.depends_on=hub:service_started:false`
      populates the reverse map entry `hub → [container]`.
- [ ] When the hub container starts, all downstream containers are started.
- [ ] When the hub container stops (idle or manual), all downstream containers
      are stopped.
- [ ] If a downstream container is already running when a start cascade fires,
      no error is logged.
- [ ] If a downstream container is already stopped when a stop cascade fires,
      no error is logged.
- [ ] An unmanaged container starting/stopping does **not** trigger any cascade.
- [ ] Log lines for cascade actions include the upstream container name and the
      downstream container name.

## Dependencies

- Existing `TargetInfo`, `Manager.EnsureRunning`, `Manager.StopContainer`,
  `TargetHandler`, and `WatchEvents` in `internal/docker/manager.go`.
- Existing `Server` in `internal/proxy/server.go`.

## Implementation Notes

- `com.docker.compose.depends_on` may list multiple upstreams (comma-separated)
  and each upstream may itself appear multiple times if multiple downstreams
  depend on it.
- The reverse map must be updated on `RegisterTarget` (add) and `RemoveTarget`
  (remove).
- Cascade operations should run in a goroutine to avoid blocking the event loop.
