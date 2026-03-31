# Fix Dependabot Security Alerts (docker/docker + otel/sdk)

**Date Added**: 2026-03-31
**Priority**: High
**Status**: Completed

## Problem Statement

Dependabot has flagged 7 vulnerabilities in `lazy-tcp-proxy/go.mod`:

| Alert | Severity | Package | CVE |
|-------|----------|---------|-----|
| Authz zero length regression | Critical | github.com/docker/docker v25.0.0 | CVE-2024-41110 |
| AuthZ plugin bypass (oversized bodies) | High | github.com/docker/docker v25.0.0 | CVE-2024-41110 |
| OpenTelemetry PATH hijacking (code exec) | High | go.opentelemetry.io/otel/sdk v1.21.0 | — |
| Classic builder cache poisoning | Moderate | github.com/docker/docker v25.0.0 | CVE-2024-24557 |
| Off-by-one in plugin privilege validation | Moderate | github.com/docker/docker v25.0.0 | CVE-2024-29018 |
| External DNS from 'internal' networks | Moderate | github.com/docker/docker v25.0.0 | CVE-2024-29018 |
| Firewalld reload removes bridge isolation | Low | github.com/docker/docker v25.0.0 | CVE-2024-36623 |

All Docker CVEs are fixed in v27.1.1+; upgrading to the latest stable (v28.5.2) resolves all of them.
The otel/sdk vulnerability is fixed in v1.22.0+; Dependabot PR #5 targets v1.40.0.

## Functional Requirements

1. `github.com/docker/docker` must be upgraded from `v25.0.0+incompatible` to `v28.5.2+incompatible` (latest stable).
2. `go.opentelemetry.io/otel/sdk` and all sibling `go.opentelemetry.io/otel/*` packages must be upgraded from `v1.21.0` to `v1.40.0`.
3. All transitive dependencies brought in by these upgrades must be consistent in `go.sum`.
4. The binary must continue to build and behave correctly after the upgrade.

## User Experience Requirements

- No behaviour change visible to users.
- No change to configuration, labels, or environment variables.

## Technical Requirements

- Use `go get` to upgrade each dependency, then `go mod tidy` to clean up.
- Upgrade docker/docker first (larger change), then otel packages.
- Confirm `go build ./...` passes after each upgrade.

## Acceptance Criteria

- [ ] `go.mod` lists `github.com/docker/docker v28.5.2+incompatible`.
- [ ] `go.mod` lists `go.opentelemetry.io/otel/sdk v1.40.0` (and matching otel siblings).
- [ ] `go build ./...` exits 0 with no errors.
- [ ] All 7 Dependabot alerts are resolved.

## Dependencies

- No dependency on other open requirements.
- Modifies `lazy-tcp-proxy/go.mod` and `lazy-tcp-proxy/go.sum` only (no application code changes expected).

## Implementation Notes

- docker/docker v28.x is still `+incompatible` (no Go module support in the main repo).
- Upgrading docker/docker may pull in newer versions of `golang.org/x/sys`, `github.com/containerd/*`, and other transitive deps; `go mod tidy` will handle this.
- Dependabot PR #5 covers otel/sdk; we will apply both upgrades together in a single branch rather than merging the bot PR separately.
