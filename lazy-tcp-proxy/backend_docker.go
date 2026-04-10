//go:build !kubernetes

package main

import (
	"log"

	"github.com/mountain-pass/lazy-tcp-proxy/internal/docker"
)

func resolveBackend() (backendManager, error) {
	log.Printf("backend: docker")
	return docker.NewManager()
}
