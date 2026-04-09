# Fix UDP First Packet Drop on Container Startup — Implementation Plan

**Requirement**: [2026-04-09-fix-udp-first-packet-drop.md](2026-04-09-fix-udp-first-packet-drop.md)
**Date**: 2026-04-09
**Status**: Draft

## Implementation Steps

1. **Add two package-level constants** to `internal/proxy/udp.go` for the
   first-datagram retry loop (analogous to `dialRetries`/`dialInterval` in
   `server.go`).

2. **Restructure the tail of `startUDPFlow`** in `internal/proxy/udp.go`:
   - Move `uls.activeFlows.Add(1)` and `go s.udpUpstreamReadLoop(...)` to
     **after** the first-datagram retry loop so there is never a concurrent
     reader on `upstreamConn`.
   - Replace the single `upstreamConn.Write(firstDatagram)` call with the retry
     loop described in the Key Code Snippets section.

3. **Verify existing tests still pass** with `go test ./...`.

No other files need to change.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/udp.go` | Modify | Add constants + retry loop in `startUDPFlow` |

## Key Code Snippets

### New constants (add after `const udpBufSize = 65535`)

```go
const (
    udpFirstDatagramRetries  = 10
    udpFirstDatagramInterval = 500 * time.Millisecond
)
```

### Revised tail of `startUDPFlow` (replaces everything after the `uls.mu.Unlock()` that registers the flow)

```go
    // (flow already registered above)

    if uls.info.WebhookURL != "" {
        go s.fireWebhook(uls.info.WebhookURL, "udp_flow_start",
            uls.info.ContainerID, uls.info.ContainerName, connID,
            clientAddr.IP.String(), clientAddr.Port)
    }

    // Send the first datagram with retries: the container process may not be
    // ready to handle packets immediately after EnsureRunning returns.
    // For request/response protocols (e.g. DNS) we send, wait briefly for a
    // response, and forward it; if no response arrives we retry.
    // udpUpstreamReadLoop is NOT started until this loop exits, preventing
    // concurrent reads on the same connection.
    buf := make([]byte, udpBufSize)
    for attempt := 1; attempt <= udpFirstDatagramRetries; attempt++ {
        if _, err := upstreamConn.Write(firstDatagram); err != nil {
            log.Printf("proxy: udp: write first datagram to \033[33m%s\033[0m failed: %v",
                uls.info.ContainerName, err)
            break
        }
        if err := upstreamConn.SetReadDeadline(time.Now().Add(udpFirstDatagramInterval)); err != nil {
            log.Printf("proxy: udp: set deadline for \033[33m%s\033[0m failed: %v",
                uls.info.ContainerName, err)
            break
        }
        n, readErr := upstreamConn.Read(buf)
        if err := upstreamConn.SetReadDeadline(time.Time{}); err != nil {
            log.Printf("proxy: udp: clear deadline for \033[33m%s\033[0m failed: %v",
                uls.info.ContainerName, err)
        }
        if readErr == nil {
            // Service responded — forward this first reply to the client.
            if _, werr := uls.listenConn.WriteToUDP(buf[:n], clientAddr); werr != nil {
                log.Printf("proxy: udp: write initial response to \033[36m%s\033[0m failed: %v",
                    clientAddr, werr)
            }
            uls.mu.Lock()
            flow.lastActive = time.Now()
            uls.lastActive = time.Now()
            uls.mu.Unlock()
            break
        }
        if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
            if attempt < udpFirstDatagramRetries {
                log.Printf("proxy: udp: upstream \033[33m%s\033[0m not ready, retrying (%d/%d)…",
                    uls.info.ContainerName, attempt, udpFirstDatagramRetries)
                continue
            }
            log.Printf("proxy: udp: upstream \033[33m%s\033[0m did not respond after %d attempts; continuing",
                uls.info.ContainerName, udpFirstDatagramRetries)
            break
        }
        // Non-timeout read error (connection closed, etc.)
        log.Printf("proxy: udp: upstream read error for \033[33m%s\033[0m: %v",
            uls.info.ContainerName, readErr)
        break
    }

    uls.activeFlows.Add(1)
    go s.udpUpstreamReadLoop(uls, flow)
```

## API Contracts

N/A — internal change only.

## Data Models

N/A — no new fields.

## Unit Tests

The existing `TestUDPProxy_ForwardsData` integration test covers the happy path:
the echo server responds immediately so the retry loop succeeds on attempt 1 and
forwards the response, then `udpUpstreamReadLoop` handles all subsequent traffic.
No new test file is required for this change; the existing test validates
the corrected code path end-to-end.

| Test | Input | Expected Output |
|------|-------|-----------------|
| `TestUDPProxy_ForwardsData` (existing) | Single "hello-udp" datagram via proxy to in-process echo server | Same datagram echoed back to client |

## Risks & Open Questions

- **Fire-and-forget UDP services**: For protocols that never send a response, the
  loop retries 10 × 500 ms = 5 s before giving up and starting
  `udpUpstreamReadLoop`. During this window the service's first datagram is
  resent up to 10 times — idempotent for most services, but noted as a
  behavioural change.
- **errcheck linter**: All `SetReadDeadline` and `WriteToUDP` return values are
  handled (logged) to satisfy golangci-lint.
