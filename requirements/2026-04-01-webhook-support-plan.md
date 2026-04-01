# Webhook Support — Implementation Plan

**Requirement**: [2026-04-01-webhook-support.md](2026-04-01-webhook-support.md)
**Date**: 2026-04-01
**Status**: Draft

## Implementation Steps

1. **Add `WebhookURL` field to `TargetInfo`** in `internal/docker/manager.go` — plain `string`; empty means no webhook.

2. **Parse `lazy-tcp-proxy.webhook-url` label in `containerToTargetInfo`** — read the label value, trim whitespace. If non-empty, attempt `url.ParseRequestURI` to validate; log a warning and leave the field empty if invalid.

3. **Add `webhookPayload` struct and `fireWebhook()` helper in `internal/proxy/server.go`** — `webhookPayload` holds `Event`, `ContainerID` (12-char), `ContainerName`, and `Timestamp` (RFC3339 UTC). `fireWebhook()` marshals the payload, POSTs it with the shared `webhookClient`, logs success at info level and failure at warning level.

4. **Add `webhookClient` to `ProxyServer`** — initialised in `NewServer()` as `&http.Client{Timeout: 5 * time.Second}`.

5. **Fire `container_started` webhook in `handleConn`** — immediately after `EnsureRunning` returns `nil` (container successfully started), dispatch `go fireWebhook(...)` with event `container_started`. Only fire if `ts.info.WebhookURL != ""`.

6. **Fire `container_stopped` webhook in `checkInactivity`** — immediately after `StopContainer` returns `nil`, dispatch `go fireWebhook(...)` with event `container_stopped`. Only fire if `ts.info.WebhookURL != ""` (read from any `ts` in the container's group, they share the same `TargetInfo`).

## File Change Summary

| File | Action | Description |
|------|--------|-------------|
| `lazy-tcp-proxy/internal/docker/manager.go` | Modify | Add `WebhookURL string` to `TargetInfo`; parse `lazy-tcp-proxy.webhook-url` label in `containerToTargetInfo` |
| `lazy-tcp-proxy/internal/proxy/server.go` | Modify | Add `webhookPayload` struct, `fireWebhook()` helper, `webhookClient` on `ProxyServer`; call from `handleConn` and `checkInactivity` |

## API Contracts

### Webhook POST request

**Method**: `POST`
**Content-Type**: `application/json`
**Timeout**: 5 seconds

**Body**:
```json
{
  "event": "container_started",
  "container_id": "a1b2c3d4e5f6",
  "container_name": "my-service",
  "timestamp": "2026-04-01T12:34:56Z"
}
```

**Events**:
| Value | When fired |
|-------|-----------|
| `container_started` | `EnsureRunning` returned `nil` — container is now up |
| `container_stopped` | `StopContainer` returned `nil` — container has been stopped |

**Response**: Any `2xx` is treated as success. Non-2xx and network errors are logged as warnings. The proxy does not retry.

## Data Models

```go
// In internal/docker/manager.go — TargetInfo gains:
WebhookURL string // empty = no webhook

// In internal/proxy/server.go:
type webhookPayload struct {
    Event         string `json:"event"`
    ContainerID   string `json:"container_id"`
    ContainerName string `json:"container_name"`
    Timestamp     string `json:"timestamp"`
}
```

## Key Code Snippets

```go
// fireWebhook sends a fire-and-forget POST to the container's webhook URL.
// Must be called in a goroutine to avoid blocking the proxy path.
func (s *ProxyServer) fireWebhook(webhookURL, event, containerID, containerName string) {
    payload := webhookPayload{
        Event:         event,
        ContainerID:   containerID[:12],
        ContainerName: containerName,
        Timestamp:     time.Now().UTC().Format(time.RFC3339),
    }
    body, _ := json.Marshal(payload)
    resp, err := s.webhookClient.Post(webhookURL, "application/json", bytes.NewReader(body))
    if err != nil {
        log.Printf("proxy: webhook: POST %s event=%s error: %v", webhookURL, event, err)
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        log.Printf("proxy: webhook: POST %s event=%s non-2xx response: %d", webhookURL, event, resp.StatusCode)
        return
    }
    log.Printf("proxy: webhook: delivered event=%s to %s (%d)", event, webhookURL, resp.StatusCode)
}
```

```go
// In handleConn, after EnsureRunning succeeds:
if ts.info.WebhookURL != "" {
    go s.fireWebhook(ts.info.WebhookURL, "container_started", ts.info.ContainerID, ts.info.ContainerName)
}

// In checkInactivity, after StopContainer succeeds:
if e.webhookURL != "" {
    go s.fireWebhook(e.webhookURL, "container_stopped", e.containerID, e.name)
}
```

## Unit Tests

| Test | Input | Expected Output |
|------|-------|-----------------|
| Valid webhook on start | container with valid URL, proxy starts it | POST fired with `event=container_started`, correct fields |
| Valid webhook on stop | container idle, proxy stops it | POST fired with `event=container_stopped`, correct fields |
| No webhook label | container without `lazy-tcp-proxy.webhook-url` | no POST made |
| Webhook URL unreachable | URL that refuses connection | warning logged, proxy continues normally |
| Webhook returns 500 | server returns non-2xx | warning logged, proxy continues normally |
| Webhook times out | server hangs > 5s | warning logged after 5s, proxy continues normally |
| Invalid URL in label | `not-a-url` | warning at parse time, field empty, no POST made |

## Risks & Open Questions

- **Only fires on proxy-initiated start/stop**: if the container is started or stopped by something external (another operator, Docker Compose, etc.), no webhook is fired. This is consistent with the requirement scope but worth documenting.
- **`container_started` fires on every inbound connection that triggers a start**, not once per container lifecycle. If the container is already running, `EnsureRunning` returns immediately without firing. This is the correct behaviour (webhook only fires when the proxy actually starts it).
- **`checkInactivity` aggregates states by container ID** — the `webhookURL` can be read from the first `ts` in the group since all port mappings for the same container share the same `TargetInfo`. The `entry` struct in `checkInactivity` should be extended to carry `webhookURL`.
