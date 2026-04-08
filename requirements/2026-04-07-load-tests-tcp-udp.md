# Load Tests for TCP and UDP Proxy

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: Completed

## Problem Statement

The proxy has functional integration tests (REQ-028) that verify correctness, but there are no
load tests to validate real-world performance under sustained or concurrent traffic. Without
these, it is impossible to detect throughput regressions or resource exhaustion under realistic
conditions.

## Functional Requirements

Two standalone load test scenarios using real Docker containers and the industry-standard
`iperf3` network benchmarking tool:

### TCP Load Test

1. Start `lazy-tcp-proxy` (via Docker Compose) connected to the Docker socket
2. Start an `iperf3` server container registered with the proxy via labels (port 5201 TCP)
3. Run an `iperf3` client outside the proxy — connecting to **proxy host:port**
4. iperf3 runs for a configurable duration (default 30 s) with configurable parallelism
5. Output: throughput (Mbps/Gbps), transfer bytes, CPU usage, per-interval stats

### UDP Load Test

1. Start `lazy-tcp-proxy` and an `iperf3` server registered for **both** TCP control (5201) and
   UDP data (5201/udp) via proxy labels — this exercises the proxy's ability to handle both
   protocols for the same container
2. Run an `iperf3` client with `-u` (UDP mode), targeting the proxy
3. Output: UDP throughput, jitter, packet loss %, datagrams/sec

## User Experience Requirements

- All infrastructure started with a single command: `docker compose -f example/load-test/docker-compose.yml up -d`
- Load tests run with simple shell scripts: `./example/load-test/run-tcp-load-test.sh` and `./example/load-test/run-udp-load-test.sh`
- Results printed directly to the terminal in iperf3 format
- Teardown: `docker compose -f example/load-test/docker-compose.yml down`
- No Go toolchain required to run the tests — only Docker and a Docker-compatible iperf3 client

## Technical Requirements

- New directory: `example/load-test/`
- Files:
  - `example/load-test/docker-compose.yml` — proxy + iperf3 server containers
  - `example/load-test/run-tcp-load-test.sh` — TCP load test runner script
  - `example/load-test/run-udp-load-test.sh` — UDP load test runner script
  - `example/load-test/README.md` — setup and usage instructions
- Docker image for iperf3: `networkstatic/iperf3` (official, widely maintained)
- iperf3 server must be on a private Docker network visible to the proxy but not the host
- Proxy must expose the relevant ports to the host
- Client runs iperf3 connecting to `localhost:<proxy-port>`
- Scripts must be POSIX shell (`#!/bin/sh`) with no external dependencies beyond Docker

## Acceptance Criteria

- [ ] `docker compose -f example/load-test/docker-compose.yml up -d` starts all services cleanly
- [ ] `run-tcp-load-test.sh` produces iperf3 output showing Mbps/Gbps throughput
- [ ] `run-udp-load-test.sh` produces iperf3 output showing UDP throughput, jitter, packet loss
- [ ] The proxy starts the iperf3 server container on first connection (lazy start), confirming the proxy is actually in the path
- [ ] Tests pass with at least 10 parallel streams (`-P 10`)
- [ ] Scripts accept optional env var overrides: `DURATION` (default 30), `PARALLEL` (default 10), `BITRATE` for UDP (default 100M)
- [ ] `README.md` documents all steps to run and interpret results

## Dependencies

- REQ-027 (UDP Traffic Support) — proxy must support UDP
- REQ-028 (Integration Tests) — confirms correctness before load testing
- Docker and Docker Compose installed on the host
- `networkstatic/iperf3` Docker image available on Docker Hub

## Implementation Notes

### Architecture

```
[iperf3 client (host)] ──TCP 5201──► [lazy-tcp-proxy :5201] ──TCP──► [iperf3-server container]
[iperf3 client (host)] ──UDP 5202──► [lazy-tcp-proxy :5202/udp] ──UDP──► [iperf3-server container]
```

### iperf3 UDP note

iperf3 always uses a TCP control channel (same port as the data port). For the UDP load test,
the proxy must forward **both** `lazy-tcp-proxy.ports=5202:5201` (TCP control) and
`lazy-tcp-proxy.udp-ports=5202:5201` (UDP data) for the iperf3-udp-server container. This
exercises mixed TCP+UDP proxying for a single target.

### Example docker-compose.yml (sketch)

```yaml
services:
  lazy-tcp-proxy:
    image: mountainpass/lazy-tcp-proxy
    volumes: [/var/run/docker.sock:/var/run/docker.sock]
    ports: ["5201:5201", "5202:5202", "5202:5202/udp"]
    environment:
      IDLE_TIMEOUT_SECS: 300
      POLL_INTERVAL_SECS: 5

  iperf3-tcp:
    image: networkstatic/iperf3
    command: ["-s"]
    networks: [load-test-net]
    labels:
      lazy-tcp-proxy.enabled: "true"
      lazy-tcp-proxy.ports: "5201:5201"

  iperf3-udp:
    image: networkstatic/iperf3
    command: ["-s"]
    networks: [load-test-net]
    labels:
      lazy-tcp-proxy.enabled: "true"
      lazy-tcp-proxy.ports: "5202:5201"
      lazy-tcp-proxy.udp-ports: "5202:5201"
```

### Example TCP run script (sketch)

```sh
#!/bin/sh
DURATION=${DURATION:-30}
PARALLEL=${PARALLEL:-10}
docker run --rm --network host networkstatic/iperf3 \
  -c localhost -p 5201 -t "$DURATION" -P "$PARALLEL"
```

### Example UDP run script (sketch)

```sh
#!/bin/sh
DURATION=${DURATION:-30}
PARALLEL=${PARALLEL:-10}
BITRATE=${BITRATE:-100M}
docker run --rm --network host networkstatic/iperf3 \
  -c localhost -p 5202 -u -b "$BITRATE" -t "$DURATION" -P "$PARALLEL"
```
