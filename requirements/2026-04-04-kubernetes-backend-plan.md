# Kubernetes Backend — Implementation Plan

**Requirement**: [2026-04-04-kubernetes-backend.md](2026-04-04-kubernetes-backend.md)
**Date**: 2026-04-04
**Status**: Implemented

## Implementation Steps

1. Create `internal/types/types.go` — move `PortMapping`, `TargetInfo`, `TargetHandler` and all three parse helpers (`parsePortMappings`, `parseIPList`, `parseIdleTimeoutLabel`) out of `docker/manager.go` into a new neutral package. No logic changes; pure relocation.

2. Update `internal/docker/manager.go` — remove the moved declarations, add `import "internal/types"`, replace all `PortMapping`/`TargetInfo`/`TargetHandler` references with `types.*`. Rename method `GetContainerIP` → `GetUpstreamHost` (signature and all internal call sites). No behavioural change.

3. Update `internal/proxy/server.go` — replace `import "internal/docker"` with `import "internal/types"`. Update all `docker.TargetInfo` → `types.TargetInfo`. Rename interface `dockerManager` → `containerBackend`; rename method `GetContainerIP` → `GetUpstreamHost`. Change `NewServer` to accept `containerBackend` interface instead of `*docker.Manager`.

4. Update `internal/proxy/udp.go` — same import swap (`docker` → `types`); rename `GetContainerIP` call to `GetUpstreamHost`.

5. Update test files — `docker/manager_test.go`, `proxy/server_test.go`, `proxy/integration_test.go`, `main_test.go` for any renamed symbols.

6. Add k8s dependencies — run `go get k8s.io/client-go@latest k8s.io/api@latest k8s.io/apimachinery@latest` inside `lazy-tcp-proxy/`.

7. Create `internal/k8s/backend.go` — implement the `containerBackend` interface plus `Discover`, `WatchEvents`, `Shutdown` (see Key Code Snippets).

8. Create `internal/k8s/backend_test.go` — unit tests using `k8s.io/client-go/kubernetes/fake` (no real cluster).

9. Update `main.go` — add `BACKEND` and `K8S_NAMESPACE` env var resolution; define a local `backendManager` interface combining all methods needed by main; select and wire the correct backend.

10. Create `example/kubernetes/` manifests — `rbac.yaml`, `proxy.yaml`, `example-app.yaml`.

11. Mark requirement Completed, update `_index.md`, commit and push.

---

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/types/types.go` | **Create** | `PortMapping`, `TargetInfo`, `TargetHandler`, 3 parse helpers |
| `internal/docker/manager.go` | Modify | Remove moved types; import `types`; rename `GetContainerIP` → `GetUpstreamHost` |
| `internal/docker/manager_test.go` | Modify | Update renamed symbols |
| `internal/proxy/server.go` | Modify | Import `types`; rename interface + method; accept interface in `NewServer` |
| `internal/proxy/udp.go` | Modify | Import `types`; rename `GetContainerIP` call |
| `internal/proxy/server_test.go` | Modify | Update renamed symbols |
| `internal/proxy/integration_test.go` | Modify | Update renamed symbols if needed |
| `main.go` | Modify | `BACKEND`/`K8S_NAMESPACE` env vars; backend selection |
| `main_test.go` | Modify | Update if symbols changed |
| `lazy-tcp-proxy/go.mod` | Modify | Add `k8s.io/client-go`, `k8s.io/api`, `k8s.io/apimachinery` |
| `lazy-tcp-proxy/go.sum` | Modify | Updated by `go get` |
| `internal/k8s/backend.go` | **Create** | Full k8s backend implementation |
| `internal/k8s/backend_test.go` | **Create** | Unit tests with fake k8s client |
| `example/kubernetes/rbac.yaml` | **Create** | ServiceAccount, ClusterRole, ClusterRoleBinding |
| `example/kubernetes/proxy.yaml` | **Create** | lazy-tcp-proxy Deployment + Service |
| `example/kubernetes/example-app.yaml` | **Create** | Sample annotated target Deployment + Service |
| `requirements/2026-04-04-kubernetes-backend.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

---

## API Contracts

### Environment Variables (new)

| Var | Default | Description |
|-----|---------|-------------|
| `BACKEND` | `docker` | `docker` or `kubernetes` |
| `K8S_NAMESPACE` | `""` (all namespaces) | Scope discovery to one namespace |

### k8s Deployment Annotations (target workloads)

| Annotation | Required | Example |
|------------|----------|---------|
| `lazy-tcp-proxy.enabled` | Yes (as **label**) | `"true"` |
| `lazy-tcp-proxy.ports` | Yes (annotation) | `"9000:80"` |
| `lazy-tcp-proxy.udp-ports` | No | `"5353:53"` |
| `lazy-tcp-proxy.idle-timeout-secs` | No | `"300"` |
| `lazy-tcp-proxy.allow-list` | No | `"10.0.0.0/8"` |
| `lazy-tcp-proxy.block-list` | No | `"192.168.1.5"` |
| `lazy-tcp-proxy.webhook-url` | No | `"https://..."` |
| `lazy-tcp-proxy.service-name` | No | Override Service name (default: Deployment name) |

> Note: `lazy-tcp-proxy.enabled=true` is a **label** (not annotation) so the k8s API server can filter by it. All configuration values are **annotations** (values contain colons/commas which are invalid in label values).

### TargetID format (k8s mode)

`ContainerID` = `"<namespace>/<deployment-name>"` — used as the unique key throughout the proxy, passed to `EnsureRunning`, `StopTarget`, `GetUpstreamHost`.

---

## Key Code Snippets

### `internal/types/types.go`

```go
package types

import (
    "net"
    "time"
)

type PortMapping struct {
    ListenPort int
    TargetPort int
}

type TargetInfo struct {
    ContainerID   string
    ContainerName string
    Ports         []PortMapping
    UDPPorts      []PortMapping
    NetworkIDs    []string       // Docker only; empty in k8s mode
    AllowList     []net.IPNet
    BlockList     []net.IPNet
    IdleTimeout   *time.Duration
    Running       bool
    WebhookURL    string
}

type TargetHandler interface {
    RegisterTarget(info TargetInfo)
    RemoveTarget(containerID string)
    ContainerStopped(containerID string)
}

// parsePortMappings, parseIPList, parseIdleTimeoutLabel move here unchanged.
```

### `internal/proxy/server.go` — renamed interface

```go
// containerBackend is the subset of backend methods used by ProxyServer.
type containerBackend interface {
    EnsureRunning(ctx context.Context, targetID string) error
    StopContainer(ctx context.Context, targetID, targetName string) error
    GetUpstreamHost(ctx context.Context, targetID, hint string) (string, error)
}
```

### `internal/k8s/backend.go` — struct and constructor

```go
package k8s

type Backend struct {
    client       *kubernetes.Clientset
    namespace    string          // "" = all namespaces
    mu           sync.RWMutex
    serviceNames map[string]string // targetID → Service name
}

func NewBackend(namespace string) (*Backend, error) {
    cfg, err := rest.InClusterConfig()
    if err != nil {
        // Fall back to kubeconfig (local dev / KUBECONFIG env var)
        cfg, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
        if err != nil {
            return nil, fmt.Errorf("k8s: could not build config: %w", err)
        }
    }
    clientset, err := kubernetes.NewForConfig(cfg)
    if err != nil {
        return nil, fmt.Errorf("k8s: could not create clientset: %w", err)
    }
    return &Backend{client: clientset, namespace: namespace, serviceNames: make(map[string]string)}, nil
}
```

### `internal/k8s/backend.go` — EnsureRunning

```go
func (b *Backend) EnsureRunning(ctx context.Context, targetID string) error {
    ns, name := splitID(targetID)
    scale, err := b.client.AppsV1().Deployments(ns).GetScale(ctx, name, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("k8s: get scale %s: %w", targetID, err)
    }
    if scale.Spec.Replicas > 0 {
        return nil // already running
    }
    scale.Spec.Replicas = 1
    _, err = b.client.AppsV1().Deployments(ns).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
    if err != nil {
        return fmt.Errorf("k8s: scale up %s: %w", targetID, err)
    }
    log.Printf("k8s: scaled up deployment \033[33m%s\033[0m", name)
    return nil
    // Readiness is handled by the existing proxy dial-retry loop (30 × 1s).
}
```

### `internal/k8s/backend.go` — StopContainer

```go
func (b *Backend) StopContainer(ctx context.Context, targetID, targetName string) error {
    ns, name := splitID(targetID)
    scale, err := b.client.AppsV1().Deployments(ns).GetScale(ctx, name, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("k8s: get scale %s: %w", targetID, err)
    }
    if scale.Spec.Replicas == 0 {
        return nil
    }
    log.Printf("k8s: scaling down deployment \033[33m%s\033[0m (idle timeout)", targetName)
    scale.Spec.Replicas = 0
    _, err = b.client.AppsV1().Deployments(ns).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
    return err
}
```

### `internal/k8s/backend.go` — GetUpstreamHost

```go
func (b *Backend) GetUpstreamHost(ctx context.Context, targetID, _ string) (string, error) {
    b.mu.RLock()
    svcName, ok := b.serviceNames[targetID]
    b.mu.RUnlock()
    if !ok {
        _, svcName = splitID(targetID) // default: Deployment name
    }
    ns, _ := splitID(targetID)
    return fmt.Sprintf("%s.%s.svc.cluster.local", svcName, ns), nil
}
```

### `internal/k8s/backend.go` — Discover

```go
func (b *Backend) Discover(ctx context.Context, handler types.TargetHandler) error {
    deployments, err := b.client.AppsV1().Deployments(b.namespace).List(ctx, metav1.ListOptions{
        LabelSelector: "lazy-tcp-proxy.enabled=true",
    })
    if err != nil {
        return fmt.Errorf("k8s: list deployments: %w", err)
    }
    for _, d := range deployments.Items {
        info, err := b.deploymentToTargetInfo(d)
        if err != nil {
            log.Printf("k8s: skipping deployment %s/%s: %v", d.Namespace, d.Name, err)
            continue
        }
        b.storeServiceName(info.ContainerID, d.Annotations)
        handler.RegisterTarget(info)
    }
    return nil
}
```

### `internal/k8s/backend.go` — WatchEvents

```go
func (b *Backend) WatchEvents(ctx context.Context, handler types.TargetHandler) {
    backoff := time.Second
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }
        watcher, err := b.client.AppsV1().Deployments(b.namespace).Watch(ctx, metav1.ListOptions{
            LabelSelector: "lazy-tcp-proxy.enabled=true",
        })
        if err != nil {
            log.Printf("k8s: watch error: %v; retrying in %s", err, backoff)
            time.Sleep(backoff)
            backoff = min(backoff*2, 30*time.Second)
            continue
        }
        backoff = time.Second
        for event := range watcher.ResultChan() {
            d, ok := event.Object.(*appsv1.Deployment)
            if !ok {
                continue
            }
            targetID := d.Namespace + "/" + d.Name
            switch event.Type {
            case watch.Added, watch.Modified:
                info, err := b.deploymentToTargetInfo(*d)
                if err != nil {
                    log.Printf("k8s: event: skipping %s: %v", targetID, err)
                    continue
                }
                b.storeServiceName(targetID, d.Annotations)
                handler.RegisterTarget(info)
            case watch.Deleted:
                handler.RemoveTarget(targetID)
            }
        }
    }
}
```

### `main.go` — backend selection

```go
const defaultBackend = "docker"

type backendManager interface {
    Discover(ctx context.Context, handler types.TargetHandler) error
    WatchEvents(ctx context.Context, handler types.TargetHandler)
    EnsureRunning(ctx context.Context, targetID string) error
    StopContainer(ctx context.Context, targetID, targetName string) error
    GetUpstreamHost(ctx context.Context, targetID, hint string) (string, error)
    Shutdown(ctx context.Context)
}

func resolveBackend(ctx context.Context) (backendManager, error) {
    switch strings.ToLower(os.Getenv("BACKEND")) {
    case "kubernetes", "k8s":
        ns := os.Getenv("K8S_NAMESPACE")
        log.Printf("backend: kubernetes (namespace=%q)", ns)
        return k8s.NewBackend(ns)
    default:
        log.Printf("backend: docker")
        return docker.NewManager()
    }
}
```

---

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| `TestDiscover_NoDeployments` | Fake client with no deployments | `handler.RegisterTarget` called 0 times |
| `TestDiscover_OneDeployment` | Fake client with 1 annotated deployment | `RegisterTarget` called once with correct TargetInfo |
| `TestDiscover_MissingPortsAnnotation` | Deployment without `lazy-tcp-proxy.ports` | Deployment skipped, no panic |
| `TestEnsureRunning_ScalesUp` | Fake deployment with replicas=0 | `UpdateScale` called with replicas=1 |
| `TestEnsureRunning_AlreadyRunning` | Fake deployment with replicas=1 | `UpdateScale` not called |
| `TestStopContainer_ScalesDown` | Fake deployment with replicas=1 | `UpdateScale` called with replicas=0 |
| `TestStopContainer_AlreadyStopped` | Fake deployment with replicas=0 | `UpdateScale` not called |
| `TestGetUpstreamHost_DefaultsToDeploymentName` | targetID=`"default/myapp"`, no service annotation | Returns `"myapp.default.svc.cluster.local"` |
| `TestGetUpstreamHost_ServiceNameOverride` | `lazy-tcp-proxy.service-name: "myapp-svc"` | Returns `"myapp-svc.default.svc.cluster.local"` |
| `TestWatchEvents_AddedTriggersRegister` | Watch event type=Added | `RegisterTarget` called |
| `TestWatchEvents_DeletedTriggersRemove` | Watch event type=Deleted | `RemoveTarget` called |

---

## Risks & Open Questions

1. **client-go transitive dependencies** — client-go pulls in large transitive trees (apimachinery, api, etc.). The binary size will increase noticeably. Acceptable for this use case.

2. **`docker.Manager.Shutdown`** — `LeaveNetworks` in Docker mode is the shutdown hook. The k8s backend's `Shutdown` is a no-op. The `backendManager` interface in `main.go` needs a `Shutdown(ctx)` method; `docker.Manager` needs a `Shutdown` method added as an alias for `LeaveNetworks` (or `main.go` type-asserts for the Docker case).

   Cleanest solution: add `Shutdown(ctx context.Context)` to `docker.Manager` that calls `LeaveNetworks` internally. Then both backends satisfy the same interface.

3. **All-namespace watch** — when `K8S_NAMESPACE=""` the proxy watches all namespaces. This requires a ClusterRole rather than a Role. The RBAC example should cover both cases with comments.

4. **Fake client limitations** — `k8s.io/client-go/kubernetes/fake` does not support the `scale` subresource natively. Tests for `EnsureRunning`/`StopContainer` will need to use `k8sfake.NewSimpleClientset` with a `reactor` to intercept scale calls, or test at a higher level of abstraction.
