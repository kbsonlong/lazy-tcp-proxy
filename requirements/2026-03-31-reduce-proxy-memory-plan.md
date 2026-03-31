# Reduce Proxy Memory via Buffer Pooling & Idle GC — Implementation Plan

**Requirement**: [2026-03-31-reduce-proxy-memory.md](2026-03-31-reduce-proxy-memory.md)
**Date**: 2026-03-31
**Status**: Implemented

## Implementation Steps

1. **Add `copyBufSize` constant** at the top of `server.go` (alongside `dialRetries`/`dialInterval`) — value `32 * 1024` (bytes), matching the default `io.Copy` internal buffer size.

2. **Add a package-level `sync.Pool`** named `copyBufPool` that returns a `*[]byte` pointing to a newly allocated `[copyBufSize]byte` slice on `New`. Using a pointer avoids an interface allocation on each `Get`/`Put`.

3. **Update `handleConn`**: replace both `io.Copy` calls with `io.CopyBuffer`, acquiring a buffer from the pool before the goroutine body and returning it with `defer`.

4. **Trigger `debug.FreeOSMemory()` on idle**: in the existing `defer` that logs "last connection closed; idle timer started", after the log line, launch `go debug.FreeOSMemory()` so GC + OS page release happen off the hot path.

5. **Add `"runtime/debug"` to the import block** in `server.go`.

6. **Update requirement status** to "In Progress" in both the requirement file and `_index.md`.

7. **Build check**: run `go build ./...` from `lazy-tcp-proxy/` to confirm compilation.

8. **Update requirement and plan status** to Completed/Implemented; commit and push all changes.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `copyBufSize` const, `copyBufPool`, replace `io.Copy` with `io.CopyBuffer`, call `debug.FreeOSMemory()` on idle |
| `requirements/2026-03-31-reduce-proxy-memory.md` | Modify | Status → In Progress, then Completed |
| `requirements/_index.md` | Modify | Status → In Progress, then Completed |

## Key Code Snippets

```go
const (
    dialRetries  = 30
    dialInterval = time.Second
    copyBufSize  = 32 * 1024
)

var copyBufPool = sync.Pool{
    New: func() any {
        b := make([]byte, copyBufSize)
        return &b
    },
}
```

Inside the two copy goroutines in `handleConn`:

```go
go func() {
    defer wg.Done()
    buf := copyBufPool.Get().(*[]byte)
    defer copyBufPool.Put(buf)
    io.CopyBuffer(upstream, conn, *buf) //nolint:errcheck
    closeAll()
}()

go func() {
    defer wg.Done()
    buf := copyBufPool.Get().(*[]byte)
    defer copyBufPool.Put(buf)
    io.CopyBuffer(conn, upstream, *buf) //nolint:errcheck
    closeAll()
}()
```

Idle trigger (inside the existing `defer` in `handleConn`):

```go
defer func() {
    if ts.activeConns.Add(-1) == 0 {
        log.Printf("proxy: last connection to \033[33m%s\033[0m closed; idle timer started (container will stop in ~%s if no new connections)",
            ts.info.ContainerName, s.idleTimeout)
        go debug.FreeOSMemory()
    }
}()
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Build check | `go build ./...` | exits 0, no errors |

## Risks & Open Questions

- `debug.FreeOSMemory()` is intentionally aggressive (two GC passes + MADV_FREE/MADV_DONTNEED). It runs in a goroutine so it does not block the connection close path, but it will briefly spike CPU. Acceptable for a proxy with infrequent idle transitions.
- `sync.Pool` buffers can be reclaimed by the GC between Get and Put if a GC cycle runs mid-goroutine — this is safe because each goroutine holds the buffer reference for the duration of the copy.
