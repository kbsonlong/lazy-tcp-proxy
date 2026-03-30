# Log All Container Starts with Rejection Reason

**Date Added**: 2026-03-30
**Priority**: High
**Status**: Planned

## Problem Statement

When a container starts, the proxy silently ignores it if its labels don't match the required filter. There is no way to tell from the logs whether the proxy never saw the event (network issue, wrong filter) or saw it but chose to skip it (wrong labels). This makes misconfiguration hard to diagnose.

Additionally, the Docker events subscription currently filters by label at the API level (`lazy-tpc-proxy.enabled=true`), which means containers with subtly wrong labels (e.g. `lazy-tcp-proxy.enable=true`) produce no log output whatsoever.

## Functional Requirements

1. The Docker events listener must receive **all** container `start` events — not just those already bearing the correct label — so that every start is visible for logging.
2. For every container start event, log one line that either:
   - Confirms it was added: *(already done — `event: container added: <name>`)*
   - Explains why it was not added, e.g.:
     - `event: container <name> started but not proxied: missing label lazy-tpc-proxy.enabled=true`
     - `event: container <name> started but not proxied: missing label lazy-tpc-proxy.port`
     - `event: container <name> started but not proxied: invalid port value "<val>"`
3. The `die` event filter should similarly be broadened, but since we only call `RemoveTarget` (which is a no-op for unknown containers) no extra logging is needed for die events on non-proxy containers.

## User Experience Requirements

- A user who misconfigures labels sees a clear, one-line reason immediately in `docker logs` when their container starts.
- A user with correct labels sees the existing `event: container added: <name>` log and nothing else changes.

## Technical Requirements

- Remove `f.Add("label", "lazy-tpc-proxy.enabled=true")` from the `WatchEvents` event filter so all container events are received.
- Keep the `type=container` filter and the `event=start` / `event=die` / `event=create` event-type filters.
- Move label validation logic into the event handler: check for `lazy-tpc-proxy.enabled=true` and a valid `lazy-tpc-proxy.port` value on the container's labels before calling `containerToTargetInfo`.
- Log a single descriptive rejection line if validation fails; do not call `containerToTargetInfo` or `RegisterTarget` for non-matching containers.
- The `Discover` (init) path is unaffected — it already uses a separate label-filtered `ContainerList` call.

## Acceptance Criteria

- [ ] Starting a container with no proxy labels logs `event: container <name> started but not proxied: missing label lazy-tpc-proxy.enabled=true`.
- [ ] Starting a container with `lazy-tpc-proxy.enabled=true` but no port label logs `event: container <name> started but not proxied: missing label lazy-tpc-proxy.port`.
- [ ] Starting a container with `lazy-tpc-proxy.enabled=true` and an invalid port logs `event: container <name> started but not proxied: invalid port value "<val>"`.
- [ ] Starting a container with both correct labels logs `event: container added: <name>` (existing behaviour preserved).
- [ ] `go build ./...` passes.

## Dependencies

- REQ-001 (Core TCP Proxy) — modifies `internal/docker/manager.go`.
- REQ-004 (Structured Logging) — builds on the event handler introduced there.

## Implementation Notes

- Container labels are available directly on `msg.Actor.Attributes` in Docker events — no extra API call needed to check them.
- `msg.Actor.Attributes` contains all container labels plus Docker-internal keys like `image` and `name`.
