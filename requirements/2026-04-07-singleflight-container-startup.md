# Singleflight Deduplication for Container Startup

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: In Progress

## Problem Statement

When many TCP connections arrive simultaneously for a stopped container, each
connection goroutine independently calls `backend.EnsureRunning` for the same
container ID. While the underlying Docker/Kubernetes backend serialises the
actual start command (e.g. via its own mutex), every goroutine still enters a
30-second polling loop independently:

```
conn-1  → EnsureRunning(id) → starts container → polls 30×
conn-2  → EnsureRunning(id) → (already starting) → polls 30×
conn-3  → EnsureRunning(id) → (already starting) → polls 30×
...
conn-N  → EnsureRunning(id) → (already starting) → polls 30×
```

This wastes goroutine resources and floods backend logs with redundant calls.
The same problem exists for UDP flows (`startUDPFlow`) and cascade starts
(`cascadeStart`).

## Functional Requirements

1. When N goroutines call `EnsureRunning` for the same container ID concurrently,
   only **one** actual call proceeds; the others wait and share its result.
2. Once the in-flight call completes (success or error), every waiter receives
   the same return value and proceeds immediately.
3. A new call arriving *after* the in-flight call finishes starts a fresh call
   (singleflight is not a cache — it only deduplicates concurrent in-flight work).
4. The deduplication must cover all call sites: TCP `handleConn`, UDP
   `startUDPFlow`, and cascade `cascadeStart`.
5. No change to the external behaviour of the proxy — connections that succeed
   today must still succeed; connections that fail today must still fail.

## User Experience Requirements

- No user-visible changes to configuration, labels, or environment variables.
- Log output may add a single debug-level note ("joined in-flight startup") but
  must not become more verbose under normal operation.

## Technical Requirements

- Use `golang.org/x/sync/singleflight` (standard Go extended library).
- The `singleflight.Group` lives on `ProxyServer` (one group for all containers).
- The singleflight key is the container ID string.
- The wrapper must forward the `context.Context` cancellation: if the context
  is cancelled while waiting, the waiter returns the context error, not the
  singleflight result.
- The implementation must not hold any proxy mutex while waiting for
  `EnsureRunning` (no deadlock risk — current code doesn't hold one either).

## Acceptance Criteria

- [ ] `golang.org/x/sync` added to `go.mod` / `go.sum`.
- [ ] `ProxyServer` has a `startGroup singleflight.Group` field.
- [ ] `handleConn` wraps its `EnsureRunning` call via `startGroup`.
- [ ] `startUDPFlow` wraps its `EnsureRunning` call via `startGroup`.
- [ ] `cascadeStart` wraps its `EnsureRunning` call via `startGroup`.
- [ ] Unit test: concurrent calls to a slow mock `EnsureRunning` result in
      exactly one actual invocation, with all callers receiving the result.
- [ ] All existing tests continue to pass (`go test -race ./...`).
- [ ] CI (golangci-lint + go vet + go test) passes.

## Dependencies

- No dependency on other open requirements.
- Touches: `lazy-tcp-proxy/internal/proxy/server.go`,
  `lazy-tcp-proxy/internal/proxy/udp.go`,
  `lazy-tcp-proxy/go.mod`, `lazy-tcp-proxy/go.sum`.

## Implementation Notes

The singleflight wrapper pattern:

```go
result, err, _ := s.startGroup.Do(containerID, func() (any, error) {
    return nil, s.backend.EnsureRunning(ctx, containerID)
})
_ = result
if err != nil { ... }
```

The third return value (bool `shared`) can be logged at debug level to show
that a goroutine "joined" an existing in-flight call.

Context cancellation handling: `singleflight.Do` blocks until the leader
returns, ignoring the caller's context. For this proxy the contexts passed to
`EnsureRunning` are `context.Background()` so cancellation is not a concern in
the current code. If that changes in future, the `DoChan` variant can be used
with a `select` over the result channel and `ctx.Done()`.
