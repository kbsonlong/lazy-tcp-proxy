# Platform Integration Documentation — Implementation Plan

**Requirement**: [2026-04-08-platform-integration-docs.md](2026-04-08-platform-integration-docs.md)
**Date**: 2026-04-08
**Status**: Implemented

## Implementation Steps

1. Create `example/podman/README.md` covering: enabling the Podman socket service, the `DOCKER_SOCK` env var override, a `docker run` example, and a minimal Compose snippet with a managed container.
2. Create `example/unraid/README.md` covering: prerequisites, adding the container via the Unraid "Add Container" UI (field-by-field walkthrough), label configuration via Extra Parameters, and a note on Community Applications.
3. Create `example/truenas/README.md` covering: TrueNAS SCALE Electric Eel (v24.10+) Docker Compose app deployment via the Custom App UI, label configuration, and accessing the status endpoint.
4. Update `requirements/2026-04-08-platform-integration-docs.md` status → Completed.
5. Update `requirements/_index.md` REQ-053 status → Completed.
6. Commit and push all changes.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `example/podman/README.md` | Create | Podman integration guide |
| `example/unraid/README.md` | Create | Unraid integration guide |
| `example/truenas/README.md` | Create | TrueNAS SCALE integration guide |
| `requirements/2026-04-08-platform-integration-docs.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | REQ-053 status → Completed |

## Key Content per Guide

### Podman
- Enable rootless socket: `systemctl --user enable --now podman.socket`
- Socket path: `/run/user/$(id -u)/podman/podman.sock`
- Set `DOCKER_SOCK` env var to point at that path
- Run example and Compose snippet

### Unraid
- Docker is the native runtime — no socket path changes needed
- "Add Container" UI walkthrough: Repository, Network (bridge), Port mappings, Volume (`/var/run/docker.sock`), Extra Parameters for labels
- Note: Community Applications template does not exist yet; manual method is the current path

### TrueNAS SCALE (Electric Eel / 24.10+)
- Apps → Custom App → paste Compose YAML
- Standard socket path (`/var/run/docker.sock`)
- Note: TrueNAS runs Docker as root; root socket is available to containers
- Status endpoint accessible on host IP

## Risks & Open Questions

- Unraid label support: Unraid passes `--label` flags via the "Extra Parameters" field — this is the correct approach and is well-documented in the Unraid community.
- TrueNAS SCALE versions prior to 24.10 used k3s, not Docker Compose — the guide should clearly state the Electric Eel minimum version requirement.
- Podman rootful vs rootless: guide covers rootless (most common) with a note on the root socket path.
