# Webhook Connection Events — Add Source IP Address

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: Completed

## Problem Statement

The `connection_started` and `connection_ended` webhook events (REQ-041) do not include the originating IP address. External systems that consume these events cannot tell which client initiated the connection without correlating against separate access logs.

## Functional Requirements

- `connection_started` and `connection_ended` webhook payloads include a `remote_addr` field containing the client's IP address (without port).
- `container_started` and `container_stopped` payloads are unaffected (field absent via `omitempty`).

## Technical Requirements

- Extract the host from `conn.RemoteAddr().String()` using `net.SplitHostPort` at the point the connection ID is generated.
- Pass the IP string into `fireWebhook` alongside `connID`.
- `webhookPayload.RemoteAddr` is tagged `json:"remote_addr,omitempty"`.
- No new dependencies.

## Acceptance Criteria

- [ ] `connection_started` payload contains `"remote_addr": "<client-ip>"`.
- [ ] `connection_ended` payload contains the same `remote_addr` value.
- [ ] `container_started` / `container_stopped` payloads have no `remote_addr` field.
- [ ] Build and tests pass.

## Dependencies

Extends REQ-041 (`2026-04-07-webhook-connection-events.md`). Changes confined to `lazy-tcp-proxy/internal/proxy/server.go` and `README.md`.
