# Cron-Based Scheduling (Docker & Kubernetes)

**Date Added**: 2026-04-07
**Priority**: Medium
**Status**: Completed

## Problem Statement

Users want to start and stop containers/deployments on a fixed schedule (e.g. business hours only) without relying on external cron jobs or orchestration tools. The lazy-tcp-proxy already manages container lifecycle for both Docker and Kubernetes backends; cron scheduling is a natural extension to both.

## Functional Requirements

1. Targets opt in by adding one or more scheduling labels/annotations:
   - `lazy-tcp-proxy.cron-start=<cron expression>` — start the target at the scheduled time.
   - `lazy-tcp-proxy.cron-stop=<cron expression>` — stop the target at the scheduled time.
2. Cron expressions follow the standard 5-field format: `minute hour day-of-month month day-of-week` (e.g. `30 8 * * 1-5`).
3. If only `cron-start` is set, the target is started on schedule and never stopped by the scheduler (idle-timeout still applies independently, unless exempted — see below).
4. If only `cron-stop` is set, the target is stopped on schedule and never started by the scheduler.
5. Both labels may be set independently; they do not need to be paired.
6. **Idempotency**: if the target is already in the desired state (running / stopped) when the cron fires, log the fact and take no action.
7. Cron schedules fire in the proxy's local timezone (UTC by default, configurable via `TZ` env var or OS timezone).
8. Invalid cron expressions are rejected at registration time with a logged warning; the target is still managed normally.
9. Targets that have either `cron-start` or `cron-stop` set are **exempt from the idle-timeout inactivity checker**. They are responsible for their own lifecycle via the cron schedule; the inactivity checker skips them silently.
10. **`cron-restart` is out of scope for this iteration.** It may be added later as `cron-stop` then `cron-start` in immediate succession.

## User Experience Requirements

- Labels/annotations are configured on the target (Docker container or Kubernetes Deployment), consistent with every other lazy-tcp-proxy label.

**Docker example:**
```yaml
labels:
  lazy-tcp-proxy.enabled: "true"
  lazy-tcp-proxy.ports: "5432:5432"
  lazy-tcp-proxy.cron-start: "30 8 * * 1-5"   # Start Mon–Fri at 08:30
  lazy-tcp-proxy.cron-stop:  "30 17 * * 1-5"  # Stop  Mon–Fri at 17:30
```

**Kubernetes example:**
```yaml
annotations:
  lazy-tcp-proxy.enabled: "true"
  lazy-tcp-proxy.ports: "5432:5432"
  lazy-tcp-proxy.cron-start: "30 8 * * 1-5"   # Start Mon–Fri at 08:30
  lazy-tcp-proxy.cron-stop:  "30 17 * * 1-5"  # Stop  Mon–Fri at 17:30
```

- Log messages use the same formatting conventions as the rest of the proxy (yellow container/deployment names, structured prefixes).

## Technical Requirements

- Use a pure-Go cron library (no CGO). Preferred: `robfig/cron` v3, which is a well-maintained zero-dependency library already widely used in the Go ecosystem.
- The scheduler runs in its own goroutine alongside the existing inactivity checker.
- Cron jobs are registered/unregistered dynamically as targets are added and removed (both Docker events and Kubernetes watch events).
- The scheduler must be shut down cleanly when the proxy's root context is cancelled.
- Works with both `BACKEND=docker` (default) and `BACKEND=kubernetes` — the scheduler calls through the existing `Backend` interface (`EnsureRunning` / `StopContainer`), so no backend-specific logic is needed in the scheduler itself.
- Targets with either `cron-start` or `cron-stop` set are exempt from the idle-timeout inactivity checker.

## Acceptance Criteria

- [ ] A target with `cron-start` is started at the scheduled time (within ±1 minute) on both Docker and Kubernetes backends.
- [ ] A target with `cron-stop` is stopped at the scheduled time (within ±1 minute) on both Docker and Kubernetes backends.
- [ ] A target with `cron-start` or `cron-stop` set is never shut down by the idle-timeout inactivity checker.
- [ ] If the target is already running when `cron-start` fires, only a log message is emitted (no API call).
- [ ] If the target is already stopped when `cron-stop` fires, only a log message is emitted (no API call).
- [ ] An invalid cron expression is logged as a warning and the target is still registered normally.
- [ ] Removing a target (Docker destroy / Kubernetes delete event) cancels its scheduled jobs.
- [ ] The scheduler shuts down cleanly on SIGTERM/SIGINT.
- [ ] Existing unit and integration tests continue to pass.
- [ ] `README.md` is updated to document the `cron-start` and `cron-stop` labels with usage examples for both Docker and Kubernetes.

## Dependencies

- Depends on: REQ-001 (core proxy), REQ-008 (keep stopped containers registered), REQ-037 (per-container label parsing pattern), REQ-038 (Kubernetes backend and `Backend` interface).
- Affects: `internal/docker/manager.go` (label parsing), `internal/backend/kubernetes.go` (annotation parsing), `internal/proxy/server.go` (scheduler integration, inactivity checker exemption), `main.go` (scheduler startup), `README.md` (documentation).
- New dependency: `github.com/robfig/cron/v3`.

## Implementation Notes

- `TargetInfo` gains two new optional fields: `CronStart string` and `CronStop string` (empty = not set). Both backends populate these from the same label/annotation keys.
- A new `Scheduler` type (e.g. `internal/scheduler/scheduler.go`) wraps `robfig/cron` and exposes `Register(info TargetInfo)` / `Unregister(targetID string)` / `Start()` / `Stop()`.
- The scheduler holds a reference to the `Backend` interface and calls `EnsureRunning` / `StopContainer` — no Docker- or Kubernetes-specific code in the scheduler.
- The inactivity checker in `proxy/server.go` checks `TargetInfo.CronStart != "" || TargetInfo.CronStop != ""` and skips the target if true.
- `cron-restart` is explicitly excluded; document it as a future enhancement.
