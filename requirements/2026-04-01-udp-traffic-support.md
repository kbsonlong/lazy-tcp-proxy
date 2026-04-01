# UDP Traffic Support

**Date Added**: 2026-04-01
**Priority**: Medium
**Status**: Planned

## Problem Statement

The proxy currently handles TCP only. Services that use UDP (e.g. DNS, game servers, VoIP, syslog, QUIC) cannot be lazily started via the proxy. Operators must either keep those containers running permanently or build separate tooling.

## Functional Requirements

- A container opts in to UDP proxying by adding the label `lazy-tcp-proxy.udp-ports=<listen>:<target>[,<listen>:<target>...]` (separate from the existing `lazy-tcp-proxy.ports` TCP label).
- For each declared UDP mapping, the proxy binds a UDP socket on the given listen port.
- On receiving the first datagram on a listen socket, the proxy ensures the target container is running (same `EnsureRunning` logic as TCP).
- After the container is running, datagrams are forwarded to the container's IP on the target port.
- Response datagrams from the container are forwarded back to the originating client address.
- UDP flows are tracked by (client IP, client port) pair with a per-flow idle timeout (reuses `IDLE_TIMEOUT_SECS`).
- A flow that has been idle for longer than the idle timeout is considered closed; the next datagram from that client starts a new flow.
- The container idle timeout (stopping the container) still applies: the container is stopped only when all TCP and UDP mappings have been idle for `IDLE_TIMEOUT_SECS`.
- Allow-list and block-list labels (`lazy-tcp-proxy.allow-list`, `lazy-tcp-proxy.block-list`) apply to UDP traffic using the same evaluation logic as TCP.

## User Experience Requirements

- UDP port mappings are declared independently of TCP mappings — a container may declare both.
- The proxy logs each new UDP flow (client address, container name, target port) at info level, consistent with TCP connection logs.
- The proxy logs when a UDP flow expires at debug/info level.
- Startup log lists all UDP listen ports alongside TCP ports.

## Technical Requirements

- UDP proxy runs in a goroutine per listen socket (analogous to `acceptLoop` for TCP).
- Each active flow is represented by a (clientAddr → upstream `net.UDPConn`) mapping held in a per-listener map protected by a mutex.
- A background goroutine per listener sweeps expired flows (reuses poll interval).
- `net.ListenPacket("udp", ...)` is used for the listen socket.
- Per-flow upstream connections use `net.DialUDP` to the container IP so responses are routed correctly.
- No new dependencies — uses only stdlib `net`.
- `TargetInfo` gains a new field `UDPPorts []PortMapping` to hold UDP mappings.
- The inactivity checker is extended to treat a container as idle only when both `Ports` (TCP) and `UDPPorts` (UDP) have no active flows and both have exceeded the idle timeout.

## Acceptance Criteria

- [ ] A container with `lazy-tcp-proxy.udp-ports=5353:53` receives forwarded UDP datagrams on port 53 when a client sends to the proxy on port 5353.
- [ ] The container is started on the first datagram if it is not already running.
- [ ] Response datagrams from the container are returned to the correct originating client.
- [ ] Multiple simultaneous UDP flows from different clients are handled independently.
- [ ] A container with both `lazy-tcp-proxy.ports` and `lazy-tcp-proxy.udp-ports` handles TCP and UDP concurrently; idle timeout considers both.
- [ ] Allow-list and block-list labels are enforced for UDP traffic (datagrams from blocked IPs are silently dropped).
- [ ] A UDP flow with no datagrams for `IDLE_TIMEOUT_SECS` is cleaned up.
- [ ] The container is only stopped when all TCP and UDP flows are idle past the timeout.
- [ ] Startup log includes UDP listen ports.

## Dependencies

- Extends `TargetInfo` in `internal/docker/manager.go` with `UDPPorts`.
- Extends `ProxyServer` in `internal/proxy/server.go` with UDP listener management.
- Extends `checkInactivity` to account for active UDP flows.
- Shares `EnsureRunning`, `GetContainerIP`, allow/block-list logic with the TCP path.

## Implementation Notes

- UDP has no connection concept — flow tracking by client address is the standard approach. Care is needed to avoid unbounded growth of the flow map; expired flows must be pruned.
- `net.UDPConn` in "connected" mode (via `DialUDP`) simplifies reading responses but means one upstream socket per flow. This is acceptable for the expected scale (lazy-started services, low concurrency).
- The buffer size per datagram read should be configurable or set to a reasonable maximum (e.g. 64 KB).
- Port conflict detection (REQ-024) should be extended to cover UDP listen ports as well.
