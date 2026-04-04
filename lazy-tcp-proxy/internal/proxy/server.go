package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mountain-pass/lazy-tcp-proxy/internal/types"
)

const (
	dialRetries  = 30
	dialInterval = time.Second
	copyBufSize  = 32 * 1024
)

// TargetSnapshot is a point-in-time copy of a single port mapping's state,
// safe to read without holding any lock.
type TargetSnapshot struct {
	ContainerID        string     `json:"container_id"`
	ContainerName      string     `json:"container_name"`
	ListenPort         int        `json:"listen_port"`
	TargetPort         int        `json:"target_port"`
	Running            bool       `json:"running"`
	ActiveConns        int32      `json:"active_conns"`
	LastActive         *time.Time `json:"last_active"`
	LastActiveRelative string     `json:"last_active_relative"`
}

// relativeTime returns a human-readable string describing how long ago t was,
// using only the single largest significant unit.
func relativeTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d >= 365*24*time.Hour:
		return fmt.Sprintf("%d years ago", int(d.Hours()/24/365))
	case d >= 30*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(d.Hours()/24/30))
	case d >= 24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	}
}

// effectiveTimeout returns the per-container idle timeout if set, otherwise the server default.
func effectiveTimeout(perContainer *time.Duration, global time.Duration) time.Duration {
	if perContainer != nil {
		return *perContainer
	}
	return global
}

var copyBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, copyBufSize)
		return &b
	},
}

// targetState holds runtime state for a single listen-port→container-port mapping.
type targetState struct {
	info        types.TargetInfo
	targetPort  int
	listener    net.Listener
	lastActive  time.Time
	activeConns atomic.Int32
	idleTimeout *time.Duration // nil = use server default
	running     bool
	removed     bool
}

// webhookPayload is the JSON body sent to a container's webhook URL.
type webhookPayload struct {
	Event         string `json:"event"`
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Timestamp     string `json:"timestamp"`
}

// containerBackend is the subset of backend methods used by ProxyServer.
type containerBackend interface {
	EnsureRunning(ctx context.Context, targetID string) error
	StopContainer(ctx context.Context, targetID, targetName string) error
	GetUpstreamHost(ctx context.Context, targetID, hint string) (string, error)
}

// ProxyServer manages TCP listeners and proxies connections to targets.
type ProxyServer struct {
	backend       containerBackend
	ctx           context.Context
	mu            sync.RWMutex
	targets       map[int]*targetState     // keyed by TCP listen port
	udpTargets    map[int]*udpListenerState // keyed by UDP listen port
	pollInterval  time.Duration
	idleTimeout   time.Duration
	startTime     time.Time
	webhookClient *http.Client
}

// NewServer creates a new ProxyServer backed by the given backend.
func NewServer(ctx context.Context, b containerBackend, startTime time.Time, idleTimeout, pollInterval time.Duration) *ProxyServer {
	return &ProxyServer{
		backend:       b,
		ctx:           ctx,
		targets:       make(map[int]*targetState),
		udpTargets:    make(map[int]*udpListenerState),
		idleTimeout:   idleTimeout,
		pollInterval:  pollInterval,
		startTime:     startTime,
		webhookClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Snapshot returns a point-in-time copy of all registered targets.
func (s *ProxyServer) Snapshot() []TargetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	out := make([]TargetSnapshot, 0, len(s.targets))
	for listenPort, ts := range s.targets {
		effective := ts.lastActive
		if effective.IsZero() {
			effective = s.startTime
		}
		t := effective
		id := ts.info.ContainerID
		if len(id) > 12 {
			id = id[:12]
		}
		out = append(out, TargetSnapshot{
			ContainerID:        id,
			ContainerName:      ts.info.ContainerName,
			ListenPort:         listenPort,
			TargetPort:         ts.targetPort,
			Running:            ts.running,
			ActiveConns:        ts.activeConns.Load(),
			LastActive:         &t,
			LastActiveRelative: relativeTime(effective, now),
		})
	}
	return out
}

// fireWebhook POSTs a lifecycle event to the container's webhook URL.
// Must be called in a goroutine — never blocks the proxy path.
func (s *ProxyServer) fireWebhook(webhookURL, event, containerID, containerName string) {
	id := containerID
	if len(id) > 12 {
		id = id[:12]
	}
	payload := webhookPayload{
		Event:         event,
		ContainerID:   id,
		ContainerName: containerName,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	resp, err := s.webhookClient.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("proxy: webhook: POST %s event=%s error: %v", webhookURL, event, err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("proxy: webhook: POST %s event=%s non-2xx response: %d", webhookURL, event, resp.StatusCode)
		return
	}
	log.Printf("proxy: webhook: delivered event=%s to %s (%d)", event, webhookURL, resp.StatusCode)
}

// RegisterTarget adds or updates a target. One listener is created per port mapping.
func (s *ProxyServer) RegisterTarget(info types.TargetInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Pre-flight: reject the entire registration if any declared TCP or UDP port
	// is already held by a different container.
	for _, m := range info.Ports {
		if existing, ok := s.targets[m.ListenPort]; ok && existing.info.ContainerID != info.ContainerID {
			log.Printf("\033[31mproxy: TCP port conflict on port %d: already registered by \033[33m%s\033[31m, ignoring \033[33m%s\033[31m\033[0m",
				m.ListenPort, existing.info.ContainerName, info.ContainerName)
			return
		}
	}
	for _, m := range info.UDPPorts {
		if existing, ok := s.udpTargets[m.ListenPort]; ok && existing.info.ContainerID != info.ContainerID {
			log.Printf("\033[31mproxy: UDP port conflict on port %d: already registered by \033[33m%s\033[31m, ignoring \033[33m%s\033[31m\033[0m",
				m.ListenPort, existing.info.ContainerName, info.ContainerName)
			return
		}
	}

	// Register TCP listeners.
	for _, m := range info.Ports {
		if existing, ok := s.targets[m.ListenPort]; ok {
			existing.info = info
			existing.targetPort = m.TargetPort
			existing.idleTimeout = info.IdleTimeout
			existing.running = info.Running
			existing.removed = false
			log.Printf("proxy: updated TCP target \033[33m%s\033[0m on port %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
			continue
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", m.ListenPort))
		if err != nil {
			log.Printf("proxy: failed to listen on TCP port %d for \033[33m%s\033[0m: %v", m.ListenPort, info.ContainerName, err)
			continue
		}

		ts := &targetState{
			info:        info,
			targetPort:  m.TargetPort,
			listener:    ln,
			lastActive:  time.Time{}, // zero — immediately idle
			idleTimeout: info.IdleTimeout,
			running:     info.Running,
		}
		s.targets[m.ListenPort] = ts
		log.Printf("proxy: registered target \033[33m%s\033[0m, TCP %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
		go s.acceptLoop(ts)
	}

	// Register UDP listeners.
	for _, m := range info.UDPPorts {
		if existing, ok := s.udpTargets[m.ListenPort]; ok {
			existing.info = info
			existing.targetPort = m.TargetPort
			existing.idleTimeout = info.IdleTimeout
			existing.running = info.Running
			existing.removed = false
			log.Printf("proxy: updated UDP target \033[33m%s\033[0m on port %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
			continue
		}

		pc, err := net.ListenPacket("udp", fmt.Sprintf(":%d", m.ListenPort))
		if err != nil {
			log.Printf("proxy: failed to listen on UDP port %d for \033[33m%s\033[0m: %v", m.ListenPort, info.ContainerName, err)
			continue
		}
		uls := &udpListenerState{
			listenConn:  pc.(*net.UDPConn),
			targetPort:  m.TargetPort,
			info:        info,
			idleTimeout: info.IdleTimeout,
			running:     info.Running,
			flows:       make(map[string]*udpFlow),
			pending:     make(map[string]bool),
		}
		s.udpTargets[m.ListenPort] = uls
		log.Printf("proxy: registered target \033[33m%s\033[0m, UDP %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
		go s.udpReadLoop(uls)
		go s.udpFlowSweeper(s.ctx, uls, s.pollInterval)
	}
}

// RemoveTarget closes and removes all listeners for the given container.
func (s *ProxyServer) RemoveTarget(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for port, ts := range s.targets {
		if ts.info.ContainerID == containerID {
			log.Printf("proxy: removing target \033[33m%s\033[0m on TCP port %d", ts.info.ContainerName, port)
			ts.removed = true
			if err := ts.listener.Close(); err != nil {
				log.Printf("proxy: error closing TCP listener on port %d: %v", port, err)
			}
			delete(s.targets, port)
		}
	}
	for port, uls := range s.udpTargets {
		if uls.info.ContainerID == containerID {
			log.Printf("proxy: removing target \033[33m%s\033[0m on UDP port %d", uls.info.ContainerName, port)
			uls.removed = true
			if err := uls.listenConn.Close(); err != nil {
				log.Printf("proxy: error closing UDP listener on port %d: %v", port, err)
			}
			delete(s.udpTargets, port)
		}
	}
}

// ContainerStopped marks all port mappings for the given container as stopped
// so the inactivity checker does not issue further stop calls.
func (s *ProxyServer) ContainerStopped(containerID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ts := range s.targets {
		if ts.info.ContainerID == containerID {
			ts.running = false
		}
	}
	for _, uls := range s.udpTargets {
		if uls.info.ContainerID == containerID {
			uls.running = false
		}
	}
}

// RunInactivityChecker periodically stops idle containers.
func (s *ProxyServer) RunInactivityChecker(ctx context.Context, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkInactivity(ctx)
		}
	}
}

func (s *ProxyServer) checkInactivity(ctx context.Context) {
	s.mu.RLock()
	tcpSnap := make([]*targetState, 0, len(s.targets))
	for _, ts := range s.targets {
		tcpSnap = append(tcpSnap, ts)
	}
	udpSnap := make([]*udpListenerState, 0, len(s.udpTargets))
	for _, uls := range s.udpTargets {
		udpSnap = append(udpSnap, uls)
	}
	s.mu.RUnlock()

	// Group by container; eligible only when ALL TCP and UDP mappings are idle.
	type entry struct {
		containerID string
		name        string
		webhookURL  string
		allIdle     bool
		tcpStates   []*targetState
		udpStates   []*udpListenerState
	}
	byContainer := map[string]*entry{}

	for _, ts := range tcpSnap {
		if ts.removed {
			continue
		}
		e, ok := byContainer[ts.info.ContainerID]
		if !ok {
			e = &entry{containerID: ts.info.ContainerID, name: ts.info.ContainerName, webhookURL: ts.info.WebhookURL, allIdle: true}
			byContainer[ts.info.ContainerID] = e
		}
		e.tcpStates = append(e.tcpStates, ts)
		eff := effectiveTimeout(ts.idleTimeout, s.idleTimeout)
		if !ts.running || ts.activeConns.Load() > 0 || time.Since(ts.lastActive) < eff {
			e.allIdle = false
		}
	}

	for _, uls := range udpSnap {
		if uls.removed {
			continue
		}
		e, ok := byContainer[uls.info.ContainerID]
		if !ok {
			e = &entry{containerID: uls.info.ContainerID, name: uls.info.ContainerName, webhookURL: uls.info.WebhookURL, allIdle: true}
			byContainer[uls.info.ContainerID] = e
		}
		e.udpStates = append(e.udpStates, uls)
		uls.mu.Lock()
		activeFlows := len(uls.flows) + len(uls.pending)
		lastActive := uls.lastActive
		uls.mu.Unlock()
		eff := effectiveTimeout(uls.idleTimeout, s.idleTimeout)
		if !uls.running || activeFlows > 0 || time.Since(lastActive) < eff {
			e.allIdle = false
		}
	}

	for _, e := range byContainer {
		if e.allIdle {
			if err := s.backend.StopContainer(ctx, e.containerID, e.name); err != nil {
				log.Printf("proxy: inactivity: error stopping \033[33m%s\033[0m: %v", e.name, err)
			} else {
				for _, ts := range e.tcpStates {
					ts.running = false
				}
				for _, uls := range e.udpStates {
					uls.running = false
				}
				if e.webhookURL != "" {
					go s.fireWebhook(e.webhookURL, "container_stopped", e.containerID, e.name)
				}
			}
		}
	}
}

// ipBlocked returns true if the remote address should be denied based on the
// target's allow-list and block-list.
func ipBlocked(remoteAddr string, info types.TargetInfo) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if len(info.AllowList) > 0 {
		allowed := false
		for _, n := range info.AllowList {
			if n.Contains(ip) {
				allowed = true
				break
			}
		}
		if !allowed {
			return true
		}
	}
	for _, n := range info.BlockList {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// acceptLoop runs in a goroutine for each target listener.
func (s *ProxyServer) acceptLoop(ts *targetState) {
	for {
		conn, err := ts.listener.Accept()
		if err != nil {
			if ts.removed {
				return
			}
			log.Printf("proxy: accept error on port %d: %v", ts.targetPort, err)
			return
		}
		go s.handleConn(conn, ts)
	}
}

// handleConn manages a single inbound connection to a target.
func (s *ProxyServer) handleConn(conn net.Conn, ts *targetState) {
	defer conn.Close() //nolint:errcheck

	ts.activeConns.Add(1)
	defer func() {
		if ts.activeConns.Add(-1) == 0 {
			eff := effectiveTimeout(ts.idleTimeout, s.idleTimeout)
			if eff == 0 {
				log.Printf("proxy: last connection to \033[33m%s\033[0m closed; idle timer started (container will stop immediately if no new connections)",
					ts.info.ContainerName)
			} else {
				log.Printf("proxy: last connection to \033[33m%s\033[0m closed; idle timer started (container will stop in ~%s if no new connections)",
					ts.info.ContainerName, eff)
			}
			go debug.FreeOSMemory()
		}
	}()

	ctx := context.Background()

	if ipBlocked(conn.RemoteAddr().String(), ts.info) {
		log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from \033[36m%s\033[0m \033[31m(blocked)\033[0m",
			ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())
		return
	}
	log.Printf("proxy: new connection to \033[33m%s\033[0m (port %d) from \033[36m%s\033[0m",
		ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())

	if err := s.backend.EnsureRunning(ctx, ts.info.ContainerID); err != nil {
		log.Printf("proxy: could not start container \033[33m%s\033[0m: %v", ts.info.ContainerName, err)
		return
	}
	if ts.info.WebhookURL != "" {
		go s.fireWebhook(ts.info.WebhookURL, "container_started", ts.info.ContainerID, ts.info.ContainerName)
	}

	// Determine preferred network hint (first network ID in list; unused in k8s mode)
	var hint string
	if len(ts.info.NetworkIDs) > 0 {
		hint = ts.info.NetworkIDs[0]
	}

	// Retry dial to upstream
	var upstream net.Conn
	var lastErr error
	for attempt := 1; attempt <= dialRetries; attempt++ {
		host, err := s.backend.GetUpstreamHost(ctx, ts.info.ContainerID, hint)
		if err != nil {
			log.Printf("proxy: attempt %d: could not get upstream host for \033[33m%s\033[0m: %v", attempt, ts.info.ContainerName, err)
			time.Sleep(dialInterval)
			continue
		}

		addr := net.JoinHostPort(host, fmt.Sprintf("%d", ts.targetPort))
		upstream, lastErr = net.DialTimeout("tcp", addr, dialInterval)
		if lastErr == nil {
			break
		}
		log.Printf("proxy: attempt %d: dial %s failed: %v", attempt, addr, lastErr)
		time.Sleep(dialInterval)
	}

	if upstream == nil {
		log.Printf("proxy: exhausted retries connecting to \033[33m%s\033[0m: %v", ts.info.ContainerName, lastErr)
		return
	}
	defer upstream.Close() //nolint:errcheck

	defer func() { ts.lastActive = time.Now() }()

	log.Printf("proxy: proxying connection to %s", upstream.RemoteAddr())

	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			conn.Close()      //nolint:errcheck
			upstream.Close() //nolint:errcheck
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := copyBufPool.Get().(*[]byte)
		defer copyBufPool.Put(buf)
		io.CopyBuffer(upstream, conn, *buf) //nolint:errcheck
		closeAll()
	}()

	go func() {
		defer wg.Done()
		buf := copyBufPool.Get().(*[]byte)
		defer copyBufPool.Put(buf)
		io.CopyBuffer(conn, upstream, *buf) //nolint:errcheck
		closeAll()
	}()

	wg.Wait()
	log.Printf("proxy: connection to \033[33m%s\033[0m closed", ts.info.ContainerName)
}
