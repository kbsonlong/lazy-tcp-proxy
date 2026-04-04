# Per-Container Idle Timeout Label Override — Implementation Plan

**Requirement**: [2026-04-03-idle-timeout-label-override.md](2026-04-03-idle-timeout-label-override.md)
**Date**: 2026-04-03
**Status**: Draft

## Implementation Steps

1. **`docker/manager.go` — add `IdleTimeout` to `TargetInfo`**
   Add `IdleTimeout *time.Duration` field. A `nil` pointer means "use global default";
   a non-nil pointer (including `0`) means "use this value for this container".

2. **`docker/manager.go` — parse label in `containerToTargetInfo()`**
   After the existing `block-list` parse block, add:
   ```go
   var idleTimeout *time.Duration
   if v, ok := inspect.Config.Labels["lazy-tcp-proxy.idle-timeout-secs"]; ok && v != "" {
       n, err := strconv.Atoi(strings.TrimSpace(v))
       if err != nil || n < 0 {
           log.Printf("docker: container %s: ignoring invalid idle-timeout-secs %q", name, v)
       } else {
           d := time.Duration(n) * time.Second
           idleTimeout = &d
       }
   }
   ```
   Add `IdleTimeout: idleTimeout` to the returned `TargetInfo`.

3. **`proxy/server.go` — add `idleTimeout *time.Duration` to `targetState`**
   Add the field to the struct.

4. **`proxy/udp.go` — add `idleTimeout *time.Duration` to `udpListenerState`**
   Add the field to the struct.

5. **`proxy/server.go` — add `effectiveTimeout` helper**
   ```go
   func effectiveTimeout(perContainer *time.Duration, global time.Duration) time.Duration {
       if perContainer != nil {
           return *perContainer
       }
       return global
   }
   ```

6. **`proxy/server.go` — `RegisterTarget()`: copy `IdleTimeout` on create and update**
   - When creating a new `targetState`, set `idleTimeout: info.IdleTimeout`.
   - In the existing-target update block, also set `existing.idleTimeout = info.IdleTimeout`.
   - Same for `udpListenerState` creation and update.

7. **`proxy/server.go` — `checkInactivity()`: use per-container effective timeout**
   Replace the two occurrences of `s.idleTimeout` in the idle condition:
   ```go
   // TCP
   eff := effectiveTimeout(ts.idleTimeout, s.idleTimeout)
   if !ts.running || ts.activeConns.Load() > 0 || time.Since(ts.lastActive) < eff {
       e.allIdle = false
   }
   // UDP
   eff := effectiveTimeout(uls.idleTimeout, s.idleTimeout)
   if !uls.running || activeFlows > 0 || time.Since(lastActive) < eff {
       e.allIdle = false
   }
   ```

8. **`proxy/udp.go` — `udpFlowSweeper()`: use per-container effective timeout**
   Replace `s.idleTimeout` in the flow expiry comparison:
   ```go
   eff := effectiveTimeout(uls.idleTimeout, s.idleTimeout)
   if now.Sub(flow.lastActive) > eff {
   ```

9. **`proxy/server.go` — `handleConn()`: show effective timeout in log**
   ```go
   eff := effectiveTimeout(ts.idleTimeout, s.idleTimeout)
   if eff == 0 {
       log.Printf("proxy: last connection to \033[33m%s\033[0m closed; idle timer started (container will stop immediately if no new connections)", ts.info.ContainerName)
   } else {
       log.Printf("proxy: last connection to \033[33m%s\033[0m closed; idle timer started (container will stop in ~%s if no new connections)", ts.info.ContainerName, eff)
   }
   ```

10. **`main.go` — `resolveIdleTimeout()`: accept `0` as valid**
    Change `n <= 0` to `n < 0`. When `n == 0`, log clearly:
    ```go
    if n == 0 {
        log.Printf("idle timeout: 0s — containers will stop immediately when all connections close")
        return 0
    }
    ```

11. **`README.md` — document the new label and updated `IDLE_TIMEOUT_SECS` semantics**
    - Add row to the Container Label Configuration table:
      `lazy-tcp-proxy.idle-timeout-secs` | No | Override the global idle timeout for this container only (seconds; `0` = stop immediately when last connection closes)
    - Update `IDLE_TIMEOUT_SECS` env var description to: "How long (in seconds) a container must be idle before being stopped. `0` = stop immediately once all connections close."

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Add `IdleTimeout *time.Duration` to `TargetInfo`; parse `lazy-tcp-proxy.idle-timeout-secs` label |
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `idleTimeout` to `targetState`; add `effectiveTimeout` helper; update `RegisterTarget`, `checkInactivity`, `handleConn` |
| `lazy-tcp-proxy/internal/proxy/udp.go` | Modify | Add `idleTimeout` to `udpListenerState`; update `udpFlowSweeper` |
| `lazy-tcp-proxy/main.go` | Modify | `resolveIdleTimeout()` accepts `0`; startup log updated |
| `README.md` | Modify | Add label row; update `IDLE_TIMEOUT_SECS` description |

## Key Code Snippets

### `effectiveTimeout` helper (server.go)
```go
func effectiveTimeout(perContainer *time.Duration, global time.Duration) time.Duration {
    if perContainer != nil {
        return *perContainer
    }
    return global
}
```

This is the single resolution point used by `checkInactivity`, `udpFlowSweeper`, and `handleConn`.

### Why `0` works without logic changes in `checkInactivity`
`time.Since(x) >= 0` always holds. When `eff == 0`:
`time.Since(ts.lastActive) < 0` → always `false` → does not prevent idle → container is
stopped on next tick once `activeConns == 0`. No special-casing needed in the comparison.

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| `effectiveTimeout` — no override | `nil`, `120s` | `120s` |
| `effectiveTimeout` — override present | `&30s`, `120s` | `30s` |
| `effectiveTimeout` — zero override | `&0s`, `120s` | `0s` |
| `resolveIdleTimeout` — empty | `""` | `120s` |
| `resolveIdleTimeout` — valid | `"60"` | `60s` |
| `resolveIdleTimeout` — zero | `"0"` | `0s` |
| `resolveIdleTimeout` — negative | `"-1"` | `120s` (+ warning) |
| `resolveIdleTimeout` — non-numeric | `"abc"` | `120s` (+ warning) |
| `containerToTargetInfo` — label absent | no label | `IdleTimeout == nil` |
| `containerToTargetInfo` — label valid | `"45"` | `IdleTimeout == &45s` |
| `containerToTargetInfo` — label zero | `"0"` | `IdleTimeout == &0s` |
| `containerToTargetInfo` — label invalid | `"-1"` or `"abc"` | `IdleTimeout == nil` (+ warning) |

## Risks & Open Questions

- `udpFlowSweeper` runs once per poll tick and currently uses `s.idleTimeout` for individual flow
  expiry. After this change it will use the container's effective timeout. This is the correct
  behaviour (consistent with TCP) but is worth noting.
- No backward-compatibility concern: the label is optional and absent-by-default, so existing
  containers are unaffected.
