# Leave Joined Networks on Shutdown

**Date Added**: 2026-03-31
**Priority**: Medium
**Status**: Completed

## Problem Statement

On startup the proxy joins the Docker networks of every registered container so it can reach them by IP. When the proxy is stopped (SIGINT/SIGTERM) it leaves those networks connected, which causes stale network memberships to accumulate in Docker.

## Functional Requirements

On graceful shutdown the proxy must disconnect itself from every Docker network it joined at runtime before the process exits.

## User Experience Requirements

- A log line is emitted for each network being left:
  `docker: leaving network <name>` (network name in green, consistent with REQ-016).
- If a disconnect fails the error is logged but shutdown continues.

## Technical Requirements

- Track joined networks in `Manager` using a `map[string]string` (networkID → name) protected by a `sync.Mutex`.
- Record each network in the map immediately after a successful `NetworkConnect` call in `JoinNetworks`.
- Add `LeaveNetworks(ctx context.Context)` to `Manager`; it copies the map under the lock, then calls `cli.NetworkDisconnect` for each entry.
- In `main.go`, after `<-ctx.Done()`, call `mgr.LeaveNetworks(context.Background())` (fresh context, since the main one is already cancelled).

## Acceptance Criteria

- [x] On SIGINT/SIGTERM, `docker: leaving network <name>` is logged for each network previously joined.
- [x] The proxy container is disconnected from those networks in Docker.
- [x] Disconnect errors are logged but do not block remaining disconnects or process exit.
- [x] If the proxy has no `selfID` (non-containerised run), `LeaveNetworks` is a no-op.
- [x] Build passes (`go build ./...`).

## Dependencies

- REQ-016 (Green Network Names) — reuses the same green ANSI convention for network names in log output.

## Implementation Notes

`LeaveNetworks` takes a snapshot of the map under the lock so the mutex is not held during the (potentially slow) Docker API calls.
