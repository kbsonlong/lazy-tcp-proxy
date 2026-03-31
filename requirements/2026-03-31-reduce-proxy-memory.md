# Reduce Proxy Memory Footprint via Buffer Pooling and Idle GC

**Date Added**: 2026-03-31
**Priority**: Medium
**Status**: Completed

## Problem Statement

Over a short period of proxying requests the process memory footprint grows from ~3 MB to ~6 MB and does not return to baseline. The root cause is two-fold:

1. **Per-connection buffer allocations**: `io.Copy` allocates a new 32 KB scratch buffer for each copy direction of each proxied connection. With many short-lived connections these buffers accumulate on the Go heap until the GC decides to collect them.
2. **No OS memory return on idle**: Even after the GC collects unreferenced buffers, Go does not immediately return freed heap pages to the OS. When a container becomes idle (all active connections have closed) there is no prompt to release that memory back to the OS.

## Functional Requirements

1. I/O copy buffers used for bidirectional TCP proxying must be reused across connections rather than freshly allocated per connection.
2. When the last active connection to a container closes (container becomes idle), the process must attempt to return freed memory to the OS.

## User Experience Requirements

- No visible change to proxy behaviour or log output beyond the existing "last connection closed; idle timer started" log line.
- Memory footprint should return closer to the pre-connection baseline after all connections to a container have closed.

## Technical Requirements

- Use `sync.Pool` holding `[]byte` slices of the same size used by `io.Copy` (32 KB) so buffers are reused rather than allocated fresh per goroutine.
- Use `io.CopyBuffer` (instead of `io.Copy`) to supply the pooled buffer.
- After decrementing `activeConns` to zero, call `debug.FreeOSMemory()` in a short-lived goroutine so the hot path (connection close) is not blocked by GC work.
- No changes to external interfaces, configuration, or Docker interaction.

## Acceptance Criteria

- [ ] A `sync.Pool` of 32 KB `[]byte` buffers is used for all `io.CopyBuffer` calls in `handleConn`.
- [ ] Buffers are returned to the pool immediately after each copy direction completes.
- [ ] `debug.FreeOSMemory()` is called (in a goroutine) when `activeConns` transitions to 0 for a container.
- [ ] Existing proxy behaviour and log output are unchanged.
- [ ] The binary still builds with `go build ./...`.

## Dependencies

- No dependency on other requirements.
- Modifies `lazy-tcp-proxy/internal/proxy/server.go` only.

## Implementation Notes

- `sync.Pool` buffers may be reclaimed by the GC at any time, so each `Get` must be followed by a `Put` in a `defer` within the same goroutine.
- `debug.FreeOSMemory()` calls `runtime.GC()` twice and then `runtime/debug.FreeOSMemory` to advise the OS to reclaim freed pages; this is intentionally aggressive but runs off the hot path.
- Buffer size should be a named constant (`copyBufSize = 32 * 1024`) for clarity.
