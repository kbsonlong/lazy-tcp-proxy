# Example README Files (Docker and Kubernetes)

**Date Added**: 2026-04-05
**Priority**: Low
**Status**: Completed

## Problem Statement

The `example/docker/` and `example/kubernetes/` directories have no user-facing documentation. First-time users have no guidance on how to start the examples, trigger on-demand scaling, observe the proxy behaviour, or shut everything down cleanly.

## Functional Requirements

1. `example/docker/README.md` covers: prerequisites, start commands, one command per proxied service to trigger cold-start, what to look for in logs/status, shutdown.
2. `example/kubernetes/README.md` covers: prerequisites, apply order, port-forward setup, trigger command to scale 0→1, what to observe, shutdown/cleanup.

## Acceptance Criteria

- [ ] `example/docker/README.md` exists with working command-line examples.
- [ ] `example/kubernetes/README.md` exists with working command-line examples.
- [ ] Both READMEs reference the status endpoint at `:8080/status`.

## Dependencies

REQ-039 (docker/ subdir), REQ-038 (kubernetes backend).
