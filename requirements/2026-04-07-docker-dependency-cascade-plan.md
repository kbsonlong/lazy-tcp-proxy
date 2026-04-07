# Docker Dependency Cascade — Implementation Plan

**Requirement**: [2026-04-07-docker-dependency-cascade.md](2026-04-07-docker-dependency-cascade.md)
**Date**: 2026-04-07
**Status**: Draft

---

## Implementation Steps

1. **Add `parseDependsOn` helper to `manager.go`**
   Parse `com.docker.compose.depends_on` label value into a slice of upstream
   service names. Format: `svc1:condition:required[,svc2:condition:required]`.
   Only the first (service-name) field of each colon-separated token is kept.
   Empty or blank values return nil.

2. **Add `DependsOn []string` to `TargetInfo`** (`manager.go`)
   New field alongside existing `AllowList`, `BlockList`, etc.

3. **Populate `DependsOn` in `containerToTargetInfo`** (`manager.go`)
   Read optional label `com.docker.compose.depends_on`; call `parseDependsOn`
   if present. No error if the label is absent.

4. **Add `ContainerStarted(containerID string)` to `TargetHandler` interface** (`manager.go`)
   Parallel to the existing `ContainerStopped`. Called after a managed
   container's Docker `start` event is processed.

5. **Call `handler.ContainerStarted` from `WatchEvents`** (`manager.go`)
   In the `"start"` case, after the existing `handler.RegisterTarget(info)`
   call, add `handler.ContainerStarted(msg.Actor.ID)`.

6. **Add `depMap` and `containerDeps` to `ProxyServer`** (`server.go`)
   ```go
   depMap       map[string][]string // upstreamName → []downstreamContainerID
   containerDeps map[string][]string // containerID  → []upstreamNames it depends on
   ```
   Both initialised to empty maps in `NewServer`.

7. **Update `RegisterTarget` to maintain the dependency maps** (`server.go`)
   At the end of `RegisterTarget`, after all port listeners are registered:
   - If `containerID` is already in `containerDeps`, remove its old
     `depMap` entries (handles re-registration with changed deps).
   - For each name in `info.DependsOn`, append `info.ContainerID` to
     `depMap[name]` (deduplicating).
   - Overwrite `containerDeps[info.ContainerID]` with `info.DependsOn`.

8. **Update `RemoveTarget` to clean up dependency maps** (`server.go`)
   After deleting port listeners, for each upstream name in
   `containerDeps[containerID]`, remove `containerID` from
   `depMap[upstreamName]`. Then delete `containerDeps[containerID]`.

9. **Implement `ProxyServer.ContainerStarted`** (`server.go`)
   - Under read-lock, find the `ContainerName` for the given `containerID`
     across `s.targets` (or `s.udpTargets`).
   - If found and there are entries in `depMap[name]`, launch
     `go s.cascadeStart(name)`.

10. **Update `ProxyServer.ContainerStopped` to cascade stop** (`server.go`)
    After the existing `running = false` loop, look up the container name
    from any matching target and launch `go s.cascadeStop(name)`.

11. **Update `checkInactivity` to cascade stop** (`server.go`)
    After a successful `s.docker.StopContainer(...)` call inside the
    `e.allIdle` block, add `go s.cascadeStop(e.name)`.
    (Both paths — idle timeout and external die event — are now covered by
    steps 10 and 11.)

12. **Add `cascadeStart(upstreamName string)` to `ProxyServer`** (`server.go`)
    ```
    func (s *ProxyServer) cascadeStart(upstreamName string)
    ```
    - Under read-lock, collect the []containerID from `depMap[upstreamName]`.
    - For each downstream containerID: find its name from targets, log the
      action, call `s.docker.EnsureRunning(s.ctx, containerID)`, and on
      success update `running = true` in all matching target/UDP states.

13. **Add `cascadeStop(upstreamName string)` to `ProxyServer`** (`server.go`)
    ```
    func (s *ProxyServer) cascadeStop(upstreamName string)
    ```
    - Under read-lock, collect []containerID from `depMap[upstreamName]`.
    - For each downstream containerID: find its name and check `running`; skip
      if already stopped. Log the action, call
      `s.docker.StopContainer(s.ctx, containerID, name)`, and on success update
      `running = false` in all matching target/UDP states.

14. **Add unit tests for `parseDependsOn`** (`manager_test.go`)
    See test table below.

15. **Add unit tests for depMap management** (`server_test.go`)
    - `RegisterTarget` populates `depMap` from `DependsOn`.
    - `RegisterTarget` handles re-registration (old entries removed, new added).
    - `RemoveTarget` cleans up depMap and containerDeps.

16. **Add unit tests for cascade start / stop** (`server_test.go`)
    Uses extended `mockDockerManager` with a `startFunc` field.
    See test table below.

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/docker/manager.go` | Modify | Add `DependsOn` to `TargetInfo`; add `parseDependsOn`; extend `TargetHandler`; trigger `ContainerStarted` in `WatchEvents` |
| `internal/proxy/server.go` | Modify | Add `depMap`/`containerDeps`; `RegisterTarget`/`RemoveTarget` maintenance; implement `ContainerStarted`; update `ContainerStopped`; update `checkInactivity`; add `cascadeStart`/`cascadeStop` |
| `internal/docker/manager_test.go` | Modify | Add `parseDependsOn` unit tests |
| `internal/proxy/server_test.go` | Modify | Extend `mockDockerManager` with `startFunc`; add depMap and cascade unit tests |

---

## Key Code Snippets

### `parseDependsOn`

```go
// parseDependsOn parses the com.docker.compose.depends_on label value into a
// slice of upstream service names. Each token has the form
// "service:condition:required"; only the service name is extracted.
func parseDependsOn(s string) []string {
    var names []string
    for _, token := range strings.Split(s, ",") {
        parts := strings.SplitN(strings.TrimSpace(token), ":", 2)
        name := strings.TrimSpace(parts[0])
        if name != "" {
            names = append(names, name)
        }
    }
    return names
}
```

### `cascadeStart` sketch

```go
func (s *ProxyServer) cascadeStart(upstreamName string) {
    s.mu.RLock()
    ids := append([]string(nil), s.depMap[upstreamName]...)
    // collect name lookup while holding lock
    nameFor := map[string]string{}
    for _, id := range ids {
        for _, ts := range s.targets {
            if ts.info.ContainerID == id {
                nameFor[id] = ts.info.ContainerName
            }
        }
    }
    s.mu.RUnlock()

    for _, id := range ids {
        name := nameFor[id]
        log.Printf("proxy: cascade start: \033[33m%s\033[0m → \033[33m%s\033[0m", upstreamName, name)
        if err := s.docker.EnsureRunning(s.ctx, id); err != nil {
            log.Printf("proxy: cascade start: error starting \033[33m%s\033[0m: %v", name, err)
            continue
        }
        s.mu.RLock()
        for _, ts := range s.targets {
            if ts.info.ContainerID == id { ts.running = true }
        }
        for _, uls := range s.udpTargets {
            if uls.info.ContainerID == id { uls.running = true }
        }
        s.mu.RUnlock()
    }
}
```

### `RegisterTarget` depMap update (append to end of method, before return)

```go
// Update dependency maps.
s.mu is already held (write lock).
// Remove stale entries for this container.
for _, upName := range s.containerDeps[info.ContainerID] {
    prev := s.depMap[upName]
    filtered := prev[:0]
    for _, cid := range prev {
        if cid != info.ContainerID {
            filtered = append(filtered, cid)
        }
    }
    s.depMap[upName] = filtered
}
// Add new entries.
s.containerDeps[info.ContainerID] = info.DependsOn
for _, upName := range info.DependsOn {
    // deduplicate
    already := false
    for _, cid := range s.depMap[upName] {
        if cid == info.ContainerID { already = true; break }
    }
    if !already {
        s.depMap[upName] = append(s.depMap[upName], info.ContainerID)
    }
}
```

---

## Unit Tests

### `parseDependsOn` (manager_test.go)

| Test | Input | Expected |
|------|-------|----------|
| Single dependency | `"hub:service_started:false"` | `["hub"]` |
| Multiple dependencies | `"hub:service_started:false,db:service_healthy:true"` | `["hub", "db"]` |
| Whitespace around token | `" hub : service_started : false "` | `["hub"]` |
| Empty string | `""` | `nil` |
| Blank token skipped | `",hub:service_started:false"` | `["hub"]` |

### depMap management (server_test.go)

| Test | Setup | Expected |
|------|-------|----------|
| RegisterTarget populates depMap | Register container with DependsOn=["hub"] | depMap["hub"] == [containerID] |
| RegisterTarget deduplicates | Register same container twice | depMap["hub"] has no duplicates |
| RegisterTarget re-register removes old dep | Register with DependsOn=["hub"], re-register with DependsOn=["db"] | depMap["hub"] empty, depMap["db"]==[containerID] |
| RemoveTarget cleans depMap | Register then remove | depMap["hub"] empty, containerDeps empty |

### Cascade (server_test.go)

| Test | Setup | Expected |
|------|-------|----------|
| cascadeStart starts downstream | hub→[chrome], call ContainerStarted(hubID) | EnsureRunning called with chromeID |
| cascadeStart no-op if no dependents | ContainerStarted for container with no dependents | EnsureRunning not called |
| cascadeStop stops downstream | hub→[chrome], chrome running, call cascadeStop("hub") | StopContainer called with chromeID |
| cascadeStop skips already-stopped | hub→[chrome], chrome not running | StopContainer not called |
| cascadeStop triggered from checkInactivity | hub idle, hub→[chrome] | after hub stop, StopContainer called for chrome |
| unmanaged container ContainerStarted | ContainerStarted for unknown containerID | no cascade |

---

## Risks & Open Questions

- **Race on `running` field**: `cascadeStart`/`cascadeStop` update `ts.running`
  under `RLock` (acceptable since `running` is a plain bool and we write under
  read-lock as the rest of the code does). If this becomes a correctness concern
  a future refactor can promote it to `atomic.Bool`.
- **Transitive chains**: not implemented in this version. A future REQ can add
  recursion with cycle detection.
- **External start/stop**: if the hub is started externally (not via proxy
  traffic), the Docker `start` event triggers `ContainerStarted` → cascade.
  This is correct behaviour.
