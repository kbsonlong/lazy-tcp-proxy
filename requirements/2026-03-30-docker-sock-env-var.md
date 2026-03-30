# DOCKER_SOCK Env Var & Dockerfile Volume

**Date Added**: 2026-03-30
**Priority**: Medium
**Status**: Completed

## Problem Statement

The Docker socket path should be configurable at runtime without rebuilding the image, and the Dockerfile should clearly communicate that `/var/run/docker.sock` must be bind-mounted in.

## Functional Requirements

- Introduce a `DOCKER_SOCK` environment variable that sets the Docker Unix socket path used by the proxy.
- `DOCKER_SOCK` takes precedence over `DOCKER_HOST`; `DOCKER_HOST` remains as a fallback.
- Add a `VOLUME` declaration in the Dockerfile to document that `/var/run/docker.sock` is expected to be mounted.
- Set `ENV DOCKER_SOCK=/var/run/docker.sock` in the Dockerfile as the default.

## User Experience Requirements

- Users can override the socket path by passing `-e DOCKER_SOCK=/custom/path.sock` at `docker run` time.
- The default value works without any extra flags when the socket is at the standard location.

## Technical Requirements

- In `NewManager()`, check `os.Getenv("DOCKER_SOCK")` before calling `client.FromEnv`.
- If set, prepend `client.WithHost("unix://<value>")` to the options slice.
- Dockerfile changes: add `ENV DOCKER_SOCK=/var/run/docker.sock` and `VOLUME ["/var/run/docker.sock"]` before `ENTRYPOINT`.

## Acceptance Criteria

- [x] Setting `DOCKER_SOCK` overrides the socket path used by the Docker client.
- [x] Omitting `DOCKER_SOCK` falls back to `DOCKER_HOST` / SDK default (unchanged behaviour).
- [x] Dockerfile `VOLUME` entry is present for `/var/run/docker.sock`.
- [x] `go build ./...` still passes after the change.

## Dependencies

- REQ-001 (Core TCP Proxy) — modifies `internal/docker/manager.go` and `Dockerfile`.

## Implementation Notes

- Changed `NewManager()` in `internal/docker/manager.go` to prepend `client.WithHost` when `DOCKER_SOCK` is set.
- `client.FromEnv` is kept so `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, etc. still work as overrides for other scenarios.
