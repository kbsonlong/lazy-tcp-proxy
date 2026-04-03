# UDP Test Container in Example Docker Compose — Implementation Plan

**Requirement**: [2026-04-03-udp-test-container.md](2026-04-03-udp-test-container.md)
**Date**: 2026-04-03
**Status**: Approved

## Implementation Steps

1. **Add `9003:9003/udp` to the `lazy-tcp-proxy` service `ports` list** in `example/docker-compose.yml`.
   - The existing `9000-9099:9000-9099` range covers TCP only; Docker requires an explicit `/udp` entry for UDP ports.
   - Insert `- "9003:9003/udp"` after the existing range entry.

2. **Append the `udp-echo` service** to `example/docker-compose.yml`, before the `volumes:` block.
   - Base image: `alpine`
   - Container name: `example-udp-echo`
   - Network: `example-private-network`
   - Command: `sh -c "apk add --no-cache socat && socat -v UDP4-RECVFROM:9003,fork EXEC:'cat'"`
     - Installs `socat` at start-up (no custom image needed).
     - `UDP4-RECVFROM:9003` — listens for incoming UDP datagrams on port 9003.
     - `fork` — spawns a subprocess per datagram so concurrent clients work.
     - `EXEC:'cat'` — echoes stdin back to the sender.
     - `-v` — verbose logging so container logs show traffic.
   - Labels:
     - `lazy-tcp-proxy.enabled=true`
     - `lazy-tcp-proxy.udp-ports=9003:9003`
   - Add a comment above the service: `# echo "hello" | nc -u localhost 9003`

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `example/docker-compose.yml` | Modify | Add UDP port to proxy service; add `udp-echo` service |

## API Contracts

N/A — no HTTP endpoints involved.

## Data Models

N/A.

## Key Code Snippets

**New `udp-echo` service block**:

```yaml
  # echo "hello" | nc -u localhost 9003
  udp-echo:
    image: alpine
    container_name: example-udp-echo
    networks:
      - example-private-network
    command: ["sh", "-c", "apk add --no-cache socat && socat -v UDP4-RECVFROM:9003,fork EXEC:'cat'"]
    labels:
      - "lazy-tcp-proxy.enabled=true"
      - "lazy-tcp-proxy.udp-ports=9003:9003"
```

**Updated `lazy-tcp-proxy` ports**:

```yaml
    ports:
      - "8080:8080"
      - "5432:5432"
      - "27017:27017"
      - "9000-9099:9000-9099"
      - "9003:9003/udp"
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Manual smoke test | `echo "hello" \| nc -u localhost 9003` | `hello` echoed back; container starts if it was stopped |

## Risks & Open Questions

- `socat` install adds a few seconds to cold-start time (first request after container is stopped). This is expected and acceptable for a test/example container.
- `nc -u` behaviour varies slightly between BSD and GNU netcat; `-u` flag for UDP is standard on both.
