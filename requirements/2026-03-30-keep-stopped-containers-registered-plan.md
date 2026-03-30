# Keep Stopped Containers Registered — Implementation Plan

**Requirement**: [2026-03-30-keep-stopped-containers-registered.md](2026-03-30-keep-stopped-containers-registered.md)
**Date**: 2026-03-30
**Status**: Approved

## Implementation Steps

1. **`internal/docker/manager.go` — swap `die` for `destroy` in the event filter**
   - Replace `f.Add("event", "die")` with `f.Add("event", "destroy")`.

2. **`internal/docker/manager.go` — update the event handler switch**
   - Rename the `"die"` case to `"destroy"`; keep the `handler.RemoveTarget` call there.
   - Add a new `"die"` case that only logs: `docker: event: container stopped: <name> (still registered)`.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/docker/manager.go` | Modify | Steps 1–2 |
| `requirements/2026-03-30-keep-stopped-containers-registered.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## Key Code Snippets

### Event filter
```go
f.Add("event", "start")
f.Add("event", "create")
f.Add("event", "die")     // for log-only
f.Add("event", "destroy") // for deregistration
```

### Event handler
```go
case "die":
    name := msg.Actor.Attributes["name"]
    log.Printf("docker: event: container stopped: %s (still registered)", name)

case "destroy":
    name := msg.Actor.Attributes["name"]
    log.Printf("docker: event: container removed: %s", name)
    handler.RemoveTarget(msg.Actor.ID)
```

## Risks & Open Questions

- None. Two files, four lines changed.
