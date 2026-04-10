//go:build kubernetes

package main

import (
	"log"
	"os"

	k8sbackend "github.com/mountain-pass/lazy-tcp-proxy/internal/k8s"
)

func resolveBackend() (backendManager, error) {
	ns := os.Getenv("K8S_NAMESPACE")
	log.Printf("backend: kubernetes (namespace=%q)", ns)
	return k8sbackend.NewBackend(ns)
}
