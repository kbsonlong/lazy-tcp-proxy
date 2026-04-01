# UDP Traffic Support — Implementation Plan

**Requirement**: [2026-04-01-udp-traffic-support.md](2026-04-01-udp-traffic-support.md)
**Date**: 2026-04-01
**Status**: Draft

## Implementation Steps

1. **Add `UDPPorts []PortMapping` to `TargetInfo`** in `internal/docker/manager.go`.

2. **Parse `lazy-tcp-proxy.udp-ports` label in `containerToTargetInfo`** — same tokenisation logic as `lazy-tcp-proxy.ports`; result stored in `TargetInfo.UDPPorts`. Missing label → empty slice (no UDP); invalid token → log and skip that token only.

3. **Extend port-conflict pre-flight in `RegisterTarget`** — before opening any listener, check that no declared UDP listen port collides with an existing UDP listener registered by a different container.

4. **Add `udpFlow` and `udpListenerState` structs** in `internal/proxy/` (new file `udp.go` for clarity):
   - `udpFlow` — tracks `upstreamConn *net.UDPConn`, `lastActive time.Time`, `clientAddr *net.UDPAddr`.
   - `udpListenerState` — wraps the shared listen `*net.UDPConn`, the `targetState` pointer (for container info / `EnsureRunning`), a `flows map[string]*udpFlow` keyed by `clientAddr.String()`, and a `sync.Mutex`.

5. **Add `udpTargets map[int]*udpListenerState` to `ProxyServer`** — keyed by listen port, protected by the existing `s.mu`.

6. **Extend `RegisterTarget`** — for each entry in `info.UDPPorts`, if no listener exists yet, call `net.ListenPacket("udp", ":<port>")`, create a `udpListenerState`, store in `s.udpTargets`, and launch `go s.udpReadLoop(uls)`. If a listener already exists for the same container, update its `targetState` reference.

7. **Implement `udpReadLoop(uls *udpListenerState)`** — runs in a goroutine, reads datagrams in a loop:
   - Parse client address from `ReadFromUDP`.
   - Check allow/block-list (reuse `ipBlocked` with the `TargetInfo` from `uls.ts.info`); silently drop if blocked.
   - Look up or create a `udpFlow` for the client address.
   - On new flow: call `EnsureRunning`, then `GetContainerIP`, then `net.DialUDP` to the container's target port; store conn in flow; launch `go udpUpstreamReadLoop(uls, flow)` to pipe responses back.
   - On existing flow: write datagram to `flow.upstreamConn`; update `flow.lastActive`.
   - Update `uls.ts.lastActive` on every forwarded datagram (for the container idle checker).

8. **Implement `udpUpstreamReadLoop(uls, flow)`** — reads datagrams from `flow.upstreamConn` and writes them back to the client via the shared listen conn. Exits when the upstream conn is closed.

9. **Implement `udpFlowSweeper`** — launched once per `udpListenerState` goroutine; ticks at the existing poll interval; locks `uls.mu`, iterates flows, closes and deletes flows idle longer than `idleTimeout`; logs each expired flow.

10. **Extend `checkInactivity`** — a container is only eligible for stop when all its TCP `targetState`s AND all its UDP `udpListenerState`s report no active flows and `lastActive` is beyond the timeout. Refactor the aggregation to gather UDP state alongside TCP state.

11. **Extend `RemoveTarget`** — close and delete all `udpListenerState`s for the removed container (close the shared UDP listen conn, which will cause `udpReadLoop` to exit).

12. **Extend `ContainerStopped`** — mark UDP listener states' `ts.running = false` alongside TCP states.

13. **Extend startup log in `RegisterTarget`** — include UDP listen ports in the log line, e.g. `proxy: registered target myservice, TCP 8080->80, UDP 5353->53`.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Add `UDPPorts []PortMapping` to `TargetInfo`; parse `lazy-tcp-proxy.udp-ports` label |
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `udpTargets` map; extend `RegisterTarget`, `RemoveTarget`, `ContainerStopped`, `checkInactivity` |
| `lazy-tcp-proxy/internal/proxy/udp.go` | Create | `udpFlow`, `udpListenerState`, `udpReadLoop`, `udpUpstreamReadLoop`, `udpFlowSweeper` |

## API Contracts

No HTTP API changes. Label interface:

| Label | Format | Example |
|-------|--------|---------|
| `lazy-tcp-proxy.udp-ports` | `<listen>:<target>[,<listen>:<target>...]` | `5353:53,1194:1194` |

## Data Models

```go
// udpFlow represents one active client→container UDP flow.
type udpFlow struct {
    clientAddr   *net.UDPAddr
    upstreamConn *net.UDPConn
    lastActive   time.Time
}

// udpListenerState holds the shared inbound UDP socket and all active flows.
type udpListenerState struct {
    listenConn *net.UDPConn
    targetPort int
    ts         *targetState // shared with TCP path for container info + lastActive
    mu         sync.Mutex
    flows      map[string]*udpFlow // key: clientAddr.String()
    removed    bool
}
```

## Key Code Snippets

```go
// udpReadLoop — core datagram dispatch loop.
func (s *ProxyServer) udpReadLoop(uls *udpListenerState) {
    buf := make([]byte, 65535)
    for {
        n, clientAddr, err := uls.listenConn.ReadFromUDP(buf)
        if err != nil {
            if uls.removed {
                return
            }
            log.Printf("proxy: udp: read error on port %d: %v", uls.targetPort, err)
            return
        }

        data := make([]byte, n)
        copy(data, buf[:n])

        if ipBlocked(clientAddr.String(), uls.ts.info) {
            log.Printf("proxy: udp: datagram from %s to %s (blocked)", clientAddr, uls.ts.info.ContainerName)
            continue
        }

        uls.mu.Lock()
        flow, ok := uls.flows[clientAddr.String()]
        if !ok {
            // New flow — start container and dial upstream
            go s.startUDPFlow(uls, clientAddr, data)
            uls.mu.Unlock()
            continue
        }
        flow.lastActive = time.Now()
        uls.ts.lastActive = time.Now()
        conn := flow.upstreamConn
        uls.mu.Unlock()

        if _, err := conn.Write(data); err != nil {
            log.Printf("proxy: udp: write to upstream failed: %v", err)
        }
    }
}
```

```go
// udpFlowSweeper — prunes idle flows.
func (s *ProxyServer) udpFlowSweeper(ctx context.Context, uls *udpListenerState, tick time.Duration) {
    ticker := time.NewTicker(tick)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            now := time.Now()
            uls.mu.Lock()
            for key, flow := range uls.flows {
                if now.Sub(flow.lastActive) > s.idleTimeout {
                    log.Printf("proxy: udp: flow %s -> %s expired", flow.clientAddr, uls.ts.info.ContainerName)
                    flow.upstreamConn.Close()
                    delete(uls.flows, key)
                }
            }
            uls.mu.Unlock()
        }
    }
}
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Single UDP datagram forwarded | client sends datagram to proxy UDP port | datagram received by container on target port |
| Response returned to client | container replies | client receives reply datagram |
| Container started on first datagram | container stopped, client sends datagram | container started, datagram forwarded |
| Multiple concurrent flows | two clients send simultaneously | each client receives its own response |
| Blocked IP datagram | client IP in block-list | datagram silently dropped, container not started |
| Flow expiry | no datagrams for `IDLE_TIMEOUT_SECS` | flow removed from map, upstream conn closed |
| Container stops only when TCP+UDP both idle | TCP active, UDP idle | container NOT stopped until TCP also idle |
| UDP port conflict | two containers declare same UDP listen port | second container registration rejected, warning logged |
| Invalid `udp-ports` token | `"abc:xyz"` | token skipped with warning, valid tokens processed |

## Risks & Open Questions

- **One upstream `UDPConn` per flow**: for high-concurrency scenarios (many simultaneous clients) this creates many sockets. Acceptable for the lazy-start/idle-stop use case where concurrency is expected to be low.
- **64 KB read buffer**: allocated once per `udpReadLoop` goroutine (on the stack/heap). This is the maximum UDP datagram size and is fine for typical use.
- **`startUDPFlow` concurrency**: multiple datagrams from the same new client could arrive before the flow is created. The goroutine approach means the first creates the flow; subsequent ones will either race or be dropped. A "pending flows" set may be needed to serialise this — flag as a risk to revisit during build.
- **Go version**: `net.ListenPacket("udp", ...)` + `ReadFromUDP` is available in all supported Go versions; no concern.
- **`checkInactivity` refactor**: currently groups TCP states by container ID. The UDP states must be included in the same grouping pass. Take care not to mark a container idle if any UDP flow is still active.
