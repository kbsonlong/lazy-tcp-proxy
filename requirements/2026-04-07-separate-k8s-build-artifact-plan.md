# Separate Kubernetes Build Artifact — Implementation Plan

**Requirement**: [2026-04-07-separate-k8s-build-artifact.md](2026-04-07-separate-k8s-build-artifact.md)
**Date**: 2026-04-07
**Status**: Draft

## Design Refinement

The design proposed build-tagging `internal/k8s/backend.go` and adding a stub inside the k8s package.
After codebase analysis, a cleaner approach is to gate at the `main` package level instead:

- Extract `resolveBackend()` from `main.go` into two build-tagged files.
- The `internal/k8s` package itself gets **no build tags** — it continues to compile and be tested
  unconditionally (good for CI coverage).
- Only the `main` package controls whether `internal/k8s` is *imported* into the binary.
- Because Go's linker only includes transitively-imported packages, the k8s deps are absent from the
  binary when the `kubernetes` tag is not set — no stub needed.

This avoids the stub entirely and keeps the k8s package clean.

## Implementation Steps

1. **`lazy-tcp-proxy/main.go`** — remove `resolveBackend()` function body and the
   `k8sbackend "github.com/mountain-pass/lazy-tcp-proxy/internal/k8s"` import.

2. **Create `lazy-tcp-proxy/backend_docker.go`** (`//go:build !kubernetes`) — provides
   `resolveBackend()` that only supports the Docker backend.

3. **Create `lazy-tcp-proxy/backend_k8s.go`** (`//go:build kubernetes`) — provides
   `resolveBackend()` that supports both Docker and Kubernetes backends; imports `internal/k8s`.

4. **`lazy-tcp-proxy/Dockerfile`** — add `ARG BUILD_TAGS=""` after the existing `ARG` declarations;
   append `-tags ${BUILD_TAGS}` to the `go build` command.

5. **`hooked.yaml`** — add `publish-k8s:` command (mirrors `publish:` with
   `--build-arg BUILD_TAGS=kubernetes` and `mountainpass/lazy-tcp-proxy-k8s` image name/tag).

6. **`.github/workflows/go-ci.yml`** — add a second `build` and `test` step that runs with
   `-tags kubernetes` so the k8s backend tests remain exercised in CI.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/main.go` | Modify | Remove `resolveBackend()` and `k8sbackend` import |
| `lazy-tcp-proxy/backend_docker.go` | Create | `//go:build !kubernetes` — docker-only `resolveBackend()` |
| `lazy-tcp-proxy/backend_k8s.go` | Create | `//go:build kubernetes` — full `resolveBackend()` with k8s |
| `lazy-tcp-proxy/Dockerfile` | Modify | Add `ARG BUILD_TAGS=""`, pass `-tags ${BUILD_TAGS}` to `go build` |
| `hooked.yaml` | Modify | Add `publish-k8s:` command |
| `.github/workflows/go-ci.yml` | Modify | Add k8s-tagged build and test steps |

## Key Code Snippets

### `backend_docker.go`
```go
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
```

### `backend_k8s.go`
```go
//go:build kubernetes

package main

import (
	"log"
	"os"
	"strings"

	"github.com/mountain-pass/lazy-tcp-proxy/internal/docker"
	k8sbackend "github.com/mountain-pass/lazy-tcp-proxy/internal/k8s"
)

func resolveBackend() (backendManager, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BACKEND"))) {
	case "kubernetes", "k8s":
		ns := os.Getenv("K8S_NAMESPACE")
		log.Printf("backend: kubernetes (namespace=%q)", ns)
		return k8sbackend.NewBackend(ns)
	default:
		log.Printf("backend: docker")
		return docker.NewManager()
	}
}
```

### `Dockerfile` diff
```dockerfile
 ARG TARGETARCH
 ARG TARGETVARIANT
+ARG BUILD_TAGS=""
 ...
-RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -a -trimpath -o lazy-tcp-proxy .
+RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -tags ${BUILD_TAGS} -a -trimpath -o lazy-tcp-proxy .
```

### `hooked.yaml` new command
```yaml
  publish-k8s:
    $cmd: |
      cd lazy-tcp-proxy
      VERSION=1.`date +%Y%m%d`.`git rev-parse --short=8 HEAD`
      docker buildx build \
        --platform linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6 \
        --build-arg BUILD_TAGS=kubernetes \
        --tag mountainpass/lazy-tcp-proxy-k8s:$VERSION \
        --tag mountainpass/lazy-tcp-proxy-k8s:latest \
        --push \
        .
      docker pull mountainpass/lazy-tcp-proxy-k8s
```

### `go-ci.yml` additions
Add two steps to the `test` job after the existing `test` step:
```yaml
      - name: build (kubernetes)
        working-directory: lazy-tcp-proxy
        run: go build -tags kubernetes ./...
      - name: test (kubernetes)
        working-directory: lazy-tcp-proxy
        run: go test -tags kubernetes -race -count=1 ./...
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Build without tag | `go build ./...` | Compiles; binary has no `k8s.io/*` symbols |
| Build with tag | `go build -tags kubernetes ./...` | Compiles; binary includes k8s backend |
| Test without tag | `go test ./...` | All non-k8s tests pass; k8s package tests still compile and pass (package has no build tag) |
| Test with tag | `go test -tags kubernetes ./...` | All tests pass including k8s backend tests |
| Runtime (no tag) | `BACKEND=docker` | Docker backend used; works normally |
| Runtime (k8s tag) | `BACKEND=kubernetes` | Kubernetes backend used; works normally |
| Runtime (no tag, k8s env) | `BACKEND=kubernetes`, no-tag binary | `resolveBackend()` falls through to docker (docker-only binary ignores `BACKEND=kubernetes`) |

## Risks & Open Questions

- **Behaviour divergence on wrong image**: A user who sets `BACKEND=kubernetes` on the lean Docker image
  will silently get the Docker backend instead of an error. This is acceptable — the image name
  `mountainpass/lazy-tcp-proxy-k8s` makes the intent clear. Could add a warning log in
  `backend_docker.go` if `BACKEND` is set to `kubernetes`, but that's out of scope for this requirement.
- **`go test ./...` in Dockerfile**: The default build already omits `-tags kubernetes`, so k8s backend
  tests run in CI (step 6) but not inside the Docker build step — this is consistent with the current
  behaviour and acceptable.
