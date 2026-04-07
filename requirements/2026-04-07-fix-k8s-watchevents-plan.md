# Fix Kubernetes WatchEvents Gaps — Implementation Plan

**Requirement**: [2026-04-07-fix-k8s-watchevents.md](2026-04-07-fix-k8s-watchevents.md)
**Date**: 2026-04-07
**Status**: Draft

## Implementation Steps

### Step 1 — Bug 1: call `ContainerStopped` on scale-to-zero (`backend.go` lines 158–160)

**Current** (`internal/k8s/backend.go:158`):

```go
handler.RegisterTarget(info)
if info.Running {
    handler.ContainerStarted(id)
}
log.Printf("k8s: event: deployment updated: \033[33m%s\033[0m", d.Name)
```

**Replace with**:

```go
handler.RegisterTarget(info)
if info.Running {
    handler.ContainerStarted(id)
} else {
    handler.ContainerStopped(id)
}
log.Printf("k8s: event: deployment updated: \033[33m%s\033[0m", d.Name)
```

This is safe for all three cases:
- **Newly discovered deployment at 0 replicas** (`Added` event, `Running: false`):
  `ContainerStopped` is called, but `ProxyServer.ContainerStopped` checks
  `ts.running` before cascading — it no-ops because the target was never marked
  running.
- **External scale-to-zero** (`Modified` event, `Running: false`): cascade stop
  propagates to dependants immediately. ✅
- **Scale-up** (`Modified` event, `Running: true`): hits the `if` branch as
  before, unchanged. ✅

---

### Step 2 — Bug 2: reset backoff on normal channel close (`backend.go` lines 168–179)

**Current**:

```go
// ResultChan closed — reconnect
select {
case <-ctx.Done():
    return
default:
    log.Printf("k8s: watch channel closed; reconnecting in %s", backoff)
    time.Sleep(backoff)
    backoff *= 2
    if backoff > maxBackoff {
        backoff = maxBackoff
    }
}
```

**Replace with**:

```go
// ResultChan closed normally — reset backoff and reconnect promptly
select {
case <-ctx.Done():
    return
default:
    backoff = time.Second
    log.Printf("k8s: watch channel closed; reconnecting...")
    time.Sleep(backoff)
}
```

The 1s sleep is kept to avoid a tight spin loop if the API server closes
immediately and repeatedly (e.g. network flap). Backoff is reset to `time.Second`
so the error-backoff sequence (in the `err != nil` branch above) always starts
fresh from 1s for the next actual error.

---

### Step 3 — Add unit tests (`backend_test.go`)

Two new test functions:

**`TestWatchEvents_ModifiedScaleToZeroCallsContainerStopped`**

```
1. Create a running fakeDeployment (ReadyReplicas=1).
2. Send a Modified event with ReadyReplicas set to 0.
3. Assert captureHandler.stopped contains the deployment's targetID.
```

**`TestWatchEvents_ModifiedStillRunningDoesNotCallContainerStopped`**

```
1. Create a running fakeDeployment (ReadyReplicas=1).
2. Send a Modified event that keeps ReadyReplicas=1 (e.g. annotation change).
3. Assert captureHandler.stopped is empty.
```

Bug 2 (backoff reset) is verified by code review rather than a unit test —
timing-based assertions are brittle and the change is a one-liner. The existing
`TestWatchEvents_AddedTriggersRegister` and `TestWatchEvents_DeletedTriggersRemove`
already exercise the reconnect path implicitly.

---

### Step 4 — Mark REQ-051 Completed

Update status in `requirements/2026-04-07-fix-k8s-watchevents.md` and
`requirements/_index.md`.

---

## File-Level Change Summary

| File | Change |
|------|--------|
| `lazy-tcp-proxy/internal/k8s/backend.go` | +3 lines (else branch) + ~8 lines (reconnect block rewrite) |
| `lazy-tcp-proxy/internal/k8s/backend_test.go` | +~40 lines (2 new test functions) |
| `requirements/2026-04-07-fix-k8s-watchevents.md` | Status → Completed |
| `requirements/_index.md` | Status → Completed |

Total: 4 files, net ~50 lines added/changed.

---

## Risk Assessment

**Very low.** Both changes are confined to `WatchEvents`:
- Bug 1 adds an `else` branch — the `if` branch is unchanged.
- Bug 2 simplifies the normal-close handler — the error-path backoff is
  unchanged.
- The `ProxyServer.ContainerStopped` implementation is already idempotent (no-ops
  if `ts.running == false`), so the new `else` call is safe for all event
  orderings.
