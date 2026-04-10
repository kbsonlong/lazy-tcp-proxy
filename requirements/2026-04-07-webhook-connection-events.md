# Webhook Connection Events (connection_started / connection_ended)

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: Completed

## Problem Statement

The existing webhook support only fires `container_started` and `container_stopped` events, which track container lifecycle. Operators have no way to know when individual TCP connections begin and end. Adding `connection_started` and `connection_ended` events—with a stable per-connection ID—lets external systems track connection durations, log individual sessions, and correlate start/end pairs.

## Functional Requirements

- When the proxy accepts an inbound TCP connection (and it is not blocked), fire a `connection_started` webhook event.
- When that same connection closes, fire a `connection_ended` webhook event.
- Each connection is assigned a universally unique `connection_id` (UUID v4) at the moment the connection is accepted.
- Both `connection_started` and `connection_ended` payloads include the same `connection_id` so the caller can correlate them.
- The `connection_id` is not persisted; it exists only for the lifetime of the proxy process.
- Events are fired in a goroutine (fire-and-forget), consistent with existing webhook behaviour.
- Only containers with a `lazy-tcp-proxy.webhook-url` label receive these events; containers without the label are unaffected.

## User Experience Requirements

- No new labels or configuration required; connection events are fired to the same URL as container lifecycle events.
- Failed deliveries are logged at warning level; successes at info level (same as existing behaviour).

## Technical Requirements

- Generate a UUID v4 using the standard library only (`crypto/rand` + `encoding/hex`); no new dependencies.
- The `webhookPayload` struct gains a `connection_id` field (omitempty so existing events are unaffected).
- `connection_started` fires after `ipBlocked` check passes (blocked connections do not generate events).
- `connection_ended` fires via `defer` at the end of `handleConn`, after the connection is fully closed.
- The connection ID is a local string variable in `handleConn`; no changes to `targetState` or `TargetInfo`.

## Acceptance Criteria

- [ ] A container with `lazy-tcp-proxy.webhook-url=<url>` receives a `connection_started` POST when a TCP connection is accepted.
- [ ] The same container receives a `connection_ended` POST when that TCP connection closes.
- [ ] Both payloads include the same `connection_id` UUID string.
- [ ] Blocked connections (IP in block-list or not in allow-list) do NOT fire connection events.
- [ ] Containers without the webhook URL label are unaffected.
- [ ] The `connection_id` field is absent from `container_started` / `container_stopped` payloads (omitempty).
- [ ] Connection event dispatch does not block the proxy path.

## Dependencies

- Extends the webhook mechanism introduced in REQ-026.
- Changes are confined to `lazy-tcp-proxy/internal/proxy/server.go`.

## Implementation Notes

- UUID generation: read 16 bytes from `crypto/rand`, set version (byte 6) and variant (byte 8) bits, then format as `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`.
- `connection_ended` should be deferred immediately after the `connection_started` event is fired so it always runs even if dial or copy fails.
