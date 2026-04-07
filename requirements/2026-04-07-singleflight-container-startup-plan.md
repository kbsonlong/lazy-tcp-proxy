# Singleflight Deduplication for Container Startup — Implementation Plan

**Requirement**: [2026-04-07-singleflight-container-startup.md](2026-04-07-singleflight-container-startup.md)
**Date**: 2026-04-07
**Status**: Draft

## Implementation Steps

### Step 1 — Add `golang.org/x/sync` dependency

```
cd lazy-tcp-proxy && go get golang.org/x/sync@latest
```

This updates `go.mod` and `go.sum`. The only package used is
`golang.org/x/sync/singleflight`.

### Step 2 — Add `startGroup` field to `ProxyServer` (`server.go`)

`singleflight.Group` is a value type with a zero value that is ready to use —
no constructor change needed.

```go
// in server.go imports
import "golang.org/x/sync/singleflight"

// in ProxyServer struct
type ProxyServer struct {
    ...
    startGroup singleflight.Group  // deduplicates concurrent EnsureRunning calls
}
```

### Step 3 — Wrap `EnsureRunning` in `handleConn` (`server.go`)

Replace (line ~560):

```go
if err := s.backend.EnsureRunning(ctx, ts.info.ContainerID); err != nil {
    log.Printf("proxy: could not start container \033[33m%s\033[0m: %v", ts.info.ContainerName, err)
    return
}
```

With:

```go
_, err, shared := s.startGroup.Do(ts.info.ContainerID, func() (any, error) {
    return nil, s.backend.EnsureRunning(ctx, ts.info.ContainerID)
})
if shared {
    log.Printf("proxy: joined in-flight startup for \033[33m%s\033[0m", ts.info.ContainerName)
}
if err != nil {
    log.Printf("proxy: could not start container \033[33m%s\033[0m: %v", ts.info.ContainerName, err)
    return
}
```

### Step 4 — Wrap `EnsureRunning` in `startUDPFlow` (`udp.go`)

Replace (line ~103):

```go
if err := s.backend.EnsureRunning(ctx, uls.info.ContainerID); err != nil {
    log.Printf("proxy: udp: could not start container \033[33m%s\033[0m: %v", uls.info.ContainerName, err)
    cleanup()
    return
}
```

With:

```go
_, err, shared := s.startGroup.Do(uls.info.ContainerID, func() (any, error) {
    return nil, s.backend.EnsureRunning(ctx, uls.info.ContainerID)
})
if shared {
    log.Printf("proxy: udp: joined in-flight startup for \033[33m%s\033[0m", uls.info.ContainerName)
}
if err != nil {
    log.Printf("proxy: udp: could not start container \033[33m%s\033[0m: %v", uls.info.ContainerName, err)
    cleanup()
    return
}
```

Note: UDP already guards against multiple goroutines starting a flow for the
same *client* address via the `pending` map. The singleflight guard sits above
that and prevents multiple simultaneous calls to `EnsureRunning` from different
*client* addresses targeting the same container.

### Step 5 — Wrap `EnsureRunning` in `cascadeStart` (`server.go`)

Replace (line ~649):

```go
if err := s.backend.EnsureRunning(s.ctx, depID); err != nil {
    log.Printf("proxy: cascade start: error starting \033[33m%s\033[0m: %v", depName, err)
    continue
}
```

With:

```go
_, err, shared := s.startGroup.Do(depID, func() (any, error) {
    return nil, s.backend.EnsureRunning(s.ctx, depID)
})
if shared {
    log.Printf("proxy: cascade start: joined in-flight startup for \033[33m%s\033[0m", depName)
}
if err != nil {
    log.Printf("proxy: cascade start: error starting \033[33m%s\033[0m: %v", depName, err)
    continue
}
```

### Step 6 — Add unit test (`server_test.go`)

Add a new test that:
1. Creates a `mockBackend` whose `EnsureRunning` sleeps briefly (to ensure
   callers truly overlap) and counts invocations via `atomic.Int32`.
2. Wires it to a `ProxyServer`.
3. Launches N goroutines all calling the internal helper that invokes
   `s.startGroup.Do(containerID, ...)`.
4. Asserts call count == 1.

Because `handleConn` requires a live `net.Conn` and full setup, the test calls
the singleflight wrapper directly (extracted as a thin helper, or tested via
the exported-enough surface). The cleanest approach is a small unexported
helper `ensureRunning(ctx, id, name)` on `ProxyServer` that holds the
singleflight call — all three call sites use it, and the test calls it
directly.

Alternatively (simpler, no refactor): test via the `integration_test.go`
pattern — open N connections to a registered listener backed by a slow mock,
assert the mock is called once, then close connections.

The simpler approach (direct singleflight wrapper function test):

```go
func TestSingleflightEnsureRunning_DeduplicatesConcurrentCalls(t *testing.T) {
    var callCount atomic.Int32
    ready := make(chan struct{})
    b := &mockBackend{
        startFunc: func(_ string) {
            callCount.Add(1)
            <-ready // block until test releases
        },
    }
    s := newTestServer()
    s.backend = b

    const N = 20
    var wg sync.WaitGroup
    wg.Add(N)
    for range N {
        go func() {
            defer wg.Done()
            _, _, _ = s.startGroup.Do("ctr-1", func() (any, error) {
                return nil, s.backend.EnsureRunning(context.Background(), "ctr-1")
            })
        }()
    }

    // Give goroutines time to all block inside startGroup.Do
    time.Sleep(20 * time.Millisecond)
    close(ready) // unblock the leader
    wg.Wait()

    if got := callCount.Load(); got != 1 {
        t.Errorf("EnsureRunning called %d times, want 1", got)
    }
}
```

### Step 7 — Update requirement status

Mark REQ-048 as Completed in both the requirement file and `_index.md`.

---

## File-Level Change Summary

| File | Change |
|------|--------|
| `lazy-tcp-proxy/go.mod` | Add `golang.org/x/sync vX.X.X` |
| `lazy-tcp-proxy/go.sum` | Updated by `go get` |
| `lazy-tcp-proxy/internal/proxy/server.go` | Add import + `startGroup` field + wrap 2 call sites |
| `lazy-tcp-proxy/internal/proxy/udp.go` | Wrap 1 call site |
| `lazy-tcp-proxy/internal/proxy/server_test.go` | Add 1 new test function |

Total: 5 files touched, ~30 lines net added.

---

## Risk Assessment

**Low risk.** `singleflight.Do` is transparent to callers when there is no
concurrency (only one goroutine calling for a given key) — it executes the
function directly with zero overhead. The only behavioural change is under
concurrent load, where waiters share the result of the leader instead of each
doing their own call. Error handling is unchanged: if `EnsureRunning` returns
an error, every waiter receives that same error and handles it identically to
today.

The `shared` flag log line is informational only and does not affect logic.
