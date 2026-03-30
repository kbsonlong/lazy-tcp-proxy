# Multi-Port Mappings via lazy-tcp-proxy.ports Label

**Date Added**: 2026-03-30
**Priority**: High
**Status**: In Progress

## Problem Statement

The current `lazy-tcp-proxy.port=<N>` label supports only a single port, and the listen port and container port are always the same. This prevents proxying containers that expose ports on different numbers than what external clients should connect to, and prevents a container exposing multiple ports from being proxied on all of them.

## Functional Requirements

1. Replace the `lazy-tcp-proxy.port` label with `lazy-tcp-proxy.ports`.
2. The new label accepts one or more `<listen>:<target>` mappings, comma-separated:
   - `lazy-tcp-proxy.ports=9000:80` — proxy listens on 9000, forwards to container port 80.
   - `lazy-tcp-proxy.ports=9000:80,9001:8080` — two listeners from one container.
3. Each mapping creates an independent TCP listener on the specified listen port.
4. The old `lazy-tcp-proxy.port` label is removed and no longer recognised.
5. Conflicting listen ports across containers continue to be ignored (existing behaviour, no change).

## User Experience Requirements

- Users update their container labels to use the new format:
  ```yaml
  labels:
    - "lazy-tcp-proxy.enabled=true"
    - "lazy-tcp-proxy.ports=9000:80"
  ```
- Starting a container with the old `lazy-tcp-proxy.port` label (and no `lazy-tcp-proxy.ports`) produces a clear rejection log:
  `event: container <name> started but not proxied: missing label lazy-tcp-proxy.ports`

## Technical Requirements

### New type: `PortMapping`
```go
type PortMapping struct {
    ListenPort int
    TargetPort int
}
```

### `TargetInfo` change
- `Port int` → `Ports []PortMapping`

### Label parsing (`containerToTargetInfo`)
- Read `lazy-tcp-proxy.ports`; split on `,`; parse each `<listen>:<target>` pair.
- Return an error if the label is absent, any mapping is malformed, or either port is not a valid integer.

### Proxy server (`RegisterTarget`)
- Iterate `info.Ports`; create one `targetState` (with its own listener) per mapping.
- `targetState` gains `targetPort int` alongside `listenPort int` (listen port is the map key, same as before).
- When dialling the upstream, use `targetPort` instead of the (previously shared) `Port`.

### Inactivity checker
- A container is considered idle when **all** of its port mappings have no active connections and all `lastActive` times are older than 2 minutes.
- Group `targetState` entries by `ContainerID` when evaluating idleness.

### `RemoveTarget`
- Must close and delete **all** `targetState` entries whose `ContainerID` matches.

### Event handler validation (REQ-005 update)
- Check for `lazy-tcp-proxy.ports` (not `lazy-tcp-proxy.port`) in `msg.Actor.Attributes`.
- Validate that at least one valid `<listen>:<target>` pair is present.

## Acceptance Criteria

- [ ] `lazy-tcp-proxy.ports=9000:80` causes the proxy to listen on 9000 and forward to container port 80.
- [ ] `lazy-tcp-proxy.ports=9000:80,9001:8080` creates two listeners and forwards each to its respective target port.
- [ ] A container with `lazy-tcp-proxy.port=80` (old label) logs a rejection: `missing label lazy-tcp-proxy.ports`.
- [ ] Removing a container closes all its listeners.
- [ ] A container with all port mappings idle for 2 minutes is stopped.
- [ ] `go build ./...` passes.

## Dependencies

- REQ-001 (Core TCP Proxy) — modifies `internal/docker/manager.go` and `internal/proxy/server.go`.
- REQ-005 (Rejection Logging) — updates the event handler validation label name.

## Implementation Notes

- `targetState` map key remains listen port (`int`) — unchanged, just one entry per mapping rather than per container.
- `TargetInfo.Port` field is removed entirely; `TargetInfo.Ports []PortMapping` replaces it.
- `strconv.Atoi` is already imported in `manager.go`.
