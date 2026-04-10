# Cron-Based Scheduling (Docker & Kubernetes) — Implementation Plan

**Requirement**: [2026-04-06-cron-scheduling.md](2026-04-06-cron-scheduling.md)
**Date**: 2026-04-07
**Status**: Implemented

## Implementation Steps

1. **Add `robfig/cron/v3` dependency** — run `go get github.com/robfig/cron/v3` inside `lazy-tcp-proxy/` and commit updated `go.mod` and `go.sum`.

2. **Extend `TargetInfo` in `internal/types/types.go`** — add two string fields `CronStart` and `CronStop` (empty = not scheduled). Both backends use the same field names; their parsers are in separate files.

3. **Parse cron labels in `internal/docker/manager.go`** — in `containerToTargetInfo()`, read `lazy-tcp-proxy.cron-start` and `lazy-tcp-proxy.cron-stop` labels. Validate each with `cron.ParseStandard(expr)` from `robfig/cron/v3`; log a warning and set the field to `""` if invalid.

4. **Parse cron annotations in `internal/k8s/backend.go`** — in `deploymentToTargetInfo()`, apply the same logic using the same annotation keys. Share the validation helper if possible.

5. **Add `CronActions` interface and proxy-server methods in `internal/proxy/server.go`**:
   - Define a `CronActions` interface (used by the scheduler to avoid a circular import):
     ```go
     type CronActions interface {
         CronStart(ctx context.Context, targetID, targetName string)
         CronStop(ctx context.Context, targetID, targetName string)
     }
     ```
   - Implement `(*ProxyServer).CronStart`: look up the target by `containerID`; if `running == true` log "already running, skipping"; otherwise call `backend.EnsureRunning`, set `ts.running = true`, fire `container_started` webhook, cascade-start dependants.
   - Implement `(*ProxyServer).CronStop`: look up target by `containerID`; if `running == false` log "already stopped, skipping"; otherwise call `backend.StopContainer`, set `ts.running = false`, fire `container_stopped` webhook, cascade-stop dependants.

6. **Exempt cron-scheduled targets from the inactivity checker in `internal/proxy/server.go`** — in `checkInactivity()`, add an early-continue for any target whose `info.CronStart != "" || info.CronStop != ""`.

7. **Create `internal/scheduler/scheduler.go`** — new package wrapping `robfig/cron`:
   ```go
   type Scheduler struct {
       cron    *cron.Cron
       mu      sync.Mutex
       entries map[string][]cron.EntryID  // targetID → entry IDs
       actions CronActions                // proxy server
       ctx     context.Context
   }

   func New(ctx context.Context, actions CronActions) *Scheduler
   func (s *Scheduler) Register(info types.TargetInfo)
   func (s *Scheduler) Unregister(targetID string)
   func (s *Scheduler) Start()
   func (s *Scheduler) Stop()
   ```
   - `Register` adds up to two cron entries (start + stop) for a target; each job calls `actions.CronStart` or `actions.CronStop` with the stored `targetID` and `targetName`.
   - If either expression is invalid at registration time, log a warning and skip that entry (container is still managed, the other schedule still applies if valid).
   - `Unregister` removes all entry IDs for a target and deletes from the map.
   - `Stop` calls `c.cron.Stop()`.

8. **Hook scheduler into `RegisterTarget` / `RemoveTarget` in `internal/proxy/server.go`** — the `ProxyServer` holds an optional `*scheduler.Scheduler`; if non-nil, `RegisterTarget` calls `sched.Register(info)` and `RemoveTarget` calls `sched.Unregister(containerID)` after closing listeners.

9. **Wire scheduler in `main.go`**:
   - After creating `srv`, construct `sched := scheduler.New(ctx, srv)` and call `sched.Start()`.
   - Pass `sched` into `srv` via a setter or constructor parameter before `Discover` is called (so initial discovery registers schedules).
   - On shutdown, `sched.Stop()` is called (the `robfig/cron` Stop method drains running jobs).

10. **Update `README.md`** — add a "Cron Scheduling" section documenting both labels, the 5-field cron format, the idle-timeout exemption, and separate Docker / Kubernetes examples.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/go.mod` | Modify | Add `github.com/robfig/cron/v3` |
| `lazy-tcp-proxy/go.sum` | Modify | Updated checksums |
| `lazy-tcp-proxy/internal/types/types.go` | Modify | Add `CronStart`, `CronStop string` to `TargetInfo` |
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Parse `cron-start`/`cron-stop` labels in `containerToTargetInfo` |
| `lazy-tcp-proxy/internal/k8s/backend.go` | Modify | Parse `cron-start`/`cron-stop` annotations in `deploymentToTargetInfo` |
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `CronActions` interface; implement `CronStart`/`CronStop`; exempt cron targets in `checkInactivity`; hold `*scheduler.Scheduler` |
| `lazy-tcp-proxy/internal/scheduler/scheduler.go` | Create | New package: `Scheduler` type wrapping `robfig/cron` |
| `lazy-tcp-proxy/main.go` | Modify | Instantiate scheduler, wire into proxy server before `Discover` |
| `README.md` | Modify | Document `cron-start` and `cron-stop` labels |

## Key Code Snippets

### TargetInfo additions (`internal/types/types.go`)
```go
type TargetInfo struct {
    // ... existing fields ...
    CronStart string  // robfig/cron 5-field expression; "" = not scheduled
    CronStop  string  // robfig/cron 5-field expression; "" = not scheduled
}
```

### Cron label parsing (shared pattern for both backends)
```go
func parseCronLabel(name, key, raw string) string {
    v := strings.TrimSpace(raw)
    if v == "" {
        return ""
    }
    if _, err := cron.ParseStandard(v); err != nil {
        log.Printf("docker: container %s: ignoring invalid %s %q: %v", name, key, v, err)
        return ""
    }
    return v
}
```

### Inactivity checker exemption (`internal/proxy/server.go`)
```go
// Inside checkInactivity, early in the per-target loop:
if ts.info.CronStart != "" || ts.info.CronStop != "" {
    continue // lifecycle managed by cron scheduler
}
```

### Scheduler core (`internal/scheduler/scheduler.go`)
```go
func (s *Scheduler) Register(info types.TargetInfo) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // Remove any existing entries for this target
    s.unregisterLocked(info.ContainerID)

    var ids []cron.EntryID
    if info.CronStart != "" {
        id, _ := s.cron.AddFunc(info.CronStart, func() {
            s.actions.CronStart(s.ctx, info.ContainerID, info.ContainerName)
        })
        ids = append(ids, id)
    }
    if info.CronStop != "" {
        id, _ := s.cron.AddFunc(info.CronStop, func() {
            s.actions.CronStop(s.ctx, info.ContainerID, info.ContainerName)
        })
        ids = append(ids, id)
    }
    if len(ids) > 0 {
        s.entries[info.ContainerID] = ids
        log.Printf("scheduler: registered \033[33m%s\033[0m (start=%q stop=%q)",
            info.ContainerName, info.CronStart, info.CronStop)
    }
}
```

### ProxyServer.CronStart
```go
func (s *ProxyServer) CronStart(ctx context.Context, targetID, targetName string) {
    s.mu.RLock()
    ts := s.findTargetByID(targetID) // returns first targetState for this containerID
    s.mu.RUnlock()
    if ts == nil {
        log.Printf("scheduler: cron-start: target \033[33m%s\033[0m not found", targetName)
        return
    }
    if ts.running {
        log.Printf("scheduler: cron-start: \033[33m%s\033[0m already running, no action", targetName)
        return
    }
    if err := s.backend.EnsureRunning(ctx, targetID); err != nil {
        log.Printf("scheduler: cron-start: failed to start \033[33m%s\033[0m: %v", targetName, err)
        return
    }
    s.mu.Lock()
    ts.running = true
    s.mu.Unlock()
    s.fireWebhook(ts.info.WebhookURL, "container_started", targetID, targetName, nil)
    s.cascadeStart(ts.info)
}
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| `TestParseCronLabel_valid` | `"30 8 * * 1-5"` | returns `"30 8 * * 1-5"` |
| `TestParseCronLabel_invalid` | `"not-a-cron"` | returns `""`, logs warning |
| `TestParseCronLabel_empty` | `""` | returns `""`, no log |
| `TestScheduler_Register_start` | TargetInfo with CronStart set | one cron entry created |
| `TestScheduler_Register_both` | TargetInfo with both set | two cron entries created |
| `TestScheduler_Unregister` | Register then Unregister | entries map empty, cron entries removed |
| `TestScheduler_Register_replaces` | Register same targetID twice | old entries removed, new ones added |
| `TestCheckInactivity_skips_cron` | targetState with CronStop set, idle, running | StopContainer NOT called |
| `TestCheckInactivity_stops_normal` | targetState with no cron, idle, running | StopContainer called |
| `TestCronStart_already_running` | ts.running = true | EnsureRunning NOT called, log emitted |
| `TestCronStart_stopped` | ts.running = false | EnsureRunning called, ts.running set true |
| `TestCronStop_already_stopped` | ts.running = false | StopContainer NOT called, log emitted |
| `TestCronStop_running` | ts.running = true | StopContainer called, ts.running set false |

## Risks & Open Questions

- **`robfig/cron` entry IDs after `Stop()`**: `robfig/cron` v3's `Remove(id)` is safe to call after `Stop()`. The cron instance must be re-`Start()`-ed if needed after a stop; since we only stop on shutdown, this is not a concern.
- **Re-registration on `RegisterTarget` update**: If a container's annotations change (e.g. a Kubernetes Deployment is modified), `RegisterTarget` is called again. The scheduler's `Register` method must remove old entries before adding new ones (handled via `unregisterLocked` before re-adding).
- **Import cycle risk**: `scheduler` imports `types` and takes a `CronActions` interface (not `*proxy.ProxyServer` directly). `proxy` imports `scheduler` to hold the reference. `main` wires them together. No cycle.
- **Timezone**: `robfig/cron` v3 uses `time.Local` by default, which respects the `TZ` env var. No additional configuration needed.
