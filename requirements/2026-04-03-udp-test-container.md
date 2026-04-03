# UDP Test Container in Example Docker Compose

**Date Added**: 2026-04-03
**Priority**: Low
**Status**: Completed

## Problem Statement

The example `docker-compose.yml` demonstrates lazy-tcp-proxy with TCP services (HTTP, SSH, Postgres, MongoDB) but has no example of the UDP proxy feature (REQ-027). Developers wanting to test or demonstrate UDP proxying have no ready-made target to send traffic to.

## Functional Requirements

1. Add a new service to `example/docker-compose.yml` that acts as a simple UDP echo server.
2. The service must be registered with `lazy-tcp-proxy` via the `lazy-tcp-proxy.udp-ports` label so the proxy manages it.
3. Sending a UDP datagram to the proxy host port must start the container (if stopped) and deliver the datagram.
4. The server echoes received datagrams back to the sender so the caller can confirm round-trip connectivity.
5. Add a comment above the service showing the exact CLI command to fire a test UDP request.
6. Expose the required UDP host port in the `lazy-tcp-proxy` service's `ports` list.

## User Experience Requirements

- A developer can copy-paste a single `nc` (netcat) command from the compose file comment to test UDP proxying end-to-end.
- No extra tooling beyond `nc` (universally available) is required on the host.

## Technical Requirements

- Use a lightweight base image (`alpine`) with `socat` installed at container start-up to act as a UDP echo server.
- Listen on container port `9003` (UDP); map to host port `9003` via the `lazy-tcp-proxy.udp-ports=9003:9003` label.
- The `lazy-tcp-proxy` service must expose `9003:9003/udp` so Docker routes incoming host UDP traffic into the proxy container.
- The container must be on `example-private-network` so the proxy can reach it after start-up.

## Acceptance Criteria

- [ ] A new `udp-echo` service appears in `example/docker-compose.yml`.
- [ ] The service has labels `lazy-tcp-proxy.enabled=true` and `lazy-tcp-proxy.udp-ports=9003:9003`.
- [ ] The service uses `alpine` and starts a `socat` UDP echo listener on port 9003.
- [ ] The `lazy-tcp-proxy` service exposes `9003:9003/udp` in its `ports` list.
- [ ] A comment above the service shows: `# echo "hello" | nc -u localhost 9003`

## Dependencies

- REQ-027 (UDP Traffic Support) — the proxy already handles UDP; this adds a test target.

## Implementation Notes

- `socat` is not in the default Alpine image; the container command installs it via `apk add --no-cache socat` before starting the listener.
- Command: `sh -c "apk add --no-cache socat && socat -v UDP4-RECVFROM:9003,fork EXEC:'cat'"` — `cat` echoes stdin back, `fork` handles concurrent datagrams.
- Port 9003 is chosen as it falls within the already-published `9000-9099` TCP range (the UDP entry is separate and explicit).
