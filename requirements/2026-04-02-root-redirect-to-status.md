# Root Redirect to /status

**Date Added**: 2026-04-02
**Priority**: Low
**Status**: Completed

## Problem Statement

Visiting the HTTP status server root path `/` returns a 404 (no handler registered). A simple redirect to `/status` would improve usability — users who navigate to the host:port in a browser land on the useful status page automatically.

## Functional Requirements

- `GET /` (and any method) redirects to `/status` with HTTP 301 (Moved Permanently).

## User Experience Requirements

- Browsers and `curl -L` automatically follow the redirect to `/status`.
- No change to existing `/status` or `/health` behaviour.

## Technical Requirements

- Add a single `mux.HandleFunc("/", ...)` handler in `runStatusServer` (`main.go:65`) that calls `http.Redirect(w, r, "/status", http.StatusMovedPermanently)`.
- No new dependencies.

## Acceptance Criteria

- [x] `GET /` returns HTTP 301 with `Location: /status`.
- [x] `GET /status` and `GET /health` are unaffected.

## Dependencies

- REQ-025 (HTTP Status Endpoint) — modifies the same `runStatusServer` function.

## Implementation Notes

- In Go's `http.ServeMux`, a pattern of `"/"` matches any path not matched by a more specific pattern, so it will also catch unknown paths (e.g. `/foo`) — acceptable behaviour for this internal endpoint.
