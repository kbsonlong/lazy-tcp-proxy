# Fix Bidirectional TCP Proxy Teardown

**Date Added**: 2026-03-30
**Priority**: High
**Status**: In Progress

## Problem Statement

When the upstream (target container) closes its TCP connection, the proxy does not terminate the external (client-facing) connection promptly. The current bidirectional copy uses two goroutines with half-close semantics:

- Goroutine A: `io.Copy(upstream, conn)` — reads from external client, writes to upstream
- Goroutine B: `io.Copy(conn, upstream)` — reads from upstream, writes to external client

When upstream closes (Goroutine B finishes), the code calls `conn.CloseWrite()` to send a FIN to the external client. However, Goroutine A is still blocked reading from `conn`, waiting for the external client to close its write side. HTTP/1.1 keep-alive clients typically keep their write side open waiting for another request, so Goroutine A hangs indefinitely.

Consequences:
- `wg.Wait()` never returns → `handleConn` never returns
- `activeConns` stays > 0 → the idle timer never starts
- The "last connection closed" log (REQ-010) never fires
- The external TCP connection is never fully torn down

This is why connections appeared to stay open for 4+ minutes in observed logs — the upstream container had already closed its side, but `handleConn` was hanging.

## Functional Requirements

1. When the upstream connection closes, the external (client) connection must also be fully closed immediately, and vice versa.
2. `handleConn` must return promptly once either side closes, so `activeConns` is decremented and the idle timer can start.

## Technical Requirements

Replace the current half-close goroutine pattern with a `sync.Once`-guarded `closeAll` function. When either `io.Copy` goroutine finishes, `closeAll()` closes **both** connections. This unblocks the other goroutine, both goroutines return, `wg.Wait()` unblocks, and `handleConn` returns.

```go
var closeOnce sync.Once
closeAll := func() {
    closeOnce.Do(func() {
        conn.Close()
        upstream.Close()
    })
}

var wg sync.WaitGroup
wg.Add(2)

go func() {
    defer wg.Done()
    io.Copy(upstream, conn)
    closeAll()
}()

go func() {
    defer wg.Done()
    io.Copy(conn, upstream)
    closeAll()
}()

wg.Wait()
```

The existing `defer conn.Close()` and `defer upstream.Close()` in `handleConn` remain as safety-net cleanup. The half-close (`CloseWrite`) calls are removed — they are redundant once both connections are closed.

`sync` is already imported.

## Acceptance Criteria

- [ ] When the upstream container closes its connection, `handleConn` returns promptly (within milliseconds, not minutes).
- [ ] The "last connection closed; idle timer started" log (REQ-010) fires immediately after the upstream closes.
- [ ] The external client connection is terminated when the upstream closes.
- [ ] `go build ./...` passes.

## Dependencies

- REQ-001 (Core TCP Proxy) — modifies `internal/proxy/server.go`.
- REQ-009 / REQ-010 — the idle timer and "last connection closed" logging introduced there now work correctly once `handleConn` returns promptly.
