# Keep Stopped Containers Registered; Deregister Only on Remove

**Date Added**: 2026-03-30
**Priority**: High
**Status**: In Progress

## Problem Statement

Currently the proxy deregisters a container (closes its listener) when the container stops (`die` event). This means a subsequent connection to that port fails immediately rather than starting the container and proxying through. The intended behaviour is lazy-start: the proxy should keep listening even when the target is stopped, start it on demand when a connection arrives, and only stop listening when the container is actually removed from Docker.

## Functional Requirements

1. **Stopped containers stay registered.** When a container stops (`die` event), its listener remains open and its entry stays in the proxy's target map.
2. **On-demand start.** When a connection arrives for a stopped container, `EnsureRunning` starts it (this logic already exists in `handleConn` — no change needed there).
3. **Deregister only on removal.** When a container is removed (`destroy` event), its listener(s) are closed and its entry is deleted from the target map.
4. **Initialisation includes stopped containers.** `Discover` already uses `ContainerList` with `All: true` — no change needed there.
5. **Log container stop without deregistering**, e.g.: `docker: event: container stopped: <name> (still registered)`.

## User Experience Requirements

- Connecting to a proxy port for a stopped container starts the container transparently — identical experience to connecting when it is running.
- No manual restart of the proxy is needed after a container stops.

## Technical Requirements

- In `WatchEvents`, replace `f.Add("event", "die")` with `f.Add("event", "destroy")`.
- In the event handler, replace the `die` case with a `destroy` case that calls `handler.RemoveTarget`.
- Add a `die` case that only logs (no `RemoveTarget` call).
- No changes to `proxy/server.go`, `handleConn`, `EnsureRunning`, or `Discover`.

## Acceptance Criteria

- [ ] Stopping a container does not close the proxy listener for that port.
- [ ] Connecting to the proxy port after the container has stopped causes the container to start and the connection to be proxied.
- [ ] Removing a container (`docker rm` / `docker compose down`) closes the proxy listener.
- [ ] Init discovery registers stopped containers (already working; verified by test).
- [ ] `go build ./...` passes.

## Dependencies

- REQ-001, REQ-005 — modifies event handling in `internal/docker/manager.go` only.

## Implementation Notes

- Docker event action `destroy` fires when `docker rm` is called (or `docker compose down` removes the container). It is distinct from `die` (process exit/stop).
- The event filter change is a one-line swap; the handler change adds one case and removes the `RemoveTarget` call from the old `die` case.
