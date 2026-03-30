# Multi-Port Mappings — Implementation Plan

**Requirement**: [2026-03-30-multi-port-mappings.md](2026-03-30-multi-port-mappings.md)
**Date**: 2026-03-30
**Status**: Implemented

## Implementation Steps

1. **`internal/docker/manager.go` — add `PortMapping` type, update `TargetInfo`**
   - Add `type PortMapping struct { ListenPort, TargetPort int }` above `TargetInfo`.
   - Replace `Port int` field with `Ports []PortMapping` in `TargetInfo`.

2. **`internal/docker/manager.go` — update `containerToTargetInfo`**
   - Remove the `lazy-tcp-proxy.port` label lookup.
   - Read `lazy-tcp-proxy.ports`; return an error if absent.
   - Split on `,`; for each token split on `:` (exactly 2 parts); parse both as `int`; accumulate into `[]PortMapping`.
   - Return an error if any mapping is malformed.

3. **`internal/docker/manager.go` — update event handler validation in `WatchEvents`**
   - Replace the `lazy-tcp-proxy.port` attribute check with `lazy-tcp-proxy.ports`.
   - Validate that the value contains at least one valid `<int>:<int>` mapping; log rejection with the appropriate reason if not.

4. **`internal/proxy/server.go` — add `targetPort` to `targetState`**
   - Add `targetPort int` field to `targetState`.

5. **`internal/proxy/server.go` — update `RegisterTarget`**
   - Iterate `info.Ports`; for each `PortMapping` check if `listenPort` is already in the map.
   - If already present: update metadata (container ID/name, target port), keep listener.
   - If new: open listener, create `targetState` with `targetPort = mapping.TargetPort`, store under `mapping.ListenPort`.

6. **`internal/proxy/server.go` — update `RemoveTarget`**
   - Collect all map keys where `ts.info.ContainerID == containerID` (there may be several), close each listener, delete each key.

7. **`internal/proxy/server.go` — update `handleConn`**
   - Replace `ts.info.Port` with `ts.targetPort` when building the upstream dial address.

8. **`internal/proxy/server.go` — update `checkInactivity`**
   - Group snapshot entries by `ContainerID`.
   - A container is eligible for stopping only when every one of its entries has `activeConns == 0` and `lastActive` older than `idleTimeout`.
   - Call `StopContainer` once per eligible container (using any entry's `ContainerID`).

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/docker/manager.go` | Modify | Steps 1–3 |
| `internal/proxy/server.go` | Modify | Steps 4–8 |
| `requirements/2026-03-30-multi-port-mappings.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## Key Code Snippets

### PortMapping + TargetInfo
```go
type PortMapping struct {
    ListenPort int
    TargetPort int
}

type TargetInfo struct {
    ContainerID   string
    ContainerName string
    Ports         []PortMapping
    NetworkIDs    []string
}
```

### Label parsing
```go
portsStr, ok := inspect.Config.Labels["lazy-tcp-proxy.ports"]
if !ok {
    return TargetInfo{}, fmt.Errorf("missing label lazy-tcp-proxy.ports")
}
var ports []PortMapping
for _, token := range strings.Split(portsStr, ",") {
    parts := strings.SplitN(strings.TrimSpace(token), ":", 2)
    if len(parts) != 2 {
        return TargetInfo{}, fmt.Errorf("invalid port mapping %q: expected <listen>:<target>", token)
    }
    lp, err := strconv.Atoi(strings.TrimSpace(parts[0]))
    if err != nil {
        return TargetInfo{}, fmt.Errorf("invalid listen port in %q: %w", token, err)
    }
    tp, err := strconv.Atoi(strings.TrimSpace(parts[1]))
    if err != nil {
        return TargetInfo{}, fmt.Errorf("invalid target port in %q: %w", token, err)
    }
    ports = append(ports, PortMapping{ListenPort: lp, TargetPort: tp})
}
```

### RegisterTarget (multi-mapping)
```go
func (s *ProxyServer) RegisterTarget(info docker.TargetInfo) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for _, m := range info.Ports {
        if existing, ok := s.targets[m.ListenPort]; ok {
            existing.info = info
            existing.targetPort = m.TargetPort
            existing.removed = false
            log.Printf("proxy: updated target %s on port %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
            continue
        }
        ln, err := net.Listen("tcp", fmt.Sprintf(":%d", m.ListenPort))
        if err != nil {
            log.Printf("proxy: failed to listen on port %d for %s: %v", m.ListenPort, info.ContainerName, err)
            continue
        }
        ts := &targetState{info: info, targetPort: m.TargetPort, listener: ln, lastActive: time.Now()}
        s.targets[m.ListenPort] = ts
        log.Printf("proxy: registered target %s, listening on port %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
        go s.acceptLoop(ts)
    }
}
```

### RemoveTarget (all mappings)
```go
func (s *ProxyServer) RemoveTarget(containerID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for port, ts := range s.targets {
        if ts.info.ContainerID == containerID {
            log.Printf("proxy: removing target %s on port %d", ts.info.ContainerName, port)
            ts.removed = true
            ts.listener.Close()
            delete(s.targets, port)
        }
    }
}
```

### checkInactivity (grouped by container)
```go
// group by container ID
type containerEntry struct {
    containerID string
    name        string
    allIdle     bool
}
byContainer := map[string]*containerEntry{}
for _, ts := range snapshot {
    e, ok := byContainer[ts.info.ContainerID]
    if !ok {
        e = &containerEntry{containerID: ts.info.ContainerID, name: ts.info.ContainerName, allIdle: true}
        byContainer[ts.info.ContainerID] = e
    }
    if ts.activeConns.Load() > 0 || time.Since(ts.lastActive) < idleTimeout {
        e.allIdle = false
    }
}
for _, e := range byContainer {
    if e.allIdle {
        s.docker.StopContainer(ctx, e.containerID)
    }
}
```

### handleConn dial address
```go
addr := fmt.Sprintf("%s:%d", ip, ts.targetPort)  // was ts.info.Port
```

## Risks & Open Questions

- None — design is straightforward. All call sites of `TargetInfo.Port` are inside `proxy/server.go` and are fully covered by the steps above.
