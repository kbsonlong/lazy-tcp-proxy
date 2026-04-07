# Fix Kubernetes WatchEvents Gaps

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: In Progress

## Problem Statement

Two bugs exist in `internal/k8s/backend.go`'s `WatchEvents` implementation:

### Bug 1 — Missing `ContainerStopped` on external scale-to-zero

**Docker behaviour**: when a container stops externally, a `die` event fires →
`handler.ContainerStopped(id)` is called → cascade-stop of dependants triggers
immediately.

**K8s behaviour**: when a Deployment is externally scaled to 0 (via `kubectl`,
HPA, or any tool outside the proxy), a `Modified` event fires →
`handler.RegisterTarget(info{Running: false})` is called → `ts.running = false`
is set, but **`ContainerStopped` is never called** → cascade-stop of
dependants is **not triggered**.

Result: dependants of an externally-stopped Deployment keep running until their
own individual idle timeouts expire rather than stopping immediately.

Note: proxy-initiated stops via `checkInactivity` call `cascadeStop` directly
and are not affected by this bug.

### Bug 2 — Reconnect backoff grows on normal channel close

When the Kubernetes API server closes the watch channel normally (as it does
periodically to enforce `resourceVersion` expiry — roughly every 5–10 minutes),
`WatchEvents` treats the closure identically to an error and increments the
backoff. After a few cycles the reconnect delay grows to 30 seconds, slowing
container discovery after normal API-server-initiated channel resets.

The Docker backend correctly resets its backoff to 1s on successful reconnects
and only increases it on actual errors.

## Functional Requirements

1. When a Deployment's watch event shows `info.Running == false`, the handler's
   `ContainerStopped(id)` must be called so cascade-stops and state updates
   propagate correctly.
2. The watch reconnect backoff must reset to 1s whenever the watch channel
   closes normally (without an error from the Watch API call itself).

## User Experience Requirements

- No change to labels, annotations, or environment variables.
- Dependants of an externally-stopped Deployment stop promptly instead of
  waiting for their idle timeout.
- The proxy reconnects to the Kubernetes watch API promptly after normal channel
  resets with no growing delay.

## Technical Requirements

- Changes confined to `lazy-tcp-proxy/internal/k8s/backend.go`.
- New unit tests added to `lazy-tcp-proxy/internal/k8s/backend_test.go`.

## Acceptance Criteria

- [ ] In `WatchEvents`, `handler.ContainerStopped(id)` is called for
      `Added`/`Modified` events where `info.Running == false`.
- [ ] Unit test: a Modified event with 0 ready replicas triggers
      `ContainerStopped`.
- [ ] Unit test: a Modified event with >0 ready replicas does NOT trigger
      `ContainerStopped`.
- [ ] The reconnect backoff resets to `time.Second` after a normal channel
      close (not just after a Watch API error).
- [ ] All existing tests continue to pass (`go test -race ./...`).
- [ ] CI (golangci-lint + go vet + go test) passes.

## Dependencies

- No dependency on REQ-050.
- Touches: `lazy-tcp-proxy/internal/k8s/backend.go`,
  `lazy-tcp-proxy/internal/k8s/backend_test.go`.

## Implementation Notes

### Bug 1 fix (one line added to WatchEvents):

```go
case watch.Added, watch.Modified:
    info, err := b.deploymentToTargetInfo(*d)
    if err != nil { ... }
    b.storeServiceName(id, d.Annotations)
    handler.RegisterTarget(info)
    if info.Running {
        handler.ContainerStarted(id)
    } else {
        handler.ContainerStopped(id)  // ← add this
    }
```

Calling `ContainerStopped` is safe even for `Added` events where
`ReadyReplicas == 0` (a newly-discovered scaled-to-zero Deployment) — the proxy
server's `ContainerStopped` implementation checks `ts.running` before cascading
a stop, so it will no-op for containers that are already marked as stopped.

### Bug 2 fix (move backoff reset inside the normal-close handler):

```go
// Before:
backoff = time.Second   // ← reset here (before the inner for-range loop)
for event := range watcher.ResultChan() { ... }
// channel closed — reconnect
log.Printf("k8s: watch channel closed; reconnecting in %s", backoff)
time.Sleep(backoff)
backoff *= 2            // ← grows even on normal close

// After:
for event := range watcher.ResultChan() { ... }
// channel closed normally — reset backoff and reconnect immediately
backoff = time.Second   // ← reset here instead
log.Printf("k8s: watch channel closed; reconnecting...")
// no sleep needed for normal close
```

Actually, sleeping briefly (1s) on normal close is still desirable to avoid a
tight reconnect loop if the API server keeps closing immediately. Keep a short
sleep but reset backoff so it doesn't accumulate:

```go
for event := range watcher.ResultChan() { ... }
select {
case <-ctx.Done():
    return
default:
    log.Printf("k8s: watch channel closed; reconnecting...")
    time.Sleep(time.Second) // brief pause, not exponential
    backoff = time.Second   // reset for next error sequence
}
```
