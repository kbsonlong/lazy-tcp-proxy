package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nickgrealy/lazy-tpc-proxy/internal/docker"
	"github.com/nickgrealy/lazy-tpc-proxy/internal/proxy"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("lazy-tpc-proxy starting")

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
	srv := proxy.NewServer(mgr)

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
		srv.RunInactivityChecker(ctx)
	}()

	log.Println("lazy-tpc-proxy running; waiting for shutdown signal")
	<-ctx.Done()
	log.Println("lazy-tpc-proxy stopped")
}
