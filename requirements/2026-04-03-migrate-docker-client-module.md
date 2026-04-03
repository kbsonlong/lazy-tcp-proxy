# Migrate from deprecated github.com/docker/docker to github.com/moby/moby/client

**Date Added**: 2026-04-03
**Priority**: High
**Status**: Completed

## Problem Statement

The Go module `github.com/docker/docker` is frozen at `v28.5.2+incompatible` and will receive no further security patches. With Docker Engine v29, the project migrated to new module paths under `github.com/moby/moby`. The old module path is deprecated and any future CVEs in the Docker client SDK (such as CVE-2026-34040) will only be fixed in the new module path.

Note: CVE-2026-34040 itself does not affect this codebase (it is a daemon-side AuthZ bypass; this project only uses the client SDK). However, staying on the deprecated module means the project will not receive future client-side security fixes.

## Functional Requirements

- Replace the dependency on `github.com/docker/docker v28.5.2+incompatible` with the canonical replacement modules under `github.com/moby/moby/...`
- All existing Docker client functionality must continue to work identically:
  - Container list, inspect, start, stop
  - Network inspect, connect, disconnect
  - Event stream subscription

## User Experience Requirements

- No change to runtime behaviour — this is a pure dependency migration
- All existing tests must pass after migration

## Technical Requirements

- Replace import paths in `lazy-tcp-proxy/internal/docker/manager.go`:
  - `github.com/docker/docker/client` → `github.com/moby/moby/client` (module `github.com/moby/moby/client v0.3.0`)
  - `github.com/docker/docker/api/types/container` → new path under `github.com/moby/moby`
  - `github.com/docker/docker/api/types/events` → new path
  - `github.com/docker/docker/api/types/filters` → new path
  - `github.com/docker/docker/api/types/network` → new path
- Update `go.mod` and `go.sum` accordingly
- Constructor `client.NewClientWithOpts` is deprecated in v29; replace with `client.New` if the new module uses a different constructor

## Acceptance Criteria

- [ ] `github.com/docker/docker` no longer appears in `go.mod`
- [ ] `github.com/moby/moby/client` (or equivalent new module path) is the direct dependency
- [ ] All import paths in `manager.go` compile successfully
- [ ] `go build ./...` passes
- [ ] All existing tests pass (`go test ./...`)

## Dependencies

- REQ-020: Previous Docker CVE fix (upgraded to v28)
- No other requirements depend on the module path directly

## Implementation Notes

- The `github.com/moby/moby/client` module is at v0.3.0 as of 2026-04-03 (895+ importers)
- The API surface is functionally equivalent; main changes are import paths and possibly the client constructor name
- A known Go tooling issue (#51754) means `go get github.com/moby/moby/api` may resolve to v28 — use explicit `go get github.com/moby/moby/client@v0.3.0`
- Migration guide being tracked at moby/moby#50973
