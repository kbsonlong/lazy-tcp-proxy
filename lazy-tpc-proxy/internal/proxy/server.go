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

	"github.com/nickgrealy/lazy-tpc-proxy/internal/docker"
)

const (
	dialRetries   = 30
	dialInterval  = time.Second
	idleTimeout   = 2 * time.Minute
	inactivityTick = 30 * time.Second
)

// targetState holds runtime state for a single proxy target.
type targetState struct {
	info        docker.TargetInfo
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

// RegisterTarget adds or updates a target. If a listener for the port does not
// yet exist it is started in a background goroutine.
func (s *ProxyServer) RegisterTarget(info docker.TargetInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.targets[info.Port]
	if ok {
		// Update metadata but keep the existing listener.
		existing.info = info
		existing.removed = false
		log.Printf("proxy: updated target %s on port %d", info.ContainerName, info.Port)
		return
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", info.Port))
	if err != nil {
		log.Printf("proxy: failed to listen on port %d for %s: %v", info.Port, info.ContainerName, err)
		return
	}

	ts := &targetState{
		info:       info,
		listener:   ln,
		lastActive: time.Now(),
	}
	s.targets[info.Port] = ts

	log.Printf("proxy: registered target %s, listening on port %d", info.ContainerName, info.Port)

	go s.acceptLoop(ts)
}

// RemoveTarget marks a target as removed and closes its listener.
func (s *ProxyServer) RemoveTarget(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for port, ts := range s.targets {
		if ts.info.ContainerID == containerID {
			log.Printf("proxy: removing target %s on port %d", ts.info.ContainerName, port)
			ts.removed = true
			ts.listener.Close()
			delete(s.targets, port)
			return
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

	for _, ts := range snapshot {
		if ts.removed {
			continue
		}
		if ts.activeConns.Load() > 0 {
			continue
		}
		if time.Since(ts.lastActive) < idleTimeout {
			continue
		}
		// Stop the container
		if err := s.docker.StopContainer(ctx, ts.info.ContainerID); err != nil {
			log.Printf("proxy: inactivity: error stopping %s: %v", ts.info.ContainerName, err)
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
			log.Printf("proxy: accept error on port %d: %v", ts.info.Port, err)
			return
		}
		go s.handleConn(conn, ts)
	}
}

// handleConn manages a single inbound connection to a target container.
func (s *ProxyServer) handleConn(conn net.Conn, ts *targetState) {
	defer conn.Close()

	ctx := context.Background()

	log.Printf("proxy: new connection to %s (port %d) from %s",
		ts.info.ContainerName, ts.info.Port, conn.RemoteAddr())

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

		addr := fmt.Sprintf("%s:%d", ip, ts.info.Port)
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

	ts.activeConns.Add(1)
	defer func() {
		ts.activeConns.Add(-1)
		ts.lastActive = time.Now()
	}()

	log.Printf("proxy: proxying connection to %s:%d", upstream.RemoteAddr(), ts.info.Port)

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
