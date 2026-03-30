# Core TCP Proxy for Docker Containers

**Date Added**: 2026-03-30
**Priority**: High
**Status**: Completed

## Problem Statement

Running all Docker containers simultaneously wastes resources. A lazy-loading proxy that starts containers on-demand and stops them after inactivity would allow many services to coexist on a host without constant resource consumption.

## Functional Requirements

- Accept inbound TCP connections and proxy them through to a target Docker container.
- If the target container is stopped when a connection arrives, start it automatically before proxying.
- Stop containers after 2 minutes of inactivity (no active connections).
- Discover proxy targets automatically via Docker container labels — no static config file.
- On startup, perform an initial discovery of all matching containers.
- Watch the Docker events API for runtime changes (new containers, containers stopping) and update proxy config accordingly.
- Automatically join Docker networks that target containers belong to, so the proxy can reach them by internal IP.

## User Experience Requirements

- Users opt-in containers by adding Docker labels:
  - `lazy-tcp-proxy.enabled=true` — marks the container as a proxy target.
  - `lazy-tcp-proxy.port=<N>` — the port inside the container to proxy to. The proxy also listens on this port.
- No manual proxy configuration is required.
- Conflicting port mappings (two containers claiming the same port) are ignored for now.

## Technical Requirements

- Written in Go.
- Communicates with Docker via the Docker socket (passed at runtime).
- Packaged as a minimal Docker image using `FROM scratch`.
- Uses the official Docker Go SDK (`github.com/docker/docker`).
- Bidirectional TCP proxying via `io.Copy` goroutines.
- Container readiness checked by retrying a TCP dial (up to 30 attempts, 1 s apart).
- Own container ID detected from `/proc/self/cgroup` (with fallback to `/etc/hostname`) for network auto-join.
- Docker events subscription filters by label `lazy-tcp-proxy.enabled=true` and event types `create`, `start`, `die`.
- Event watcher reconnects with exponential backoff (max 30 s) on error.

## Acceptance Criteria

- [x] Proxy listens on the port declared in `lazy-tcp-proxy.port`.
- [x] Connecting to the proxy port starts the target container if stopped.
- [x] Connection is forwarded to the container's internal IP once it is running.
- [x] Container is stopped after 2 minutes with no active connections.
- [x] Proxy joins any Docker network the target container belongs to.
- [x] New containers added at runtime are discovered without restart.
- [x] `go build ./...` produces a working binary with no errors.
- [x] Dockerfile produces a runnable `scratch`-based image.

## Dependencies

None — this is the foundational requirement.

## Implementation Notes

- Project lives at `lazy-tcp-proxy/` within the repo root.
- Module path: `github.com/nickgrealy/lazy-tcp-proxy`.
- Key files:
  - `main.go` — wiring, signal handling, goroutine lifecycle.
  - `internal/docker/manager.go` — all Docker API interactions.
  - `internal/proxy/server.go` — TCP listener, connection handler, inactivity checker.
  - `Dockerfile` — multi-stage build.
- Future work (deferred): port conflict resolution, healthchecks, Docker Stacks support.
