# Leave Joined Networks on Shutdown — Implementation Plan

**Requirement**: [2026-03-31-leave-networks-on-shutdown.md](2026-03-31-leave-networks-on-shutdown.md)
**Date**: 2026-03-31
**Status**: Implemented

## Implementation Steps

1. `docker/manager.go` — add `"sync"` to imports.
2. `docker/manager.go` — add `mu sync.Mutex` and `joinedNets map[string]string` fields to `Manager`.
3. `docker/manager.go` — initialise `joinedNets` in `NewManager`: `joinedNets: make(map[string]string)`.
4. `docker/manager.go` — in `JoinNetworks`, after a successful `NetworkConnect`, record `m.joinedNets[netID] = netInfo.Name` under the mutex.
5. `docker/manager.go` — add `LeaveNetworks(ctx context.Context)`: snapshot the map under lock, then call `cli.NetworkDisconnect` for each entry with green-coloured log lines.
6. `main.go` — after `<-ctx.Done()`, log `"lazy-tcp-proxy shutting down"` then call `mgr.LeaveNetworks(context.Background())`.
7. Run `go build ./...`.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Add mutex + joinedNets map; record joins; add LeaveNetworks method |
| `lazy-tcp-proxy/main.go` | Modify | Call LeaveNetworks after ctx.Done() |

## API Contracts

N/A

## Data Models

N/A

## Key Code Snippets

```go
// LeaveNetworks — snapshot map first to avoid holding lock during API calls
func (m *Manager) LeaveNetworks(ctx context.Context) {
    if m.selfID == "" {
        return
    }
    m.mu.Lock()
    nets := make(map[string]string, len(m.joinedNets))
    for id, name := range m.joinedNets {
        nets[id] = name
    }
    m.mu.Unlock()
    for id, name := range nets {
        log.Printf("docker: leaving network \033[32m%s\033[0m", name)
        if err := m.cli.NetworkDisconnect(ctx, id, m.selfID, false); err != nil {
            log.Printf("docker: failed to leave network \033[32m%s\033[0m: %v", name, err)
        }
    }
}
```

## Unit Tests

No automated tests; acceptance criteria verified by running the proxy, observing network joins, then sending SIGINT and confirming leave log lines and Docker network state.

## Risks & Open Questions

- If the proxy is killed (SIGKILL) rather than gracefully stopped, `LeaveNetworks` will not run. This is expected and acceptable.

## Deviations from Plan

None.
