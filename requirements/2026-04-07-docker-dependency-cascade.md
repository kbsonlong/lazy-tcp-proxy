# Dependency Cascade (lazy-tcp-proxy.dependants)

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: In Progress

## Problem Statement

When a managed "hub" target (e.g. `selenium-hub`) starts or stops due to
lazy-tcp-proxy's idle lifecycle management, its downstream dependants
(e.g. `chromium`, `firefox`) are not automatically started or stopped.  This
leaves orphaned running nodes when the hub is idle, and unready nodes when the
hub is first woken by traffic.

Docker Compose's `depends_on` addresses startup ordering but is a
Docker-only concept and is not persisted in a way that is accessible at
runtime on Kubernetes.  A backend-agnostic, explicit label/annotation is
required.

## Functional Requirements

1. **Explicit opt-in label** — The upstream (hub) container/deployment declares
   its dependants via a single label (Docker) or annotation (Kubernetes):
   ```
   lazy-tcp-proxy.dependants=<name1>,<name2>,...
   ```
   Values are the `ContainerName` / Deployment names of managed downstream
   targets.

2. **Dependency stored in TargetInfo** — `Dependants []string` is added to
   `types.TargetInfo` and populated by both the Docker manager and the k8s
   backend when they parse labels/annotations.

3. **Start cascade** — When a managed upstream target starts (Docker `start`
   event; k8s `Modified` event with replicas > 0; or traffic-triggered
   `EnsureRunning`), immediately start all registered dependant targets.

4. **Stop cascade** — When a managed upstream target stops (idle timeout or
   external stop event), immediately stop all registered dependant targets.

5. **Managed-only** — Only targets registered with the proxy are eligible for
   cascade. Unrecognised names in the `dependants` label are logged and
   skipped.

6. **No-op on correct state** — Starting an already-running dependant or
   stopping an already-stopped dependant is a silent no-op.

7. **Backend-agnostic** — Works identically with `BACKEND=docker` and
   `BACKEND=kubernetes`.

## User Experience Requirements

- **Docker** — Add a label to the upstream container:
  ```yaml
  labels:
    lazy-tcp-proxy.enabled: "true"
    lazy-tcp-proxy.ports: "4444:4444"
    lazy-tcp-proxy.dependants: "selenium-chromium,selenium-firefox"
  ```

- **Kubernetes** — Add an annotation to the upstream Deployment:
  ```yaml
  annotations:
    lazy-tcp-proxy.ports: "4444:4444"
    lazy-tcp-proxy.dependants: "selenium-chromium,selenium-firefox"
  ```
  (label `lazy-tcp-proxy.enabled: "true"` still required on the Deployment)

- Log lines clearly indicate cascade actions:
  ```
  proxy: cascade start: selenium-hub → selenium-chromium
  proxy: cascade start: selenium-hub → selenium-firefox
  proxy: cascade stop:  selenium-hub → selenium-chromium
  ```

## Technical Requirements

- New field `Dependants []string` in `internal/types/types.go` `TargetInfo`.
- New `ParseDependants(s string) []string` helper in `internal/types/types.go`.
- Docker manager (`internal/docker/manager.go`): read optional label
  `lazy-tcp-proxy.dependants`.
- k8s backend (`internal/k8s/backend.go`): read optional annotation
  `lazy-tcp-proxy.dependants`.
- New method `ContainerStarted(containerID string)` added to
  `types.TargetHandler` interface and implemented by `ProxyServer`.
- Cascade maps live in `ProxyServer`, protected by the existing `s.mu`.
- Cascade operations run in a goroutine to avoid blocking the event loop.

## Acceptance Criteria

- [ ] A managed upstream target with `lazy-tcp-proxy.dependants=a,b` causes
      `a` and `b` to be started when the upstream starts.
- [ ] A managed upstream target with `lazy-tcp-proxy.dependants=a,b` causes
      `a` and `b` to be stopped when the upstream stops (idle or external).
- [ ] Already-running dependants are not double-started (no error logged).
- [ ] Already-stopped dependants are not double-stopped (no error logged).
- [ ] An unrecognised dependant name is logged and skipped; other valid
      dependants are still processed.
- [ ] Feature works with both `BACKEND=docker` and `BACKEND=kubernetes`.
- [ ] README documents the `lazy-tcp-proxy.dependants` label/annotation.

## Dependencies

- `internal/types/types.go` — `TargetInfo`, `TargetHandler`
- `internal/docker/manager.go` — Docker label parsing
- `internal/k8s/backend.go` — k8s annotation parsing
- `internal/proxy/server.go` — `ProxyServer`, cascade logic

## Implementation Notes

- Cascade is **not** recursive in this version (no transitive chains).
- The `lazy-tcp-proxy.dependants` label replaces any use of the Docker Compose
  `com.docker.compose.depends_on` label — the latter is not read.
- For k8s, `ContainerStarted` is called from `WatchEvents` on every
  `Modified` event where `info.Running == true`; since `EnsureRunning` is
  already idempotent, spurious calls (e.g. annotation-only updates) are safe.
