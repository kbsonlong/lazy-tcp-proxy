# Structured Init and Change Logging — Implementation Plan

**Requirement**: [2026-03-30-structured-init-and-change-logging.md](2026-03-30-structured-init-and-change-logging.md)
**Date**: 2026-03-30
**Status**: Approved

## Implementation Steps

1. **Change `JoinNetworks` signature** in `internal/docker/manager.go`:
   - Return type: `error` → `([]string, error)`
   - Collect the name of each network successfully joined into a local `joined []string` slice
   - Return `joined, nil` on success (existing per-join `log.Printf` lines stay as-is)

2. **Update `Discover`** in `internal/docker/manager.go`:
   - Declare `foundNames []string` and `allNetworks []string` before the loop
   - After a successful `containerToTargetInfo`, append `info.ContainerName` to `foundNames`
   - Replace `m.JoinNetworks(ctx, info.NetworkIDs)` call to capture returned names; append to `allNetworks`
   - After the loop, emit two summary log lines:
     - `"docker: init: found containers: web, api"` or `"docker: init: no proxy containers found"`
     - `"docker: init: joined networks: frontend, backend"` or `"docker: init: no networks joined"`

3. **Update the events handler inside `WatchEvents`** in `internal/docker/manager.go`:
   - `create`/`start` case: read `name := msg.Actor.Attributes["name"]`; log `"docker: event: container added: <name>"`; capture networks from `JoinNetworks` and log each as `"docker: event: joined network: <netName>"`
   - `die` case: read `name := msg.Actor.Attributes["name"]`; log `"docker: event: container removed: <name>"`

No changes to `proxy/server.go`, `main.go`, or any other file.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/docker/manager.go` | Modify | Steps 1–3 above |
| `requirements/2026-03-30-structured-init-and-change-logging.md` | Modify | Status → Completed + notes |
| `requirements/_index.md` | Modify | Status → Completed |

## Key Code Snippets

### `JoinNetworks` new signature and return
```go
func (m *Manager) JoinNetworks(ctx context.Context, networkIDs []string) ([]string, error) {
    if m.selfID == "" {
        return nil, nil
    }
    var joined []string
    for _, netID := range networkIDs {
        // ... existing inspect + already-connected check ...
        if err := m.cli.NetworkConnect(ctx, netID, m.selfID, nil); err != nil {
            if !strings.Contains(err.Error(), "already exists") {
                log.Printf("docker: failed to join network %s: %v", netInfo.Name, err)
            }
        } else {
            log.Printf("docker: joined network %s (%s)", netInfo.Name, netID[:12])
            joined = append(joined, netInfo.Name)
        }
    }
    return joined, nil
}
```

### `Discover` summary logs
```go
var foundNames []string
var allNetworks []string
for _, c := range containers {
    info, err := m.containerToTargetInfo(ctx, c.ID)
    if err != nil { ... continue }
    joined, _ := m.JoinNetworks(ctx, info.NetworkIDs)
    allNetworks = append(allNetworks, joined...)
    handler.RegisterTarget(info)
    foundNames = append(foundNames, info.ContainerName)
}
if len(foundNames) == 0 {
    log.Printf("docker: init: no proxy containers found")
} else {
    log.Printf("docker: init: found containers: %s", strings.Join(foundNames, ", "))
}
if len(allNetworks) == 0 {
    log.Printf("docker: init: no networks joined")
} else {
    log.Printf("docker: init: joined networks: %s", strings.Join(allNetworks, ", "))
}
```

### Events handler additions
```go
case "create", "start":
    name := msg.Actor.Attributes["name"]
    log.Printf("docker: event: container added: %s", name)
    // ... existing containerToTargetInfo ...
    joined, _ := m.JoinNetworks(ctx, info.NetworkIDs)
    for _, n := range joined {
        log.Printf("docker: event: joined network: %s", n)
    }
    handler.RegisterTarget(info)

case "die":
    name := msg.Actor.Attributes["name"]
    log.Printf("docker: event: container removed: %s", name)
    handler.RemoveTarget(msg.Actor.ID)
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Discover with 0 containers | empty container list | logs `no proxy containers found` and `no networks joined` |
| Discover with 2 containers | two labelled containers, already on their networks | logs `found containers: a, b` and `no networks joined` |
| Discover with 2 containers, new networks | two labelled containers, proxy not on their networks | logs `found containers: a, b` and `joined networks: net1, net2` |
| Event: start | start event for labelled container | logs `container added: web` |
| Event: die | die event for labelled container | logs `container removed: web` |

## Risks & Open Questions

- `msg.Actor.Attributes["name"]` is reliable for Docker container events; no known edge cases.
- Deduplication of network names (if two containers share a network) is not required — each join is logged individually by `JoinNetworks`, and the summary for init aggregates across all containers (may repeat a name if already connected to a network from a second container's perspective, but `JoinNetworks` won't re-join it).
