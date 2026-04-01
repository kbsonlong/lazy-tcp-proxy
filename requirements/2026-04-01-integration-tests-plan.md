# Integration Tests (TCP and UDP Proxy) — Implementation Plan

**Requirement**: [2026-04-01-integration-tests.md](2026-04-01-integration-tests.md)
**Date**: 2026-04-01
**Status**: Implemented

## Implementation Steps

1. Update requirement status to "In Progress" in `requirements/2026-04-01-integration-tests.md` and `requirements/_index.md`.

2. Create `lazy-tcp-proxy/internal/proxy/integration_test.go` (`package proxy`):
   a. Define `integrationMock` — a local `dockerManager` stub whose `GetContainerIP` returns a configurable IP (used to point the proxy at the in-process echo server).
   b. Define `startTCPEchoServer(t)` — starts a goroutine that accepts one connection and `io.Copy`s it back to itself (echo). Returns the bound port. Registers `t.Cleanup` to close the listener.
   c. Define `startUDPEchoServer(t)` — starts a goroutine that reads datagrams and writes each one back to the sender. Returns the bound port. Registers `t.Cleanup` to close the conn.
   d. Define `newIntegrationServer(t, ip)` — creates a `ProxyServer` with a `context.WithCancel`, sets `docker` to `&integrationMock{ip: ip}`, registers `t.Cleanup(cancel)`. Returns the server.
   e. Write `TestTCPProxy_ForwardsData`.
   f. Write `TestUDPProxy_ForwardsData`.

3. Run `go build ./... && go vet ./...` to confirm no compile errors.

4. Run `go test -race ./lazy-tcp-proxy/internal/proxy/...` to confirm both tests pass with no race conditions.

5. Update requirement status to "Completed" and plan status to "Implemented". Commit and push.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/integration_test.go` | Create | Two integration tests + helpers |
| `requirements/2026-04-01-integration-tests.md` | Modify | Status updates |
| `requirements/_index.md` | Modify | Status update |

## Key Code Snippets

### TCP echo server
```go
func startTCPEchoServer(t *testing.T) int {
    t.Helper()
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("startTCPEchoServer: %v", err)
    }
    t.Cleanup(func() { ln.Close() })
    go func() {
        for {
            conn, err := ln.Accept()
            if err != nil {
                return
            }
            go func() { defer conn.Close(); io.Copy(conn, conn) }()
        }
    }()
    return ln.Addr().(*net.TCPAddr).Port
}
```

### UDP echo server
```go
func startUDPEchoServer(t *testing.T) int {
    t.Helper()
    pc, err := net.ListenPacket("udp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("startUDPEchoServer: %v", err)
    }
    t.Cleanup(func() { pc.Close() })
    go func() {
        buf := make([]byte, 65535)
        for {
            n, addr, err := pc.ReadFrom(buf)
            if err != nil {
                return
            }
            pc.WriteTo(buf[:n], addr) //nolint:errcheck
        }
    }()
    return pc.LocalAddr().(*net.UDPAddr).Port
}
```

### Integration mock
```go
type integrationMock struct{ ip string }
func (m *integrationMock) EnsureRunning(_ context.Context, _ string) error       { return nil }
func (m *integrationMock) StopContainer(_ context.Context, _, _ string) error    { return nil }
func (m *integrationMock) GetContainerIP(_ context.Context, _, _ string) (string, error) {
    return m.ip, nil
}
```

### TCP test — getting the OS-assigned proxy listen port
`RegisterTarget` is called with `ListenPort: 0`, which causes `net.Listen("tcp", ":0")`. The entry is stored at `s.targets[0]`. The actual bound port is read from the listener:
```go
proxyPort := s.targets[0].listener.Addr().(*net.TCPAddr).Port
```

### TCP test flow
```go
func TestTCPProxy_ForwardsData(t *testing.T) {
    echoPort := startTCPEchoServer(t)
    s := newIntegrationServer(t, "127.0.0.1")
    s.RegisterTarget(docker.TargetInfo{
        ContainerID: "ctr-1", ContainerName: "echo-tcp", Running: true,
        Ports: []docker.PortMapping{{ListenPort: 0, TargetPort: echoPort}},
    })
    proxyPort := s.targets[0].listener.Addr().(*net.TCPAddr).Port

    conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 5*time.Second)
    // ... write "hello", read response, assert equal
}
```

### UDP test — getting the OS-assigned proxy listen port
```go
proxyPort := s.udpTargets[0].listenConn.LocalAddr().(*net.UDPAddr).Port
```

### UDP test timing
After sending the datagram, the proxy must call `EnsureRunning`, `GetContainerIP`, and `DialUDP` before the response arrives. Use a read deadline of 2 seconds on the client side to avoid hanging.

## Integration Tests

| Test | What it exercises |
|------|-------------------|
| `TestTCPProxy_ForwardsData` | `acceptLoop` → `handleConn` → `EnsureRunning` → `GetContainerIP` → dial → bidirectional copy |
| `TestUDPProxy_ForwardsData` | `udpReadLoop` → `startUDPFlow` → `EnsureRunning` → `GetContainerIP` → `DialUDP` → `udpUpstreamReadLoop` → `WriteToUDP` |

## Risks & Open Questions

- **UDP timing**: The proxy's `startUDPFlow` runs in a goroutine. The client must wait for the upstream read loop to be ready before the response arrives. A 2-second read deadline is generous enough without slowing the suite.
- **`s.targets[0]`**: Relies on `RegisterTarget` storing the entry under key `0` when `ListenPort: 0` is passed. This is true for the current implementation but worth noting as an internal detail.
