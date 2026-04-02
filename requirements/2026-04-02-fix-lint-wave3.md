# Fix govet hostport IPv6 Violation

**Date Added**: 2026-04-02
**Priority**: High
**Status**: Completed

## Problem Statement

`server.go:492` constructs a dial address using `fmt.Sprintf("%s:%d", ip, port)`, which produces an invalid address for IPv6 hosts (e.g. `::1:8080` instead of `[::1]:8080`), flagged by `govet` as `hostport` violation.

## Functional Requirements

Use `net.JoinHostPort` to correctly format host:port for both IPv4 and IPv6.

## Technical Requirements

- `server.go:492`: replace `fmt.Sprintf("%s:%d", ip, ts.targetPort)` with `net.JoinHostPort(ip, fmt.Sprintf("%d", ts.targetPort))`
- No new imports required (`net` already imported)

## Acceptance Criteria

- [ ] `golangci-lint run ./...` passes with zero violations
- [ ] `go test -race -count=1 ./...` still passes

## Dependencies

- Follows REQ-033
