# Sort /status Services by Name

**Date Added**: 2026-04-07
**Priority**: Low
**Status**: Planned

## Problem Statement

The `/status` endpoint returns services in non-deterministic order because the underlying data structure is a Go map. This makes the output hard to read and compare across requests.

## Functional Requirements

- The `/status` endpoint must return services sorted alphabetically by `ContainerName` (ascending).
- If two entries share the same `ContainerName`, they should be further sorted by `ContainerID` (ascending) as a stable tie-breaker.

## User Experience Requirements

- Users reading the JSON response always see services in a predictable, consistent order.

## Technical Requirements

- Sort is applied inside `ProxyServer.Snapshot()` before returning the slice, so all callers benefit.
- Use the standard `sort.Slice` (or `slices.SortFunc`) from the Go standard library — no new dependencies.

## Acceptance Criteria

- [ ] `GET /status` returns services sorted alphabetically by `ContainerName`.
- [ ] When container names are equal, entries are further sorted by `ContainerID`.
- [ ] Existing unit/integration tests continue to pass.

## Dependencies

- REQ-025 (HTTP Status Endpoint)

## Implementation Notes

The sort needs to go into `Snapshot()` in `lazy-tcp-proxy/internal/proxy/server.go`, after the slice is populated and before it is returned.
