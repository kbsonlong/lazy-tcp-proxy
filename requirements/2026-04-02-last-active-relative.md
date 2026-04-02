# Last Active Default & Relative Time Field

**Date Added**: 2026-04-02
**Priority**: Medium
**Status**: Planned

## Problem Statement

The HTTP status endpoint returns `null` for `last_active` on containers that have never received a connection. This is misleading — operators see `null` and cannot tell how long the service has been idle since startup. Additionally, raw RFC3339 timestamps are inconvenient for quick human inspection; a human-readable relative time field (e.g. "8 hours ago") would improve operator experience.

## Functional Requirements

1. If a container's `last_active` timestamp is `null` (i.e. no connection has ever been proxied through it), the value returned by `GET /status` for that container MUST fall back to the service start time (the moment `main()` began).
2. A new field `last_active_relative` is added to each entry in the `GET /status` JSON response.
3. `last_active_relative` expresses the duration between `last_active` and the current wall-clock time, using only the single largest significant unit:
   - `"N years ago"`
   - `"N months ago"`
   - `"N days ago"`
   - `"N hours ago"`
   - `"N minutes ago"`
   - `"N seconds ago"`
4. Units are plural even for N=1 (e.g. "1 minutes ago") — keep it simple and consistent.

## User Experience Requirements

- `last_active` is never `null` in the response; it always shows at least the service start time.
- `last_active_relative` is a plain string placed immediately after `last_active` in the JSON output.
- Example response entry:
  ```json
  {
    "container_id": "abc123def456",
    "container_name": "my-service",
    "listen_port": 3000,
    "target_port": 3000,
    "running": false,
    "active_conns": 0,
    "last_active": "2026-04-02T08:00:00Z",
    "last_active_relative": "8 hours ago"
  }
  ```

## Technical Requirements

- The service start time is captured once at the top of `main()` (e.g. `startTime := time.Now()`).
- `startTime` is passed into `NewServer(...)` (or a dedicated setter) so `Snapshot()` can use it as the fallback.
- The relative-time calculation lives in a small pure function, e.g. `relativeTime(t time.Time, now time.Time) string`, so it is independently unit-testable.
- Thresholds (largest unit wins):
  - ≥ 1 year  → `"N years ago"`
  - ≥ 1 month (30 days) → `"N months ago"`
  - ≥ 1 day   → `"N days ago"`
  - ≥ 1 hour  → `"N hours ago"`
  - ≥ 1 minute → `"N minutes ago"`
  - otherwise → `"N seconds ago"`
- No new external dependencies — pure stdlib.
- `TargetSnapshot` gains a `LastActiveRelative string` field (`json:"last_active_relative"`).

## Acceptance Criteria

- [ ] `last_active` is never `null` in the `/status` response; a container with no traffic shows the service start time.
- [ ] `last_active_relative` appears in every entry in the `/status` response.
- [ ] Relative time reflects the correct largest unit (years/months/days/hours/minutes/seconds).
- [ ] Unit tests cover the `relativeTime` helper for each threshold boundary.
- [ ] Existing acceptance criteria for REQ-025 remain satisfied.

## Dependencies

- Extends REQ-025 (HTTP Status Endpoint).

## Implementation Notes

- `startTime` can be stored as a field on `ProxyServer` and set in `NewServer`.
- In `Snapshot()`, when `ts.lastActive.IsZero()`, use `s.startTime` as the fallback value.
- `LastActiveRelative` is computed in `Snapshot()` using `relativeTime(effectiveLastActive, time.Now())`.
