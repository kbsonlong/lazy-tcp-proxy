# Separate Kubernetes Build Artifact

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: Completed

## Problem Statement

Adding the Kubernetes backend (REQ-038) caused the Docker image to grow from ~5 MB to ~24 MB due to
the `k8s.io/client-go`, `k8s.io/api`, and `k8s.io/apimachinery` dependencies and their ~25 transitive
packages (protobuf, OpenAPI, oauth2, etc.). The vast majority of users only need the Docker backend,
yet they receive an image nearly 5× larger than necessary.

## Functional Requirements

1. The default `mountainpass/lazy-tcp-proxy` image must NOT include the Kubernetes backend code or its
   dependencies — restoring the ~5 MB image size.
2. A new `mountainpass/lazy-tcp-proxy-k8s` image must be published that includes the Kubernetes backend
   and behaves identically to the current image for users who need it.
3. Both images must continue to support multi-platform builds
   (`linux/amd64`, `linux/arm64`, `linux/arm/v7`, `linux/arm/v6`).
4. The `hooked.yaml` `publish` command must remain unchanged and produce the lean Docker-only image.
5. A new `publish-k8s` command in `hooked.yaml` must build and push the Kubernetes image.

## User Experience Requirements

- Docker users pull `mountainpass/lazy-tcp-proxy` (unchanged workflow, smaller image).
- Kubernetes users pull `mountainpass/lazy-tcp-proxy-k8s` (explicit, clearly named image).
- No runtime behaviour changes — backend selection still via `BACKEND=kubernetes` env var.

## Technical Requirements

- Use a Go build tag (`kubernetes`) to gate inclusion of the k8s backend at compile time.
- A stub file provides a `NewBackend()` that returns an "unsupported backend" error when built without
  the tag, ensuring the binary compiles and fails gracefully at runtime.
- The existing `Dockerfile` gains a single `ARG BUILD_TAGS=""` and passes `-tags ${BUILD_TAGS}` to
  `go build`. No second Dockerfile is needed.
- The `publish-k8s` hooked command passes `--build-arg BUILD_TAGS=kubernetes` and uses the
  `mountainpass/lazy-tcp-proxy-k8s` image name/tag.

## Acceptance Criteria

- [ ] `internal/k8s/backend.go` has `//go:build kubernetes` at the top
- [ ] `internal/k8s/backend_stub.go` exists with `//go:build !kubernetes` and returns an error from `NewBackend()`
- [ ] `go build` (no tags) produces a binary with no `k8s.io/*` imports (verified via `go tool nm` or image size)
- [ ] `go build -tags kubernetes` produces a binary that includes the Kubernetes backend
- [ ] `Dockerfile` accepts `ARG BUILD_TAGS` and passes it to `go build`
- [ ] `hooked.yaml` has a `publish-k8s:` command targeting `mountainpass/lazy-tcp-proxy-k8s`
- [ ] Default image size is restored to approximately ~5 MB
- [ ] Kubernetes image size is approximately ~24 MB
- [ ] Unit tests pass for both build variants

## Dependencies

- Depends on REQ-038 (Kubernetes Backend) — modifies its build integration
- Modifies `lazy-tcp-proxy/Dockerfile`, `lazy-tcp-proxy/internal/k8s/`, `hooked.yaml`

## Implementation Notes

- `go test ./...` in the Dockerfile runs without tags, so k8s backend tests will be excluded from the
  default build. The CI workflow may need a separate test run with `-tags kubernetes` to keep k8s tests
  exercised.
- The stub only needs to export `NewBackend(ns string) (backendManager, error)` — the same signature
  as the real implementation.
