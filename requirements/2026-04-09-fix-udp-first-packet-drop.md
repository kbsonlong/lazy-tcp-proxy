# Fix UDP First Packet Drop on Container Startup

**Date Added**: 2026-04-09
**Priority**: High
**Status**: In Progress

## Problem Statement

When a UDP flow arrives for a stopped container, the proxy starts the container
and immediately sends the first client datagram to the upstream. However, the
process inside the container (e.g. pihole's DNS daemon) may not yet be listening
on the UDP port when that datagram arrives, causing it to be silently dropped.
The client (e.g. `dig`) receives no response and, depending on its retry
behaviour, may or may not recover on subsequent retries.

Observed log sequence:

```
proxy: udp: new flow from 192.168.1.37:56954 to pihole (port 53)
docker: starting container pihole
docker: container pihole started              ← container started in ~125 ms
proxy: updated UDP target pihole on port 53->53
(first DNS query already forwarded and dropped — pihole not ready yet)
```

For TCP the proxy already handles this: it retries the dial up to 30 times
(1 s apart) until the upstream TCP port accepts a connection.  UDP has no
equivalent readiness gate.

## Functional Requirements

1. After starting the container, the proxy must retry the first UDP datagram
   with a short delay between attempts until either:
   - A response is received from the upstream (service is ready), **or**
   - A configurable maximum number of attempts is exhausted.
2. When a response is received during the retry loop, forward it immediately
   to the originating client via the shared listen socket.
3. After the retry loop (success or exhaustion), the normal `udpUpstreamReadLoop`
   goroutine must be started for all subsequent datagrams.
4. For fire-and-forget UDP services (no response expected), the retry loop must
   eventually give up gracefully without blocking the proxy indefinitely.
5. No new data races may be introduced (read loop and retry loop must not
   read from the same connection concurrently).

## User Experience Requirements

- First DNS query (or any first UDP query) to a stopped container must succeed
  rather than timing out.
- The additional latency on first use is bounded and acceptable (≤ total retry
  window, e.g. 5 seconds).
- Subsequent queries are unaffected.

## Technical Requirements

- The retry logic lives in `startUDPFlow` (`internal/proxy/udp.go`).
- `udpUpstreamReadLoop` must not start until the first-datagram retry loop
  completes, preventing concurrent reads on the same `upstreamConn`.
- Use `upstreamConn.SetReadDeadline` to implement the per-attempt timeout.
- Clear the deadline unconditionally before starting `udpUpstreamReadLoop`.
- Retry constants: 10 attempts, 500 ms per-attempt deadline.
  (Total max wait for fire-and-forget services: 5 s — comparable to a single
  `dig` retry timeout, so the client's next retry arrives just as the loop
  finishes.)

## Acceptance Criteria

- [ ] `dig google.com @<proxy-ip>` succeeds on first invocation when the target
      container (e.g. pihole) is stopped.
- [ ] The proxy log shows retry attempts when the container is slow to start.
- [ ] The proxy log shows the upstream read loop starting after the retry loop.
- [ ] No data race detected by `go test -race`.
- [ ] Existing TCP behaviour is unchanged.
- [ ] Existing UDP integration tests continue to pass.

## Dependencies

- REQ-027 UDP Traffic Support (existing UDP flow implementation)
- REQ-050 Singleflight Deduplication for Container Startup

## Implementation Notes

Mirror TCP's `dialRetries`/`dialInterval` pattern but with UDP-specific constants
(`udpFirstDatagramRetries = 10`, `udpFirstDatagramInterval = 500ms`) and a
response-probe loop instead of a connection-retry loop.
