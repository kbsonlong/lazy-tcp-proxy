# Container Name in Start/Stop Log Messages

**Date Added**: 2026-03-31
**Priority**: Low
**Status**: Completed

## Problem Statement

The `docker: starting/started/stopping/stopped container` log lines displayed a 12-character container ID (e.g. `c29ec25b5724`), which requires cross-referencing elsewhere to identify which container was affected. The human-readable container name is more useful at a glance.

## Functional Requirements

The four start/stop log messages must display the container name instead of the container ID:

```
docker: starting container <name>
docker: container <name> started
docker: stopping container <name> (idle timeout)
docker: container <name> stopped
```

## User Experience Requirements

- Container name must be rendered in yellow (consistent with REQ-014).
- No other changes to log message wording or structure.

## Technical Requirements

- `EnsureRunning`: derive name from `inspect.Name` (already fetched in the function); strip the leading `/`.
- `StopContainer`: add a `containerName string` parameter; update the single call site in `proxy/server.go` to pass `e.name`.
- No additional Docker API calls required.

## Acceptance Criteria

- [x] `docker: starting container` log line shows container name in yellow.
- [x] `docker: container … started` log line shows container name in yellow.
- [x] `docker: stopping container … (idle timeout)` log line shows container name in yellow.
- [x] `docker: container … stopped` log line shows container name in yellow.
- [x] Build passes (`go build ./...`).

## Dependencies

- REQ-014 (Yellow Container Names in Log Output) — extends the same yellow-name convention to start/stop messages.

## Implementation Notes

`StopContainer` signature changed from `(ctx, containerID)` to `(ctx, containerID, containerName)`. The only call site is `checkInactivity` in `proxy/server.go`, which already holds `e.name`.
