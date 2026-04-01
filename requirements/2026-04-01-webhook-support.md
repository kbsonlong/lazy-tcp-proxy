# Webhook Support

**Date Added**: 2026-04-01
**Priority**: Medium
**Status**: In Progress

## Problem Statement

When the proxy starts or stops a container, there is no way for external systems to be notified. Operators who want to trigger alerts, update dashboards, or chain automations currently must poll the logs or the status endpoint. Webhook support allows containers to declare a URL that the proxy will POST to on lifecycle events.

## Functional Requirements

- A container opts in by adding the label `lazy-tcp-proxy.webhook-url=<url>`.
- The proxy fires an HTTP POST to that URL on the following events:
  - **container_started** — proxy successfully started the container (on first inbound connection)
  - **container_stopped** — proxy stopped the container due to idle timeout
- The request body is JSON with the following fields:
  - `event` — one of `container_started`, `container_stopped`
  - `container_id` — short 12-char container ID
  - `container_name` — container name
  - `timestamp` — RFC3339 UTC timestamp
- Webhook calls are fire-and-forget: failures are logged but do not affect proxy behaviour.
- A timeout of 5 seconds is applied to each webhook HTTP call.
- If the label is absent or empty, no webhook is fired for that container.

## User Experience Requirements

- No global configuration needed — webhook URL is per-container via Docker label.
- Multiple containers may each declare different webhook URLs.
- Failed webhook deliveries are logged at warning level including the URL, event, and error.
- Successful deliveries are logged at info level.

## Technical Requirements

- The webhook URL is read from the `lazy-tcp-proxy.webhook-url` label during `containerToTargetInfo` and stored on `TargetInfo`.
- Webhook calls are dispatched in a goroutine so they never block the proxy's connection-handling path.
- Uses stdlib `net/http` only — no new dependencies.
- The HTTP client used for webhooks should have a 5-second timeout and should NOT reuse the default `http.DefaultClient`.

## Acceptance Criteria

- [ ] A container with `lazy-tcp-proxy.webhook-url=<url>` receives a POST on `container_started` when the proxy starts it.
- [ ] A container with `lazy-tcp-proxy.webhook-url=<url>` receives a POST on `container_stopped` when the proxy stops it due to idle timeout.
- [ ] The POST body is valid JSON containing `event`, `container_id`, `container_name`, and `timestamp`.
- [ ] A failed webhook POST (network error, non-2xx response) is logged as a warning and does not disrupt proxying.
- [ ] Containers without the `lazy-tcp-proxy.webhook-url` label are unaffected.
- [ ] Webhook dispatch does not block inbound connection handling.
- [ ] Webhook HTTP calls time out after 5 seconds.

## Dependencies

- Reads container labels — builds on the existing label-parsing pattern in `internal/docker/manager.go`.
- Webhook firing hooks into `EnsureRunning` (start event) and `StopContainer` (stop event) in `internal/docker/manager.go`, or alternatively at the call sites in `internal/proxy/server.go`.

## Implementation Notes

- Prefer firing webhooks from `proxy/server.go` call sites rather than inside the Docker manager, to keep the manager focused on Docker API interactions.
- A dedicated `webhookClient` (`&http.Client{Timeout: 5 * time.Second}`) should be created once (e.g. as a package-level var or on `ProxyServer`) and reused across calls.
- The webhook URL should be validated as a non-empty string at label-parse time; invalid URLs are logged and ignored rather than causing a startup failure.
