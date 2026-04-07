# Webhook Connection Events — Implementation Plan

**Requirement**: [2026-04-07-webhook-connection-events.md](2026-04-07-webhook-connection-events.md)
**Date**: 2026-04-07
**Status**: Implemented

## Implementation Steps

1. **Update `webhookPayload`** in `server.go`: add `ConnectionID string \`json:"connection_id,omitempty"\`` field. The `omitempty` tag means existing `container_started`/`container_stopped` payloads are unchanged.

2. **Extend `fireWebhook` signature** to accept a `connID string` parameter, which is written into the payload. Pass `""` at the two existing call sites (container events); they produce no `connection_id` field due to `omitempty`.

3. **Add `newConnectionID()` helper** in `server.go` using `crypto/rand` to read 16 bytes, set UUID v4 version/variant bits, and format as `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`. Import `crypto/rand` and `encoding/hex`.

4. **Modify `handleConn`** in `server.go`:
   - After the `ipBlocked` guard (blocked conns produce no events), generate `connID := newConnectionID()`.
   - Fire `connection_started` immediately: `go s.fireWebhook(ts.info.WebhookURL, "connection_started", ts.info.ContainerID, ts.info.ContainerName, connID)`.
   - Defer `connection_ended` right after: `defer func() { go s.fireWebhook(..., "connection_ended", ..., connID) }()`.
   - Both are guarded by `if ts.info.WebhookURL != ""`.

5. **Update `README.md`** Webhooks section:
   - Add `connection_started` and `connection_ended` rows to the events table.
   - Add `connection_id` to the example payload (with a note that it is absent for container lifecycle events).

6. **Update requirement and index status** to Completed.

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add UUID helper, extend `webhookPayload` + `fireWebhook`, update `handleConn` |
| `README.md` | Modify | Document two new webhook events and `connection_id` field |
| `requirements/2026-04-07-webhook-connection-events.md` | Modify | Status → Completed |
| `requirements/_index.md` | Modify | Status → Completed |

## API Contracts

New webhook events fire to the same URL as existing container events.

`connection_started` payload:
```json
{
  "event": "connection_started",
  "connection_id": "550e8400-e29b-41d4-a716-446655440000",
  "container_id": "a1b2c3d4e5f6",
  "container_name": "my-service",
  "timestamp": "2026-04-07T10:00:00Z"
}
```

`connection_ended` payload:
```json
{
  "event": "connection_ended",
  "connection_id": "550e8400-e29b-41d4-a716-446655440000",
  "container_id": "a1b2c3d4e5f6",
  "container_name": "my-service",
  "timestamp": "2026-04-07T10:00:05Z"
}
```

Existing `container_started` / `container_stopped` payloads are unchanged (no `connection_id` field).

## Key Code Snippets

```go
func newConnectionID() string {
    var b [16]byte
    if _, err := rand.Read(b[:]); err != nil {
        return "00000000-0000-0000-0000-000000000000"
    }
    b[6] = (b[6] & 0x0f) | 0x40 // version 4
    b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
    return fmt.Sprintf("%s-%s-%s-%s-%s",
        hex.EncodeToString(b[0:4]),
        hex.EncodeToString(b[4:6]),
        hex.EncodeToString(b[6:8]),
        hex.EncodeToString(b[8:10]),
        hex.EncodeToString(b[10:]))
}
```

## Risks & Open Questions

- None. The change is entirely additive; no existing behaviour is modified.
