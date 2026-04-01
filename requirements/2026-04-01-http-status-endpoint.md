# HTTP Status Endpoint (List Managed Containers)

**Date Added**: 2026-04-01
**Priority**: High
**Status**: Planned

## Problem Statement

Currently there is no way to inspect which containers the proxy is managing without trawling through logs. Operators need a simple, machine-readable way to query the live state of all registered targets — useful for dashboards, health checks, and debugging.

## Functional Requirements

- The proxy exposes an HTTP server on a configurable port (default `8080`, overridden by `STATUS_PORT` env var).
- `GET /status` returns a JSON array of all currently registered targets, including:
  - Container ID (short 12-char form)
  - Container name
  - Port mappings (listen port → target port)
  - Whether the container is currently running
  - Number of active connections
  - Last active timestamp (RFC3339, or `null` if never active)
- `GET /healthz` returns `200 OK` with body `ok` — a minimal liveness probe.
- The HTTP server starts alongside the proxy and shuts down gracefully on the same context cancellation.
- If `STATUS_PORT=0` the HTTP server is disabled entirely (opt-out).

## User Experience Requirements

- No authentication — this is an internal/sidecar endpoint; network-level controls are assumed.
- JSON output is pretty-printed for easy `curl` inspection.
- The listen port is logged at startup alongside idle timeout and poll interval.

## Technical Requirements

- Implemented as a `net/http` server inside the existing `main.go` startup sequence.
- Reads state from `ProxyServer` — a new exported method (e.g. `Snapshot() []TargetSnapshot`) returns a safe copy of current target state under the read lock.
- No new dependencies — uses only stdlib `net/http` and `encoding/json`.
- `TargetSnapshot` is a plain struct (no mutexes, no pointers to shared state) safe to marshal and return.

## Acceptance Criteria

- [ ] `GET /status` returns `200` with a valid JSON array when at least one target is registered.
- [ ] `GET /status` returns `200` with an empty JSON array `[]` when no targets are registered.
- [ ] `GET /healthz` always returns `200 OK` with body `ok`.
- [ ] The port is configurable via `STATUS_PORT` env var.
- [ ] Setting `STATUS_PORT=0` disables the HTTP server (no port bound, no log line).
- [ ] Startup log includes the status endpoint address (e.g. `status endpoint: :8080`).
- [ ] Graceful shutdown: HTTP server closes when the main context is cancelled.
- [ ] Response includes all fields: container ID (short), name, port mappings, running, active connections, last active.

## Dependencies

- No dependencies on other planned requirements.
- Reads state from `ProxyServer` (`internal/proxy/server.go`) — requires a new `Snapshot()` method.

## Implementation Notes

- The HTTP server should use `http.Server.Shutdown(ctx)` tied to the root context so it drains in-flight requests cleanly.
- `lastActive` zero value should marshal as `null` (use `*time.Time` in the snapshot struct).
- Port conflict with a managed container's listen port is possible if `STATUS_PORT` clashes — log a fatal and exit clearly.
