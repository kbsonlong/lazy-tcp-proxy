# REQ-041: UDP-Only Config Validation Fix

**Date Added**: 2026-04-07
**Priority**: High
**Status**: Planned

## Problem Statement

Services that only expose UDP ports (no TCP ports) are rejected by the proxy with the error:

```
docker: event: container udp-echo started but not proxied: missing label lazy-tcp-proxy.ports
```

This happens because config validation unconditionally requires the `lazy-tcp-proxy.ports` label even when `lazy-tcp-proxy.udp-ports` is provided and sufficient on its own.

## Functional Requirements

- If a container has `lazy-tcp-proxy.udp-ports` but no `lazy-tcp-proxy.ports`, it must be accepted and proxied (UDP-only service).
- If a container has `lazy-tcp-proxy.ports` but no `lazy-tcp-proxy.udp-ports`, it must be accepted and proxied (TCP-only service — existing behaviour, unchanged).
- If a container has both labels, both are used (existing behaviour, unchanged).
- If a container has neither label, it must be rejected with a clear message.

## User Experience Requirements

Rejection log messages must clearly explain which label(s) are missing, e.g.:

```
docker: event: container udp-echo started but not proxied: missing label lazy-tcp-proxy.ports or lazy-tcp-proxy.udp-ports
```

## Technical Requirements

Two code locations in `lazy-tcp-proxy/internal/docker/manager.go` must be updated:

1. **`containerToTargetInfo`** (~line 246): Currently requires `lazy-tcp-proxy.ports`. Change to allow the label to be absent when `lazy-tcp-proxy.udp-ports` is present.
2. **Event handler** (~line 483): Early-rejection guard currently checks only for `lazy-tcp-proxy.ports`. Change to also accept containers that have `lazy-tcp-proxy.udp-ports`.

## Acceptance Criteria

- [ ] A container with only `lazy-tcp-proxy.udp-ports` is registered and proxied successfully.
- [ ] A container with only `lazy-tcp-proxy.ports` continues to work as before.
- [ ] A container with both labels continues to work as before.
- [ ] A container with neither label is rejected with an updated error message mentioning both labels.
- [ ] All existing tests pass.

## Dependencies

- REQ-027 (UDP Traffic Support) — adds UDP proxying; this fix enables UDP-only containers.

## Implementation Notes

Minimal, targeted change: two conditional blocks in one file.
