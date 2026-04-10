# Load Tests (TCP and UDP Proxy) — Implementation Plan

**Requirement**: [2026-04-07-load-tests-tcp-udp.md](2026-04-07-load-tests-tcp-udp.md)
**Date**: 2026-04-08
**Status**: Implemented

## Implementation Steps

1. Create the `example/load-test/` directory.

2. Create `example/load-test/docker-compose.yml`:
   - Service `lazy-tcp-proxy`: mounts Docker socket, exposes ports 5201 (TCP) and 5202 (TCP+UDP) to the host, sets `IDLE_TIMEOUT_SECS=300` and `POLL_INTERVAL_SECS=5`.
   - Service `iperf3-tcp`: `networkstatic/iperf3 -s` on the private network, labelled for TCP proxy on port 5201.
   - Service `iperf3-udp`: `networkstatic/iperf3 -s` on the private network, labelled for both TCP control (5202) and UDP data (5202/udp) — this exercises mixed-protocol proxying.
   - Private internal network `load-test-net` shared by both iperf3 containers (proxy joins it automatically via Docker socket).

3. Create `example/load-test/run-tcp-load-test.sh`:
   - POSIX shell, executable.
   - Reads env vars `DURATION` (default 30), `PARALLEL` (default 10).
   - Runs `docker run --rm --network host networkstatic/iperf3 -c localhost -p 5201 -t $DURATION -P $PARALLEL`.
   - Prints a header line explaining what's happening before running.

4. Create `example/load-test/run-udp-load-test.sh`:
   - POSIX shell, executable.
   - Reads env vars `DURATION` (default 30), `PARALLEL` (default 10), `BITRATE` (default 100M).
   - Runs `docker run --rm --network host networkstatic/iperf3 -c localhost -p 5202 -u -b $BITRATE -t $DURATION -P $PARALLEL`.
   - Prints a header line explaining what's happening before running.

5. Create `example/load-test/README.md`:
   - Prerequisites, start/stop commands, example output, env var table.

No changes to proxy source code are required.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `example/load-test/docker-compose.yml` | Create | Stack definition: proxy + two iperf3 servers |
| `example/load-test/run-tcp-load-test.sh` | Create | TCP load test runner (executable shell script) |
| `example/load-test/run-udp-load-test.sh` | Create | UDP load test runner (executable shell script) |
| `example/load-test/README.md` | Create | Setup and usage documentation |

## API Contracts

N/A — no new HTTP or socket APIs.

## Data Models

N/A

## Key Code Snippets

### docker-compose.yml

```yaml
# example/load-test/docker-compose.yml
services:

  lazy-tcp-proxy:
    image: mountainpass/lazy-tcp-proxy
    container_name: load-test-proxy
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - "5201:5201"       # TCP load test
      - "5202:5202"       # UDP load test control (TCP)
      - "5202:5202/udp"   # UDP load test data
      - "8080:8080"       # status endpoint
    environment:
      IDLE_TIMEOUT_SECS: "300"
      POLL_INTERVAL_SECS: "5"

  iperf3-tcp:
    image: networkstatic/iperf3
    container_name: load-test-iperf3-tcp
    command: ["-s"]
    networks:
      - load-test-net
    labels:
      lazy-tcp-proxy.enabled: "true"
      lazy-tcp-proxy.ports: "5201:5201"

  iperf3-udp:
    image: networkstatic/iperf3
    container_name: load-test-iperf3-udp
    command: ["-s"]
    networks:
      - load-test-net
    labels:
      lazy-tcp-proxy.enabled: "true"
      lazy-tcp-proxy.ports: "5202:5201"
      lazy-tcp-proxy.udp-ports: "5202:5201"

networks:
  load-test-net:
    driver: bridge
    internal: true
```

### run-tcp-load-test.sh

```sh
#!/bin/sh
set -e
DURATION=${DURATION:-30}
PARALLEL=${PARALLEL:-10}

echo "==> TCP load test: ${PARALLEL} parallel streams for ${DURATION}s via proxy on :5201"
docker run --rm --network host networkstatic/iperf3 \
  -c localhost -p 5201 -t "$DURATION" -P "$PARALLEL"
```

### run-udp-load-test.sh

```sh
#!/bin/sh
set -e
DURATION=${DURATION:-30}
PARALLEL=${PARALLEL:-10}
BITRATE=${BITRATE:-100M}

echo "==> UDP load test: ${PARALLEL} parallel streams at ${BITRATE} for ${DURATION}s via proxy on :5202"
docker run --rm --network host networkstatic/iperf3 \
  -c localhost -p 5202 -u -b "$BITRATE" -t "$DURATION" -P "$PARALLEL"
```

## Unit Tests

N/A — load test output is validated by eye / iperf3 exit code. Success criteria:
- Exit code 0 from both scripts
- iperf3 output contains a "Receiver" summary line with non-zero throughput (TCP)
- iperf3 output contains jitter and packet-loss fields (UDP)

## Risks & Open Questions

1. **iperf3 UDP through a stateful UDP proxy**: iperf3 UDP mode uses a TCP control channel on the same port as UDP data. The proxy must handle both TCP (control) and UDP (data) on port 5202 for the same container. This is supported by using both `lazy-tcp-proxy.ports` and `lazy-tcp-proxy.udp-ports` on `iperf3-udp`. Needs smoke-testing after build.

2. **`--network host` on Linux only**: The client scripts use `docker run --network host` which is Linux-only. On Mac/Windows, users must replace `localhost` with `host.docker.internal`. Document this in the README.

3. **Idle timeout during test**: Set to 300 s (5 min) so the iperf3 server is not stopped mid-test. Default test duration is 30 s, well within this window.

4. **iperf3 server already running**: If the load test stack is brought up with the iperf3 containers already running (e.g., `--no-deps` flag absent), the proxy may not see them as "idle" on first connection. Setting `POLL_INTERVAL_SECS=5` ensures the proxy discovers them within 5 seconds of stack start.

5. **Port 5201 conflicts**: iperf3's default port 5201 may conflict with other local iperf3 instances. Document that users should stop any local iperf3 servers before running.
