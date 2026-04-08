# Platform Integration Documentation (Podman, Unraid, TrueNAS SCALE)

**Date Added**: 2026-04-08
**Priority**: Medium
**Status**: Completed

## Problem Statement

lazy-tcp-proxy works with any Docker-compatible runtime, but the documentation only covers vanilla Docker and Kubernetes. Users running Podman, Unraid, or TrueNAS SCALE have no guidance on how to deploy the proxy in their environment, even though all three platforms support the standard Docker socket API with little or no extra configuration.

## Functional Requirements

1. Add a documentation page (or example directory) for each of the three platforms:
   - **Podman** — how to expose the Podman socket and point `DOCKER_SOCK` at it
   - **Unraid** — how to add lazy-tcp-proxy as a Docker container via the Unraid UI / Community Applications
   - **TrueNAS SCALE** — how to deploy as a Docker Compose stack (Electric Eel / 24.10+)
2. Each guide must explain any platform-specific steps that differ from the vanilla Docker setup.
3. Each guide must include a minimal working example (labels, ports, socket path).

## User Experience Requirements

- A user arriving from Unraid, Podman, or TrueNAS SCALE can follow the guide from scratch without needing to read the full Docker guide first.
- Guides live under `example/<platform>/README.md` to match the existing `example/docker/` and `example/kubernetes/` structure.

## Technical Requirements

- **Podman**: `DOCKER_SOCK` must point to the Podman socket (typically `/run/user/<UID>/podman/podman.sock` for rootless, `/run/podman/podman.sock` for root). The proxy image is otherwise unchanged.
- **Unraid**: Unraid runs the standard Docker daemon. No socket path change needed. Guide covers Community Applications template or manual "Add Container" steps, plus label configuration.
- **TrueNAS SCALE**: Since Electric Eel (v24.10) the platform uses Docker Compose for apps. Guide shows a Compose snippet. Socket path is standard (`/var/run/docker.sock`).

## Acceptance Criteria

- [ ] `example/podman/README.md` exists and covers: enabling the Podman socket, `DOCKER_SOCK` env var, and a minimal compose/run example
- [ ] `example/unraid/README.md` exists and covers: adding via Community Applications or "Add Container", required fields (image, network, ports, volumes, labels)
- [ ] `example/truenas/README.md` exists and covers: Electric Eel Docker Compose app deployment, label configuration, and status endpoint access
- [ ] Each README notes any known limitations or caveats for that platform

## Dependencies

- REQ-002 (DOCKER_SOCK env var) — Podman guide depends on this feature
- REQ-040 (Example README Files) — establishes the pattern these docs follow

## Implementation Notes

- No code changes required — this is documentation only.
- Keep guides concise and self-contained; link back to the main README for full label/env-var reference.
