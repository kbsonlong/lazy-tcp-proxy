package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mountain-pass/lazy-tcp-proxy/internal/docker"
	"github.com/mountain-pass/lazy-tcp-proxy/internal/proxy"
)

const (
	defaultPollInterval = 15 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

func resolveIdleTimeout() time.Duration {
	raw := os.Getenv("IDLE_TIMEOUT_SECS")
	if raw == "" {
		return defaultIdleTimeout
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("IDLE_TIMEOUT_SECS=%q is invalid; using default %s", raw, defaultIdleTimeout)
		return defaultIdleTimeout
	}
	return time.Duration(n) * time.Second
}

func resolvePollInterval() time.Duration {
	raw := os.Getenv("POLL_INTERVAL_SECS")
	if raw == "" {
		return defaultPollInterval
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("POLL_INTERVAL_SECS=%q is invalid; using default %s", raw, defaultPollInterval)
		return defaultPollInterval
	}
	return time.Duration(n) * time.Second
}

const defaultStatusPort = 8080

func resolveStatusPort() int {
	raw := os.Getenv("STATUS_PORT")
	if raw == "" {
		return defaultStatusPort
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		log.Printf("STATUS_PORT=%q is invalid; using default %d", raw, defaultStatusPort)
		return defaultStatusPort
	}
	return n // 0 means disabled
}

func runStatusServer(ctx context.Context, srv *proxy.ProxyServer, port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(srv.Snapshot()) //nolint:errcheck
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	hs := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	context.AfterFunc(ctx, func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(shutCtx) //nolint:errcheck
	})
	go func() {
		if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("status server: %v", err)
		}
	}()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("lazy-tcp-proxy starting")

	// Root context cancelled on shutdown signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %s; shutting down", sig)
		cancel()
	}()

	// Create the Docker manager
	mgr, err := docker.NewManager()
	if err != nil {
		log.Fatalf("failed to create docker manager: %v", err)
	}

	// Create the proxy server
	idleTimeout := resolveIdleTimeout()
	log.Printf("idle timeout: %s (set IDLE_TIMEOUT_SECS to override)", idleTimeout)
	tick := resolvePollInterval()
	log.Printf("inactivity check interval: %s (set POLL_INTERVAL_SECS to override)", tick)
	srv := proxy.NewServer(ctx, mgr, idleTimeout, tick)

	// Start the HTTP status server
	statusPort := resolveStatusPort()
	if statusPort == 0 {
		log.Println("status server: disabled (STATUS_PORT=0)")
	} else {
		log.Printf("status server: listening on :%d (set STATUS_PORT=0 to disable)", statusPort)
		runStatusServer(ctx, srv, statusPort)
	}

	// Initial discovery of all matching containers
	log.Println("performing initial container discovery...")
	if err := mgr.Discover(ctx, srv); err != nil {
		log.Printf("initial discovery error: %v", err)
	}

	// Watch Docker events for runtime changes
	go func() {
		mgr.WatchEvents(ctx, srv)
	}()

	// Periodically stop idle containers
	go func() {
		srv.RunInactivityChecker(ctx, tick)
	}()

	log.Println("lazy-tcp-proxy running; waiting for shutdown signal")
	<-ctx.Done()
	log.Println("lazy-tcp-proxy shutting down")
	mgr.LeaveNetworks(context.Background())
	log.Println("lazy-tcp-proxy stopped")
}
