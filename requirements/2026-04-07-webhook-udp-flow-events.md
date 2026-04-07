# Webhook UDP Flow Events & Rename TCP Event Names

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: Completed

## Problem Statement

TCP connection webhook events exist (`connection_started`/`connection_ended`) but UDP flows produce no webhook events. Additionally the existing TCP event names are ambiguous. This requirement renames the TCP events, splits `remote_addr` into separate address and port fields, and adds equivalent events for UDP flows.

## Functional Requirements

- Rename existing TCP webhook events:
  - `connection_started` → `tcp_conn_start`
  - `connection_ended` → `tcp_conn_end`
- Add UDP flow webhook events:
  - `udp_flow_start` — fired when a new UDP flow is successfully established (container running, upstream dialled)
  - `udp_flow_end` — fired when a UDP flow is reaped by the idle sweeper
- All four events carry:
  - `connection_id` — UUID v4 (same value in start and end pairs)
  - `remote_addr` — client IP address (no port)
  - `remote_port` — client port as integer
- Container lifecycle events (`container_started`, `container_stopped`) are unaffected (no `remote_addr`, `remote_port`, or `connection_id`).
- Blocked UDP datagrams (IP in block/allow list) do not fire events, consistent with TCP behaviour.

## Technical Requirements

- `webhookPayload.RemoteAddr` stays as `string` (IP only); add `RemotePort int \`json:"remote_port,omitempty"\``.
- `fireWebhook` signature gains `remotePort int` parameter.
- `udpFlow` struct gains `connectionID string` field, set at flow creation in `startUDPFlow`.
- `udp_flow_start` fires inside `startUDPFlow` after the flow is registered (container running, upstream conn established).
- `udp_flow_end` fires inside `udpFlowSweeper` when a flow is deleted due to idle timeout.
- `newConnectionID()` is already in `server.go` and accessible from `udp.go` (same package).

## Acceptance Criteria

- [ ] A container with `lazy-tcp-proxy.webhook-url` receives `tcp_conn_start` on inbound TCP connection.
- [ ] Same container receives `tcp_conn_end` when that connection closes.
- [ ] A container receives `udp_flow_start` when a new UDP flow is established.
- [ ] Same container receives `udp_flow_end` when that flow expires via idle sweeper.
- [ ] All four events carry matching `connection_id`, `remote_addr` (IP only), and `remote_port` (int).
- [ ] `container_started` / `container_stopped` payloads unchanged.
- [ ] Build and tests pass.

## Dependencies

Extends REQ-041 and REQ-044. Changes to `server.go` and `udp.go`.
