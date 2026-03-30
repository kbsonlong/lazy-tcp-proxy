# Structured Init and Change Logging

**Date Added**: 2026-03-30
**Priority**: Medium
**Status**: Planned

## Problem Statement

The current logging is per-action and scattered. There is no summary at startup showing what was discovered, and runtime change events don't clearly state what container or network name changed. This makes it hard to verify the proxy is configured correctly at a glance.

## Functional Requirements

### On initialisation (`Discover`)

1. After discovery completes, log the names of all found proxy containers, e.g.:
   - `init: found containers: web, api, worker`
   - `init: no proxy containers found`
2. After all network joins complete, log the names of all networks this proxy container was added to, e.g.:
   - `init: joined networks: frontend, backend`
   - `init: no networks joined`

### On runtime change (Docker events)

3. When a new container is detected (create/start event), log the container name, e.g.:
   - `event: container added: web`
4. When a container is removed (die event), log the container name, e.g.:
   - `event: container removed: web`
5. When networks are joined as a result of a new container being detected, log each new network name, e.g.:
   - `event: joined network: backend`
   - (no log if no new networks were joined)

## User Experience Requirements

- Logs should be readable in a single `docker logs` pass to understand current state.
- Network and container names must appear in the log message (not just IDs).

## Technical Requirements

- `JoinNetworks` must return the list of network names it newly joined (changing return type from `error` to `([]string, error)`).
- `Discover` must collect container names and aggregated network join results across all discovered containers, then emit two summary log lines at the end.
- The Docker events handler in `WatchEvents` must read `msg.Actor.Attributes["name"]` to get the container name for add/remove log lines.
- All call sites of `JoinNetworks` must be updated to handle the new return value.
- No changes to `proxy/server.go` are required.

## Acceptance Criteria

- [ ] Starting the proxy with no labelled containers logs `init: no proxy containers found` and `init: no networks joined`.
- [ ] Starting the proxy with labelled containers logs a single line listing all container names.
- [ ] Starting the proxy with labelled containers logs a single line listing all network names joined (or `no networks joined` if already a member of all of them).
- [ ] A container create/start event logs `event: container added: <name>`.
- [ ] A container die event logs `event: container removed: <name>`.
- [ ] A create/start event that causes a new network join logs `event: joined network: <name>`.
- [ ] `go build ./...` passes.

## Dependencies

- REQ-001 (Core TCP Proxy) — modifies `internal/docker/manager.go`.

## Implementation Notes

- Docker event `Actor.Attributes` map contains a `"name"` key with the container name (without leading `/`).
- `JoinNetworks` already logs individual joins with `log.Printf`; those per-join logs can be kept or removed — prefer keeping them so per-join detail is visible in verbose scenarios.
