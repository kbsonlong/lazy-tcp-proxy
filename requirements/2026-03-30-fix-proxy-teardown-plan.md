# Fix Bidirectional TCP Proxy Teardown — Implementation Plan

**Requirement**: [2026-03-30-fix-proxy-teardown.md](2026-03-30-fix-proxy-teardown.md)
**Date**: 2026-03-30
**Status**: Implemented

## Implementation Steps

All changes are in `lazy-tcp-proxy/internal/proxy/server.go`, inside `handleConn`.

### Step 1 — replace the bidirectional copy block

**Remove** the current block (lines ~237–262):

```go
// Bidirectional copy
var wg sync.WaitGroup
wg.Add(2)

go func() {
    defer wg.Done()
    if _, err := io.Copy(upstream, conn); err != nil {
        // Ignore closed connection errors
    }
    // Half-close
    if tc, ok := upstream.(*net.TCPConn); ok {
        tc.CloseWrite()
    }
}()

go func() {
    defer wg.Done()
    if _, err := io.Copy(conn, upstream); err != nil {
        // Ignore closed connection errors
    }
    if tc, ok := conn.(*net.TCPConn); ok {
        tc.CloseWrite()
    }
}()

wg.Wait()
```

**Replace** with:

```go
// Bidirectional copy. When either direction closes, both connections are
// shut down immediately so the other goroutine is never left hanging.
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
    io.Copy(upstream, conn) //nolint:errcheck
    closeAll()
}()

go func() {
    defer wg.Done()
    io.Copy(conn, upstream) //nolint:errcheck
    closeAll()
}()

wg.Wait()
```

No other changes needed. `sync` is already imported. The existing `defer conn.Close()` and `defer upstream.Close()` higher in `handleConn` remain as safety-net cleanup and are harmless when called again (idempotent in Go's net package).

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Step 1 — replace bidirectional copy block |
| `requirements/2026-03-30-fix-proxy-teardown.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## Key Code Snippets

See Step 1 above — the replacement block is the complete final code.

## Unit Tests

| Test | Scenario | Expected |
|------|----------|----------|
| Upstream closes first | upstream sends EOF | `handleConn` returns, `activeConns` decremented, `conn` closed |
| Client closes first | client sends EOF | `handleConn` returns, `activeConns` decremented, `upstream` closed |
| Both close simultaneously | race between the two | `sync.Once` ensures exactly one `closeAll` executes; both return |

## Risks & Open Questions

- **Loss of half-duplex grace period**: the old code sent FIN to one side and waited for the other to close gracefully. The new code closes both sides immediately when either finishes. For the HTTP use cases in this project this is correct behaviour; the risk is only for protocols that require independent half-close (rare, and not in scope).
- No new imports needed.
