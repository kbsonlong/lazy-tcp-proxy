# Kubernetes Backend

**Date Added**: 2026-04-04
**Priority**: High
**Status**: Completed
**Superseded By**: REQ-049 (Separate Kubernetes Build Artifact) — the `BACKEND=kubernetes` env var
selection mechanism is replaced by image-based selection: use `mountainpass/lazy-tcp-proxy-k8s`
instead of setting `BACKEND=kubernetes`.

## Problem Statement

lazy-tcp-proxy currently supports only Docker as a container backend. Users running workloads on Kubernetes cannot benefit from scale-to-zero proxying without a separate tool. Adding a Kubernetes backend — selected via `BACKEND=kubernetes` — lets the proxy manage Deployment replicas instead of Docker containers, with no change to the proxy's core connection-handling logic.

## Functional Requirements

1. When `BACKEND=kubernetes` is set, the proxy discovers, starts, and stops Kubernetes Deployments instead of Docker containers.
2. Discovery finds all Deployments in the configured namespace(s) annotated with `lazy-tcp-proxy.enabled=true`.
3. "Start" scales the target Deployment from 0 → 1 replica and waits for the pod to become Ready.
4. "Stop" scales the target Deployment from 1 → 0 replicas.
5. The proxy connects to the upstream via the Deployment's Kubernetes Service (by DNS name), not by pod IP.
6. The proxy watches for Deployment annotation changes at runtime (equivalent to `WatchEvents` in Docker mode).
7. When `BACKEND=docker` (default) the proxy behaves exactly as today — no regression.
8. An example Kubernetes project is created under `example/kubernetes/` demonstrating a working setup.

## User Experience Requirements

- Users set `BACKEND=kubernetes` on the proxy Deployment (or pod spec).
- Target Deployments are annotated with the same label convention as Docker containers:
  - `lazy-tcp-proxy.enabled: "true"`
  - `lazy-tcp-proxy.ports: "9000:80"` (listen:targetPort)
  - `lazy-tcp-proxy.udp-ports: "5353:53"` (optional)
  - `lazy-tcp-proxy.idle-timeout-secs: "300"` (optional)
  - `lazy-tcp-proxy.allow-list: "..."` (optional)
  - `lazy-tcp-proxy.block-list: "..."` (optional)
  - `lazy-tcp-proxy.webhook-url: "..."` (optional)
- A Service must exist for each target Deployment so the proxy can reach it by DNS.
- The proxy requires a ServiceAccount with RBAC permissions to list/watch/scale Deployments and list/watch Pods.
- `KUBECONFIG` env var or in-cluster service account token are both supported (auto-detected by client-go).
- `K8S_NAMESPACE` env var scopes discovery to a single namespace; omit (or set to `""`) to watch all namespaces.

## Technical Requirements

- Use `k8s.io/client-go` — the official Go Kubernetes client.
- Extract a `Backend` interface (covering Discover, WatchEvents, EnsureRunning, StopContainer, GetUpstreamAddr) into a new package `internal/backend`.
- `TargetInfo.ContainerID` maps to `namespace/deployment-name` in k8s mode (unique identifier).
- `TargetInfo.ContainerName` maps to the Deployment name (display name).
- `TargetInfo.NetworkIDs` is unused in k8s mode (networking is handled by k8s Services).
- The `dockerManager` interface in `proxy/server.go` is replaced/extended to cover the k8s path without changing proxy logic.
- `GetUpstreamAddr` replaces `GetContainerIP` — in k8s mode it returns the Service DNS address; in Docker mode it returns the container IP as before.
- `JoinNetworks` / `LeaveNetworks` are Docker-specific; the k8s backend is a no-op for these.
- EnsureRunning in k8s mode: patch `scale` subresource to `replicas: 1`, then watch Pod for `Ready` condition (timeout after `dialRetries * dialInterval` = 30 s).
- StopContainer in k8s mode: patch `scale` subresource to `replicas: 0`.

## Acceptance Criteria

- [ ] `BACKEND=docker` (default) passes all existing tests unchanged.
- [ ] `BACKEND=kubernetes` compiles and the k8s backend correctly implements the `Backend` interface.
- [ ] Unit tests for k8s backend using a fake k8s client (no real cluster required).
- [ ] `example/kubernetes/` contains: proxy Deployment, RBAC manifests, and an annotated example target Deployment + Service.
- [ ] `README` or inline comments explain the required annotations and RBAC.
- [ ] `go vet` and `golangci-lint` pass with no new violations.

## Dependencies

- Adds `k8s.io/client-go` and `k8s.io/api` to `go.mod`.
- No changes to existing Docker backend behaviour.
- Depends on REQ-001 (core proxy), REQ-007 (multi-port), REQ-013 (idle timeout), REQ-022 (allow/block lists), REQ-026 (webhooks), REQ-027 (UDP).

## Implementation Notes

- `client-go` auto-detects config: in-cluster (mounted service account token) when running inside a pod; falls back to `~/.kube/config` for local development.
- The existing `dockerManager` interface in `proxy/server.go` (3 methods) is the right seam — extend it or introduce a thin adapter so the proxy server remains unchanged.
- `TargetInfo` lives in `internal/docker` today; it should be moved to `internal/backend` (or a neutral `internal/types` package) so both backends can use it without circular imports.
