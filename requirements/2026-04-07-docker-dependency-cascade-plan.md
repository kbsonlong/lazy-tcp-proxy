# Dependency Cascade — Implementation Plan

**Requirement**: [2026-04-07-docker-dependency-cascade.md](2026-04-07-docker-dependency-cascade.md)
**Date**: 2026-04-07
**Status**: Implemented

---

## Implementation Steps

1. **Add `ParseDependants` helper to `internal/types/types.go`**
   Parse a comma-separated list of target names (e.g.
   `"selenium-chromium,selenium-firefox"`) into `[]string`. Whitespace is
   trimmed; blank tokens are skipped.

2. **Add `Dependants []string` to `types.TargetInfo`** (`internal/types/types.go`)
   New optional field alongside existing `AllowList`, `BlockList`, etc.

3. **Add `ContainerStarted(containerID string)` to `types.TargetHandler`** (`internal/types/types.go`)
   Parallel to the existing `ContainerStopped`. Signals that a managed target
   has just transitioned to running, so the proxy server can cascade-start its
   dependants.

4. **Parse `lazy-tcp-proxy.dependants` in the Docker manager** (`internal/docker/manager.go`)
   In `containerToTargetInfo`, read the optional label
   `lazy-tcp-proxy.dependants` and populate `info.Dependants` via
   `types.ParseDependants`.

5. **Call `handler.ContainerStarted` from Docker `WatchEvents`** (`internal/docker/manager.go`)
   In the `"start"` case, after the existing `handler.RegisterTarget(info)`
   call, add `handler.ContainerStarted(msg.Actor.ID)`.

6. **Parse `lazy-tcp-proxy.dependants` in the k8s backend** (`internal/k8s/backend.go`)
   In `deploymentToTargetInfo`, read the optional annotation
   `lazy-tcp-proxy.dependants` and populate `info.Dependants` via
   `types.ParseDependants`.

7. **Call `handler.ContainerStarted` from k8s `WatchEvents`** (`internal/k8s/backend.go`)
   In the `watch.Added` / `watch.Modified` case, after `handler.RegisterTarget(info)`,
   add `handler.ContainerStarted(id)` when `info.Running == true`. Because
   `EnsureRunning` is already idempotent, spurious calls on annotation-only
   updates are harmless.

8. **Add `nameToID` lookup map to `ProxyServer`** (`internal/proxy/server.go`)
   ```go
   nameToID map[string]string // ContainerName → ContainerID
   ```
   Initialised in `NewServer`. Updated in `RegisterTarget` (add/overwrite) and
   `RemoveTarget` (delete). Enables O(1) cascade lookup by dependant name.

9. **Update `RegisterTarget` to maintain `nameToID`** (`internal/proxy/server.go`)
   After the existing port-listener registration, add:
   ```go
   s.nameToID[info.ContainerName] = info.ContainerID
   ```

10. **Update `RemoveTarget` to maintain `nameToID`** (`internal/proxy/server.go`)
    After closing listeners, delete `s.nameToID[ts.info.ContainerName]` for
    each removed target (only when no other port mappings for that container
    remain).

11. **Implement `ProxyServer.ContainerStarted`** (`internal/proxy/server.go`)
    - Under read-lock, find the `TargetInfo` for the given containerID (scan
      `s.targets` for a matching ID).
    - If the target has non-empty `Dependants`, launch
      `go s.cascadeStart(info)`.

12. **Update `ProxyServer.ContainerStopped` to cascade stop** (`internal/proxy/server.go`)
    After the existing `running = false` loop, find the `TargetInfo` for the
    container and launch `go s.cascadeStop(info)` if it has `Dependants`.

13. **Update `checkInactivity` to cascade stop** (`internal/proxy/server.go`)
    After a successful `s.backend.StopContainer(...)` call inside the
    `e.allIdle` block, store the stopped container's `TargetInfo` and launch
    `go s.cascadeStop(info)`.
    *(Covers the idle-timeout path; step 12 covers the external-stop path.)*

14. **Add `cascadeStart(upstream types.TargetInfo)` to `ProxyServer`** (`internal/proxy/server.go`)
    For each name in `upstream.Dependants`:
    - Under read-lock, resolve `name → containerID` via `s.nameToID`.
    - If not found, log a warning and skip.
    - Log: `proxy: cascade start: <upstream> → <dependant>`.
    - Call `s.backend.EnsureRunning(s.ctx, depID)`.
    - On success, update `running = true` for all matching target/UDP states.

15. **Add `cascadeStop(upstream types.TargetInfo)` to `ProxyServer`** (`internal/proxy/server.go`)
    For each name in `upstream.Dependants`:
    - Under read-lock, resolve `name → containerID` via `s.nameToID`.
    - If not found, log a warning and skip.
    - If all matching target/UDP states already have `running = false`, skip (no-op).
    - Log: `proxy: cascade stop: <upstream> → <dependant>`.
    - Call `s.backend.StopContainer(s.ctx, depID, name)`.
    - On success, update `running = false` for all matching target/UDP states.

16. **Update `README.md`**
    Document the `lazy-tcp-proxy.dependants` label/annotation under the
    configuration reference table, with Docker and Kubernetes examples.

17. **Add unit tests for `ParseDependants`** (`internal/types/types_test.go`)
    See test table below.

18. **Add unit tests for cascade start / stop** (`internal/proxy/server_test.go`)
    Extend `mockBackend` with a `startFunc` field to observe `EnsureRunning`
    calls. Add cascade test cases (see table below).

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/types/types.go` | Modify | Add `Dependants []string` to `TargetInfo`; add `ParseDependants`; add `ContainerStarted` to `TargetHandler` |
| `internal/types/types_test.go` | Modify | Add `ParseDependants` unit tests |
| `internal/docker/manager.go` | Modify | Parse `lazy-tcp-proxy.dependants` label; call `ContainerStarted` from `WatchEvents` |
| `internal/k8s/backend.go` | Modify | Parse `lazy-tcp-proxy.dependants` annotation; call `ContainerStarted` from `WatchEvents` |
| `internal/proxy/server.go` | Modify | Add `nameToID`; maintain in `RegisterTarget`/`RemoveTarget`; implement `ContainerStarted`; update `ContainerStopped` and `checkInactivity`; add `cascadeStart`/`cascadeStop` |
| `internal/proxy/server_test.go` | Modify | Extend mock with `startFunc`; add cascade unit tests |
| `README.md` | Modify | Document `lazy-tcp-proxy.dependants` label/annotation |

---

## API Contracts

### `types.TargetHandler` (updated interface)

```go
type TargetHandler interface {
    RegisterTarget(info TargetInfo)
    RemoveTarget(containerID string)
    ContainerStopped(containerID string)
    ContainerStarted(containerID string)   // NEW
}
```

### `types.TargetInfo` (updated struct, new field)

```go
type TargetInfo struct {
    // ... existing fields ...
    Dependants []string // names of managed targets to start/stop alongside this one
}
```

### Label / Annotation

| Backend | Key | Example value |
|---------|-----|---------------|
| Docker (label) | `lazy-tcp-proxy.dependants` | `selenium-chromium,selenium-firefox` |
| Kubernetes (annotation) | `lazy-tcp-proxy.dependants` | `selenium-chromium,selenium-firefox` |

---

## Key Code Snippets

### `ParseDependants`

```go
func ParseDependants(s string) []string {
    var names []string
    for _, token := range strings.Split(s, ",") {
        name := strings.TrimSpace(token)
        if name != "" {
            names = append(names, name)
        }
    }
    return names
}
```

### `cascadeStart` sketch

```go
func (s *ProxyServer) cascadeStart(upstream types.TargetInfo) {
    for _, depName := range upstream.Dependants {
        s.mu.RLock()
        depID, ok := s.nameToID[depName]
        s.mu.RUnlock()

        if !ok {
            log.Printf("proxy: cascade start: \033[33m%s\033[0m → %q not registered, skipping",
                upstream.ContainerName, depName)
            continue
        }
        log.Printf("proxy: cascade start: \033[33m%s\033[0m → \033[33m%s\033[0m",
            upstream.ContainerName, depName)

        if err := s.backend.EnsureRunning(s.ctx, depID); err != nil {
            log.Printf("proxy: cascade start: error starting \033[33m%s\033[0m: %v", depName, err)
            continue
        }
        s.mu.RLock()
        for _, ts := range s.targets {
            if ts.info.ContainerID == depID {
                ts.running = true
            }
        }
        for _, uls := range s.udpTargets {
            if uls.info.ContainerID == depID {
                uls.running = true
            }
        }
        s.mu.RUnlock()
    }
}
```

---

## Unit Tests

### `ParseDependants` (`types_test.go`)

| Test | Input | Expected |
|------|-------|----------|
| Single name | `"selenium-chromium"` | `["selenium-chromium"]` |
| Multiple names | `"a,b,c"` | `["a","b","c"]` |
| Whitespace trimmed | `" a , b "` | `["a","b"]` |
| Empty string | `""` | `nil` |
| Blank token skipped | `",a,"` | `["a"]` |

### Cascade (`server_test.go`)

| Test | Setup | Expected |
|------|-------|----------|
| cascadeStart starts dependant | hub Dependants=["chrome"], chrome registered; ContainerStarted(hubID) | EnsureRunning called with chromeID |
| cascadeStart skips unknown name | Dependants=["unknown"] | no EnsureRunning call, warning logged |
| cascadeStart no-op if no dependants | ContainerStarted for container with empty Dependants | no EnsureRunning call |
| cascadeStop stops dependant | hub Dependants=["chrome"], chrome running; ContainerStopped(hubID) | StopContainer called with chromeID |
| cascadeStop skips already-stopped | chrome not running | no StopContainer call |
| cascadeStop triggered by checkInactivity | hub idle, Dependants=["chrome"] | after hub stop, StopContainer called for chrome |
| ContainerStarted for unknown ID | containerID not in targets | no cascade, no panic |

---

## Risks & Open Questions

- **`TargetHandler` interface change**: adding `ContainerStarted` is a breaking
  change for any external implementors. The existing test mocks must be updated.
  No external users are known so impact is limited to internal test files.
- **k8s spurious start events**: every `Modified` event triggers
  `ContainerStarted`; cascade calls `EnsureRunning` on dependants that may
  already be running. Safe (idempotent) but adds extra k8s API calls. A future
  optimisation could track previous replica count and only fire on 0→1
  transitions.
- **Transitive chains**: not implemented in this version.
