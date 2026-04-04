package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mountain-pass/lazy-tcp-proxy/internal/docker"
)

const udpBufSize = 65535

// udpFlow represents one active client→container UDP flow.
type udpFlow struct {
	clientAddr   *net.UDPAddr
	upstreamConn *net.UDPConn
	lastActive   time.Time
}

// udpListenerState holds the shared inbound UDP socket and all active flows
// for a single UDP listen-port → container-port mapping.
type udpListenerState struct {
	listenConn  *net.UDPConn
	targetPort  int
	info        docker.TargetInfo
	lastActive  time.Time
	activeFlows atomic.Int32
	idleTimeout *time.Duration // nil = use server default
	running     bool
	removed     bool
	mu          sync.Mutex
	flows       map[string]*udpFlow // key: clientAddr.String()
	pending     map[string]bool     // client addrs whose flows are being established
}

// udpReadLoop is the core datagram dispatch loop. Runs in a goroutine per listener.
func (s *ProxyServer) udpReadLoop(uls *udpListenerState) {
	buf := make([]byte, udpBufSize)
	for {
		n, clientAddr, err := uls.listenConn.ReadFromUDP(buf)
		if err != nil {
			if uls.removed {
				return
			}
			log.Printf("proxy: udp: read error on port %d: %v", uls.targetPort, err)
			return
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		if ipBlocked(clientAddr.String(), uls.info) {
			log.Printf("proxy: udp: datagram from \033[36m%s\033[0m to \033[33m%s\033[0m \033[31m(blocked)\033[0m",
				clientAddr, uls.info.ContainerName)
			continue
		}

		key := clientAddr.String()

		uls.mu.Lock()
		flow, exists := uls.flows[key]
		if !exists {
			if uls.pending[key] {
				// Flow establishment already in progress; drop — UDP clients retransmit.
				uls.mu.Unlock()
				continue
			}
			uls.pending[key] = true
			uls.mu.Unlock()
			log.Printf("proxy: udp: new flow from \033[36m%s\033[0m to \033[33m%s\033[0m (port %d)",
				clientAddr, uls.info.ContainerName, uls.targetPort)
			go s.startUDPFlow(uls, clientAddr, data)
			continue
		}
		flow.lastActive = time.Now()
		uls.lastActive = time.Now()
		conn := flow.upstreamConn
		uls.mu.Unlock()

		if _, err := conn.Write(data); err != nil {
			log.Printf("proxy: udp: write to upstream for \033[33m%s\033[0m failed: %v", uls.info.ContainerName, err)
		}
	}
}

// startUDPFlow ensures the container is running, dials the upstream UDP port,
// registers the flow, and launches the upstream read loop.
func (s *ProxyServer) startUDPFlow(uls *udpListenerState, clientAddr *net.UDPAddr, firstDatagram []byte) {
	key := clientAddr.String()
	ctx := context.Background()

	cleanup := func() {
		uls.mu.Lock()
		delete(uls.pending, key)
		uls.mu.Unlock()
	}

	if err := s.docker.EnsureRunning(ctx, uls.info.ContainerID); err != nil {
		log.Printf("proxy: udp: could not start container \033[33m%s\033[0m: %v", uls.info.ContainerName, err)
		cleanup()
		return
	}

	var preferNet string
	if len(uls.info.NetworkIDs) > 0 {
		preferNet = uls.info.NetworkIDs[0]
	}
	ip, err := s.docker.GetContainerIP(ctx, uls.info.ContainerID, preferNet)
	if err != nil {
		log.Printf("proxy: udp: could not get IP for \033[33m%s\033[0m: %v", uls.info.ContainerName, err)
		cleanup()
		return
	}

	upstreamAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, uls.targetPort))
	if err != nil {
		log.Printf("proxy: udp: could not resolve upstream addr for \033[33m%s\033[0m: %v", uls.info.ContainerName, err)
		cleanup()
		return
	}
	upstreamConn, err := net.DialUDP("udp", nil, upstreamAddr)
	if err != nil {
		log.Printf("proxy: udp: could not dial upstream for \033[33m%s\033[0m: %v", uls.info.ContainerName, err)
		cleanup()
		return
	}

	flow := &udpFlow{
		clientAddr:   clientAddr,
		upstreamConn: upstreamConn,
		lastActive:   time.Now(),
	}

	uls.mu.Lock()
	delete(uls.pending, key)
	uls.flows[key] = flow
	uls.lastActive = time.Now()
	uls.mu.Unlock()

	uls.activeFlows.Add(1)
	go s.udpUpstreamReadLoop(uls, flow)

	if _, err := upstreamConn.Write(firstDatagram); err != nil {
		log.Printf("proxy: udp: write first datagram to \033[33m%s\033[0m failed: %v", uls.info.ContainerName, err)
	}
}

// udpUpstreamReadLoop reads response datagrams from the container and forwards
// them back to the originating client via the shared listen socket.
func (s *ProxyServer) udpUpstreamReadLoop(uls *udpListenerState, flow *udpFlow) {
	defer uls.activeFlows.Add(-1)
	buf := make([]byte, udpBufSize)
	for {
		n, err := flow.upstreamConn.Read(buf)
		if err != nil {
			return // conn closed by sweeper or removal
		}
		if _, err := uls.listenConn.WriteToUDP(buf[:n], flow.clientAddr); err != nil {
			log.Printf("proxy: udp: write response to client \033[36m%s\033[0m failed: %v", flow.clientAddr, err)
		}
		uls.mu.Lock()
		flow.lastActive = time.Now()
		uls.lastActive = time.Now()
		uls.mu.Unlock()
	}
}

// udpFlowSweeper prunes flows that have been idle longer than idleTimeout.
func (s *ProxyServer) udpFlowSweeper(ctx context.Context, uls *udpListenerState, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if uls.removed {
				return
			}
			now := time.Now()
			eff := effectiveTimeout(uls.idleTimeout, s.idleTimeout)
			uls.mu.Lock()
			for key, flow := range uls.flows {
				if now.Sub(flow.lastActive) > eff {
					log.Printf("proxy: udp: flow \033[36m%s\033[0m -> \033[33m%s\033[0m expired",
						flow.clientAddr, uls.info.ContainerName)
					if err := flow.upstreamConn.Close(); err != nil {
						log.Printf("proxy: udp: error closing upstream conn for flow %s: %v", flow.clientAddr, err)
					}
					delete(uls.flows, key)
				}
			}
			uls.mu.Unlock()
		}
	}
}
