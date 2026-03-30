package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nickgrealy/lazy-tcp-proxy/internal/docker"
)

const (
	dialRetries   = 30
	dialInterval  = time.Second
	idleTimeout   = 2 * time.Minute
	inactivityTick = 30 * time.Second
)

// targetState holds runtime state for a single listen-port→container-port mapping.
type targetState struct {
	info        docker.TargetInfo
	targetPort  int
	listener    net.Listener
	lastActive  time.Time
	activeConns atomic.Int32
	removed     bool
}

// ProxyServer manages TCP listeners and proxies connections to Docker containers.
type ProxyServer struct {
	docker  *docker.Manager
	mu      sync.RWMutex
	// keyed by port number
	targets map[int]*targetState
}

// NewServer creates a new ProxyServer backed by the given DockerManager.
func NewServer(d *docker.Manager) *ProxyServer {
	return &ProxyServer{
		docker:  d,
		targets: make(map[int]*targetState),
	}
}

// RegisterTarget adds or updates a target. One listener is created per port mapping.
func (s *ProxyServer) RegisterTarget(info docker.TargetInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range info.Ports {
		if existing, ok := s.targets[m.ListenPort]; ok {
			existing.info = info
			existing.targetPort = m.TargetPort
			existing.removed = false
			log.Printf("proxy: updated target %s on port %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
			continue
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", m.ListenPort))
		if err != nil {
			log.Printf("proxy: failed to listen on port %d for %s: %v", m.ListenPort, info.ContainerName, err)
			continue
		}

		ts := &targetState{
			info:       info,
			targetPort: m.TargetPort,
			listener:   ln,
			lastActive: time.Now(),
		}
		s.targets[m.ListenPort] = ts
		log.Printf("proxy: registered target %s, listening on port %d->%d", info.ContainerName, m.ListenPort, m.TargetPort)
		go s.acceptLoop(ts)
	}
}

// RemoveTarget closes and removes all listeners for the given container.
func (s *ProxyServer) RemoveTarget(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for port, ts := range s.targets {
		if ts.info.ContainerID == containerID {
			log.Printf("proxy: removing target %s on port %d", ts.info.ContainerName, port)
			ts.removed = true
			ts.listener.Close()
			delete(s.targets, port)
		}
	}
}

// RunInactivityChecker periodically stops idle containers.
func (s *ProxyServer) RunInactivityChecker(ctx context.Context) {
	ticker := time.NewTicker(inactivityTick)
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
	snapshot := make([]*targetState, 0, len(s.targets))
	for _, ts := range s.targets {
		snapshot = append(snapshot, ts)
	}
	s.mu.RUnlock()

	// Group by container; a container is eligible only when ALL its mappings are idle.
	type entry struct {
		containerID string
		name        string
		allIdle     bool
		states      []*targetState
	}
	byContainer := map[string]*entry{}
	for _, ts := range snapshot {
		if ts.removed {
			continue
		}
		e, ok := byContainer[ts.info.ContainerID]
		if !ok {
			e = &entry{containerID: ts.info.ContainerID, name: ts.info.ContainerName, allIdle: true}
			byContainer[ts.info.ContainerID] = e
		}
		e.states = append(e.states, ts)
		if ts.activeConns.Load() > 0 || time.Since(ts.lastActive) < idleTimeout {
			e.allIdle = false
		}
	}
	for _, e := range byContainer {
		if e.allIdle {
			if err := s.docker.StopContainer(ctx, e.containerID); err != nil {
				log.Printf("proxy: inactivity: error stopping %s: %v", e.name, err)
			} else {
				// Reset lastActive so the checker backs off for a full idleTimeout
				// before attempting to stop this container again.
				now := time.Now()
				for _, ts := range e.states {
					ts.lastActive = now
				}
			}
		}
	}
}

// acceptLoop runs in a goroutine for each target listener.
func (s *ProxyServer) acceptLoop(ts *targetState) {
	for {
		conn, err := ts.listener.Accept()
		if err != nil {
			if ts.removed {
				return
			}
			// Check if listener was closed
			select {
			default:
			}
			log.Printf("proxy: accept error on port %d: %v", ts.targetPort, err)
			return
		}
		go s.handleConn(conn, ts)
	}
}

// handleConn manages a single inbound connection to a target container.
func (s *ProxyServer) handleConn(conn net.Conn, ts *targetState) {
	defer conn.Close()

	// Increment activeConns immediately so the inactivity checker does not stop
	// the container while we are starting it or waiting for the upstream dial.
	ts.activeConns.Add(1)
	defer ts.activeConns.Add(-1)

	ctx := context.Background()

	log.Printf("proxy: new connection to %s (port %d) from %s",
		ts.info.ContainerName, ts.targetPort, conn.RemoteAddr())

	// Ensure the container is running
	if err := s.docker.EnsureRunning(ctx, ts.info.ContainerID); err != nil {
		log.Printf("proxy: could not start container %s: %v", ts.info.ContainerName, err)
		return
	}

	// Determine preferred network (first in list)
	var preferNet string
	if len(ts.info.NetworkIDs) > 0 {
		preferNet = ts.info.NetworkIDs[0]
	}

	// Retry dial to container
	var upstream net.Conn
	var lastErr error
	for attempt := 1; attempt <= dialRetries; attempt++ {
		ip, err := s.docker.GetContainerIP(ctx, ts.info.ContainerID, preferNet)
		if err != nil {
			log.Printf("proxy: attempt %d: could not get IP for %s: %v", attempt, ts.info.ContainerName, err)
			time.Sleep(dialInterval)
			continue
		}

		addr := fmt.Sprintf("%s:%d", ip, ts.targetPort)
		upstream, lastErr = net.DialTimeout("tcp", addr, dialInterval)
		if lastErr == nil {
			break
		}
		log.Printf("proxy: attempt %d: dial %s failed: %v", attempt, addr, lastErr)
		time.Sleep(dialInterval)
	}

	if upstream == nil {
		log.Printf("proxy: exhausted retries connecting to %s: %v", ts.info.ContainerName, lastErr)
		return
	}
	defer upstream.Close()

	// Update lastActive when the proxied connection closes (successful activity).
	defer func() { ts.lastActive = time.Now() }()

	log.Printf("proxy: proxying connection to %s", upstream.RemoteAddr())

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(upstream, conn); err != nil {
			// Ignore closed connection errors
		}
		// Half-close
		if tc, ok := upstream.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn, upstream); err != nil {
			// Ignore closed connection errors
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	wg.Wait()
	log.Printf("proxy: connection to %s closed", ts.info.ContainerName)
}
