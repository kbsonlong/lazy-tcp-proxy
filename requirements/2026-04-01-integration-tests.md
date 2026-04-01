# Integration Tests (TCP and UDP Proxy)

**Date Added**: 2026-04-01
**Priority**: Medium
**Status**: Completed

## Problem Statement

The existing unit tests cover pure logic (parsing, idle checks, IP filtering) but cannot verify that the proxy actually accepts connections and forwards traffic. There is no automated check that the TCP or UDP proxy path works end-to-end against a real listener.

## Functional Requirements

1. A TCP integration test starts a real in-process TCP echo server, registers it as a proxy target, dials through the proxy, sends data, and asserts the correct response is received.
2. A UDP integration test starts a real in-process UDP echo server, registers it as a proxy target, sends a datagram through the proxy, and asserts the correct response is received.
3. Tests must not require a Docker daemon — container start/stop calls are satisfied by a mock that returns the loopback echo server address.
4. Tests must clean up all listeners and goroutines on completion (no port leaks between tests).

## User Experience Requirements

- Run with `go test ./...` alongside existing unit tests — no special flags or external services needed.
- Tests complete in well under 10 seconds.

## Technical Requirements

- Use only the Go standard library (`net`, `testing`, `time`) — no third-party test frameworks.
- Reuse the existing `mockDockerManager` from `server_test.go` (same package, `package proxy`).
- Bind listeners on `:0` (OS-assigned ports) to avoid conflicts.
- Echo servers run in goroutines started inside each test; test cancels the proxy `context` to trigger shutdown.

## Acceptance Criteria

- [ ] `TestTCPProxy_ForwardsData` — dials through the proxy, sends "hello", receives "hello" back.
- [ ] `TestUDPProxy_ForwardsData` — sends a datagram through the proxy, receives the same payload back.
- [ ] `go test ./lazy-tcp-proxy/internal/proxy/...` passes with no race detector errors (`-race`).
- [ ] No goroutine or port leaks detectable by running the test suite multiple times consecutively.

## Dependencies

- REQ-027 (UDP Traffic Support) — must be merged (it is)
- REQ-025 (HTTP Status Endpoint) — `NewServer` signature used
- `dockerManager` interface extracted in `feat/unit-tests` (merged to main)

## Implementation Notes

- Echo server for TCP: `net.Listen("tcp", "127.0.0.1:0")` → `io.Copy(conn, conn)` per accepted connection.
- Echo server for UDP: `net.ListenPacket("udp", "127.0.0.1:0")` → read then write back to same addr.
- `mockDockerManager.GetContainerIP` returns the echo server's IP+port split appropriately.
- Use `t.Cleanup` to cancel context and close listeners.
