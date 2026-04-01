package proxy

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mountain-pass/lazy-tcp-proxy/internal/docker"
)

// ---- ipBlocked ----

func TestIPBlocked_NoLists(t *testing.T) {
	info := docker.TargetInfo{} // empty allow/block lists
	if ipBlocked("10.0.0.1:1234", info) {
		t.Error("expected not blocked with no lists")
	}
}

func TestIPBlocked_AllowList_MatchAllows(t *testing.T) {
	info := docker.TargetInfo{
		AllowList: mustParseNets("192.168.0.0/16"),
	}
	if ipBlocked("192.168.1.100:1234", info) {
		t.Error("192.168.1.100 should be allowed by 192.168.0.0/16")
	}
}

func TestIPBlocked_AllowList_NoMatchBlocks(t *testing.T) {
	info := docker.TargetInfo{
		AllowList: mustParseNets("192.168.0.0/16"),
	}
	if !ipBlocked("10.0.0.1:1234", info) {
		t.Error("10.0.0.1 should be blocked — not in allow-list")
	}
}

func TestIPBlocked_BlockList_MatchBlocks(t *testing.T) {
	info := docker.TargetInfo{
		BlockList: mustParseNets("10.0.0.1"),
	}
	if !ipBlocked("10.0.0.1:1234", info) {
		t.Error("10.0.0.1 should be blocked by block-list")
	}
}

func TestIPBlocked_BlockList_NoMatchAllows(t *testing.T) {
	info := docker.TargetInfo{
		BlockList: mustParseNets("10.0.0.1"),
	}
	if ipBlocked("10.0.0.2:1234", info) {
		t.Error("10.0.0.2 should not be blocked")
	}
}

func TestIPBlocked_BlockListEvaluatedAfterAllowList(t *testing.T) {
	// IP passes allow-list check, but block-list is evaluated next and wins.
	info := docker.TargetInfo{
		AllowList: mustParseNets("10.0.0.0/8"),
		BlockList: mustParseNets("10.0.0.1"),
	}
	if !ipBlocked("10.0.0.1:1234", info) {
		t.Error("10.0.0.1 passes allow-list but is in block-list — should be blocked")
	}
}

func TestIPBlocked_NotInAllowList_BlockListNotEvaluated(t *testing.T) {
	// IP is not in allow-list → blocked immediately, block-list irrelevant.
	info := docker.TargetInfo{
		AllowList: mustParseNets("192.168.0.0/16"),
		BlockList: mustParseNets("10.0.0.1"),
	}
	if !ipBlocked("10.0.0.1:1234", info) {
		t.Error("10.0.0.1 not in allow-list, should be blocked")
	}
}

func TestIPBlocked_UnparsableAddr(t *testing.T) {
	info := docker.TargetInfo{BlockList: mustParseNets("10.0.0.1")}
	// Malformed address — should not block (fail open)
	if ipBlocked("not-an-addr", info) {
		t.Error("unparsable address should not be blocked")
	}
}

func TestIPBlocked_IPv6(t *testing.T) {
	info := docker.TargetInfo{
		BlockList: mustParseNets("::1"),
	}
	if !ipBlocked("[::1]:1234", info) {
		t.Error("::1 should be blocked")
	}
}

// ---- Snapshot ----

func TestSnapshot_Empty(t *testing.T) {
	s := newTestServer()
	snap := s.Snapshot()
	if len(snap) != 0 {
		t.Errorf("expected empty snapshot, got %d entries", len(snap))
	}
}

func TestSnapshot_Fields(t *testing.T) {
	s := newTestServer()
	now := time.Now()
	ts := &targetState{
		info: docker.TargetInfo{
			ContainerID:   strings.Repeat("a", 64),
			ContainerName: "my-service",
		},
		targetPort: 80,
		running:    true,
		lastActive: now,
	}
	ts.activeConns.Store(3)
	s.targets[9000] = ts

	snap := s.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 snapshot entry, got %d", len(snap))
	}
	e := snap[0]
	if e.ContainerID != strings.Repeat("a", 12) {
		t.Errorf("ContainerID: got %q, want 12-char prefix", e.ContainerID)
	}
	if e.ContainerName != "my-service" {
		t.Errorf("ContainerName: got %q", e.ContainerName)
	}
	if e.ListenPort != 9000 {
		t.Errorf("ListenPort: got %d, want 9000", e.ListenPort)
	}
	if e.TargetPort != 80 {
		t.Errorf("TargetPort: got %d, want 80", e.TargetPort)
	}
	if !e.Running {
		t.Error("Running: expected true")
	}
	if e.ActiveConns != 3 {
		t.Errorf("ActiveConns: got %d, want 3", e.ActiveConns)
	}
	if e.LastActive == nil {
		t.Error("LastActive: expected non-nil")
	}
}

func TestSnapshot_NeverActiveMarshalsAsNil(t *testing.T) {
	s := newTestServer()
	s.targets[9000] = &targetState{
		info:       docker.TargetInfo{ContainerID: "abc123"},
		targetPort: 80,
		lastActive: time.Time{}, // zero value
	}
	snap := s.Snapshot()
	if snap[0].LastActive != nil {
		t.Error("LastActive should be nil for zero time")
	}
}

// ---- RegisterTarget port conflict ----

func TestRegisterTarget_TCPPortConflict(t *testing.T) {
	s := newTestServer()

	// Pre-populate a fake TCP listener for container A on port 9000.
	// Use a real listener so the port is actually held.
	ln, err := net.Listen("tcp", ":0") // OS-assigned port
	if err != nil {
		t.Fatalf("could not open test listener: %v", err)
	}
	defer ln.Close()
	listenPort := ln.Addr().(*net.TCPAddr).Port

	s.targets[listenPort] = &targetState{
		info:     docker.TargetInfo{ContainerID: "container-a", ContainerName: "svc-a"},
		listener: ln,
	}

	// Container B tries to register on the same port — should be rejected.
	s.RegisterTarget(docker.TargetInfo{
		ContainerID:   "container-b",
		ContainerName: "svc-b",
		Ports:         []docker.PortMapping{{ListenPort: listenPort, TargetPort: 80}},
	})

	// Container A's entry must still be there, not overwritten.
	if s.targets[listenPort].info.ContainerID != "container-a" {
		t.Error("conflict: container-a's registration should not have been replaced")
	}
}

func TestRegisterTarget_UDPPortConflict(t *testing.T) {
	s := newTestServer()

	// Pre-populate a fake UDP listener for container A.
	pc, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("could not open test UDP listener: %v", err)
	}
	defer pc.Close()
	listenPort := pc.LocalAddr().(*net.UDPAddr).Port

	s.udpTargets[listenPort] = &udpListenerState{
		listenConn: pc.(*net.UDPConn),
		info:       docker.TargetInfo{ContainerID: "container-a", ContainerName: "svc-a"},
	}

	// Container B tries to register on the same UDP port — should be rejected.
	s.RegisterTarget(docker.TargetInfo{
		ContainerID:   "container-b",
		ContainerName: "svc-b",
		Ports:         []docker.PortMapping{{ListenPort: 19999, TargetPort: 80}}, // different TCP port
		UDPPorts:      []docker.PortMapping{{ListenPort: listenPort, TargetPort: 53}},
	})

	if s.udpTargets[listenPort].info.ContainerID != "container-a" {
		t.Error("conflict: container-a's UDP registration should not have been replaced")
	}
}

// ---- checkInactivity grouping ----

func TestCheckInactivity_StopsIdleContainer(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	// One TCP mapping, running, no active conns, lastActive long ago.
	s.targets[9000] = &targetState{
		info:       docker.TargetInfo{ContainerID: "ctr-1", ContainerName: "svc"},
		targetPort: 80,
		running:    true,
		lastActive: time.Now().Add(-10 * time.Minute),
	}

	s.checkInactivity(context.Background())

	select {
	case id := <-stopped:
		if id != "ctr-1" {
			t.Errorf("expected ctr-1 to be stopped, got %s", id)
		}
	default:
		t.Error("expected StopContainer to be called")
	}
}

func TestCheckInactivity_DoesNotStopActiveConnections(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	ts := &targetState{
		info:       docker.TargetInfo{ContainerID: "ctr-1", ContainerName: "svc"},
		targetPort: 80,
		running:    true,
		lastActive: time.Now().Add(-10 * time.Minute),
	}
	ts.activeConns.Store(1)
	s.targets[9000] = ts

	s.checkInactivity(context.Background())

	select {
	case <-stopped:
		t.Error("should not stop container with active connections")
	default:
	}
}

func TestCheckInactivity_DoesNotStopRecentlyActive(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	s.targets[9000] = &targetState{
		info:       docker.TargetInfo{ContainerID: "ctr-1", ContainerName: "svc"},
		targetPort: 80,
		running:    true,
		lastActive: time.Now(), // just active
	}

	s.checkInactivity(context.Background())

	select {
	case <-stopped:
		t.Error("should not stop recently active container")
	default:
	}
}

func TestCheckInactivity_DoesNotStopAlreadyStopped(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	s.targets[9000] = &targetState{
		info:       docker.TargetInfo{ContainerID: "ctr-1", ContainerName: "svc"},
		targetPort: 80,
		running:    false, // already stopped
		lastActive: time.Now().Add(-10 * time.Minute),
	}

	s.checkInactivity(context.Background())

	select {
	case <-stopped:
		t.Error("should not stop already-stopped container")
	default:
	}
}

func TestCheckInactivity_MultiPortAllIdleStops(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	// Same container on two ports — both idle.
	info := docker.TargetInfo{ContainerID: "ctr-1", ContainerName: "svc"}
	for _, port := range []int{9000, 9001} {
		s.targets[port] = &targetState{
			info:       info,
			targetPort: 80,
			running:    true,
			lastActive: time.Now().Add(-10 * time.Minute),
		}
	}

	s.checkInactivity(context.Background())

	select {
	case id := <-stopped:
		if id != "ctr-1" {
			t.Errorf("expected ctr-1, got %s", id)
		}
	default:
		t.Error("expected stop when all ports are idle")
	}
}

func TestCheckInactivity_MultiPortOneActiveDoesNotStop(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	info := docker.TargetInfo{ContainerID: "ctr-1", ContainerName: "svc"}
	// Port 9000: idle
	s.targets[9000] = &targetState{
		info:       info,
		targetPort: 80,
		running:    true,
		lastActive: time.Now().Add(-10 * time.Minute),
	}
	// Port 9001: active connection
	ts := &targetState{
		info:       info,
		targetPort: 8080,
		running:    true,
		lastActive: time.Now().Add(-10 * time.Minute),
	}
	ts.activeConns.Store(1)
	s.targets[9001] = ts

	s.checkInactivity(context.Background())

	select {
	case <-stopped:
		t.Error("should not stop when one port still has active connections")
	default:
	}
}

func TestCheckInactivity_UDPIdleWithNoTCPStops(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	// UDP-only container, idle.
	pc, _ := net.ListenPacket("udp", ":0")
	defer pc.Close()
	s.udpTargets[5353] = &udpListenerState{
		listenConn: pc.(*net.UDPConn),
		info:       docker.TargetInfo{ContainerID: "ctr-udp", ContainerName: "dns"},
		running:    true,
		lastActive: time.Now().Add(-10 * time.Minute),
		flows:      make(map[string]*udpFlow),
		pending:    make(map[string]bool),
	}

	s.checkInactivity(context.Background())

	select {
	case id := <-stopped:
		if id != "ctr-udp" {
			t.Errorf("expected ctr-udp, got %s", id)
		}
	default:
		t.Error("expected UDP-only container to be stopped when idle")
	}
}

func TestCheckInactivity_TCPIdleButUDPActiveDoesNotStop(t *testing.T) {
	stopped := make(chan string, 1)
	s := newTestServerWithStopper(func(id string) { stopped <- id })

	info := docker.TargetInfo{ContainerID: "ctr-1", ContainerName: "svc"}

	// TCP: idle
	s.targets[9000] = &targetState{
		info:       info,
		targetPort: 80,
		running:    true,
		lastActive: time.Now().Add(-10 * time.Minute),
	}

	// UDP: has an active flow
	pc, _ := net.ListenPacket("udp", ":0")
	defer pc.Close()
	uls := &udpListenerState{
		listenConn: pc.(*net.UDPConn),
		info:       info,
		running:    true,
		lastActive: time.Now().Add(-10 * time.Minute),
		flows: map[string]*udpFlow{
			"10.0.0.1:5000": {lastActive: time.Now()},
		},
		pending: make(map[string]bool),
	}
	s.udpTargets[5353] = uls

	s.checkInactivity(context.Background())

	select {
	case <-stopped:
		t.Error("should not stop when UDP has active flows")
	default:
	}
}

// ---- helpers ----

// mockDockerManager satisfies dockerManager for inactivity tests.
type mockDockerManager struct {
	stopFunc func(containerID string)
}

func (m *mockDockerManager) EnsureRunning(_ context.Context, _ string) error { return nil }
func (m *mockDockerManager) GetContainerIP(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockDockerManager) StopContainer(_ context.Context, containerID, _ string) error {
	if m.stopFunc != nil {
		m.stopFunc(containerID)
	}
	return nil
}

func newTestServer() *ProxyServer {
	return &ProxyServer{
		ctx:          context.Background(),
		targets:      make(map[int]*targetState),
		udpTargets:   make(map[int]*udpListenerState),
		idleTimeout:  5 * time.Minute,
		pollInterval: 15 * time.Second,
	}
}

// newTestServerWithStopper returns a ProxyServer whose checkInactivity will
// call stopFn instead of a real Docker API.
func newTestServerWithStopper(stopFn func(id string)) *ProxyServer {
	s := newTestServer()
	s.docker = &mockDockerManager{stopFunc: stopFn}
	return s
}

// mustParseNets is a test helper that parses CIDRs/IPs into []net.IPNet.
func mustParseNets(entries ...string) []net.IPNet {
	var out []net.IPNet
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err == nil {
			out = append(out, *ipNet)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		out = append(out, net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return out
}
